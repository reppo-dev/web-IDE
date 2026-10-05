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

func TestProjectBrain_FactWithoutEvidenceDowngradesToAssumption(t *testing.T) {
	tempDir := t.TempDir()
	store, err := memory.NewSQLiteMemoryStore(tempDir)
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	brain := NewProjectBrain(store)
	ctx := context.Background()
	proj := "strict-evidence-project"

	// 1. Fact WITH evidence -> Must stay domain.MemoryFact
	err = brain.RecordFact(ctx, proj, domain.RoleTester, "All tests pass", 1, "go test stdout: PASS 100%")
	if err != nil {
		t.Fatalf("failed to record fact with evidence: %v", err)
	}

	// 2. Fact WITHOUT evidence -> Must be downgraded to domain.MemoryAssumption (PROJECT_VISION §10)
	err = brain.RecordFact(ctx, proj, domain.RoleCoder, "Service handles 100k requests/sec", 1) // no evidence provided
	if err != nil {
		t.Fatalf("failed to record fact without evidence: %v", err)
	}

	// Query store to inspect underlying stored types
	entries, err := store.QueryProject(ctx, proj, memory.QueryFilter{})
	if err != nil {
		t.Fatalf("failed to query entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	var foundFact, foundAssumption bool
	for _, e := range entries {
		if e.Content == "All tests pass" {
			foundFact = true
			if e.Type != domain.MemoryFact {
				t.Errorf("expected MemoryFact for entry with evidence, got %v", e.Type)
			}
			if e.Evidence != "go test stdout: PASS 100%" {
				t.Errorf("expected evidence to be preserved, got %q", e.Evidence)
			}
		}
		if e.Content == "Service handles 100k requests/sec" {
			foundAssumption = true
			if e.Type != domain.MemoryAssumption {
				t.Errorf("expected MemoryAssumption for fact lacking evidence (PROJECT_VISION §10), got %v", e.Type)
			}
		}
	}
	if !foundFact || !foundAssumption {
		t.Fatalf("missing expected entries in store (foundFact: %v, foundAssumption: %v)", foundFact, foundAssumption)
	}

	// Build context and check headers
	contextStr := brain.BuildContext(ctx, proj)
	if !strings.Contains(contextStr, "--- VERIFIED FACTS (With Concrete Evidence) ---") {
		t.Errorf("expected verified facts header in context:\n%s", contextStr)
	}
	if !strings.Contains(contextStr, "--- UNVERIFIED ASSUMPTIONS (Need Evidence / Verification) ---") {
		t.Errorf("expected unverified assumptions header in context:\n%s", contextStr)
	}
}

