package agent

import (
	"github.com/nexusai/nexusai/backend/internal/domain"
)

// Running several agents at once requires more than goroutines.
//
// The fields an agent writes — CriticReview, SecurityReport, Blueprint — are
// assigned directly, without the context mutex, because a single agent has no
// reason to lock against itself. Two coders sharing one context would race on
// exactly those assignments, and the result would not be a crash so much as a
// quietly corrupted review: one agent's findings attached to another's work.
//
// So a parallel agent is not given the shared context at all. It gets its own,
// seeded from the shared state and merged back afterwards, in one place, by one
// goroutine. This is the difference between agents that happen to run at the same
// time and agents that can safely be allowed to.

// Fork returns a private context for one agent working in parallel.
//
// The fork shares what is safe to share — the memory store, the API keys, the
// lock coordinator, the model assignments — and copies what is not. Copying the
// file list matters in particular: a coder must see the project as it was when
// it started, not a set another agent is mutating underneath it.
func (s *SwarmContext) Fork(role domain.AgentRole, taskID string) *SwarmContext {
	if s == nil {
		return nil
	}

	fork := &SwarmContext{
		ProjectID:      s.ProjectID,
		Round:          s.Round,
		UserPrompt:     s.UserPrompt,
		MemStore:       s.MemStore,
		WorkspacePath:  s.WorkspacePath,
		ToolsAccess:    s.ToolsAccess,
		ToolsDisabled:  s.ToolsDisabled,
		ToolCallBudget: s.ToolCallBudget,
		ToolTimeout:    s.ToolTimeout,
		Detector:       s.Detector,
		ReviewSequentially: s.ReviewSequentially,
		// The lock coordinator is deliberately shared, not copied. Forking it
		// would give each agent its own private set of locks, and mutual
		// exclusion would be exactly zero.
		FileLocks: s.FileLocks,
	}

	// The maps are rebuilt rather than aliased. Two agents appending to one
	// shared map is a data race even if each key is distinct, and the token
	// ledger is exactly the place where concurrent writes are most likely.
	if s.AgentModels != nil {
		fork.AgentModels = make(map[domain.AgentRole]string, len(s.AgentModels))
		for k, v := range s.AgentModels {
			fork.AgentModels[k] = v
		}
	}
	if s.APIKeys != nil {
		fork.APIKeys = make(map[string]string, len(s.APIKeys))
		for k, v := range s.APIKeys {
			fork.APIKeys[k] = v
		}
	}
	if s.UsageByAgent != nil {
		fork.UsageByAgent = make(map[string]*domain.AgentUsage, len(s.UsageByAgent))
		for k, v := range s.UsageByAgent {
			copied := *v
			fork.UsageByAgent[k] = &copied
		}
	}

	// A snapshot of the project as it is right now. Deep enough that a coder
	// iterating the files cannot be disturbed by a concurrent coder appending.
	fork.CodeFiles = s.CodeFilesSnapshot()
	fork.Plan = s.PlanSnapshot()

	// The task this fork is working on, so the agent's prompt and its report
	// refer to the same thing the orchestrator meant.
	if task := s.FindTask(taskID); task != nil {
		taskCopy := *task
		taskCopy.Files = append([]string(nil), task.Files...)
		fork.ActiveTask = &taskCopy
	} else if s.ActiveTask != nil {
		activeCopy := *s.ActiveTask
		fork.ActiveTask = &activeCopy
	}

	return fork
}

// FindTask looks up a task by id without assuming the caller holds the lock.
func (s *SwarmContext) FindTask(id string) *domain.ProjectTask {
	if s == nil || id == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Plan == nil {
		return nil
	}
	for i := range s.Plan.Tasks {
		if s.Plan.Tasks[i].ID == id {
			found := s.Plan.Tasks[i]
			return &found
		}
	}
	return nil
}

// MergeResult folds one parallel agent's output back into the shared context.
//
// It runs on a single goroutine after the parallel phase finishes, which is what
// keeps the merge itself free of races. Only fields an agent genuinely produced
// are taken: merging a nil would erase whatever another agent already reported.
type ForkResult struct {
	Role    domain.AgentRole
	TaskID  string
	Message *domain.AgentMessage

	CriticReview   *domain.CriticPayload
	SecurityReport *domain.SecurityPayload
	Blueprint      *domain.BlueprintPayload

	Files     []domain.CodeFile
	Usage     map[string]*domain.AgentUsage
	HasFiles  bool
	Reasoning string
}

// MergeResult applies a fork's output to the shared context.
//
// The files case is the delicate one. A parallel agent that wrote code is
// claiming ownership of paths, and two agents claiming the same path would mean
// the lock failed. Rather than silently letting the later write win, the merge
// reports the collision so the caller can treat it as a real conflict.
func (s *SwarmContext) MergeResult(res ForkResult) []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if res.CriticReview != nil {
		s.CriticReview = res.CriticReview
	}
	if res.SecurityReport != nil {
		s.SecurityReport = res.SecurityReport
	}
	if res.Blueprint != nil {
		s.Blueprint = res.Blueprint
	}
	if res.Usage != nil {
		if s.UsageByAgent == nil {
			s.UsageByAgent = make(map[string]*domain.AgentUsage)
		}
		for name, usage := range res.Usage {
			if usage == nil {
				continue
			}
			copied := *usage
			s.UsageByAgent[name] = &copied
		}
	}

	if !res.HasFiles || len(res.Files) == 0 {
		return nil
	}

	// Index what is already there so a rewritten file replaces its previous
	// version rather than appearing twice, and so a genuine double-write by two
	// agents is detectable instead of being papered over.
	existing := make(map[string]int, len(s.CodeFiles))
	for i, f := range s.CodeFiles {
		existing[f.Path] = i
	}

	var collisions []string
	for _, incoming := range res.Files {
		if idx, exists := existing[incoming.Path]; exists {
			collisions = append(collisions, incoming.Path)
			s.CodeFiles[idx] = incoming
			continue
		}
		existing[incoming.Path] = len(s.CodeFiles)
		s.CodeFiles = append(s.CodeFiles, incoming)
	}
	return collisions
}

// TakeUsage removes and returns a fork's token ledger, so it can be merged
// separately from the rest of the result.
//
// Usage is tracked separately because it accumulates on every call and is
// therefore the one field where a fork's value is meaningful even when the agent
// produced no other output.
func (f *SwarmContext) TakeUsage() map[string]*domain.AgentUsage {
	if f == nil || f.UsageByAgent == nil {
		return nil
	}
	out := f.UsageByAgent
	f.UsageByAgent = nil
	return out
}
