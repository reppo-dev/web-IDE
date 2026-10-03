package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/team"
)

func setupTestTeamHandler(t *testing.T) (*TeamHandler, func()) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := team.NewSQLiteStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create team store: %v", err)
	}

	handler := NewTeamHandler(store)
	cleanup := func() {
		store.Close()
	}
	return handler, cleanup
}

func TestTeamHandler_ListAndCreate(t *testing.T) {
	handler, cleanup := setupTestTeamHandler(t)
	defer cleanup()

	// 1. Initial list should be empty
	req := httptest.NewRequest(http.MethodGet, "/api/teams", nil)
	w := httptest.NewRecorder()
	handler.HandleTeams(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var teams []*domain.Team
	if err := json.NewDecoder(w.Body).Decode(&teams); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(teams) != 0 {
		t.Errorf("expected 0 teams, got %d", len(teams))
	}

	// 2. Create team
	newTeam := domain.Team{
		ID:                 "team-alpha",
		Name:               "Alpha Trio",
		ConsensusThreshold: 80.0,
		MaxRounds:          3,
		Agents: []domain.AgentSpec{
			{ID: "arch", Name: "Architect", Role: domain.RoleArchitect, Model: "flash_5", Enabled: true},
			{ID: "coder", Name: "Coder", Role: domain.RoleCoder, Model: "mimo", Enabled: true},
			{ID: "arb", Name: "Judge", Role: domain.RoleArbiter, Model: "bunny", Enabled: true},
		},
	}
	body, _ := json.Marshal(newTeam)

	reqPost := httptest.NewRequest(http.MethodPost, "/api/teams", bytes.NewReader(body))
	wPost := httptest.NewRecorder()
	handler.HandleTeams(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("expected 200 on create, got %d: %s", wPost.Code, wPost.Body.String())
	}

	// 3. Get single team by id
	reqGet := httptest.NewRequest(http.MethodGet, "/api/teams?id=team-alpha", nil)
	wGet := httptest.NewRecorder()
	handler.HandleTeams(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d", wGet.Code)
	}
	var fetched domain.Team
	if err := json.NewDecoder(wGet.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode fetched failed: %v", err)
	}
	if fetched.Name != "Alpha Trio" {
		t.Errorf("expected Alpha Trio, got %s", fetched.Name)
	}

	// 4. Set Default
	reqDef := httptest.NewRequest(http.MethodPost, "/api/teams/default", bytes.NewReader([]byte(`{"id":"team-alpha"}`)))
	wDef := httptest.NewRecorder()
	handler.HandleDefaultTeam(wDef, reqDef)
	if wDef.Code != http.StatusOK {
		t.Fatalf("expected 200 on set default, got %d", wDef.Code)
	}

	// 5. Get Default
	reqGetDef := httptest.NewRequest(http.MethodGet, "/api/teams/default", nil)
	wGetDef := httptest.NewRecorder()
	handler.HandleDefaultTeam(wGetDef, reqGetDef)
	if wGetDef.Code != http.StatusOK {
		t.Fatalf("expected 200 on get default, got %d", wGetDef.Code)
	}
	var def domain.Team
	json.NewDecoder(wGetDef.Body).Decode(&def)
	if def.ID != "team-alpha" {
		t.Errorf("expected default id team-alpha, got %s", def.ID)
	}

	// 6. Delete team
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/teams?id=team-alpha", nil)
	wDel := httptest.NewRecorder()
	handler.HandleTeams(wDel, reqDel)
	if wDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", wDel.Code)
	}
}
