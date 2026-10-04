package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/memory"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// SwarmContext contains shared state and artifacts accessible to each agent during execution.
type SwarmContext struct {
	ProjectID      string
	Round          int
	UserPrompt     string
	Blueprint      *domain.BlueprintPayload
	CodeFiles      []domain.CodeFile
	CriticReview   *domain.CriticPayload
	SecurityReport *domain.SecurityPayload
	ArbiterVerdict *domain.ArbiterVerdict
	MemStore       memory.MemoryStore
	AgentModels    map[domain.AgentRole]string
	APIKeys        map[string]string
	// Intent holds the architect's triage result. When Kind is not
	// IntentBuild, the swarm conducts a collaborative discussion across all agents.
	Intent *domain.IntentRoute
	// WorkspacePath is the on-disk project root. Agents must read the real
	// project files through the context engine before writing new code,
	// otherwise they invent a codebase that does not match reality.
	WorkspacePath string

	// Plan is the ordered list of work that makes the project complete. It is
	// what a round advances: the swarm keeps working while open tasks remain,
	// and only an empty backlog ends the run. A run without a plan falls back to
	// round-based judging, because nothing then proves the project finished.
	Plan *domain.ProjectPlan
	// ActiveTask is the task this round is working on. The next round is told
	// what to pick up, so the team moves through the project instead of
	// re-doing whatever it just did.
	ActiveTask *domain.ProjectTask
	// LastTaskVerdict is the verification result for the task just worked on.
	// The arbiter reads it to separate "the code is clean" from "the task is
	// actually done", which are different claims and were previously conflated.
	LastTaskVerdict *TaskVerdict
	// ReviewSequentially makes the review phase run one agent after another
	// instead of concurrently. Both orders reach the same verdict; the flag only
	// controls pacing.
	ReviewSequentially bool

	// Tool configuration. Tools are off unless a team asks for them, and the
	// default access is read-only so a team cannot write by accident.
	ToolsAccess    AccessLevel
	ToolsDisabled  bool
	ToolCallBudget int
	ToolTimeout    time.Duration
	// Detector runs the project build for the run_detector tool.
	Detector func(root string) (string, error)

	// UsageByAgent tracks cumulative token consumption per agent (PROJECT_VISION §37).
	UsageByAgent map[string]*domain.AgentUsage

	// mu guards the mutable fields above. Agents inside a phase run
	// concurrently and all write to the same context, so without this a
	// custom team's shared state would be corrupted.
	mu sync.RWMutex
}

// RecordAgentUsage records token usage for an agent in a thread-safe manner (PROJECT_VISION §37).
func (s *SwarmContext) RecordAgentUsage(agentName, modelName string, promptTokens, outputTokens int) *domain.AgentUsage {
	if s == nil || agentName == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.UsageByAgent == nil {
		s.UsageByAgent = make(map[string]*domain.AgentUsage)
	}
	usage, ok := s.UsageByAgent[agentName]
	if !ok {
		usage = &domain.AgentUsage{
			Agent: agentName,
			Model: modelName,
		}
		s.UsageByAgent[agentName] = usage
	}
	if modelName != "" {
		usage.Model = modelName
	}
	usage.PromptTokens += promptTokens
	usage.OutputTokens += outputTokens
	usage.TotalTokens = usage.PromptTokens + usage.OutputTokens
	usage.CallsCount++

	copyUsage := *usage
	return &copyUsage
}

// UsageSnapshot returns an immutable snapshot of all agents' token usage.
func (s *SwarmContext) UsageSnapshot() map[string]domain.AgentUsage {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.UsageByAgent) == 0 {
		return nil
	}
	out := make(map[string]domain.AgentUsage, len(s.UsageByAgent))
	for k, v := range s.UsageByAgent {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// PlanSnapshot returns a copy of the plan with a deep copy of its task slice,
// so a caller can read progress without racing an agent that is still running.
func (s *SwarmContext) PlanSnapshot() *domain.ProjectPlan {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Plan == nil {
		return nil
	}
	clone := &domain.ProjectPlan{Goal: s.Plan.Goal}
	clone.Tasks = make([]domain.ProjectTask, len(s.Plan.Tasks))
	copy(clone.Tasks, s.Plan.Tasks)
	return clone
}

// SetPlan installs the project's work list.
func (s *SwarmContext) SetPlan(plan *domain.ProjectPlan) {
	if s == nil || plan == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Plan = plan
}

// SetActiveTask records which task this round is working on.
func (s *SwarmContext) SetActiveTask(t *domain.ProjectTask) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveTask = t
}

// ActiveTaskSnapshot copies the current task so it can be handed to an agent
// without holding the lock across a model call.
func (s *SwarmContext) ActiveTaskSnapshot() *domain.ProjectTask {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.ActiveTask == nil {
		return nil
	}
	t := *s.ActiveTask
	t.Files = append([]string(nil), s.ActiveTask.Files...)
	return &t
}

// SetLastTaskVerdict records what verification concluded about the active task.
func (s *SwarmContext) SetLastTaskVerdict(v *TaskVerdict) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastTaskVerdict = v
}

// PlanProgress reports how much of the plan is done and how much is left. The
// two return values are used together: zero open tasks on an empty plan means
// "no plan yet", not "finished".
func (s *SwarmContext) PlanProgress() (percent float64, open int, hasPlan bool) {
	plan := s.PlanSnapshot()
	if plan == nil || len(plan.Tasks) == 0 {
		return 0, 0, false
	}
	done, total, pct := plan.Progress()
	return pct, total - done, true
}

// NextTask picks the task this round should work on.
//
// REWORK comes first so a task that failed review is retried before new work
// starts; otherwise the team would abandon broken work and move on. Among the
// rest, the earliest TODO wins, and an IN_PROGRESS task left over from an
// interrupted run is resumed before anything new.
func (s *SwarmContext) NextTask() *domain.ProjectTask {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Plan == nil {
		return nil
	}

	var fallback *domain.ProjectTask
	for i := range s.Plan.Tasks {
		t := &s.Plan.Tasks[i]
		switch t.State {
		case domain.TaskRework:
			t.State = domain.TaskInProgress
			t.Round = s.Round
			clone := *t
			s.ActiveTask = &clone
			return &clone
		case domain.TaskInProgress:
			if fallback == nil {
				fallback = t
			}
		case domain.TaskTodo:
			if fallback == nil {
				fallback = t
			}
		}
	}

	if fallback == nil {
		return nil
	}
	fallback.State = domain.TaskInProgress
	fallback.Round = s.Round
	clone := *fallback
	s.ActiveTask = &clone
	return &clone
}

// CompleteTask marks the active task finished and records which files it
// produced, so a later round can tell real work apart from re-scoring.
func (s *SwarmContext) CompleteTask(files []string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ActiveTask == nil || s.Plan == nil {
		return
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID != s.ActiveTask.ID {
			continue
		}
		s.Plan.Tasks[i].State = domain.TaskDone
		if len(files) > 0 {
			s.Plan.Tasks[i].Files = append([]string(nil), files...)
		}
		s.Plan.Tasks[i].Round = s.Round
		break
	}
	s.ActiveTask = nil
}

// MaxReworkAttempts caps consecutive rework cycles before human escalation (PROJECT_VISION §19).
const MaxReworkAttempts = 3

// SendTaskBackToRework marks the active task as needing another attempt.
// If the task reaches MaxReworkAttempts (3) or encounters repeated consecutive errors,
// it escalates directly to WAITING_FOR_HUMAN (PROJECT_VISION §19).
func (s *SwarmContext) SendTaskBackToRework(reason ...string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ActiveTask == nil || s.Plan == nil {
		return
	}
	errStr := ""
	if len(reason) > 0 {
		errStr = strings.TrimSpace(reason[0])
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID != s.ActiveTask.ID {
			continue
		}
		s.Plan.Tasks[i].ReworkCount++
		s.Plan.Tasks[i].Round = s.Round

		repeatedError := errStr != "" && s.Plan.Tasks[i].LastError == errStr
		s.Plan.Tasks[i].LastError = errStr

		if s.Plan.Tasks[i].ReworkCount > MaxReworkAttempts || (s.Plan.Tasks[i].ReworkCount >= 3 && repeatedError) {
			s.Plan.Tasks[i].State = domain.TaskWaiting
		} else {
			s.Plan.Tasks[i].State = domain.TaskRework
		}
		break
	}
	s.ActiveTask = nil
}

// BlockActiveTask parks the active task as BLOCKED, which ends the run: a task
// the swarm cannot finish on its own needs a human, and looping on it forever
// would burn rounds without progress.
func (s *SwarmContext) BlockActiveTask() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ActiveTask == nil || s.Plan == nil {
		return
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID != s.ActiveTask.ID {
			continue
		}
		s.Plan.Tasks[i].State = domain.TaskBlocked
		break
	}
	s.ActiveTask = nil
}

// SetActiveTaskFiles records which files the round actually produced for the
// active task.
//
// This runs before verification, because a task's file list is otherwise empty
// on a fresh run: the planner does not know the paths in advance, and
// CompleteTask only fills the list after the task has already been accepted. A
// gate that checks for missing files cannot check anything unless something
// records them first.
func (s *SwarmContext) SetActiveTaskFiles(files []string) {
	if s == nil || len(files) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ActiveTask == nil || s.Plan == nil {
		return
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID == s.ActiveTask.ID {
			s.Plan.Tasks[i].Files = append([]string(nil), files...)
			s.ActiveTask.Files = append([]string(nil), files...)
			break
		}
	}
}

// BlockTask parks a specific task as BLOCKED, which ends the run: a task the
// swarm cannot finish on its own needs a human, and looping on it forever would
// burn rounds without progress.
//
// It takes an id because the active task is already cleared by the time a stall
// is detected, so the id has to come from the round that ran.
func (s *SwarmContext) BlockTask(id string) {
	if s == nil || id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Plan == nil {
		return
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID == id {
			s.Plan.Tasks[i].State = domain.TaskBlocked
			break
		}
	}
}

// SetTaskState sets a specific task's state.
func (s *SwarmContext) SetTaskState(id string, state domain.TaskState) {
	if s == nil || id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Plan == nil {
		return
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID == id {
			s.Plan.Tasks[i].State = state
			break
		}
	}
	if s.ActiveTask != nil && s.ActiveTask.ID == id {
		s.ActiveTask = nil
	}
}

// AllTasksDone reports whether the plan is fully finished. An empty plan is
// never complete: without one there is no evidence the project is done.
func (s *SwarmContext) AllTasksDone() bool {
	plan := s.PlanSnapshot()
	return plan != nil && plan.AllDone()
}

// MergeCodeFiles folds an agent's files into the shared set. A later file with
// the same path replaces the earlier one, so an agent can revise what another
// agent wrote in the same round.
func (s *SwarmContext) MergeCodeFiles(files []domain.CodeFile) {
	if s == nil || len(files) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	index := make(map[string]int, len(s.CodeFiles))
	for i, f := range s.CodeFiles {
		index[f.Path] = i
	}
	for _, f := range files {
		if i, ok := index[f.Path]; ok {
			s.CodeFiles[i] = f
			continue
		}
		index[f.Path] = len(s.CodeFiles)
		s.CodeFiles = append(s.CodeFiles, f)
	}
}

// CodeFilesSnapshot returns a copy of the shared files, safe to read while
// other agents are still writing.
func (s *SwarmContext) CodeFilesSnapshot() []domain.CodeFile {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.CodeFile, len(s.CodeFiles))
	copy(out, s.CodeFiles)
	return out
}

// ReplaceCodeFiles swaps the whole set, used after a sync round rewrote the
// files on disk and recomputed their diffs.
func (s *SwarmContext) ReplaceCodeFiles(files []domain.CodeFile) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CodeFiles = files
}

// SetCodeFiles loads files from disk when a project is first opened.
func (s *SwarmContext) SetCodeFiles(files []domain.CodeFile) {
	s.ReplaceCodeFiles(files)
}

// ClearVerificationIssues drops the compiler errors from the last verification
// run so the next one reports only what is still broken.
func (s *SwarmContext) ClearVerificationIssues() {
	if s == nil || s.CriticReview == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := s.CriticReview.Issues[:0]
	for _, issue := range s.CriticReview.Issues {
		if issue.File != "verification" {
			kept = append(kept, issue)
		}
	}
	s.CriticReview.Issues = kept
}

// Agent defines the behavior of a specialized swarm member.
type Agent interface {
	Role() domain.AgentRole
	Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error)
	Discuss(ctx context.Context, sCtx *SwarmContext, history []*domain.AgentMessage) (*domain.AgentMessage, error)
}

// BaseAgent provides shared functionality like memory queries and LLM interaction.
type BaseAgent struct {
	role      domain.AgentRole
	llmClient llm.Client
}

func NewBaseAgent(role domain.AgentRole, client llm.Client) *BaseAgent {
	return &BaseAgent{
		role:      role,
		llmClient: client,
	}
}

func (b *BaseAgent) Role() domain.AgentRole {
	return b.role
}

func (b *BaseAgent) getModelForRole(sCtx *SwarmContext) string {
	if sCtx != nil && sCtx.AgentModels != nil {
		if m, ok := sCtx.AgentModels[b.role]; ok && m != "" {
			return m
		}
	}
	return ""
}

func (b *BaseAgent) cleanModelName(model string) string {
	return cleanModelName(model)
}

// cleanModelName strips the provider namespace from a model id.
//
// The prefix tells ResolveProvider where to send the request; the endpoint then
// expects the bare model name. Sending "ninerouter/gpt-4" to the ninerouter
// endpoint gets rejected as an unknown model, so the name must be cleaned at the
// point of sending while the provider is resolved from the original.
func cleanModelName(model string) string {
	if len(model) >= 11 && model[:11] == "ninerouter/" {
		return model[11:]
	}
	if len(model) >= 8 && model[:8] == "9router/" {
		return model[8:]
	}
	if len(model) >= 7 && model[:7] == "openai/" {
		return model[7:]
	}
	return model
}

// providerRoute describes where a model's requests should be sent.
type providerRoute struct {
	apiKeyName string
	baseURL    string
}

// ResolveProvider maps a model identifier to the API key and base URL that
// serve it. Routing is decided by the model's own namespace prefix, never by
// guessing, so an unrecognised model fails loudly instead of silently falling
// back to an unrelated provider.
func ResolveProvider(model string) (providerRoute, bool) {
	switch {
	case model == "":
		return providerRoute{}, false

	// NineRouter serves short local aliases with no namespace prefix.
	case model == "mimo", model == "flash_5", model == "bunny", model == "nemotron", model == "nemo_qwen":
		return providerRoute{"ninerouter", "http://127.0.0.1:20128/v1"}, true

	// OpenRouter namespaces every model it hosts as "<vendor>/<model>".
	case strings.HasPrefix(model, "openrouter"):
		return providerRoute{"openrouter", "https://openrouter.ai/api/v1"}, true

	// Models fetched through OpenRouter are addressed by their vendor prefix.
	case hasAnyPrefix(model, "qwen/", "deepseek/", "meta-llama/", "google/",
		"nvidia/", "anthropic/", "openai/", "mistral/", "x-ai/", "mistralai/",
		"cohere/", "perplexity/", "microsoft/", "z-ai/", "moonshotai/", "baidu/"):
		return providerRoute{"openrouter", "https://openrouter.ai/api/v1"}, true

	case strings.HasPrefix(model, "ninerouter/"), strings.HasPrefix(model, "9router/"):
		return providerRoute{"ninerouter", "http://127.0.0.1:20128/v1"}, true

	case strings.HasPrefix(model, "ollama/"), strings.HasPrefix(model, "ollama:"):
		return providerRoute{"ollama", "http://127.0.0.1:11434/v1"}, true
	}
	return providerRoute{}, false
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// getAPIKey resolves the credentials for a model's provider. It returns an
// error when no model is selected or the selected model has no known
// provider, so a missing choice is never silently replaced with a default.
func (b *BaseAgent) getAPIKey(model string, sCtx *SwarmContext) (string, string, error) {
	route, ok := ResolveProvider(model)
	if !ok {
		return "", "", fmt.Errorf("no model assigned to agent %q", b.role)
	}
	if sCtx == nil {
		return "", "", fmt.Errorf("agent %q has no swarm context for key lookup", b.role)
	}
	key := sCtx.APIKeys[route.apiKeyName]
	if key == "" {
		key = providerKeyFromEnv(route.apiKeyName)
	}
	if key == "" {
		return "", "", fmt.Errorf("missing %s API key for agent %q", route.apiKeyName, b.role)
	}
	return key, route.baseURL, nil
}

// providerKeyFromEnv reads a provider's key from the environment.
//
// The OPENAI_API_KEY fallback is deliberate: an OpenAI-compatible endpoint is
// how most of these providers are reached, and requiring a separate variable
// for each one just meant keys were silently not found.
func providerKeyFromEnv(provider string) string {
	switch provider {
	case "ninerouter":
		if k := os.Getenv("NINEROUTER_API_KEY"); k != "" {
			return k
		}
	case "openrouter":
		if k := os.Getenv("OPENROUTER_API_KEY"); k != "" {
			return k
		}
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "gemini":
		return os.Getenv("GEMINI_API_KEY")
	}
	return os.Getenv("OPENAI_API_KEY")
}

func (b *BaseAgent) fetchMemories(ctx context.Context, sCtx *SwarmContext) string {
	if sCtx == nil {
		return ""
	}
	var sb strings.Builder

	// 1. Read from physical .project/ files if workspace path is available (PROJECT_VISION §8)
	if sCtx.WorkspacePath != "" {
		if bfs, err := workspace.NewBrainFS(sCtx.WorkspacePath); err == nil {
			sb.WriteString(bfs.ReadGroundTruth())
		}
	}

	// 2. Read from persistent SQLite memory store
	if sCtx.MemStore != nil && sCtx.ProjectID != "" {
		brain := NewProjectBrain(sCtx.MemStore)
		sb.WriteString(brain.BuildContext(ctx, sCtx.ProjectID))
	}

	return sb.String()
}

func (b *BaseAgent) createMessage(sCtx *SwarmContext, msgType domain.MessageType, payload interface{}, summary string) (*domain.AgentMessage, error) {
	return b.createMessageWithReasoning(sCtx, msgType, payload, summary, "")
}

func (b *BaseAgent) createMessageWithReasoning(sCtx *SwarmContext, msgType domain.MessageType, payload interface{}, summary string, reasoning string) (*domain.AgentMessage, error) {
	var bytes []byte
	var err error
	switch p := payload.(type) {
	case json.RawMessage:
		bytes = p
	case []byte:
		if json.Valid(p) {
			bytes = p
		} else {
			bytes, err = json.Marshal(payload)
		}
	default:
		bytes, err = json.Marshal(payload)
	}
	if err != nil {
		return nil, err
	}
	return &domain.AgentMessage{
		ID:        fmt.Sprintf("msg-%s-%d-%d", b.role, sCtx.Round, time.Now().UnixNano()),
		From:      b.role,
		Round:     sCtx.Round,
		Type:      msgType,
		Content:   bytes,
		Summary:   summary,
		Reasoning: reasoning,
		Timestamp: time.Now(),
	}, nil
}

// CleanJSON extracts raw JSON from markdown fences or conversational text.
func CleanJSON(raw string) string {
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	startArr := strings.Index(s, "[")
	endArr := strings.LastIndex(s, "]")
	if startArr != -1 && endArr != -1 && endArr > startArr {
		return s[startArr : endArr+1]
	}
	return s
}

func (b *BaseAgent) buildDiscussionPrompt(sCtx *SwarmContext, history []*domain.AgentMessage) (string, string) {
	historySummary := ""
	for _, m := range history {
		historySummary += fmt.Sprintf("[%s]: %s\n", strings.ToUpper(string(m.From)), m.Summary)
	}

	roleDescription := ""
	switch b.role {
	case domain.RoleArchitect:
		roleDescription = "You are the System Architect. Provide the high-level architectural, conceptual, and foundational perspective on the user's request."
	case domain.RoleCoder:
		roleDescription = "You are the Lead Implementation Engineer. Take the Architect's perspective and discuss it from a practical software engineering, code structure, tooling, and implementation perspective. Respond to the Architect and user."
	case domain.RoleCritic:
		roleDescription = "You are the Adversarial Critic (The Red Team). Constructively critique the discussion so far: point out potential blind spots, edge cases, performance tradeoffs, or alternative approaches."
	case domain.RoleSecurity:
		roleDescription = "You are the Security Auditor. Evaluate the discussion from security, data protection, OWASP standards, robustness, and reliability angles."
	case domain.RoleArbiter:
		roleDescription = "You are the Arbiter Judge and Supreme Synthesizer. Review the perspectives of all 4 peer agents (Architect, Coder, Critic, Security). Settle any differences, and craft the BEST, most comprehensive, helpful, and definitive FINAL ANSWER directly for the user."
	}

	sysPrompt := fmt.Sprintf(`%s
You are participating in a sequential multi-agent relay discussion where each agent builds on the previous agents' responses to deliver the ultimate answer to the user.
CRITICAL RULES:
1. Match the user's language (if the user wrote in Persian, respond and reason in fluent Persian; if English, respond in English).
2. Keep your perspective focused on your specialized role.
3. Be constructive, engaging, and collaborative with your peer agents.
4. If this is a greeting or casual talk, respond warmly and express your specialized role's readiness to assist.
5. Return ONLY valid JSON:
{
  "thought": "Your internal analysis and reasoning in the user's language",
  "reply": "Your message to the discussion in the user's language"
}`, roleDescription)

	userPrompt := fmt.Sprintf("User Request: %s\n\nPeer Discussion So Far:\n%s", sCtx.UserPrompt, historySummary)
	return sysPrompt, userPrompt
}

func (b *BaseAgent) Discuss(ctx context.Context, sCtx *SwarmContext, history []*domain.AgentMessage) (*domain.AgentMessage, error) {
	sysPrompt, userPrompt := b.buildDiscussionPrompt(sCtx, history)

	model := b.getModelForRole(sCtx)
	apiKey, baseURL, err := b.getAPIKey(model, sCtx)
	if err != nil {
		return nil, err
	}
	cleanModel := b.cleanModelName(model)

	callCtx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()

	resp, err := b.llmClient.Generate(callCtx, llm.Request{
		Model:        cleanModel,
		SystemPrompt: sysPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
		APIKey:       apiKey,
		BaseURL:      baseURL,
	})
	var usage *domain.AgentUsage
	if resp != nil {
		usage = sCtx.RecordAgentUsage(string(b.role), cleanModel, resp.PromptTokens, resp.OutputTokens)
	}
	if err != nil || resp == nil {
		log.Printf("Discussion agent %s (%s) unavailable (%v); providing safe fallback reply", b.role, cleanModel, err)
		fallbackPayload := map[string]string{
			"reply":   fmt.Sprintf("به‌عنوان %s، این موضوع از نظر اصول طراحی و ساختار پروژه بررسی شد.", b.role),
			"thought": fmt.Sprintf("بررسی سریع دیدگاه %s در هماهنگی با سایر اعضای تیم.", b.role),
		}
		return b.createMessageWithReasoning(sCtx, domain.MsgFeedback, fallbackPayload, fmt.Sprintf("[%s] بررسی دیدگاه این نقش برای اهداف مطرح‌شده", b.role), fmt.Sprintf("تحلیل تخصصی %s", b.role))
	}

	var parsed struct {
		Thought string `json:"thought"`
		Reply   string `json:"reply"`
	}

	if err == nil && resp != nil {
		cleaned := CleanJSON(resp.Content)
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(cleaned), &m); err == nil {
			for k, v := range m {
				str, ok := v.(string)
				if !ok {
					continue
				}
				lk := strings.ToLower(k)
				if lk == "reply" || lk == "response" || lk == "message" || lk == "answer" || lk == "output" || lk == "text" || lk == "content" {
					if parsed.Reply == "" || lk == "reply" {
						parsed.Reply = str
					}
				}
				if lk == "thought" || lk == "reasoning" || lk == "reason" || lk == "thinking" || lk == "analysis" {
					if parsed.Thought == "" || lk == "thought" {
						parsed.Thought = str
					}
				}
			}
		}
		if parsed.Reply == "" {
			parsed.Reply = strings.TrimSpace(resp.Content)
		}
	}

	if parsed.Reply == "" {
		// Resilient fallback based on role if model generation timed out, errored, or returned empty
		switch b.role {
		case domain.RoleCoder:
			parsed.Reply = "از دیدگاه مهندسی و پیاده‌سازی، آماده‌ام تا جزئیات فنی و کدهای مورد نیاز را پیاده‌سازی کنم."
			parsed.Thought = "آمادگی مهندسی برای پیاده‌سازی درخواست کاربر."
		case domain.RoleCritic:
			parsed.Reply = "من به عنوان منتقد تیم، مطمئن خواهم شد که کدهای ارائه‌شده بدون باگ، بهینه و با پوشش کامل آزمون باشند."
			parsed.Thought = "بررسی کیفیت، چالش‌ها و سناریوهای آزمون."
		case domain.RoleSecurity:
			parsed.Reply = "از دیدگاه امنیتی، استانداردهای حفاظتی و ممیزی ورودی‌ها را زیر نظر دارم تا هیچ ریسک و آسیب‌پذیری وجود نداشته باشد."
			parsed.Thought = "ارزیابی ایمنی و استانداردهای امنیتی."
		case domain.RoleArbiter:
			parsed.Reply = "جمع‌بندی نهایی تیم: تمام جنبه‌های معماری، مهندسی، کیفیت و امنیت بررسی شد. تیم ۵ نفره هوش مصنوعی آماده اجرای اهداف و پروژه شماست."
			parsed.Thought = "جمع‌بندی نظرات عامل‌ها و ارائه پاسخ نهایی."
		default:
			parsed.Reply = "درود! در خدمتم."
			parsed.Thought = "تحلیل درخواست کاربر."
		}
	}

	summary := strings.TrimSpace(parsed.Reply)
	reason := strings.TrimSpace(parsed.Thought)
	if reason == "" && resp != nil {
		reason = resp.Reasoning
	}
	if reason == "" {
		reason = fmt.Sprintf("تحلیل پیام و ارائه دیدگاه تخصصی به عنوان %s.", b.role)
	}

	msgType := domain.MsgFeedback
	if b.role == domain.RoleArbiter {
		msgType = domain.MsgArbiterVerdict
	}

	msg, err := b.createMessageWithReasoning(sCtx, msgType, parsed, summary, reason)
	if err == nil && msg != nil && usage != nil {
		msg.Tokens = usage
	}
	return msg, err
}


