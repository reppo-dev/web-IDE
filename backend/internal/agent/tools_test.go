package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

func seedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "auth.go"),
		[]byte("package main\n\nfunc login() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestListFiles(t *testing.T) {
	e := NewToolExecutor(seedProject(t), AccessReadOnly, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{Name: ToolListFiles})
	if !res.OK {
		t.Fatalf("list_files failed: %s", res.Content)
	}
	if !strings.Contains(res.Content, "main.go") || !strings.Contains(res.Content, "auth.go") {
		t.Errorf("expected both files, got:\n%s", res.Content)
	}
}

func TestReadFile(t *testing.T) {
	e := NewToolExecutor(seedProject(t), AccessReadOnly, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{
		Name: ToolReadFile, Args: map[string]interface{}{"path": "auth.go"},
	})
	if !res.OK {
		t.Fatalf("read_file failed: %s", res.Content)
	}
	if !strings.Contains(res.Content, "func login()") {
		t.Errorf("expected file contents, got:\n%s", res.Content)
	}
}

func TestReadFileMissingArg(t *testing.T) {
	e := NewToolExecutor(seedProject(t), AccessReadOnly, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{Name: ToolReadFile})
	if res.OK {
		t.Error("read_file without a path must fail")
	}
	if !strings.Contains(res.Content, "needs a path") {
		t.Errorf("expected a clear message, got %q", res.Content)
	}
}

func TestReadFileRefusesTraversal(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "secret.txt"),
		[]byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(5, 0))
	for _, p := range []string{"../secret.txt", "..\\secret.txt", "C:\\Windows\\System32\\hosts"} {
		res := e.Run(context.Background(), &ToolCall{
			Name: ToolReadFile, Args: map[string]interface{}{"path": p},
		})
		if res.OK {
			t.Errorf("traversal %q was allowed: %s", p, res.Content)
		}
		if strings.Contains(res.Content, "SECRET") {
			t.Errorf("traversal %q leaked file contents", p)
		}
	}
}

// The whole point of access levels: a read-only agent must not gain a write
// tool just because it asked for one by name.
func TestReadOnlyCannotWrite(t *testing.T) {
	root := seedProject(t)
	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(5, 0))

	if e.HasTool(ToolWriteFile) {
		t.Error("a read-only executor must not advertise write_file")
	}

	res := e.Run(context.Background(), &ToolCall{
		Name: ToolWriteFile,
		Args: map[string]interface{}{"path": "evil.go", "content": "package main"},
	})
	if res.OK {
		t.Fatal("a read-only agent must not be able to write")
	}
	if _, err := os.Stat(filepath.Join(root, "evil.go")); err == nil {
		t.Fatal("the file was created despite read-only access")
	}
}

func TestWriteFileAtWriteAccess(t *testing.T) {
	root := seedProject(t)
	e := NewToolExecutor(root, AccessWrite, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{
		Name: ToolWriteFile,
		Args: map[string]interface{}{
			"path":    "lib/helper.go",
			"content": "package lib\n\nfunc Help() {}\n",
		},
	})
	if !res.OK {
		t.Fatalf("write_file failed: %s", res.Content)
	}

	data, err := os.ReadFile(filepath.Join(root, "lib", "helper.go"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !strings.Contains(string(data), "func Help()") {
		t.Errorf("unexpected contents: %s", data)
	}
	if e.Writes() != 1 {
		t.Errorf("expected 1 write recorded, got %d", e.Writes())
	}
}

func TestWriteFileRefusesProse(t *testing.T) {
	root := seedProject(t)
	e := NewToolExecutor(root, AccessWrite, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{
		Name: ToolWriteFile,
		Args: map[string]interface{}{"path": "notes.go", "content": "Just a helpful note"},
	})
	if res.OK {
		t.Error("prose must not be written into a source file")
	}
	if _, err := os.Stat(filepath.Join(root, "notes.go")); err == nil {
		t.Error("a prose file was created")
	}
}

func TestWriteFileRefusesEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}

	e := NewToolExecutor(root, AccessWrite, nil, NewToolBudget(5, 0))
	res := e.Run(context.Background(), &ToolCall{
		Name: ToolWriteFile,
		Args: map[string]interface{}{"path": "../escaped.go", "content": "package main"},
	})
	if res.OK {
		t.Error("writing outside the project must be refused")
	}
	if _, err := os.Stat(filepath.Join(base, "escaped.go")); err == nil {
		t.Fatal("a file was created outside the project")
	}
}

func TestDetectorNeedsFullAccess(t *testing.T) {
	root := seedProject(t)

	write := NewToolExecutor(root, AccessWrite, nil, NewToolBudget(5, 0))
	if write.HasTool(ToolRunDetector) {
		t.Error("write access must not include run_detector")
	}

	full := NewToolExecutor(root, AccessFull, nil, NewToolBudget(5, 0))
	if !full.HasTool(ToolRunDetector) {
		t.Error("full access should include run_detector")
	}
}

func TestDetectorOutputReachesTheModel(t *testing.T) {
	root := seedProject(t)
	e := NewToolExecutor(root, AccessFull, nil, NewToolBudget(5, 0))
	e.SetDetector(func(string) (string, error) {
		return "dart errors: undefined name 'prompt'", nil
	})

	res := e.Run(context.Background(), &ToolCall{Name: ToolRunDetector})
	if !res.OK {
		t.Fatalf("run_detector failed: %s", res.Content)
	}
	if !strings.Contains(res.Content, "undefined name") {
		t.Errorf("expected the real compiler error, got %q", res.Content)
	}
}

func TestSearchCode(t *testing.T) {
	e := NewToolExecutor(seedProject(t), AccessReadOnly, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{
		Name: ToolSearchCode, Args: map[string]interface{}{"query": "func login"},
	})
	if !res.OK {
		t.Fatalf("search_code failed: %s", res.Content)
	}
	if !strings.Contains(res.Content, "auth.go:") {
		t.Errorf("expected a file:line hit, got:\n%s", res.Content)
	}
}

func TestSearchCodeNoMatch(t *testing.T) {
	e := NewToolExecutor(seedProject(t), AccessReadOnly, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{
		Name: ToolSearchCode, Args: map[string]interface{}{"query": "zzz-not-here"},
	})
	if !res.OK {
		t.Error("no matches should still be a successful search")
	}
	if !strings.Contains(res.Content, "no matches") {
		t.Errorf("expected an explicit no-match message, got %q", res.Content)
	}
}

func TestUnknownToolIsRejected(t *testing.T) {
	e := NewToolExecutor(seedProject(t), AccessFull, nil, NewToolBudget(5, 0))

	res := e.Run(context.Background(), &ToolCall{Name: "delete_everything"})
	if res.OK {
		t.Error("an unknown tool must not run")
	}
	if !strings.Contains(res.Content, "not available") {
		t.Errorf("expected a clear rejection, got %q", res.Content)
	}
}

// A runaway model must not be able to loop forever.
func TestToolBudgetStopsRunawayAgent(t *testing.T) {
	e := NewToolExecutor(seedProject(t), AccessReadOnly, nil, NewToolBudget(3, 0))

	for i := 0; i < 3; i++ {
		if res := e.Run(context.Background(), &ToolCall{Name: ToolListFiles}); !res.OK {
			t.Fatalf("call %d should have succeeded: %s", i+1, res.Content)
		}
	}

	res := e.Run(context.Background(), &ToolCall{Name: ToolListFiles})
	if res.OK {
		t.Fatal("the fourth call must be refused")
	}
	if !strings.Contains(res.Content, "Budget exhausted") {
		t.Errorf("expected a budget message, got %q", res.Content)
	}
}

func TestParseToolCallShapes(t *testing.T) {
	wrapped := `{"tool_call":{"name":"read_file","args":{"path":"main.go"}}}`
	call, err := ParseToolCall(wrapped)
	if err != nil {
		t.Fatalf("wrapped form should parse: %v", err)
	}
	if call.Name != ToolReadFile || call.Args["path"] != "main.go" {
		t.Errorf("unexpected parse: %+v", call)
	}

	// Models often wrap the call in prose or a code fence.
	messy := "Here you go:\n```json\n{\"tool_call\":{\"name\":\"list_files\"}}\n```"
	if _, err := ParseToolCall(messy); err != nil {
		t.Errorf("a fenced call should still parse: %v", err)
	}

	// An ordinary answer is not a tool call.
	if _, err := ParseToolCall(`{"project_overview":"a design"}`); err == nil {
		t.Error("a normal answer must not be read as a tool call")
	}
}

func TestToolBudgetRemaining(t *testing.T) {
	b := NewToolBudget(2, 0)
	if b.Remaining() != 2 {
		t.Errorf("expected 2 remaining, got %d", b.Remaining())
	}
	_ = b.Take()
	_ = b.Take()
	if b.Remaining() != 0 {
		t.Errorf("expected 0 remaining, got %d", b.Remaining())
	}
	if err := b.Take(); err == nil {
		t.Error("expected the budget to be spent")
	}
}

func TestToolPromptMentionsAvailableTools(t *testing.T) {
	readOnly := ToolPrompt(AccessReadOnly)
	if strings.Contains(readOnly, ToolWriteFile) {
		t.Error("a read-only prompt must not offer write_file")
	}

	write := ToolPrompt(AccessWrite)
	if !strings.Contains(write, ToolWriteFile) {
		t.Error("write access should offer write_file")
	}
	if strings.Contains(write, ToolRunDetector) {
		t.Error("write access should not offer run_detector")
	}
}

func TestToolSpecJSONIsValid(t *testing.T) {
	for _, tool := range DefaultTools(AccessFull) {
		if _, err := json.Marshal(tool); err != nil {
			t.Errorf("%s does not serialise: %v", tool.Name, err)
		}
		if tool.Description == "" {
			t.Errorf("%s has no description, so a model cannot choose it", tool.Name)
		}
		if tool.Parameters == nil {
			t.Errorf("%s has no parameter schema", tool.Name)
		}
	}
}

// scriptedLLM answers with a canned sequence so the tool loop can be tested
// end to end without a network call.
type scriptedLLM struct {
	responses []string
	calls     int
	prompts   []string
}

func (m *scriptedLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	m.prompts = append(m.prompts, req.SystemPrompt+"\n"+req.UserPrompt)
	if m.calls >= len(m.responses) {
		m.calls++
		return &llm.Response{Content: `{"summary":"done"}`}, nil
	}
	out := m.responses[m.calls]
	m.calls++
	return &llm.Response{Content: out}, nil
}

// The whole point of the loop: the agent reads a real file, then answers based
// on what it actually saw rather than on a guess.
func TestToolLoopReadsThenAnswers(t *testing.T) {
	root := seedProject(t)
	client := &scriptedLLM{responses: []string{
		`{"tool_call":{"name":"read_file","args":{"path":"auth.go"}}}`,
		`{"files":[{"path":"auth.go","language":"go","content":"package main\n\nfunc login() {}"}],"explanation":"fixed"}`,
	}}

	e := NewToolExecutor(root, AccessWrite, nil, NewToolBudget(6, 0))
	loop := NewToolLoop(e, client)

	res := loop.Run(context.Background(), llm.Request{SystemPrompt: "you are a coder", UserPrompt: "fix login"})
	if res.Err != nil {
		t.Fatalf("loop failed: %v", res.Err)
	}
	if len(res.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(res.ToolCalls))
	}
	if res.ToolCalls[0].Name != ToolReadFile {
		t.Errorf("expected read_file, got %q", res.ToolCalls[0].Name)
	}

	// The second model call must carry the file contents back to the model.
	if len(client.prompts) < 2 {
		t.Fatal("expected the model to be called twice")
	}
	if !strings.Contains(client.prompts[1], "func login()") {
		t.Errorf("the tool result was not fed back to the model:\n%s", client.prompts[1])
	}
	if !strings.Contains(client.prompts[1], "TOOL RESULTS") {
		t.Error("the follow-up turn should label the tool results")
	}
}

func TestToolLoopWritesFile(t *testing.T) {
	root := seedProject(t)
	client := &scriptedLLM{responses: []string{
		`{"tool_call":{"name":"write_file","args":{"path":"new.go","content":"package main\n\nfunc New() {}"}}}`,
		`{"summary":"added new.go"}`,
	}}

	e := NewToolExecutor(root, AccessWrite, nil, NewToolBudget(6, 0))
	loop := NewToolLoop(e, client)

	res := loop.Run(context.Background(), llm.Request{SystemPrompt: "sys", UserPrompt: "add new.go"})
	if res.Err != nil {
		t.Fatalf("loop failed: %v", res.Err)
	}
	if e.Writes() != 1 {
		t.Errorf("expected the write to happen, got %d", e.Writes())
	}
	if _, err := os.Stat(filepath.Join(root, "new.go")); err != nil {
		t.Errorf("file not written: %v", err)
	}
}

// A read-only agent must not be able to write even through the loop.
func TestToolLoopRespectsReadOnly(t *testing.T) {
	root := seedProject(t)
	client := &scriptedLLM{responses: []string{
		`{"tool_call":{"name":"write_file","args":{"path":"evil.go","content":"package main"}}}`,
		`{"summary":"gave up"}`,
	}}

	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(6, 0))
	loop := NewToolLoop(e, client)

	if res := loop.Run(context.Background(), llm.Request{SystemPrompt: "sys", UserPrompt: "do something"}); res.Err != nil {
		t.Fatalf("loop failed: %v", res.Err)
	}
	if _, err := os.Stat(filepath.Join(root, "evil.go")); err == nil {
		t.Fatal("a read-only agent wrote a file")
	}
	if len(client.prompts) < 2 {
		t.Error("the model should have been told the tool was refused")
	}
	if !strings.Contains(client.prompts[1], "not available") {
		t.Errorf("expected the refusal in the next turn:\n%s", client.prompts[1])
	}
}

// A model that keeps calling tools must stop rather than run forever.
func TestToolLoopStopsAfterMaxTurns(t *testing.T) {
	root := seedProject(t)
	call := `{"tool_call":{"name":"list_files"}}`
	client := &scriptedLLM{responses: []string{call, call, call, call, call, call, call, call}}

	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(0, 0))
	loop := NewToolLoop(e, client)
	loop.maxTurns = 3

	res := loop.Run(context.Background(), llm.Request{SystemPrompt: "sys", UserPrompt: "loop forever"})
	if res.Err != nil {
		t.Fatalf("loop failed: %v", res.Err)
	}
	if res.Turns != 3 {
		t.Errorf("expected to stop after 3 turns, stopped at %d", res.Turns)
	}
	if !strings.Contains(res.Content, "Stopped after") {
		t.Errorf("expected an explanation for the model, got %q", res.Content)
	}
}

// The loop must report when the budget, not the turn limit, stopped it.
func TestToolLoopReportsBudgetStop(t *testing.T) {
	root := seedProject(t)
	call := `{"tool_call":{"name":"list_files"}}`
	client := &scriptedLLM{responses: []string{call, call, call, call, call}}

	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(2, 0))
	loop := NewToolLoop(e, client)

	res := loop.Run(context.Background(), llm.Request{SystemPrompt: "sys", UserPrompt: "loop"})
	if !strings.Contains(res.Content, "Tool budget spent") {
		t.Errorf("expected a budget message, got %q", res.Content)
	}
}

func TestToolLoopSurfacesModelErrors(t *testing.T) {
	root := seedProject(t)
	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(3, 0))
	loop := NewToolLoop(e, &failingLLM{})

	res := loop.Run(context.Background(), llm.Request{SystemPrompt: "sys", UserPrompt: "task"})
	if res.Err == nil {
		t.Fatal("a failing model must produce an error, not a silent success")
	}
}

type failingLLM struct{}

func (f *failingLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	return nil, fmt.Errorf("upstream is down")
}

// A local provider that stops answering will usually stay silent on the next
// turn too. The agent must give up on its own rather than burning every
// remaining turn, and the rest of the team must be able to continue.
func TestIsProviderTimeout(t *testing.T) {
	yes := []error{
		fmt.Errorf("api request failed: Post \"http://127.0.0.1:20128/v1\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"),
		context.DeadlineExceeded,
		fmt.Errorf("read tcp: i/o timeout"),
		fmt.Errorf("unexpected EOF"),
	}
	for _, err := range yes {
		if !IsProviderTimeout(err) {
			t.Errorf("expected a timeout for: %v", err)
		}
	}

	no := []error{
		nil,
		fmt.Errorf("api error (status 400): bad request"),
		fmt.Errorf("api error (status 401): unauthorized"),
		fmt.Errorf("api error (status 404): model not found"),
		context.Canceled,
		fmt.Errorf("agent \"x\" has no model assigned"),
	}
	for _, err := range no {
		if IsProviderTimeout(err) {
			t.Errorf("did not expect a timeout for: %v", err)
		}
	}
}

// timeoutLLM succeeds on the first turns, then goes silent forever.
type timeoutLLM struct {
	callsBeforeTimeout int
	calls              int
}

func (m *timeoutLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	m.calls++
	if m.calls <= m.callsBeforeTimeout {
		return &llm.Response{Content: `{"tool_call":{"name":"list_files"}}`}, nil
	}
	return nil, fmt.Errorf(
		"api request failed: Post \"http://127.0.0.1:20128/v1\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
}

func TestToolLoopStopsOnTimeout(t *testing.T) {
	root := seedProject(t)
	client := &timeoutLLM{callsBeforeTimeout: 2}
	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(0, 0))
	loop := NewToolLoop(e, client)
	loop.agentName = "MiMo"

	res := loop.Run(context.Background(), llm.Request{UserPrompt: "work"})

	if !res.TimedOut {
		t.Fatal("the loop should report that the provider timed out")
	}
	if res.Err == nil {
		t.Error("a timeout is still an error, even if it is handled")
	}

	// It must stop early instead of using every remaining turn.
	if res.Turns >= maxToolTurns {
		t.Errorf("expected an early stop, used %d of %d turns", res.Turns, maxToolTurns)
	}
	if client.calls != 3 {
		t.Errorf("expected exactly 3 calls (2 working + 1 timeout), got %d", client.calls)
	}
	if !strings.Contains(res.Content, "MiMo") {
		t.Errorf("the message should name the agent that stalled, got %q", res.Content)
	}
}

// A stall on one agent must not abort the round: the message has to survive so
// the orchestrator can keep going with the other agents.
func TestTimedOutAgentStillReports(t *testing.T) {
	root := seedProject(t)
	client := &timeoutLLM{callsBeforeTimeout: 1}
	e := NewToolExecutor(root, AccessReadOnly, nil, NewToolBudget(0, 0))
	loop := NewToolLoop(e, client)

	a := NewCustomAgent(domain.AgentSpec{
		ID: "mimo", Name: "MiMo", Role: domain.RoleCustom,
		Model: "flash_5", Phase: domain.PhaseReview, Enabled: true,
	}, client)

	sCtx := &SwarmContext{
		ProjectID:    "p",
		Round:        1,
		UserPrompt:   "check",
		WorkspacePath: root,
		ToolsAccess:  AccessReadOnly,
		ToolTimeout:  0,
	}

	loop.executor = e
	msg, err := a.executeWithTools(context.Background(), sCtx, loop,
		llm.Request{Model: "flash_5", BaseURL: "http://127.0.0.1:20128/v1",
			UserPrompt: "check"})

	if err != nil {
		t.Fatalf("a timed-out agent must not be a hard error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected a message so the team can continue")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Content, &payload); err != nil {
		t.Fatalf("payload should be JSON: %v (%s)", err, msg.Content)
	}
	if payload["status"] != "timed_out" {
		t.Errorf("expected status timed_out, got %v", payload["status"])
	}
}
