package orchestrator

import (
	"time"

	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// Step kinds used by the live task stream. The UI switches on these, so the set
// is deliberately small: a new kind means the frontend cannot render it yet.
const (
	StepThought  = "thought"
	StepTool     = "tool"
	StepEdit     = "edit"
	StepTest     = "test"
	StepReview   = "review"
	StepDecision = "decision"
	StepLock     = "lock"
)

// beginTaskStream opens the live card for a task.
//
// The reset flag matters on a retry. A task that failed and is being picked up
// again should show the attempt that is actually running, not the accumulated
// attempts of a run that already failed — otherwise the user cannot tell which
// work produced the code now on disk.
func (o *Orchestrator) beginTaskStream(projectID string, round int, task *domain.ProjectTask) {
	if task == nil {
		return
	}
	o.broadcast(SwarmEvent{
		Type:        EventTaskStep,
		ProjectID:   projectID,
		Round:       round,
		TaskID:      task.ID,
		TaskTitle:   task.Title,
		StreamReset: true,
		Agent:       domain.RoleManager,
		Stream: &TaskStreamStep{
			TaskID: task.ID,
			Agent:  domain.RoleManager,
			Kind:   StepDecision,
			Text:   "کار روی این تسک شروع شد: " + task.Title,
			At:     time.Now(),
		},
	})
}

// taskStep appends one entry to a task's live card.
//
// Every function here is a thin wrapper, and that is the point: the alternative
// was each call site constructing its own event, which is how the same step
// ended up rendered three different ways in three different places.
func (o *Orchestrator) taskStep(projectID string, round int, taskID string, role domain.AgentRole, kind, text string, files []string) {
	if taskID == "" {
		// Run-level chatter has no task to attach to. Dropping it here keeps the
		// contract simple: a step either belongs to a task card or is not a step.
		return
	}
	o.broadcast(SwarmEvent{
		Type:      EventTaskStep,
		ProjectID: projectID,
		Round:     round,
		TaskID:    taskID,
		Agent:     role,
		Stream: &TaskStreamStep{
			TaskID: taskID,
			Agent:  role,
			Kind:   kind,
			Text:   text,
			Files:  files,
			At:     time.Now(),
		},
	})
}

// stepThought records what an agent is about to do, using its own reasoning when
// it offered any.
//
// The reasoning is the part a user actually wants to see. An agent that says
// "I will add the null guard here" is showing its work; one that only reports
// "file edited" leaves the user unable to tell a considered change from a
// random one.
func (o *Orchestrator) stepThought(projectID string, round int, taskID string, role domain.AgentRole, reasoning, fallback string) {
	text := reasoning
	if text == "" {
		text = fallback
	}
	o.taskStep(projectID, round, taskID, role, StepThought, text, nil)
}

// stepEdit records which files an agent wrote.
func (o *Orchestrator) stepEdit(projectID string, round int, taskID string, role domain.AgentRole, summary string, files []string) {
	o.taskStep(projectID, round, taskID, role, StepEdit, summary, files)
}

// stepTest records a verification result, so the user sees the evidence a task
// was closed on rather than only being told it passed.
func (o *Orchestrator) stepTest(projectID string, round int, taskID string, summary string) {
	o.taskStep(projectID, round, taskID, domain.RoleTester, StepTest, summary, nil)
}

// stepReview records what a reviewer found.
func (o *Orchestrator) stepReview(projectID string, round int, taskID string, role domain.AgentRole, summary string) {
	o.taskStep(projectID, round, taskID, role, StepReview, summary, nil)
}

// stepLock records a file-lock event.
//
// A blocked agent is the most confusing thing a user can watch happen, so the
// reason is stated in the stream: which file, who holds it, and on which task.
// Silence there looks like a hang.
func (o *Orchestrator) stepLock(projectID string, round int, taskID string, role domain.AgentRole, text string) {
	o.taskStep(projectID, round, taskID, role, StepLock, text, nil)
}

// broadcastLocks republishes the current lock table.
//
// This is what lets the UI show who is editing where. Without it two agents can
// be working on the same project and the user has no way to see it until the
// results disagree.
func (o *Orchestrator) broadcastLocks(sCtx *agent.SwarmContext, projectID string, round int) {
	if sCtx == nil {
		return
	}
	locks := sCtx.LockedFiles()
	if locks == nil {
		locks = []workspace.FileLock{}
	}
	o.broadcast(SwarmEvent{
		Type:      EventLocksChanged,
		ProjectID: projectID,
		Round:     round,
		Locks:     locks,
	})
}
