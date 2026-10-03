package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/memory"
)

// ProjectBrain provides structured, categorised ground truth across the entire swarm.
// Instead of a fragile linear message chain where details are lost, every agent
// reads from and writes to the shared Project Brain.
type ProjectBrain struct {
	store memory.MemoryStore
}

// NewProjectBrain creates a new ProjectBrain backed by the given MemoryStore.
func NewProjectBrain(store memory.MemoryStore) *ProjectBrain {
	return &ProjectBrain{store: store}
}

// BaselineConstitution returns the foundational non-negotiable invariants of any NexusAI project.
func BaselineConstitution() []string {
	return []string{
		"Do not modify public APIs or exported signatures without explicit user requirement.",
		"Preserve existing working functionality; never silently delete code or feature modules.",
		"Handle all errors explicitly; never ignore returned errors or suppress crashes.",
		"Ensure all new code is self-consistent with the existing repository architecture.",
	}
}

// BuildContext queries project knowledge and constructs a structured Ground Truth context
// categorised into Invariants, Decisions, Contracts, Facts, and Warnings.
func (b *ProjectBrain) BuildContext(ctx context.Context, projectID string) string {
	if b == nil || b.store == nil || projectID == "" {
		return ""
	}

	// Query up to 30 items for the project (solves the 5-item limit issue)
	entries, err := b.store.QueryProject(ctx, projectID, memory.QueryFilter{
		Limit: 30,
	})
	if err != nil && len(entries) == 0 {
		return ""
	}

	var invariants []string
	var decisions []string
	var contracts []string
	var facts []string
	var warnings []string
	var mistakes []string

	for _, e := range entries {
		content := strings.TrimSpace(e.Content)
		if content == "" {
			continue
		}
		item := fmt.Sprintf("[%s]: %s", e.Agent, content)

		switch e.Type {
		case domain.MemoryInvariant:
			invariants = append(invariants, item)
		case domain.MemoryDecision:
			decisions = append(decisions, item)
		case domain.MemoryContract:
			contracts = append(contracts, item)
		case domain.MemoryFact:
			facts = append(facts, item)
		case domain.MemoryWarning:
			warnings = append(warnings, item)
		case domain.MemoryMistake:
			mistakes = append(mistakes, item)
		default:
			// General insights
			decisions = append(decisions, item)
		}
	}

	// Always include baseline constitution in invariants if empty
	if len(invariants) == 0 {
		for _, rule := range BaselineConstitution() {
			invariants = append(invariants, fmt.Sprintf("[constitution]: %s", rule))
		}
	}

	var sb strings.Builder
	sb.WriteString("\n=== SHARED PROJECT BRAIN (Ground Truth) ===\n")

	if len(invariants) > 0 {
		sb.WriteString("--- INVARIANTS & PROJECT RULES (Must NEVER be violated) ---\n")
		for _, inv := range invariants {
			sb.WriteString("- " + inv + "\n")
		}
	}

	if len(decisions) > 0 {
		sb.WriteString("\n--- ARCHITECTURAL & DESIGN DECISIONS (Accepted Truth) ---\n")
		for _, dec := range decisions {
			sb.WriteString("- " + dec + "\n")
		}
	}

	if len(contracts) > 0 {
		sb.WriteString("\n--- API & SYSTEM CONTRACTS ---\n")
		for _, con := range contracts {
			sb.WriteString("- " + con + "\n")
		}
	}

	if len(facts) > 0 {
		sb.WriteString("\n--- VERIFIED FACTS ---\n")
		for _, f := range facts {
			sb.WriteString("- " + f + "\n")
		}
	}

	if len(warnings) > 0 {
		sb.WriteString("\n--- ACTIVE WARNINGS & DETECTED RISKS ---\n")
		for _, w := range warnings {
			sb.WriteString("- " + w + "\n")
		}
	}

	if len(mistakes) > 0 {
		sb.WriteString("\n--- PREVIOUS MISTAKES & LESSONS (Avoid Repeating) ---\n")
		for _, m := range mistakes {
			sb.WriteString("- " + m + "\n")
		}
	}

	sb.WriteString("=== END PROJECT BRAIN ===\n\n")
	return sb.String()
}

// RecordInvariant saves a non-negotiable project invariant.
func (b *ProjectBrain) RecordInvariant(ctx context.Context, projectID string, agent domain.AgentRole, rule string, round int) error {
	return b.recordEntry(ctx, projectID, agent, domain.MemoryInvariant, rule, round, 1.0)
}

// RecordDecision saves an accepted architectural decision.
func (b *ProjectBrain) RecordDecision(ctx context.Context, projectID string, agent domain.AgentRole, decision string, round int) error {
	return b.recordEntry(ctx, projectID, agent, domain.MemoryDecision, decision, round, 0.95)
}

// RecordContract saves an API or interface contract.
func (b *ProjectBrain) RecordContract(ctx context.Context, projectID string, agent domain.AgentRole, contract string, round int) error {
	return b.recordEntry(ctx, projectID, agent, domain.MemoryContract, contract, round, 0.95)
}

// RecordFact saves a verified codebase fact.
func (b *ProjectBrain) RecordFact(ctx context.Context, projectID string, agent domain.AgentRole, fact string, round int) error {
	return b.recordEntry(ctx, projectID, agent, domain.MemoryFact, fact, round, 1.0)
}

// RecordWarning saves an active risk or vulnerability warning.
func (b *ProjectBrain) RecordWarning(ctx context.Context, projectID string, agent domain.AgentRole, warning string, round int) error {
	return b.recordEntry(ctx, projectID, agent, domain.MemoryWarning, warning, round, 0.90)
}

func (b *ProjectBrain) recordEntry(ctx context.Context, projectID string, agent domain.AgentRole, memType domain.MemoryType, content string, round int, conf float64) error {
	if b == nil || b.store == nil || projectID == "" || strings.TrimSpace(content) == "" {
		return nil
	}
	return b.store.WriteProject(ctx, domain.MemoryEntry{
		ID:         fmt.Sprintf("brain-%s-%d", uuid.New().String()[:8], time.Now().UnixNano()),
		ProjectID:  projectID,
		Agent:      agent,
		Type:       memType,
		Content:    strings.TrimSpace(content),
		Round:      round,
		Confidence: conf,
		CreatedAt:  time.Now(),
	})
}
