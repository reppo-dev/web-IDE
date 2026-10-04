package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// maxToolTurns stops a model from looping tool calls forever. Even with a
// budget, a model can alternate read/write and never converge.
const maxToolTurns = 6

// ToolLoop drives a request/response cycle in which the model may call tools.
//
// This is what makes an agent able to work on a real codebase: it can read a
// file, look at what it says, then edit that file, rather than guessing the
// contents from the prompt alone.
type ToolLoop struct {
	executor *ToolExecutor
	client   llm.Client
	// agentName appears in the message shown when this agent gives up, so the
	// user can tell which one stalled.
	agentName string
	// maxTurns overrides the default when smaller than maxToolTurns.
	maxTurns int
}

// NewToolLoop builds a loop over an executor.
func NewToolLoop(executor *ToolExecutor, client llm.Client) *ToolLoop {
	return &ToolLoop{executor: executor, client: client, maxTurns: maxToolTurns, agentName: "agent"}
}

// IsProviderTimeout reports whether an error is a provider that stopped
// answering. Local models behind a gateway are the usual cause: they queue
// behind each other and the HTTP client gives up waiting for headers.
func IsProviderTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"context deadline exceeded",
		"client.timeout exceeded",
		"timeout awaiting headers",
		"i/o timeout",
		"tls handshake timeout",
		"eof",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// Result is the outcome of a tool-assisted turn.
type Result struct {
	// Content is the model's final answer with no pending tool call.
	Content string
	// Reasoning is the model's chain-of-thought, if it provided one.
	Reasoning string
	// ToolCalls records what the agent did, for the UI and for memory.
	ToolCalls []ToolResult
	// Turns is how many model calls it took to finish.
	Turns int
	// TimedOut means the provider stopped responding. The agent stopped early
	// instead of retrying a local model that is clearly stuck.
	TimedOut bool
	// Err is non-nil when the model never reached an answer.
	Err error
	PromptTokens int
	OutputTokens int
}

// ToolHistory renders past tool results for the next model turn.
func ToolHistory(results []ToolResult) string {
	if len(results) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n--- TOOL RESULTS ---\n")
	for _, r := range results {
		status := "ok"
		if !r.OK {
			status = "error"
		}
		content := r.Content
		if len(content) > 8000 {
			content = content[:8000] + "\n... [truncated]"
		}
		fmt.Fprintf(&b, "%s (%s):\n%s\n", r.Name, status, content)
	}
	return b.String()
}

// Run performs one task, letting the model call tools until it answers.
func (l *ToolLoop) Run(ctx context.Context, baseReq llm.Request) Result {
	out := Result{}

	turns := l.maxTurns
	if turns <= 0 || turns > maxToolTurns {
		turns = maxToolTurns
	}

	history := baseReq.UserPrompt

	for turn := 1; turn <= turns; turn++ {
		out.Turns = turn

		req := baseReq
		req.UserPrompt = history

		resp, err := l.client.Generate(ctx, req)
		if err != nil {
			// A local provider that times out will usually time out again, so
			// stop this agent rather than burning the remaining turns. The
			// work it already did is kept, and the rest of the team carries on.
			if IsProviderTimeout(err) {
				out.Content = fmt.Sprintf(
					"%s stopped after %d turn(s): the model did not respond in time.",
					l.agentName, turn)
				out.TimedOut = true
				out.Err = fmt.Errorf("model call timed out on turn %d: %w", turn, err)
				return out
			}
			out.Err = fmt.Errorf("model call failed on turn %d: %w", turn, err)
			return out
		}
		if resp == nil {
			out.Err = fmt.Errorf("model returned nothing on turn %d", turn)
			return out
		}
		out.PromptTokens += resp.PromptTokens
		out.OutputTokens += resp.OutputTokens
		if resp.Reasoning != "" {
			out.Reasoning = resp.Reasoning
		}

		call, parseErr := ParseToolCall(resp.Content)
		if parseErr != nil {
			// No tool call: this is the answer.
			out.Content = resp.Content
			return out
		}

		result := l.executor.Run(ctx, call)
		out.ToolCalls = append(out.ToolCalls, result)
		if call.ID == "" {
			result.CallID = call.Name
		}

		// Tell the model exactly what happened, then let it continue.
		history = baseReq.UserPrompt + "\n\n" + ToolHistory(out.ToolCalls) +
			"\nContinue. If the tool gave you what you needed, answer now."

		if remaining := l.executor.budget.Remaining(); remaining == 0 {
			out.Content = fmt.Sprintf(
				"Tool budget spent after %d call(s). Last result: %s",
				len(out.ToolCalls), truncate(lastContent(out.ToolCalls), 500))
			return out
		}
	}

	out.Content = fmt.Sprintf(
		"Stopped after %d tool turns without a final answer. Last result: %s",
		out.Turns, truncate(lastContent(out.ToolCalls), 500))
	return out
}

func lastContent(results []ToolResult) string {
	if len(results) == 0 {
		return "none"
	}
	return results[len(results)-1].Content
}

// AttachTools wires an executor into an agent so its Execute path can use it.
func AttachTools(ws *workspace.Workspace, root string, access AccessLevel, budget *ToolBudget) *ToolExecutor {
	return NewToolExecutor(root, access, ws, budget)
}

// ToolResultsToJSON renders a tool trace for the message payload so the UI and
// the shared memory both see what actually happened.
func ToolResultsToJSON(results []ToolResult) string {
	if len(results) == 0 {
		return ""
	}
	b, err := json.Marshal(results)
	if err != nil {
		return ""
	}
	return string(b)
}
