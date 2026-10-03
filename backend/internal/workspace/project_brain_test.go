package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNewBrainFS_SeedsDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("failed to create brain: %v", err)
	}

	expectedFiles := []string{
		FileRequirements,
		FileArchitecture,
		FileDecisions,
		FileInvariants,
		FileTasks,
		FileState,
	}

	for _, name := range expectedFiles {
		path := filepath.Join(tmpDir, ProjectBrainDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist under .project/", name)
		}
	}

	state, err := brain.ReadState()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.TaskStatus != TaskTODO {
		t.Errorf("expected TaskStatus %s, got %s", TaskTODO, state.TaskStatus)
	}
	if state.MaxRework != 3 {
		t.Errorf("expected MaxRework 3, got %d", state.MaxRework)
	}
}

func TestBrainFS_AppendLedger(t *testing.T) {
	tmpDir := t.TempDir()
	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("failed to create brain: %v", err)
	}

	err = brain.AppendLedger("task_started", "Architect", "Planning system files", "commit-123")
	if err != nil {
		t.Fatalf("failed to append ledger: %v", err)
	}

	ledgerPath := filepath.Join(tmpDir, ProjectBrainDir, FileLedger)
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("failed to read ledger: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "task_started") || !strings.Contains(content, "Architect") {
		t.Errorf("ledger content missing expected data: %s", content)
	}
}

func TestBrainFS_UpdateState(t *testing.T) {
	tmpDir := t.TempDir()
	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("failed to create brain: %v", err)
	}

	err = brain.UpdateState(func(s *ProjectState) {
		s.ActiveTaskID = "T-101"
		s.TaskStatus = TaskInProgress
		s.ReworkCount = 1
	})
	if err != nil {
		t.Fatalf("failed to update state: %v", err)
	}

	state, err := brain.ReadState()
	if err != nil {
		t.Fatalf("failed to read state: %v", err)
	}
	if state.ActiveTaskID != "T-101" || state.TaskStatus != TaskInProgress || state.ReworkCount != 1 {
		t.Errorf("state did not update properly: %+v", state)
	}
}

func TestBrainFS_AppendDecisionAndInvariant(t *testing.T) {
	tmpDir := t.TempDir()
	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("failed to create brain: %v", err)
	}

	_ = brain.AppendDecision("D-002", "Cache Layer", "DECIDED", "Use in-memory sync.Map")
	_ = brain.AppendInvariant("No external network calls in unit tests")

	gt := brain.ReadGroundTruth()
	if !strings.Contains(gt, "D-002") || !strings.Contains(gt, "Cache Layer") {
		t.Errorf("ground truth missing decision: %s", gt)
	}
	if !strings.Contains(gt, "No external network calls in unit tests") {
		t.Errorf("ground truth missing invariant: %s", gt)
	}
}

func TestBrainFS_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	brain, err := NewBrainFS(tmpDir)
	if err != nil {
		t.Fatalf("failed to create brain: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = brain.AppendLedger("audit", "Security", "Checked injection", "")
			_ = brain.UpdateState(func(s *ProjectState) {
				s.Round = idx
			})
		}(i)
	}
	wg.Wait()

	state, err := brain.ReadState()
	if err != nil {
		t.Fatalf("read state failed after concurrent writes: %v", err)
	}
	if state.UpdatedAt == "" {
		t.Errorf("state UpdatedAt is empty")
	}
}
