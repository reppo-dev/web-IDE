package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/memory"
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
}

// Agent defines the behavior of a specialized swarm member.
type Agent interface {
	Role() domain.AgentRole
	Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error)
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

func (b *BaseAgent) fetchMemories(ctx context.Context, sCtx *SwarmContext) string {
	if sCtx.MemStore == nil {
		return ""
	}
	memories, err := sCtx.MemStore.QueryProject(ctx, sCtx.ProjectID, memory.QueryFilter{
		Limit: 5,
	})
	if err != nil || len(memories) == 0 {
		return ""
	}
	out := "\n[Project Knowledge & History]:\n"
	for _, m := range memories {
		out += fmt.Sprintf("- [%s by %s]: %s\n", m.Type, m.Agent, m.Content)
	}
	return out
}

func (b *BaseAgent) createMessage(sCtx *SwarmContext, msgType domain.MessageType, payload interface{}, summary string) (*domain.AgentMessage, error) {
	bytes, err := json.Marshal(payload)
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
		Timestamp: time.Now(),
	}, nil
}
