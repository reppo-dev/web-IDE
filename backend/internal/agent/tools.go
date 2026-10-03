package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// AccessLevel decides what an agent may do without asking the user. It exists
// because a weak model given an unrestricted WriteFile can quietly destroy a
// project; the default is deliberately cautious.
type AccessLevel string

const (
	// AccessReadOnly allows reading files and running the detector only.
	AccessReadOnly AccessLevel = "read_only"
	// AccessWrite adds file creation and edits inside the project.
	AccessWrite AccessLevel = "write"
	// AccessFull additionally allows running builds and tests.
	AccessFull AccessLevel = "full"
)

// ToolNames used in the model's function-calling output.
const (
	ToolReadFile    = "read_file"
	ToolWriteFile   = "write_file"
	ToolListFiles   = "list_files"
	ToolSearchCode  = "search_code"
	ToolRunDetector = "run_detector"
)

// ToolSpec describes one callable tool.
type ToolSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	// Parameters is the JSON Schema the model fills in.
	Parameters map[string]interface{} `json:"parameters"`
	// MinimumAccess is the least privilege this tool can run under.
	MinimumAccess AccessLevel `json:"minimum_access"`
}

// ToolCall is a single request from the model to use a tool.
type ToolCall struct {
	ID   string                 `json:"id"`
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

// ToolResult is what the agent sees after a tool ran.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Content string `json:"content"`
	// Truncated tells the model output was cut, so it knows to read in pieces.
	Truncated bool `json:"truncated,omitempty"`
}

// ToolResultFor is the JSON Schema the model must follow to request a tool.
// It is embedded in the prompt so the model learns the exact shape.
const ToolResultFor = `To use a tool, respond with ONLY this JSON and nothing else:
{"tool_call": {"name": "<tool>", "args": { ... }}}

Available tools:
- {"name": "list_files",   "args": {}}
- {"name": "read_file",    "args": {"path": "relative/path.ext"}}
- {"name": "write_file",   "args": {"path": "relative/path.ext", "content": "full file contents"}}
- {"name": "search_code",  "args": {"query": "text to find", "path": "optional/subfolder"}}
- {"name": "run_detector", "args": {}}

The system will run the tool and return its result in the next turn. Then answer normally.`

// DefaultTools returns the tool catalogue for an access level.
func DefaultTools(access AccessLevel) []ToolSpec {
	read := []ToolSpec{
		{
			Name:            ToolListFiles,
			Description:     "List every source file in the project, as project-relative paths.",
			Parameters:      map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			MinimumAccess:   AccessReadOnly,
		},
		{
			Name:        ToolReadFile,
			Description: "Read one source file. The path must be relative to the project root.",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{"path": map[string]interface{}{"type": "string"}},
				"required":   []string{"path"},
			},
			MinimumAccess: AccessReadOnly,
		},
		{
			Name:        ToolSearchCode,
			Description: "Find files whose contents contain a string.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
					"path":  map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
			MinimumAccess: AccessReadOnly,
		},
	}

	if access == AccessReadOnly {
		return read
	}

	withWrite := append(read,
		ToolSpec{
			Name:        ToolWriteFile,
			Description: "Create or replace a file. The path must be relative to the project root and the content must be the complete file.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path", "content"},
			},
			MinimumAccess: AccessWrite,
		},
	)

	if access == AccessFull {
		withWrite = append(withWrite, ToolSpec{
			Name:            ToolRunDetector,
			Description:     "Compile and statically analyse the project, then return the real errors.",
			Parameters:      map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			MinimumAccess:   AccessFull,
		})
	}
	return withWrite
}

// ToolBudget bounds how much an agent may do in one round. Without a ceiling a
// model can loop forever, and a runaway loop on a real project is expensive.
type ToolBudget struct {
	mu           sync.Mutex
	MaxCalls     int
	Calls        int
	MaxReadBytes int
	Deadline     time.Time
}

// NewToolBudget creates a budget. A non-positive maxCalls means unlimited.
func NewToolBudget(maxCalls int, timeout time.Duration) *ToolBudget {
	b := &ToolBudget{MaxCalls: maxCalls, MaxReadBytes: 64 * 1024}
	if timeout > 0 {
		b.Deadline = time.Now().Add(timeout)
	}
	return b
}

// ErrBudgetExhausted is returned when an agent runs out of its allowance.
var ErrBudgetExhausted = fmt.Errorf("tool budget exhausted")

// Take reserves one call, reporting false when the budget is spent.
func (b *ToolBudget) Take() error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.Deadline.IsZero() && time.Now().After(b.Deadline) {
		return fmt.Errorf("tool deadline passed")
	}
	if b.MaxCalls > 0 && b.Calls >= b.MaxCalls {
		return ErrBudgetExhausted
	}
	b.Calls++
	return nil
}

// Remaining reports how many calls are left, for the caller to tell the model.
func (b *ToolBudget) Remaining() int {
	if b == nil || b.MaxCalls <= 0 {
		return -1
	}
	left := b.MaxCalls - b.Calls
	if left < 0 {
		return 0
	}
	return left
}

// ToolExecutor runs the tools an agent asks for, always inside the project
// folder. It is the only place agents touch the filesystem.
type ToolExecutor struct {
	root   string
	access AccessLevel
	ws     *workspace.Workspace
	// Detector runs the project build. Nil means run_detector is unavailable.
	Detector func(root string) (string, error)
	budget   *ToolBudget
	// writes counts successful writes so the orchestrator can report them.
	writes int
}

// NewToolExecutor builds an executor scoped to a project root.
func NewToolExecutor(root string, access AccessLevel, ws *workspace.Workspace, budget *ToolBudget) *ToolExecutor {
	return &ToolExecutor{root: root, access: access, ws: ws, budget: budget}
}

// SetDetector installs the build command used by run_detector.
func (e *ToolExecutor) SetDetector(fn func(root string) (string, error)) {
	e.Detector = fn
}

// Writes reports how many files the agent has changed.
func (e *ToolExecutor) Writes() int { return e.writes }

// HasTool reports whether a tool is allowed at the current access level.
func (e *ToolExecutor) HasTool(name string) bool {
	for _, t := range DefaultTools(e.access) {
		if t.Name == name {
			return true
		}
	}
	return false
}

// ParseToolCall extracts a tool request from a model response. It tolerates the
// two shapes models actually produce: a wrapper key, or the call at the root.
func ParseToolCall(content string) (*ToolCall, error) {
	cleaned := CleanJSON(content)
	if cleaned == "" {
		return nil, fmt.Errorf("empty response")
	}

	var wrapper struct {
		ToolCall *ToolCall `json:"tool_call"`
	}
	if err := json.Unmarshal([]byte(cleaned), &wrapper); err == nil && wrapper.ToolCall != nil {
		if wrapper.ToolCall.Name == "" {
			return nil, fmt.Errorf("tool_call has no name")
		}
		if wrapper.ToolCall.Args == nil {
			wrapper.ToolCall.Args = map[string]interface{}{}
		}
		return wrapper.ToolCall, nil
	}

	var direct ToolCall
	if err := json.Unmarshal([]byte(cleaned), &direct); err == nil && direct.Name != "" {
		if direct.Args == nil {
			direct.Args = map[string]interface{}{}
		}
		return &direct, nil
	}

	return nil, fmt.Errorf("no tool call in response")
}

// Run executes one tool call and always returns a result rather than an error,
// because the model needs to read the failure to correct itself.
func (e *ToolExecutor) Run(ctx context.Context, call *ToolCall) ToolResult {
	res := ToolResult{CallID: call.ID, Name: call.Name}

	if err := e.budget.Take(); err != nil {
		res.Content = "Budget exhausted: " + err.Error()
		return res
	}
	if !e.HasTool(call.Name) {
		res.Content = fmt.Sprintf("Tool %q is not available at access level %q.", call.Name, e.access)
		return res
	}

	switch call.Name {
	case ToolListFiles:
		res.Content, res.Truncated = e.listFiles()
		res.OK = !strings.HasPrefix(res.Content, "error:")
	case ToolReadFile:
		res.Content, res.Truncated = e.readFile(call.Args)
		res.OK = !strings.HasPrefix(res.Content, "error:")
	case ToolSearchCode:
		res.Content, res.Truncated = e.searchCode(call.Args)
		res.OK = !strings.HasPrefix(res.Content, "error:")
	case ToolWriteFile:
		errStr := e.writeFile(call.Args)
		if errStr == "" {
			res.OK = true
			res.Content = "written"
			e.writes++
		} else {
			res.OK = false
			res.Content = errStr
		}
	case ToolRunDetector:
		res.Content = e.runDetector()
		res.OK = !strings.HasPrefix(res.Content, "error:") && !strings.HasPrefix(res.Content, "detector failed:")
	default:
		res.OK = false
		res.Content = "Unknown tool " + call.Name
		return res
	}

	return res
}

func (e *ToolExecutor) listFiles() (string, bool) {
	engine := workspace.NewContextEngine(e.root)
	files, err := engine.ListSourceFiles()
	if err != nil {
		return "error: " + err.Error(), false
	}
	var b strings.Builder
	for _, f := range files {
		b.WriteString(f.RelPath)
		b.WriteByte('\n')
	}
	return b.String(), false
}

func (e *ToolExecutor) readFile(args map[string]interface{}) (string, bool) {
	path, _ := args["path"].(string)
	if path == "" {
		return "error: read_file needs a path", false
	}

	abs, err := workspace.SafeReadPath(e.root, path)
	if err != nil {
		return "error: " + err.Error(), false
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return "error: " + err.Error(), false
	}

	limit := 64 * 1024
	if e.budget != nil && e.budget.MaxReadBytes > 0 {
		limit = e.budget.MaxReadBytes
	}
	truncated := false
	if len(data) > limit {
		data = data[:limit]
		truncated = true
	}
	out := string(data)
	if truncated {
		out += "\n... [truncated; read the file in smaller pieces]"
	}
	return out, truncated
}

func (e *ToolExecutor) searchCode(args map[string]interface{}) (string, bool) {
	query, _ := args["query"].(string)
	if query == "" {
		return "error: search_code needs a query", false
	}
	sub, _ := args["path"].(string)

	engine := workspace.NewContextEngine(e.root)
	files, err := engine.ListSourceFiles()
	if err != nil {
		return "error: " + err.Error(), false
	}

	var b strings.Builder
	hits := 0
	for _, f := range files {
		if sub != "" && !strings.HasPrefix(f.RelPath, strings.TrimPrefix(sub, "/")) {
			continue
		}
		abs, err := workspace.SafeReadPath(e.root, f.RelPath)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, query) {
				continue
			}
			hits++
			if hits > 200 {
				return b.String() + "\n... [more than 200 matches]", true
			}
			fmt.Fprintf(&b, "%s:%d: %s\n", f.RelPath, i+1, strings.TrimSpace(line))
		}
	}
	if hits == 0 {
		return "no matches", false
	}
	return b.String(), false
}

func (e *ToolExecutor) writeFile(args map[string]interface{}) string {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "error: write_file needs a path"
	}
	if strings.TrimSpace(content) == "" {
		return "error: write_file needs non-empty content"
	}

	// Reuse the project validator so a conversational answer cannot become a
	// source file, and refuse anything outside the project.
	if err := workspace.ValidateCodeFile(path, content); err != nil {
		return "error: " + err.Error()
	}

	if e.ws != nil {
		if err := e.ws.SaveFile(path, content); err != nil {
			return "error: " + err.Error()
		}
		return ""
	}

	abs, err := workspace.SafePath(e.root, path)
	if err != nil {
		return "error: " + err.Error()
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return "error: " + err.Error()
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		return "error: " + err.Error()
	}
	return ""
}

func (e *ToolExecutor) runDetector() string {
	if e.Detector == nil {
		return "error: no detector configured for this project"
	}
	out, err := e.Detector(e.root)
	if err != nil {
		return "detector failed: " + err.Error() + "\n" + out
	}
	return out
}

// ToolPrompt renders the tool instructions for the system prompt.
func ToolPrompt(access AccessLevel) string {
	tools := DefaultTools(access)
	var b strings.Builder
	b.WriteString("--- TOOLS ---\nYou can inspect and change this project directly. ")
	switch access {
	case AccessReadOnly:
		b.WriteString("You may READ files but not change them. ")
	case AccessWrite:
		b.WriteString("You may read and write files inside the project. ")
	default:
		b.WriteString("You may read, write, and run the build. ")
	}
	b.WriteString("Always read a file before editing it.\n\n")
	for _, t := range tools {
		props, _ := json.Marshal(t.Parameters["properties"])
		required, _ := json.Marshal(t.Parameters["required"])
		b.WriteString(fmt.Sprintf("- %s: %s\n  args: %s required: %s\n",
			t.Name, t.Description, string(props), string(required)))
	}
	b.WriteString("\n")
	b.WriteString(toolExamplesFor(access))
	return b.String()
}

func toolExamplesFor(access AccessLevel) string {
	var b strings.Builder
	b.WriteString("To use a tool, respond with ONLY this JSON and nothing else:\n")
	b.WriteString("{\"tool_call\": {\"name\": \"<tool>\", \"args\": { ... }}}\n\n")
	b.WriteString("Available tools:\n")
	b.WriteString("- {\"name\": \"list_files\",   \"args\": {}}\n")
	b.WriteString("- {\"name\": \"read_file\",    \"args\": {\"path\": \"relative/path.ext\"}}\n")
	b.WriteString("- {\"name\": \"search_code\",  \"args\": {\"query\": \"text to find\", \"path\": \"optional/subfolder\"}}\n")
	if access == AccessWrite || access == AccessFull {
		b.WriteString("- {\"name\": \"write_file\",   \"args\": {\"path\": \"relative/path.ext\", \"content\": \"full file contents\"}}\n")
	}
	if access == AccessFull {
		b.WriteString("- {\"name\": \"run_detector\", \"args\": {}}\n")
	}
	b.WriteString("\nThe system will run the tool and return its result in the next turn. Then answer normally.")
	return b.String()
}
