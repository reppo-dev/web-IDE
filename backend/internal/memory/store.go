package memory

import (
	"context"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

// QueryFilter parameters for searching memories.
type QueryFilter struct {
	Agent    domain.AgentRole   `json:"agent,omitempty"`
	Type     domain.MemoryType  `json:"type,omitempty"`
	Tag      string             `json:"tag,omitempty"`
	MinScore float64            `json:"min_score,omitempty"`
	Limit    int                `json:"limit,omitempty"`
}

// MemoryStore defines interface for persistent Mem0-style memory.
type MemoryStore interface {
	// WriteProject saves a memory entry scoped to a specific project.
	WriteProject(ctx context.Context, entry domain.MemoryEntry) error

	// QueryProject fetches memories for a given project matching the filter.
	QueryProject(ctx context.Context, projectID string, filter QueryFilter) ([]domain.MemoryEntry, error)

	// WriteGlobal saves a cross-project memory entry.
	WriteGlobal(ctx context.Context, entry domain.MemoryEntry) error

	// QueryGlobal fetches cross-project memories matching the filter.
	QueryGlobal(ctx context.Context, filter QueryFilter) ([]domain.MemoryEntry, error)

	// DeleteMemory removes a specific memory item.
	DeleteMemory(ctx context.Context, id string, isGlobal bool) error

	// Close closes database handles.
	Close() error
}
