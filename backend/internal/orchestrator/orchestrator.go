package orchestrator

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/memory"
	"github.com/nexusai/nexusai/backend/internal/sandbox"
	"github.com/nexusai/nexusai/backend/internal/scoring"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// EventType defines the kind of orchestrator event broadcast to listeners.
type EventType string

const (
	EventAgentThinking    EventType = "agent_thinking"
	EventAgentOutput      EventType = "agent_output"
	EventRoundStarted     EventType = "round_started"
	EventRoundComplete    EventType = "round_complete"
	EventConsensusReached EventType = "consensus_reached"
	EventCodeUpdated      EventType = "code_updated"
	EventError            EventType = "error"
	EventPlanUpdated      EventType = "plan_updated"
	EventTaskStep         EventType = "task_step"
	EventLocksChanged     EventType = "locks_changed"
)

// SwarmEvent is the real-time event dispatched during swarm execution.
type SwarmEvent struct {
	Type      EventType              `json:"type"`
	ProjectID string                 `json:"project_id"`
	Round     int                    `json:"round"`
	Agent     domain.AgentRole       `json:"agent,omitempty"`
	Message   *domain.AgentMessage   `json:"message,omitempty"`
	Verdict   *domain.ArbiterVerdict `json:"verdict,omitempty"`
	Files     []domain.CodeFile      `json:"files,omitempty"`
	Payload   interface{}                  `json:"payload,omitempty"`
	Plan      *domain.ProjectPlan          `json:"plan,omitempty"`
	Usage     map[string]domain.AgentUsage `json:"usage,omitempty"`
	Timestamp time.Time                    `json:"timestamp"`

	// TaskID and Stream carry the live per-task conversation.
	//
	// Every step an agent took used to arrive as its own chat card, so a
	// twenty-five task project produced hundreds of cards the user had to scroll
	// back through to reassemble. One task is one stream instead: steps append to
	// it and the card fills in as the work happens. TaskID names the stream and is
	// empty for run-level events that belong to no single task.
	TaskID string `json:"task_id,omitempty"`
	Stream *TaskStreamStep `json:"stream,omitempty"`
	// StreamReset tells the client to start a fresh stream. A retried task needs
	// this: its earlier attempts are history, not the current state of the work.
	StreamReset bool `json:"stream_reset,omitempty"`
	// Locks lists the files currently held, so the UI can show who is editing
	// what instead of two agents silently colliding on a shared file.
	Locks []workspace.FileLock `json:"locks,omitempty"`
}

// TaskStreamStep is one entry in a task's live conversation.
//
// Kind is what the UI renders: a line of reasoning, a tool call, a set of edits,
// a test result. Distinguishing them is what makes the card read as work in
// progress rather than as a wall of prose.
type TaskStreamStep struct {
	// TaskID repeats the stream's owner so a client can route a step that
	// arrives without the enclosing event.
	TaskID string `json:"task_id"`
	// Agent is who took this step.
	Agent domain.AgentRole `json:"agent"`
	// Kind is the step type: thought, tool, edit, test, review, decision.
	Kind string `json:"kind"`
	// Text is the human-readable content of the step.
	Text string `json:"text"`
	// Files lists the paths this step wrote or changed, if any.
	Files []string `json:"files,omitempty"`
	// At is when the step happened.
	At time.Time `json:"at"`
}

// Config holds runtime parameters for the orchestrator.
type Config struct {
	MaxRounds int                  `json:"max_rounds"`
	Mode      domain.ExecutionMode `json:"mode"`
}

// Orchestrator coordinates the 5-agent swarm across rounds.
type Orchestrator struct {
	config    Config
	architect agent.Agent
	coder     agent.Agent
	critic    agent.Agent
	security  agent.Agent
	arbiter   agent.Agent
	memStore  memory.MemoryStore
	// client builds user-defined team agents that are not one of the five
	// built-in personas.
	client llm.Client

	matrixEngine *agent.MatrixEngine
	sandbox      *sandbox.Runner
	detector     *scoring.ConvergenceDetector
	baseDir      string

	eventsMu    sync.RWMutex
	subscribers []chan SwarmEvent
}

// NewOrchestrator creates a new Orchestrator instance.
func NewOrchestrator(
	cfg Config,
	arch agent.Agent,
	coder agent.Agent,
	critic agent.Agent,
	sec agent.Agent,
	arb agent.Agent,
	mem memory.MemoryStore,
	matrix *agent.MatrixEngine,
	baseDir string,
) *Orchestrator {
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 5
	}
	if cfg.Mode == "" {
		cfg.Mode = domain.ModeRoleBased
	}

	return &Orchestrator{
		config:       cfg,
		architect:    arch,
		coder:        coder,
		critic:       critic,
		security:     sec,
		arbiter:      arb,
		memStore:     mem,
		matrixEngine: matrix,
		sandbox:      sandbox.NewRunner(10 * time.Second),
		detector:     scoring.NewConvergenceDetector(3),
		baseDir:      baseDir,
		subscribers:  make([]chan SwarmEvent, 0),
	}
}

// SetLLMClient registers the client used to build user-defined team agents.
// It is separate from the constructor so existing call sites keep working.
func (o *Orchestrator) SetLLMClient(c llm.Client) {
	o.client = c
}

// GetBaseDir returns the base directory of the orchestrator.
func (o *Orchestrator) GetBaseDir() string {
	return o.baseDir
}

// Subscribe returns a channel that receives live swarm events.
func (o *Orchestrator) Subscribe() chan SwarmEvent {
	o.eventsMu.Lock()
	defer o.eventsMu.Unlock()
	ch := make(chan SwarmEvent, 100)
	o.subscribers = append(o.subscribers, ch)
	return ch
}

// Unsubscribe removes an event channel.
func (o *Orchestrator) Unsubscribe(ch chan SwarmEvent) {
	o.eventsMu.Lock()
	defer o.eventsMu.Unlock()
	for i, sub := range o.subscribers {
		if sub == ch {
			o.subscribers = append(o.subscribers[:i], o.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

func (o *Orchestrator) broadcast(evt SwarmEvent) {
	o.eventsMu.RLock()
	defer o.eventsMu.RUnlock()
	evt.Timestamp = time.Now()
	for _, ch := range o.subscribers {
		select {
		case ch <- evt:
		default:
			// Non-blocking drop if channel buffer is full
		}
	}
}

// Run executes the swarm using the configured mode until consensus is reached or max rounds hit.
func (o *Orchestrator) Run(ctx context.Context, projectID string, userPrompt string) (*agent.SwarmContext, error) {
	return o.RunWithOptions(ctx, projectID, userPrompt, o.config.Mode, nil, nil)
}

// RunWithMode executes the swarm using a dynamic execution mode.
func (o *Orchestrator) RunWithMode(ctx context.Context, projectID string, userPrompt string, mode domain.ExecutionMode) (*agent.SwarmContext, error) {
	return o.RunWithOptions(ctx, projectID, userPrompt, mode, nil, nil)
}

// RunWithOptions executes the swarm with custom mode and per-agent model assignments.
func (o *Orchestrator) RunWithOptions(ctx context.Context, projectID string, userPrompt string, mode domain.ExecutionMode, agentModels map[domain.AgentRole]string, apiKeys map[string]string) (*agent.SwarmContext, error) {
	if mode == "" {
		mode = o.config.Mode
	}
	switch mode {
	case domain.ModeSequential:
		return o.runSequential(ctx, projectID, userPrompt, agentModels, apiKeys)
	case domain.ModeConcurrent:
		return o.runConcurrent(ctx, projectID, userPrompt, agentModels, apiKeys)
	default:
		return o.runRoleBased(ctx, projectID, userPrompt, agentModels, apiKeys)
	}
}

// runRelayDiscussion conducts a sequential multi-agent relay discussion where
// all 5 agents (Architect, Coder, Critic, Security, Arbiter) participate in turn,
// conversing with each other and formulating the best final answer for the user.
func (o *Orchestrator) runRelayDiscussion(ctx context.Context, sCtx *agent.SwarmContext, projectID string, archMsg *domain.AgentMessage) (*agent.SwarmContext, error) {
	// A nil architect message would put a hole in the history the other agents
	// read, so the relay starts from only the messages that exist.
	var history []*domain.AgentMessage
	if archMsg != nil {
		history = append(history, archMsg)
	}

	steps := []struct {
		role  domain.AgentRole
		agent agent.Agent
		desc  string
	}{
		{domain.RoleCoder, o.coder, "در حال بررسی فنی و اضافه کردن دیدگاه مهندسی کد به گفتگو..."},
		{domain.RoleCritic, o.critic, "در حال ارزیابی نقادانه، بررسی چالش‌ها و نکات قابل تامل..."},
		{domain.RoleSecurity, o.security, "در حال بررسی جنبه‌های امنیتی، پایداری و استانداردهای حفاظتی..."},
		{domain.RoleArbiter, o.arbiter, "در حال جمع‌بندی نظرات تمام هوش‌های مصنوعی و تدوین بهترین پاسخ نهایی..."},
	}

	for _, step := range steps {
		o.broadcast(SwarmEvent{
			Type:      EventAgentThinking,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Agent:     step.role,
			Payload:   step.desc,
		})

		msg, err := step.agent.Discuss(ctx, sCtx, history)
		if err != nil {
			log.Printf("relay discussion step %s error: %v", step.role, err)
			continue
		}

		history = append(history, msg)
		o.broadcast(SwarmEvent{
			Type:      EventAgentOutput,
			ProjectID: projectID,
			Round:     sCtx.Round,
			Agent:     step.role,
			Message:   msg,
			Usage:     sCtx.UsageSnapshot(),
		})
	}

	lastMsg := archMsg
	if len(history) > 0 {
		lastMsg = history[len(history)-1]
	}

	o.broadcast(SwarmEvent{
		Type:      EventConsensusReached,
		ProjectID: projectID,
		Round:     sCtx.Round,
		Message:   lastMsg,
		Payload:   sCtx.Intent,
		Usage:     sCtx.UsageSnapshot(),
	})

	return sCtx, nil
}

