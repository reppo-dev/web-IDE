package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	ProjectBrainDir   = ".project"
	FileRequirements  = "requirements.md"
	FileArchitecture  = "architecture.md"
	FileDecisions     = "decisions.md"
	FileInvariants    = "invariants.md"
	FileTasks         = "tasks.md"
	FilePlan          = "plan.json"
	FileState         = "state.json"
	FileLedger        = "ledger.jsonl"
	DirMemory         = "memory"
	DirReports        = "reports"
)

// TaskStatus defines the state machine for project tasks (PROJECT_VISION §8, §19).
type TaskStatus string

const (
	TaskTODO            TaskStatus = "TODO"
	TaskInProgress      TaskStatus = "IN_PROGRESS"
	TaskDone            TaskStatus = "DONE"
	TaskBlocked         TaskStatus = "BLOCKED"
	TaskRework          TaskStatus = "REWORK"
	TaskWaitingForHuman TaskStatus = "WAITING_FOR_HUMAN"
)

// LedgerEntry represents a legacy record in ledger.jsonl.
type LedgerEntry struct {
	Timestamp string `json:"timestamp"`
	Event     string `json:"event"`
	Agent     string `json:"agent"`
	Details   string `json:"details"`
	Commit    string `json:"commit,omitempty"`
}

// ChangeLedgerEntry records a structured, immutable change record in ledger.jsonl (PROJECT_VISION §30).
type ChangeLedgerEntry struct {
	ID          string            `json:"id,omitempty"`
	TaskID      string            `json:"task_id,omitempty"`
	File        string            `json:"file,omitempty"`
	Agent       string            `json:"agent,omitempty"`
	Commit      string            `json:"commit,omitempty"`
	Reason      string            `json:"reason,omitempty"`
	DecisionRef string            `json:"decision_ref,omitempty"`
	Constraints []string          `json:"constraints,omitempty"`
	Tests       *TestLedgerRecord `json:"tests,omitempty"`
	Timestamp   string            `json:"timestamp"`
	Event       string            `json:"event,omitempty"`
	Details     string            `json:"details,omitempty"`
}

// TestLedgerRecord records test execution results associated with a change.
type TestLedgerRecord struct {
	Ran    []string `json:"ran,omitempty"`
	Result string   `json:"result,omitempty"`
}

// ProjectState represents the runtime state stored in state.json.
type ProjectState struct {
	ProjectID      string     `json:"project_id"`
	ActiveTaskID   string     `json:"active_task_id,omitempty"`
	TaskStatus     TaskStatus `json:"task_status,omitempty"`
	Round          int        `json:"round"`
	ReworkCount    int        `json:"rework_count"`
	MaxRework      int        `json:"max_rework"`
	LastCheckpoint string     `json:"last_checkpoint,omitempty"`
	UpdatedAt      string     `json:"updated_at"`
}

// BrainFS encapsulates single-writer operations on the physical .project/ directory.
type BrainFS struct {
	root string
	dir  string
	mu   sync.Mutex
}

// NewBrainFS initializes or attaches to a project's .project/ directory.
func NewBrainFS(root string) (*BrainFS, error) {
	if root == "" {
		return nil, fmt.Errorf("root path cannot be empty")
	}

	dir := filepath.Join(root, ProjectBrainDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create %s directory: %w", ProjectBrainDir, err)
	}

	// Sub-directories
	_ = os.MkdirAll(filepath.Join(dir, DirMemory), 0755)
	_ = os.MkdirAll(filepath.Join(dir, DirReports), 0755)

	b := &BrainFS{
		root: root,
		dir:  dir,
	}

	// Seed defaults if files do not exist
	if err := b.seedDefaults(); err != nil {
		return nil, err
	}

	return b, nil
}

// seedDefaults ensures the standard markdown and json files exist.
func (b *BrainFS) seedDefaults() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	defaults := map[string]string{
		FileRequirements: `# Project Requirements
## Overview
[PROPOSED] Defined requirements and user expectations for this project.

## Acceptance Criteria
- [ ] Working implementation passes test suites.
- [ ] No regression of existing functionality.
`,
		FileArchitecture: `# System Architecture
## High-Level Design
[DECIDED] Modular structure following clean architecture principles.

## Tech Stack & Tooling
- Core Language & Framework
- Test Runner & Linters
`,
		FileDecisions: `# Architectural Decision Records (ADRs)
> Tag convention: [DECIDED], [PROPOSED], [VERIFY]

## D-001: Project Foundation
- Status: [DECIDED]
- Decision: Ground truth maintained under .project/ with single-writer governance.
- Date: ` + time.Now().Format("2006-01-02") + `
`,
		FileInvariants: `# Non-Negotiable Project Invariants
> These rules must NEVER be violated by any agent pass.

1. [INVARIANT]: Never delete working functionality or public exported APIs without explicit instruction.
2. [INVARIANT]: All errors must be handled explicitly; do not swallow errors.
3. [INVARIANT]: Verify code with compiler/tests before declaring completion.
`,
		FileTasks: `# Task State Machine
> States: TODO | IN_PROGRESS | DONE | BLOCKED | REWORK | WAITING_FOR_HUMAN

- [TODO] T-001: Initial project workspace setup and validation.
`,
	}

	for filename, content := range defaults {
		target := filepath.Join(b.dir, filename)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := os.WriteFile(target, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", filename, err)
			}
		}
	}

	statePath := filepath.Join(b.dir, FileState)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		initialState := ProjectState{
			ProjectID:   filepath.Base(b.root),
			TaskStatus:  TaskTODO,
			Round:       1,
			ReworkCount: 0,
			MaxRework:   3,
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		data, _ := json.MarshalIndent(initialState, "", "  ")
		_ = os.WriteFile(statePath, data, 0644)
	}

	return nil
}

// AppendLedger records an append-only event into ledger.jsonl.
func (b *BrainFS) AppendLedger(event, agent, details, commit string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry := LedgerEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Event:     event,
		Agent:     agent,
		Details:   details,
		Commit:    commit,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(b.dir, FileLedger), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

// RecordChange appends an immutable change record according to PROJECT_VISION §30.
func (b *BrainFS) RecordChange(entry ChangeLedgerEntry) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("C-%04d", time.Now().UnixNano()%100000)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to encode change ledger entry: %w", err)
	}

	f, err := os.OpenFile(filepath.Join(b.dir, FileLedger), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open ledger: %w", err)
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

// ReadLedger reads all ChangeLedgerEntry lines from ledger.jsonl.
func (b *BrainFS) ReadLedger() ([]ChangeLedgerEntry, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	path := filepath.Join(b.dir, FileLedger)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ChangeLedgerEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read ledger: %w", err)
	}

	var entries []ChangeLedgerEntry
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var entry ChangeLedgerEntry
		if err := json.Unmarshal([]byte(trimmed), &entry); err == nil {
			if entry.File != "" || entry.Commit != "" || entry.Event != "" || entry.Details != "" {
				entries = append(entries, entry)
			}
		}
	}
	if entries == nil {
		entries = []ChangeLedgerEntry{}
	}
	return entries, nil
}

// RecordProjectChange is a convenience helper for appending to the project change ledger.
func RecordProjectChange(root string, entry ChangeLedgerEntry) error {
	brain, err := NewBrainFS(root)
	if err != nil {
		return err
	}
	return brain.RecordChange(entry)
}

// LoadProjectLedger is a convenience helper for reading the project change ledger.
func LoadProjectLedger(root string) ([]ChangeLedgerEntry, error) {
	brain, err := NewBrainFS(root)
	if err != nil {
		return nil, err
	}
	return brain.ReadLedger()
}


// ReadState returns the current state.json.
func (b *BrainFS) ReadState() (*ProjectState, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(b.dir, FileState))
	if err != nil {
		return nil, err
	}

	var state ProjectState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// UpdateState modifies state.json with mutex protection.
func (b *BrainFS) UpdateState(mutate func(*ProjectState)) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	statePath := filepath.Join(b.dir, FileState)
	var state ProjectState

	if data, err := os.ReadFile(statePath); err == nil {
		_ = json.Unmarshal(data, &state)
	}

	mutate(&state)
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0644)
}

// AppendDecision adds an ADR item to decisions.md.
func (b *BrainFS) AppendDecision(id, title, status, explanation string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry := fmt.Sprintf("\n## %s: %s\n- Status: [%s]\n- Decision: %s\n- Date: %s\n",
		id, title, strings.ToUpper(status), explanation, time.Now().Format("2006-01-02"))

	f, err := os.OpenFile(filepath.Join(b.dir, FileDecisions), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	return err
}

// AppendInvariant adds a non-negotiable rule to invariants.md.
func (b *BrainFS) AppendInvariant(rule string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	entry := fmt.Sprintf("- [INVARIANT]: %s\n", rule)
	f, err := os.OpenFile(filepath.Join(b.dir, FileInvariants), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(entry)
	return err
}

// ReadGroundTruth loads invariants, decisions, and architecture as unified ground-truth context.
func (b *BrainFS) ReadGroundTruth() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("\n=== PROJECT BRAIN (.project/ Ground Truth) ===\n")

	// 1. Invariants
	if invData, err := os.ReadFile(filepath.Join(b.dir, FileInvariants)); err == nil && len(invData) > 0 {
		sb.WriteString("\n--- INVARIANTS ---\n")
		sb.Write(invData)
	}

	// 2. Decisions
	if decData, err := os.ReadFile(filepath.Join(b.dir, FileDecisions)); err == nil && len(decData) > 0 {
		sb.WriteString("\n--- DECISIONS (ADRs) ---\n")
		sb.Write(decData)
	}

	// 3. Requirements
	if reqData, err := os.ReadFile(filepath.Join(b.dir, FileRequirements)); err == nil && len(reqData) > 0 {
		sb.WriteString("\n--- REQUIREMENTS ---\n")
		sb.Write(reqData)
	}

	return sb.String()
}
