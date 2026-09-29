package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/memory"
	"github.com/nexusai/nexusai/backend/internal/sandbox"
	"github.com/nexusai/nexusai/backend/internal/scoring"
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
)

// SwarmEvent is the real-time event dispatched during swarm execution.
type SwarmEvent struct {
	Type      EventType            `json:"type"`
	ProjectID string               `json:"project_id"`
	Round     int                  `json:"round"`
	Agent     domain.AgentRole     `json:"agent,omitempty"`
	Message   *domain.AgentMessage `json:"message,omitempty"`
	Verdict   *domain.ArbiterVerdict `json:"verdict,omitempty"`
	Files     []domain.CodeFile    `json:"files,omitempty"`
	Payload   interface{}          `json:"payload,omitempty"`
	Timestamp time.Time            `json:"timestamp"`
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
	switch o.config.Mode {
	case domain.ModeSequential:
		return o.runSequential(ctx, projectID, userPrompt)
	case domain.ModeConcurrent:
		return o.runConcurrent(ctx, projectID, userPrompt)
	default:
		return o.runRoleBased(ctx, projectID, userPrompt)
	}
}
