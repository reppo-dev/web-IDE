package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/memory"
)

type customTestMockLLM struct {
	lastReq llm.Request
	resp    *llm.Response
	err     error
}

func (m *customTestMockLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestCustomAgent_BuildPrompt(t *testing.T) {
	mock := &customTestMockLLM{}

	// Test 1: Architect with extra custom instructions
	spec := domain.AgentSpec{
		ID:           "lead-arch",
		Name:         "Lead Architect",
		Role:         domain.RoleArchitect,
		Model:        "flash_5",
		SystemPrompt: "Always use Microservices patterns.",
		Enabled:      true,
	}

	agent := NewCustomAgent(spec, mock)
	prompt := agent.buildPrompt()

	if !strings.Contains(prompt, "System Architect in NexusAI") {
		t.Errorf("expected built-in architect prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "Always use Microservices patterns.") {
		t.Errorf("expected custom system prompt addition, got: %s", prompt)
	}
	if !strings.Contains(prompt, "project_overview") {
		t.Errorf("expected json contract for architect, got: %s", prompt)
	}

	// Test 2: Completely custom specialist role
	customSpec := domain.AgentSpec{
		ID:           "db-opt",
		Name:         "Database Optimizer",
		Role:         "custom",
		Model:        "mimo",
		SystemPrompt: "Inspect PostgreSQL index usage and queries.",
		Enabled:      true,
	}

	customAgent := NewCustomAgent(customSpec, mock)
	customPrompt := customAgent.buildPrompt()

	if strings.Contains(customPrompt, "System Architect") {
		t.Errorf("did not expect architect prompt in custom role")
	}
	if !strings.Contains(customPrompt, "Inspect PostgreSQL index usage and queries.") {
		t.Errorf("expected custom prompt, got: %s", customPrompt)
	}
	if !strings.Contains(customPrompt, "files") || !strings.Contains(customPrompt, "relative/path.ext") {
		t.Errorf("expected default implement files json contract, got: %s", customPrompt)
	}
}

func TestCustomAgent_Execute(t *testing.T) {
	mock := &customTestMockLLM{
		resp: &llm.Response{
			Content:   `{"summary": "Audit complete", "issues": []}`,
			Reasoning: "Examined query planner traces and found no seq scans.",
		},
	}

	spec := domain.AgentSpec{
		ID:          "sec-auditor",
		Name:        "Security Auditor",
		Role:        domain.RoleSecurity,
		Model:       "flash_5",
		Temperature: 0.2,
		Enabled:     true,
	}

	agent := NewCustomAgent(spec, mock)

	sCtx := &SwarmContext{
		ProjectID:  "proj-test",
		UserPrompt: "Check authentication endpoint security",
		AgentModels: map[domain.AgentRole]string{
			domain.RoleSecurity: "flash_5",
		},
		APIKeys: map[string]string{
			"ninerouter": "test-key",
		},
	}

	msg, err := agent.Execute(context.Background(), sCtx)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got: %v", err)
	}

	if msg.From != domain.RoleSecurity {
		t.Errorf("expected From=RoleSecurity, got: %s", msg.From)
	}
	if msg.Type != domain.MsgAgentReport {
		t.Errorf("expected Type=MsgAgentReport, got: %s", msg.Type)
	}
	if msg.Reasoning != "Examined query planner traces and found no seq scans." {
		t.Errorf("unexpected reasoning: %s", msg.Reasoning)
	}
	if mock.lastReq.Temperature != 0.2 {
		t.Errorf("expected temperature 0.2, got: %f", mock.lastReq.Temperature)
	}
}

func TestCustomAgent_BuildWork(t *testing.T) {
	mock := &customTestMockLLM{}
	spec := domain.AgentSpec{
		ID:      "test-coder",
		Name:    "Coder",
		Role:    domain.RoleCoder,
		Model:   "mimo",
		Enabled: true,
	}

	agent := NewCustomAgent(spec, mock)

	sCtx := &SwarmContext{
		ProjectID:  "proj-xyz",
		UserPrompt: "Create login handler",
		Blueprint: &domain.BlueprintPayload{
			ProjectOverview: "Auth system",
			FileTree:        []string{"auth.go"},
		},
		CriticReview: &domain.CriticPayload{
			GeneralReview: "Looks mostly solid",
		},
	}

	work := agent.buildWork(context.Background(), sCtx)

	if !strings.Contains(work, "ARCHITECT BLUEPRINT") {
		t.Errorf("expected blueprint in work context, got: %s", work)
	}
	if !strings.Contains(work, "Auth system") {
		t.Errorf("expected auth system in blueprint, got: %s", work)
	}
	if !strings.Contains(work, "CRITIC FINDINGS") {
		t.Errorf("expected critic findings in work context, got: %s", work)
	}
}

// A custom agent must run from its own spec alone, with no shared AgentModels
// entry. That is what allows teams larger than the five built-in roles.
func TestCustomAgent_RunsFromSpecAlone(t *testing.T) {
	mock := &customTestMockLLM{resp: &llm.Response{Content: `{"summary":"ok"}`}}

	agent := NewCustomAgent(domain.AgentSpec{
		ID:      "perf",
		Name:    "Performance Reviewer",
		Role:    domain.RoleCustom,
		Model:   "openrouter/free",
		Enabled: true,
	}, mock)

	sCtx := &SwarmContext{
		ProjectID:  "team-test",
		UserPrompt: "add rate limiting",
		APIKeys:    map[string]string{"openrouter": "test-key"},
	}

	if _, err := agent.Execute(context.Background(), sCtx); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if mock.lastReq.Model != "openrouter/free" {
		t.Errorf("expected the spec model to be used, got %q", mock.lastReq.Model)
	}
	if mock.lastReq.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("expected OpenRouter routing, got %q", mock.lastReq.BaseURL)
	}
}

func TestCustomAgent_RefusesWithoutModel(t *testing.T) {
	mock := &customTestMockLLM{resp: &llm.Response{Content: `{}`}}

	agent := NewCustomAgent(domain.AgentSpec{ID: "x", Role: domain.RoleCustom}, mock)

	sCtx := &SwarmContext{
		ProjectID:  "team-test",
		UserPrompt: "anything",
		APIKeys:    map[string]string{"openrouter": "test-key"},
	}

	if _, err := agent.Execute(context.Background(), sCtx); err == nil {
		t.Fatal("expected an error when the agent has no model")
	}
}

// Ten independent agents must each route to their own model. This is the core
// promise of arbitrary-sized teams.
func TestCustomAgentsAreIndependent(t *testing.T) {
	models := []string{
		"openrouter/free",
		"deepseek/deepseek-r1",
		"google/gemini-2.5-flash",
		"openrouter/free",
		"meta-llama/llama-3.3-70b-instruct",
		"openrouter/free",
		"qwen/qwen3.8-27b:free",
		"openrouter/free",
		"nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free",
		"openrouter/free",
	}

	for i, model := range models {
		t.Run(model, func(t *testing.T) {
			mock := &customTestMockLLM{resp: &llm.Response{Content: `{"summary":"ok"}`}}
			agent := NewCustomAgent(domain.AgentSpec{
				ID:      "agent-" + strings.Repeat("x", i+1),
				Name:    "Agent",
				Role:    domain.RoleCustom,
				Model:   model,
				Enabled: true,
			}, mock)

			sCtx := &SwarmContext{
				ProjectID:  "big-team",
				UserPrompt: "do a thing",
				APIKeys:    map[string]string{"openrouter": "k", "ninerouter": "k"},
			}

			if _, err := agent.Execute(context.Background(), sCtx); err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if mock.lastReq.Model != model {
				t.Errorf("expected model %q, got %q", model, mock.lastReq.Model)
			}
		})
	}
}

// Agents must see the real project, not just the prompt, or they will invent a
// codebase that does not match the repository.
func TestCustomAgent_ReadsRealProjectCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.go"),
		[]byte("package auth\n\nfunc Login() {}\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	mock := &customTestMockLLM{resp: &llm.Response{Content: `{}`}}
	agent := NewCustomAgent(domain.AgentSpec{
		ID: "coder2", Role: domain.RoleCoder, Model: "openrouter/free", Enabled: true,
	}, mock)

	sCtx := &SwarmContext{
		ProjectID:     "team-test",
		UserPrompt:    "improve login",
		WorkspacePath: dir,
		APIKeys:       map[string]string{"openrouter": "test-key"},
	}

	if _, err := agent.Execute(context.Background(), sCtx); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(mock.lastReq.UserPrompt, "func Login()") {
		t.Errorf("expected real project code in the prompt, got:\n%s", mock.lastReq.UserPrompt)
	}
}

// Every agent in a team must contribute to the shared project brain, otherwise
// a user-built team learns nothing between runs.
func TestCustomAgentsWriteToSharedBrain(t *testing.T) {
	dir := t.TempDir()
	memStore, err := memory.NewSQLiteMemoryStore(dir)
	if err != nil {
		t.Fatalf("memory store: %v", err)
	}
	defer memStore.Close()

	cases := []struct {
		name     string
		spec     domain.AgentSpec
		response string
		contains string
	}{
		{
			name:     "architect records the design",
			spec:     domain.AgentSpec{ID: "a", Name: "Arch", Role: domain.RoleArchitect, Model: "openrouter/free", Enabled: true},
			response: `{"project_overview":"An API gateway with caching","file_tree":["main.go"]}`,
			contains: "API gateway",
		},
		{
			name:     "coder records what it changed",
			spec:     domain.AgentSpec{ID: "c", Name: "Coder", Role: domain.RoleCoder, Model: "openrouter/free", Enabled: true},
			response: `{"files":[{"path":"cache.go","language":"go","content":"package a"}],"explanation":"ok"}`,
			contains: "cache.go",
		},
		{
			name:     "critic records a critical flaw",
			spec:     domain.AgentSpec{ID: "r", Name: "Critic", Role: domain.RoleCritic, Model: "openrouter/free", Enabled: true},
			response: `{"general_review":"Problems found","issues":[{"severity":"critical","file":"db.go","description":"SQL injection"}],"refactor_plan":"fix"}`,
			contains: "SQL injection",
		},
		{
			name:     "security records a vulnerability",
			spec:     domain.AgentSpec{ID: "s", Name: "Sec", Role: domain.RoleSecurity, Model: "openrouter/free", Enabled: true},
			response: `{"attack_surface":"public API","vulnerabilities":[{"severity":"major","file":"auth.go","description":"plaintext passwords"}],"is_clean":false}`,
			contains: "plaintext passwords",
		},
		{
			name:     "arbiter records the score",
			spec:     domain.AgentSpec{ID: "j", Name: "Judge", Role: domain.RoleArbiter, Model: "openrouter/free", Enabled: true},
			response: `{"round":1,"final_score":91.5,"threshold":85.0,"passed":true,"dimensions":[],"feedback_to_coder":"none","summary":"Accepted after review"}`,
			contains: "Accepted after review",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &customTestMockLLM{resp: &llm.Response{Content: tc.response}}
			agent := NewCustomAgent(tc.spec, mock)

			sCtx := &SwarmContext{
				ProjectID:  "shared-brain",
				Round:      1,
				UserPrompt: "build it",
				MemStore:   memStore,
				AgentModels: map[domain.AgentRole]string{tc.spec.Role: tc.spec.Model},
				APIKeys:    map[string]string{"openrouter": "k"},
			}

			if _, err := agent.Execute(context.Background(), sCtx); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			entries, err := memStore.QueryProject(context.Background(), "shared-brain",
				memory.QueryFilter{Limit: 30})
			if err != nil {
				t.Fatalf("QueryProject: %v", err)
			}

			found := false
			for _, e := range entries {
				if strings.Contains(e.Content, tc.contains) {
					found = true
					break
				}
			}
			if !found {
				var got []string
				for _, e := range entries {
					got = append(got, string(e.Type)+": "+e.Content)
				}
				t.Errorf("expected the brain to contain %q, got:\n%s",
					tc.contains, strings.Join(got, "\n"))
			}
		})
	}
}

// A memory written by one agent must be visible to the next one.
func TestSharedBrainCarriesAcrossAgents(t *testing.T) {
	memStore, err := memory.NewSQLiteMemoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("memory store: %v", err)
	}
	defer memStore.Close()

	brain := NewProjectBrain(memStore)
	if err := brain.RecordDecision(context.Background(), "shared", domain.RoleArchitect,
		"Use PostgreSQL for persistence", 1); err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	mock := &customTestMockLLM{resp: &llm.Response{Content: `{"summary":"ok"}`}}
	coder := NewCustomAgent(domain.AgentSpec{
		ID: "c", Name: "Coder", Role: domain.RoleCoder,
		Model: "openrouter/free", Enabled: true,
	}, mock)

	sCtx := &SwarmContext{
		ProjectID:  "shared",
		Round:      2,
		UserPrompt: "implement it",
		MemStore:   memStore,
		APIKeys:    map[string]string{"openrouter": "k"},
	}

	if _, err := coder.Execute(context.Background(), sCtx); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(mock.lastReq.UserPrompt, "PostgreSQL") {
		t.Errorf("a later agent must see an earlier decision, got:\n%s", mock.lastReq.UserPrompt)
	}
}

// The developers team is made entirely of custom agents. Every one of them was
// told to return only {"summary": ...}, so no file was ever produced and the
// project stayed empty. A custom agent in an implement phase must be asked for
// real files.
func TestCustomAgentInImplementPhaseAsksForFiles(t *testing.T) {
	spec := domain.AgentSpec{
		ID:      "dev-1",
		Name:    "Dev One",
		Role:    domain.RoleCustom,
		Model:   "flash_5",
		Phase:   domain.PhaseImplement,
		Enabled: true,
	}
	a := NewCustomAgent(spec, &customTestMockLLM{})

	prompt := a.buildPrompt()
	if !strings.Contains(prompt, `"files"`) {
		t.Errorf("an implement-phase custom agent must be asked for files, got:\n%s", prompt)
	}
	if strings.Contains(prompt, `{"summary":""}`) {
		t.Errorf("implement phase must not request a summary-only response:\n%s", prompt)
	}
}

// Design and review phases get different shapes, so an architect is not asked
// to write files and a reviewer is not asked to design.
func TestCustomAgentPhaseSpecificContracts(t *testing.T) {
	cases := []struct {
		phase    domain.AgentPhase
		contains string
	}{
		{domain.PhaseDesign, "project_overview"},
		{domain.PhaseReview, "general_review"},
		{domain.PhaseJudge, "final_score"},
		{domain.PhaseImplement, `"files"`},
	}

	for _, tc := range cases {
		t.Run(string(tc.phase), func(t *testing.T) {
			a := NewCustomAgent(domain.AgentSpec{
				ID: "x", Name: "X", Role: domain.RoleCustom,
				Model: "flash_5", Phase: tc.phase, Enabled: true,
			}, &customTestMockLLM{})

			prompt := a.buildPrompt()
			if !strings.Contains(prompt, tc.contains) {
				t.Errorf("phase %s should ask for %q, got:\n%s", tc.phase, tc.contains, prompt)
			}
		})
	}
}

// A team of custom agents must actually populate CodeFiles, otherwise the run
// ends with nothing written to disk.
func TestCustomAgentsProduceCodeFiles(t *testing.T) {
	mock := &customTestMockLLM{resp: &llm.Response{
		Content: `{"files":[{"path":"lib/main.dart","language":"dart","content":"void main() {}"}],"explanation":"entry point"}`,
	}}
	a := NewCustomAgent(domain.AgentSpec{
		ID: "dev", Name: "Dev", Role: domain.RoleCustom,
		Model: "openrouter/free", Phase: domain.PhaseImplement, Enabled: true,
	}, mock)

	sCtx := &SwarmContext{
		ProjectID:  "dart-app",
		Round:      1,
		UserPrompt: "make a flutter app",
		APIKeys:    map[string]string{"openrouter": "k"},
	}

	if _, err := a.Execute(context.Background(), sCtx); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	files := sCtx.CodeFilesSnapshot()
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "lib/main.dart" {
		t.Errorf("unexpected path %q", files[0].Path)
	}
}

// Agents in a phase run concurrently, so merging must be safe and a later
// agent must be able to revise an earlier agent's file.
func TestMergeCodeFilesIsSafeAndLastWriteWins(t *testing.T) {
	sCtx := &SwarmContext{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			sCtx.MergeCodeFiles([]domain.CodeFile{
				{Path: "shared.go", Content: fmt.Sprintf("v%d", i)},
				{Path: fmt.Sprintf("f%d.go", i), Content: "x"},
			})
		}
	}()
	for i := 0; i < 50; i++ {
		sCtx.MergeCodeFiles([]domain.CodeFile{{Path: "shared.go", Content: "other"}})
	}
	<-done

	files := sCtx.CodeFilesSnapshot()
	if len(files) == 0 {
		t.Fatal("expected files after concurrent merges")
	}

	// No duplicate paths.
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f.Path] {
			t.Fatalf("duplicate path after concurrent merge: %q", f.Path)
		}
		seen[f.Path] = true
	}
}
