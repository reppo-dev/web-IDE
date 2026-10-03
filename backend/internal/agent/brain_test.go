package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/memory"
)

func TestProjectBrain_BuildContext_EmptyReturnsBaseline(t *testing.T) {
	tempDir := t.TempDir()
	store, err := memory.NewSQLiteMemoryStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	brain := NewProjectBrain(store)
	ctx := context.Background()

	output := brain.BuildContext(ctx, "test-proj")
	if !strings.Contains(output, "SHARED PROJECT BRAIN") {
		t.Fatalf("expected brain header in output, got: %s", output)
	}
	if !strings.Contains(output, "INVARIANTS & PROJECT RULES") {
		t.Fatalf("expected baseline constitution in output, got: %s", output)
	}
}

func TestProjectBrain_RecordAndCategoriseEntries(t *testing.T) {
	tempDir := t.TempDir()
	store, err := memory.NewSQLiteMemoryStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	brain := NewProjectBrain(store)
	ctx := context.Background()
	proj := "ecommerce-service"

	// Record distinct categories
	_ = brain.RecordInvariant(ctx, proj, domain.RoleArchitect, "User deletion must never purge financial invoices", 1)
	_ = brain.RecordDecision(ctx, proj, domain.RoleArchitect, "PostgreSQL 16 with pgxpool used for database connection", 1)
	_ = brain.RecordContract(ctx, proj, domain.RoleArchitect, "POST /api/v1/checkout requires Idempotency-Key header", 1)
	_ = brain.RecordFact(ctx, proj, domain.RoleCoder, "Package uses Go 1.23 modules with chi router", 1)
	_ = brain.RecordWarning(ctx, proj, domain.RoleSecurity, "JWT signing secret must not use default dev keys in production", 1)

	output := brain.BuildContext(ctx, proj)

	checks := []string{
		"User deletion must never purge financial invoices",
		"PostgreSQL 16 with pgxpool",
		"POST /api/v1/checkout requires Idempotency-Key header",
		"Package uses Go 1.23 modules",
		"JWT signing secret must not use default dev keys",
	}

	for _, c := range checks {
		if !strings.Contains(output, c) {
			t.Errorf("expected brain output to contain %q, but it was missing in:\n%s", c, output)
		}
	}
}
