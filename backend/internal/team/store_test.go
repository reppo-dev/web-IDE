package team

import (
	"context"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestTeamStore_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create team store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Initial GetDefault should return built-in DefaultTeam
	def, err := store.GetDefault(ctx)
	if err != nil {
		t.Fatalf("expected default team fallback, got: %v", err)
	}
	if def == nil || len(def.Agents) != 5 {
		t.Fatalf("expected 5 agents in default team, got %v", def)
	}

	// 2. Save a custom team
	customTeam := &domain.Team{
		ID:                 "custom-pair",
		Name:               "Dynamic Duo",
		Description:        "Pair programming team",
		ConsensusThreshold: 90.0,
		MaxRounds:          2,
		Agents: []domain.AgentSpec{
			{
				ID:      "coder",
				Name:    "Coder",
				Role:    domain.RoleCoder,
				Model:   "mimo",
				Phase:   domain.PhaseImplement,
				Enabled: true,
			},
			{
				ID:      "judge",
				Name:    "Judge",
				Role:    domain.RoleArbiter,
				Model:   "flash_5",
				Phase:   domain.PhaseJudge,
				Enabled: true,
			},
		},
	}

	if err := store.Save(ctx, customTeam); err != nil {
		t.Fatalf("failed to save team: %v", err)
	}

	// 3. Get team
	fetched, err := store.Get(ctx, "custom-pair")
	if err != nil {
		t.Fatalf("failed to fetch saved team: %v", err)
	}
	if fetched.Name != "Dynamic Duo" {
		t.Errorf("expected name Dynamic Duo, got: %s", fetched.Name)
	}
	if len(fetched.Agents) != 2 {
		t.Errorf("expected 2 agents, got: %d", len(fetched.Agents))
	}

	// 4. Set as default
	if err := store.SetDefault(ctx, "custom-pair"); err != nil {
		t.Fatalf("failed to set default: %v", err)
	}

	newDef, err := store.GetDefault(ctx)
	if err != nil {
		t.Fatalf("failed to get default team: %v", err)
	}
	if newDef.ID != "custom-pair" {
		t.Errorf("expected default team id custom-pair, got: %s", newDef.ID)
	}

	// 5. List teams
	list, err := store.List(ctx)
	if err != nil {
		t.Fatalf("failed to list teams: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 team in list, got: %d", len(list))
	}

	// 6. Delete team
	if err := store.Delete(ctx, "custom-pair"); err != nil {
		t.Fatalf("failed to delete team: %v", err)
	}

	_, err = store.Get(ctx, "custom-pair")
	if err != domain.ErrTeamNotFound {
		t.Errorf("expected ErrTeamNotFound, got: %v", err)
	}
}

func TestTeamStore_Validation(t *testing.T) {
	tempDir := t.TempDir()
	store, err := NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create team store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Missing name
	badTeam1 := &domain.Team{
		Agents: []domain.AgentSpec{
			{ID: "c", Model: "mimo", Enabled: true},
		},
	}
	if err := store.Save(ctx, badTeam1); err != domain.ErrTeamNoName {
		t.Errorf("expected ErrTeamNoName, got: %v", err)
	}

	// No enabled agents
	badTeam2 := &domain.Team{
		Name: "Empty team",
		Agents: []domain.AgentSpec{
			{ID: "c", Model: "mimo", Enabled: false},
		},
	}
	if err := store.Save(ctx, badTeam2); err != domain.ErrTeamNoAgents {
		t.Errorf("expected ErrTeamNoAgents, got: %v", err)
	}
}