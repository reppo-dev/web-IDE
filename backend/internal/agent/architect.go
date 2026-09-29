package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
)

type ArchitectAgent struct {
	*BaseAgent
}

func NewArchitectAgent(client llm.Client) *ArchitectAgent {
	return &ArchitectAgent{
		BaseAgent: NewBaseAgent(domain.RoleArchitect, client),
	}
}

func (a *ArchitectAgent) Execute(ctx context.Context, sCtx *SwarmContext) (*domain.AgentMessage, error) {
	memories := a.fetchMemories(ctx, sCtx)

	systemPrompt := `You are the System Architect in NexusAI.
Your role:
- Analyze user requests (whether in Persian or English).
- Decompose requirements into structured file trees, architecture designs, and API contracts.
- Select the cleanest architectural patterns (Clean Architecture, DDD, Repository Pattern).
- If the user prompt is in Persian, reason and describe components in fluent Persian, but keep file paths and API contracts in standard English.
- Return ONLY valid JSON conforming to the following structure:
{
  "project_overview": "Summary of the system architecture",
  "file_tree": ["path/to/file1.go", "path/to/file2.go"],
  "key_components": ["Component A description", "Component B description"],
  "api_contracts": ["Endpoint/Interface specs"]
}`

	userPrompt := fmt.Sprintf("User Request: %s\n%s", sCtx.UserPrompt, memories)

	resp, err := a.llmClient.Generate(ctx, llm.Request{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		JSONSchema:   map[string]interface{}{"type": "object"},
	})
	if err != nil {
		return nil, fmt.Errorf("architect generation failed: %w", err)
	}

	var blueprint domain.BlueprintPayload
	if err := json.Unmarshal([]byte(resp.Content), &blueprint); err != nil {
		// Fallback for structured parsing
		blueprint = domain.BlueprintPayload{
			ProjectOverview: resp.Content,
			FileTree:        []string{"main.go"},
			KeyComponents:   []string{"Main Service"},
		}
	}

	sCtx.Blueprint = &blueprint

	// Write decision to memory
	if sCtx.MemStore != nil {
		_ = sCtx.MemStore.WriteProject(ctx, domain.MemoryEntry{
			ProjectID:  sCtx.ProjectID,
			Agent:      domain.RoleArchitect,
			Type:       domain.MemoryDecision,
			Content:    fmt.Sprintf("Architectural Blueprint designed: %s", blueprint.ProjectOverview),
			Tags:       []string{"blueprint", "architecture"},
			Round:      sCtx.Round,
			Confidence: 0.95,
		})
	}

	summary := fmt.Sprintf("Architect prepared blueprint with %d files planned.", len(blueprint.FileTree))
	return a.createMessage(sCtx, domain.MsgBlueprint, blueprint, summary)
}
