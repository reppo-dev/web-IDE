package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// These tests exist because the original defect was invisible to the suite that
// existed. Every test in this project passed while the swarm declared a
// one-file project finished after a single round. The behaviour is now pinned
// down directly: a project finishes only when its plan says so.

// scriptedLLM lets a test decide what each role returns, and records what it was
// asked. It exists so a test can model a project that needs several rounds
// rather than one that is finished by the first answer.
type scriptedLLM struct {
	mu sync.Mutex
	// tasks is the work list the planner hands back.
	tasks []domain.ProjectTask
	// verifierGapsBefore makes the verifier reject the first N attempts, so a
	// test can prove that a failed task is retried rather than abandoned.
	verifierGapsBefore int
	verifierCalls      int
	// coderCalls counts how many times the coder was asked to write code.
	coderCalls int
	// tasksSeen records the active task title the coder was given each round,
	// which is how a test proves the team moved on instead of repeating itself.
	tasksSeen []string
	// staticCode makes the coder return the same code files every round,
	// allowing stall detection to trigger on unresolvable issues.
	staticCode bool
}

func (s *scriptedLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case strings.HasPrefix(req.SystemPrompt, "You are the request triage layer"):
		return &llm.Response{
			Content: `{"kind":"build","reply":"","reason":"build request","reworded":"Build the requested project"}`,
		}, nil

	case strings.Contains(req.SystemPrompt, "Project Planner"):
		raw, _ := json.Marshal(map[string]interface{}{
			"goal":  "Ship the whole project",
			"tasks": s.tasks,
		})
		return &llm.Response{Content: string(raw)}, nil

	case strings.HasPrefix(req.SystemPrompt, "You are the System Architect"):
		return &llm.Response{
			Content: `{"project_overview":"Test","file_tree":[],"key_components":[],"api_contracts":[]}`,
		}, nil

	case strings.Contains(req.SystemPrompt, "strict verifier"):
		s.verifierCalls++
		if s.verifierCalls <= s.verifierGapsBefore {
			return &llm.Response{
				Content: `{"gaps":["the required behaviour is not implemented yet"],"summary":"Not done yet."}`,
			}, nil
		}
		return &llm.Response{
			Content: `{"gaps":[],"summary":"Verified against the real code."}`,
		}, nil

	case strings.Contains(req.SystemPrompt, "Lead Engineer"), strings.Contains(req.SystemPrompt, "relay discussion"):
		// The coder prompt carries the active task, so recording it shows which
		// task the round was working on.
		s.coderCalls++
		if t := activeTaskTitle(req.UserPrompt); t != "" {
			s.tasksSeen = append(s.tasksSeen, t)
		}
		fileIdx := s.coderCalls
		if s.staticCode {
			fileIdx = 1
		}
		return &llm.Response{
			Content: fmt.Sprintf(
				`{"files":[{"path":"file%d.go","language":"go","content":"package main\n\n// build %d\n"}],"explanation":"work"}`,
				fileIdx, fileIdx),
		}, nil

	case strings.Contains(req.SystemPrompt, "relay discussion"):
		return &llm.Response{Content: `{"thought":"t","reply":"r"}`}, nil
	}

	// Critics and the arbiter see a clean round unless told otherwise.
	return &llm.Response{
		Content: `{"issues":[],"vulnerabilities":[],"general_review":"looks good","is_clean":true}`,
	}, nil
}

// activeTaskTitle pulls the task marker the loop writes into the coder prompt.
func activeTaskTitle(prompt string) string {
	const marker = "TASK TITLE:"
	idx := strings.Index(prompt, marker)
	if idx < 0 {
		const fallback = "TASK:"
		idx = strings.Index(prompt, fallback)
		if idx < 0 {
			return ""
		}
		rest := prompt[idx+len(fallback):]
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			rest = rest[:nl]
		}
		return strings.TrimSpace(rest)
	}
	rest := prompt[idx+len(marker):]
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		rest = rest[:nl]
	}
	return strings.TrimSpace(rest)
}

func newScriptedOrchestrator(t *testing.T, script *scriptedLLM) (*Orchestrator, string) {
	t.Helper()

	arch := agent.NewArchitectAgent(script)
	coder := agent.NewCoderAgent(script)
	critic := agent.NewCriticAgent(script)
	sec := agent.NewSecurityAgent(script)
	arb := agent.NewArbiterAgent(script)
	matrix := agent.NewMatrixEngine(script)

	tempDir := t.TempDir()
	orch := NewOrchestrator(
		Config{MaxRounds: 3, Mode: domain.ModeRoleBased},
		arch, coder, critic, sec, arb, nil, matrix, tempDir,
	)
	// The verifier needs a client to ask about acceptance. Without this it would
	// silently fall back to "files exist", and the tests below would pass without
	// proving anything about the acceptance gate.
	orch.SetLLMClient(script)
	return orch, tempDir
}

func testPlan(tasks ...domain.ProjectTask) []domain.ProjectTask {
	for i := range tasks {
		tasks[i].ID = fmt.Sprintf("T-%03d", i+1)
		tasks[i].State = domain.TaskTodo
		if tasks[i].Title == "" {
			tasks[i].Title = fmt.Sprintf("Task %d", i+1)
		}
		if tasks[i].Acceptance == "" {
			tasks[i].Acceptance = fmt.Sprintf("task %d is implemented", i+1)
		}
	}
	return tasks
}

// TestLoopFinishesEveryTaskInPlan is the central regression test for the original
// bug: a project with several tasks must take several rounds, and must not stop
// at the first clean one.
func TestLoopFinishesEveryTaskInPlan(t *testing.T) {
	script := &scriptedLLM{
		tasks: testPlan(
			domain.ProjectTask{Title: "Build the parser"},
			domain.ProjectTask{Title: "Write the tests"},
			domain.ProjectTask{Title: "Write the README"},
		),
	}
	orch, _ := newScriptedOrchestrator(t, script)

	sCtx, err := orch.RunWithOptions(context.Background(), "multi-task", "Build it", domain.ModeRoleBased, testModels(), testAPIKeys())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if sCtx == nil {
		t.Fatal("no context returned")
	}

	plan := sCtx.PlanSnapshot()
	if plan == nil {
		t.Fatal("no plan was built, so nothing can be claimed as finished")
	}
	if !plan.AllDone() {
		t.Fatalf("plan is not complete: %d/%d done", countDone(plan), len(plan.Tasks))
	}

	// Each task must have been worked on. Repeating one task three times would
	// also finish the plan eventually under some loops, so check the sequence.
	seen := map[string]bool{}
	for _, title := range script.tasksSeen {
		seen[title] = true
	}
	for _, task := range plan.Tasks {
		if !seen[task.Title] {
			t.Errorf("task %q was never worked on; the coder saw %v", task.Title, script.tasksSeen)
		}
	}
}

// TestIncompletePlanNeverReportsDone proves a clean round on an unfinished
// project cannot end the run. This is the exact 87%-and-stop failure.
func TestIncompletePlanNeverReportsDone(t *testing.T) {
	script := &scriptedLLM{
		tasks: testPlan(
			domain.ProjectTask{Title: "First"},
			domain.ProjectTask{Title: "Second"},
		),
		// The first two verification attempts fail, so the plan cannot be
		// complete after a single round no matter how clean the code is.
		verifierGapsBefore: 2,
	}
	orch, _ := newScriptedOrchestrator(t, script)

	sCtx, err := orch.RunWithOptions(context.Background(), "incomplete", "Build it", domain.ModeRoleBased, testModels(), testAPIKeys())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if script.verifierCalls < 3 {
		t.Errorf("expected the failed task to be retried, verifier was called %d times", script.verifierCalls)
	}
	plan := sCtx.PlanSnapshot()
	if plan != nil && plan.AllDone() && script.verifierCalls <= 2 {
		t.Errorf("plan reported complete after only %d verifier calls", script.verifierCalls)
	}
}

// TestTaskGoesToReworkThenCloses proves a rejected task is retried and can close
// once its acceptance is met, rather than being abandoned or marked done anyway.
func TestTaskGoesToReworkThenCloses(t *testing.T) {
	script := &scriptedLLM{
		tasks:            testPlan(domain.ProjectTask{Title: "Only task"}),
		verifierGapsBefore: 2,
	}
	orch, _ := newScriptedOrchestrator(t, script)

	sCtx, err := orch.RunWithOptions(context.Background(), "rework", "Build it", domain.ModeRoleBased, testModels(), testAPIKeys())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if script.verifierCalls < 3 {
		t.Errorf("expected at least 3 verification attempts, got %d", script.verifierCalls)
	}
	plan := sCtx.PlanSnapshot()
	if plan == nil || !plan.AllDone() {
		t.Errorf("task should have closed after passing verification, plan=%v", plan)
	}
}

// TestPlanPersistedToDisk proves the work list survives the run, so a resumed
// session continues instead of restarting the project from nothing.
func TestPlanPersistedToDisk(t *testing.T) {
	script := &scriptedLLM{
		tasks: testPlan(
			domain.ProjectTask{Title: "One"},
			domain.ProjectTask{Title: "Two"},
		),
	}
	orch, tempDir := newScriptedOrchestrator(t, script)

	if _, err := orch.RunWithOptions(context.Background(), "persist", "Build it", domain.ModeRoleBased, testModels(), testAPIKeys()); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	ws, err := workspace.NewWorkspace(tempDir, "persist")
	if err != nil {
		t.Fatalf("open workspace: %v", err)
	}
	plan, err := workspace.LoadPlan(ws.Path())
	if err != nil {
		t.Fatalf("load plan from disk: %v", err)
	}
	if plan == nil || len(plan.Tasks) != 2 {
		t.Fatalf("expected 2 tasks on disk, got %v", plan)
	}

	// tasks.md is what a human reads, so it must agree with plan.json.
	board, err := os.ReadFile(ws.Path() + "/.project/tasks.md")
	if err != nil {
		t.Fatalf("tasks.md not written: %v", err)
	}
	if !strings.Contains(string(board), "One") || !strings.Contains(string(board), "Two") {
		t.Errorf("task board does not list the tasks:\n%s", board)
	}
}

// TestRunTeamBuildsAPlan proves custom teams get the same treatment as the
// built-in swarm. The team path used to run a fixed number of rounds and accept
// a passing score as completion, which is the defect this whole change is about.
func TestRunTeamBuildsAPlan(t *testing.T) {
	script := &scriptedLLM{
		tasks: testPlan(
			domain.ProjectTask{Title: "Team task one"},
			domain.ProjectTask{Title: "Team task two"},
		),
	}

	arch := agent.NewArchitectAgent(script)
	coder := agent.NewCoderAgent(script)
	critic := agent.NewCriticAgent(script)
	sec := agent.NewSecurityAgent(script)
	arb := agent.NewArbiterAgent(script)
	matrix := agent.NewMatrixEngine(script)
	tempDir := t.TempDir()

	orch := NewOrchestrator(
		Config{MaxRounds: 3, Mode: domain.ModeRoleBased},
		arch, coder, critic, sec, arb, nil, matrix, tempDir,
	)
	orch.SetLLMClient(script)

	team := &domain.Team{
		ID:   "t1",
		Name: "Test Team",
		Agents: []domain.AgentSpec{
			{
				ID: "designer", Name: "Designer", Role: domain.RoleArchitect,
				Model: "openrouter/free", Phase: domain.PhaseDesign, Enabled: true,
			},
			{
				ID: "builder", Name: "Builder", Role: domain.RoleCoder,
				Model: "openrouter/free", Phase: domain.PhaseImplement, Enabled: true,
			},
			{
				ID: "judge", Name: "Judge", Role: domain.RoleArbiter,
				Model: "openrouter/free", Phase: domain.PhaseJudge, Enabled: true,
			},
		},
	}

	sCtx, err := orch.RunTeam(context.Background(), "team-proj", "Build the thing", TeamRun{
		Team: team, APIKeys: testAPIKeys(),
	})
	if err != nil {
		t.Fatalf("team run failed: %v", err)
	}

	plan := sCtx.PlanSnapshot()
	if plan == nil {
		t.Fatal("custom team produced no plan, so its completion cannot be trusted")
	}
	if len(plan.Tasks) != 2 {
		t.Errorf("expected 2 tasks in the team plan, got %d", len(plan.Tasks))
	}
}

// TestVerificationFailureBlocksTaskCompletion proves the gate is wired to the
// plan: when acceptance fails, the task must not be marked done even though the
// code itself is clean and scores well.
func TestVerificationFailureBlocksTaskCompletion(t *testing.T) {
	script := &scriptedLLM{
		tasks:              testPlan(domain.ProjectTask{Title: "Never satisfied"}),
		verifierGapsBefore: 1000, // never passes
		staticCode:         true,
	}
	orch, _ := newScriptedOrchestrator(t, script)

	sCtx, err := orch.RunWithOptions(context.Background(), "never-done", "Build it", domain.ModeRoleBased, testModels(), testAPIKeys())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	plan := sCtx.PlanSnapshot()
	if plan == nil {
		t.Fatal("no plan")
	}
	if plan.Completed() {
		t.Error("plan reported complete even though the task never passed verification")
	}

	// A stalled task must be recorded as blocked so the user can see what
	// stopped it rather than finding a silently unfinished project.
	blocked := false
	for _, task := range plan.Tasks {
		if task.State == domain.TaskBlocked || task.State == domain.TaskRework || task.State == domain.TaskWaiting {
			blocked = true
		}
	}
	if !blocked {
		t.Errorf("expected the stuck task to be marked BLOCKED, REWORK or WAITING_FOR_HUMAN, got states %v", taskStates(plan))
	}
}

func taskStates(plan *domain.ProjectPlan) []domain.TaskState {
	var out []domain.TaskState
	for _, t := range plan.Tasks {
		out = append(out, t.State)
	}
	return out
}