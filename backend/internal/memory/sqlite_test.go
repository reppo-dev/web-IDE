package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

func TestSQLiteMemoryStore(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-mem-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewSQLiteMemoryStore(tempDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	projectID := "proj-demo"

	// 1. Write Project Memory
	mem1 := domain.MemoryEntry{
		ProjectID:  projectID,
		Agent:      domain.RoleArchitect,
		Type:       domain.MemoryDecision,
		Content:    "Selected Clean Architecture with repository pattern",
		Tags:       []string{"architecture", "clean-code"},
		Round:      1,
		Confidence: 0.95,
	}
	if err := store.WriteProject(ctx, mem1); err != nil {
		t.Fatalf("failed to write project memory: %v", err)
	}

	mem2 := domain.MemoryEntry{
		ProjectID:  projectID,
		Agent:      domain.RoleCritic,
		Type:       domain.MemoryMistake,
		Content:    "Flagged missing nil check in router initialization",
		Tags:       []string{"bug", "router"},
		Round:      2,
		Confidence: 0.90,
	}
	if err := store.WriteProject(ctx, mem2); err != nil {
		t.Fatalf("failed to write critic memory: %v", err)
	}

	// 2. Query Project Memory with Filter
	results, err := store.QueryProject(ctx, projectID, QueryFilter{
		Type: domain.MemoryMistake,
	})
	if err != nil {
		t.Fatalf("failed to query project memories: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 mistake memory, got %d", len(results))
	}
	if results[0].Agent != domain.RoleCritic {
		t.Errorf("expected critic agent, got %v", results[0].Agent)
	}

	// 3. Write Global Memory
	gmem := domain.MemoryEntry{
		ProjectID:  projectID,
		Type:       domain.MemoryPreference,
		Content:    "User prefers strict types and comprehensive error handling",
		Tags:       []string{"style", "preference"},
		Confidence: 1.0,
		CreatedAt:  time.Now(),
	}
	if err := store.WriteGlobal(ctx, gmem); err != nil {
		t.Fatalf("failed to write global memory: %v", err)
	}

	// 4. Query Global Memory
	gResults, err := store.QueryGlobal(ctx, QueryFilter{
		Tag: "preference",
	})
	if err != nil {
		t.Fatalf("failed to query global memories: %v", err)
	}
	if len(gResults) != 1 {
		t.Errorf("expected 1 global memory, got %d", len(gResults))
	}
	if gResults[0].Content != gmem.Content {
		t.Errorf("content mismatch in global memory: %s", gResults[0].Content)
	}

	// Verify project database file actually created
	projDBFile := filepath.Join(tempDir, "projects", projectID, "memories.db")
	if _, err := os.Stat(projDBFile); os.IsNotExist(err) {
		t.Errorf("expected project DB file to exist at %s", projDBFile)
	}
}

func TestMemoryEvidenceAndExpiration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nexus-mem-exp-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store, err := NewSQLiteMemoryStore(tempDir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	projectID := "proj-evidence-test"

	// 1. Entry with evidence
	factMem := domain.MemoryEntry{
		ProjectID:  projectID,
		Agent:      domain.RoleTester,
		Type:       domain.MemoryFact,
		Content:    "All 25 Flutter unit tests pass cleanly",
		Evidence:   "flutter test stdout: 00:00 +25: All tests passed!",
		Confidence: 1.0,
	}
	if err := store.WriteProject(ctx, factMem); err != nil {
		t.Fatalf("failed to write fact memory: %v", err)
	}

	// 2. Entry already expired (1 hour ago)
	past := time.Now().Add(-1 * time.Hour)
	expiredMem := domain.MemoryEntry{
		ProjectID:  projectID,
		Agent:      domain.RoleCoder,
		Type:       domain.MemoryProposal,
		Content:    "Temporary refactor suggestion for review",
		ExpiresAt:  &past,
		Confidence: 0.8,
	}
	if err := store.WriteProject(ctx, expiredMem); err != nil {
		t.Fatalf("failed to write expired memory: %v", err)
	}

	// 3. Entry with future expiry (1 hour later)
	future := time.Now().Add(1 * time.Hour)
	validExpMem := domain.MemoryEntry{
		ProjectID:  projectID,
		Agent:      domain.RoleSecurity,
		Type:       domain.MemoryWarning,
		Content:    "Lease lock warning for active session",
		ExpiresAt:  &future,
		Confidence: 0.9,
	}
	if err := store.WriteProject(ctx, validExpMem); err != nil {
		t.Fatalf("failed to write future expiry memory: %v", err)
	}

	// Query memories - should get factMem and validExpMem, but NOT expiredMem
	results, err := store.QueryProject(ctx, projectID, QueryFilter{})
	if err != nil {
		t.Fatalf("failed to query memories: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 non-expired memories, got %d", len(results))
	}

	foundFact := false
	for _, r := range results {
		if r.Type == domain.MemoryFact {
			foundFact = true
			if r.Evidence != factMem.Evidence {
				t.Errorf("evidence mismatch: expected %q, got %q", factMem.Evidence, r.Evidence)
			}
		}
		if r.Type == domain.MemoryProposal {
			t.Errorf("expired memory should have been excluded from query results")
		}
	}
	if !foundFact {
		t.Errorf("fact memory with evidence was not returned")
	}
}

