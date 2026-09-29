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
