package domain

import (
	"encoding/json"
	"time"
)

// AgentPhase orders an agent within the team pipeline. Agents sharing a phase
// run concurrently; phases run in order.
type AgentPhase string

const (
	PhaseDesign    AgentPhase = "design"
	PhaseImplement AgentPhase = "implement"
	PhaseReview    AgentPhase = "review"
	PhaseJudge     AgentPhase = "judge"
	PhaseCustom    AgentPhase = "custom"
)

// AgentSpec defines one member of a user-built team. The prompt is what makes
// a custom agent distinct, so any role the user imagines can be expressed
// without new Go code.
type AgentSpec struct {
	// ID is stable within the team and used by the frontend as a React key.
	ID string `json:"id"`
	// Name is the human label shown in the UI.
	Name string `json:"name"`
	// Role selects built-in behaviour (architect, coder, critic, security,
	// arbiter) or "custom" to use Prompt verbatim.
	Role AgentRole `json:"role"`
	// Model is the model id this agent runs on.
	Model string `json:"model"`
	// Phase controls ordering. Empty means PhaseImplement.
	Phase AgentPhase `json:"phase,omitempty"`
	// SystemPrompt overrides the built-in prompt for custom agents, and augments
	// it for built-in roles.
	SystemPrompt string `json:"system_prompt,omitempty"`
	// ToolsAccess grants this agent file access: "read_only" (default),
	// "write" or "full". Leaving it empty means read-only, so a team cannot
	// modify a project by accident.
	ToolsAccess string `json:"tools_access,omitempty"`
	// MaxToolCalls caps tool use per round. Zero uses the team default.
	MaxToolCalls int `json:"max_tool_calls,omitempty"`
	// Enabled lets the user keep an agent in the team but skip it for a run.
	Enabled bool `json:"enabled"`
	// Temperature overrides the client default for more creative or stricter
	// agents. Zero means use the client default.
	Temperature float64 `json:"temperature,omitempty"`
}

// TaskState tracks one unit of project work.
//
// Without this the team scores the code it happens to have produced and declares
// victory: a clean two-file snapshot of a fifty-file project passes the arbiter.
type TaskState string

const (
	TaskTodo       TaskState = "TODO"
	TaskInProgress TaskState = "IN_PROGRESS"
	TaskDone       TaskState = "DONE"
	TaskBlocked    TaskState = "BLOCKED"
	TaskRework     TaskState = "REWORK"
	TaskWaiting    TaskState = "WAITING_FOR_HUMAN"
	// TaskCancelled and TaskFailed are terminal states from the project's point
	// of view: the swarm will not return to them. They exist so a task the user
	// dropped, or one abandoned after repeated failure, is recorded as settled
	// rather than left looking like outstanding work forever.
	TaskCancelled TaskState = "CANCELLED"
	TaskFailed    TaskState = "FAILED"
)

// Settled reports whether a task needs no further work from the swarm.
//
// DONE is the success case. WAITING_FOR_HUMAN and BLOCKED are cases the agents
// have already failed to close and a person must now decide about. CANCELLED and
// FAILED are terminal. None of the four may keep the project open, because the
// swarm cannot make progress on any of them and would otherwise loop forever.
func (t TaskState) Settled() bool {
	switch t {
	case TaskDone, TaskWaiting, TaskBlocked, TaskCancelled, TaskFailed:
		return true
	default:
		return false
	}
}

// ProjectTask is one item on the project's plan.
type ProjectTask struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Acceptance  string    `json:"acceptance,omitempty"`
	State       TaskState `json:"state"`
	// Files records what this task touched, so a later round can tell whether
	// the work was actually carried out or merely re-scored.
	Files       []string  `json:"files,omitempty"`
	Round       int       `json:"round,omitempty"`
	ReworkCount int       `json:"rework_count,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// ProjectPlan is the ordered list of work that makes the project complete.
type ProjectPlan struct {
	Goal   string        `json:"goal"`
	Prompt string        `json:"prompt,omitempty"`
	Tasks  []ProjectTask `json:"tasks"`
}

// Progress reports how much of the plan is finished and what is still open.
//
// Completion is what decides whether the round ends the whole project, so it is
// computed from the plan rather than from the quality of the current code.
func (p *ProjectPlan) Progress() (done, total int, percent float64) {
	if p == nil || len(p.Tasks) == 0 {
		return 0, 0, 0
	}
	for _, t := range p.Tasks {
		if t.State == TaskDone {
			done++
		}
	}
	total = len(p.Tasks)
	percent = float64(done) / float64(total) * 100.0
	return done, total, percent
}

// OpenTasks lists the tasks still standing, so the next round can be told what
// to work on instead of redoing whatever it just did.
func (p *ProjectPlan) OpenTasks() []ProjectTask {
	if p == nil {
		return nil
	}
	var open []ProjectTask
	for _, t := range p.Tasks {
		if t.State != TaskDone {
			open = append(open, t)
		}
	}
	return open
}

// AllDone reports whether the whole plan is finished. An empty plan is not
// complete: without a plan the system has no evidence the project is finished.
func (p *ProjectPlan) AllDone() bool {
	if p == nil || len(p.Tasks) == 0 {
		return false
	}
	// A task parked as WAITING_FOR_HUMAN or BLOCKED is not something the swarm
	// can close on its own, so it must not hold the whole run open forever.
	// Counting it as settled is what lets the rest of the project finish and
	// leaves the user a short list of what is left, instead of the loop
	// spinning on one task the agents have already failed three times.
	for _, t := range p.Tasks {
		if !t.State.Settled() {
			return false
		}
	}
	return true
}

// Completed reports whether every task actually passed verification.
//
// This is deliberately stricter than AllDone. A project can be settled with
// work still outstanding, which is a finished run; it is only "complete" when
// nothing is outstanding at all.
func (p *ProjectPlan) Completed() bool {
	if p == nil || len(p.Tasks) == 0 {
		return false
	}
	for _, t := range p.Tasks {
		if t.State != TaskDone {
			return false
		}
	}
	return true
}

// Outstanding lists the tasks the swarm did not finish, so a run that ends can
// tell the user precisely what is left instead of only saying "not done".
//
// It deliberately includes every settled-but-unfinished state. Excluding
// cancelled or failed tasks here while AllDone counted them as settled made the
// end-of-run report contradict itself: it would claim N tasks were open while
// its own tallies showed every task accounted for.
func (p *ProjectPlan) Outstanding() []ProjectTask {
	if p == nil {
		return nil
	}
	var out []ProjectTask
	for _, t := range p.Tasks {
		if t.State != TaskDone {
			out = append(out, t)
		}
	}
	return out
}

// Team is a named, reusable group of agents.
type Team struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Agents      []AgentSpec `json:"agents"`
	// ConsensusThreshold is the score the judge must reach, 0-100.
	ConsensusThreshold float64   `json:"consensus_threshold"`
	MaxRounds          int       `json:"max_rounds"`
	// ToolsAccess is the default for every agent in the team. Empty means
	// read-only.
	ToolsAccess        string    `json:"tools_access,omitempty"`
	// MaxToolCalls bounds tool use per agent per round.
	MaxToolCalls       int       `json:"max_tool_calls,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// DefaultToolCallBudget is used when a team does not set its own limit.
const DefaultToolCallBudget = 12

// AgentToolsAccess resolves the access an agent gets, preferring its own
// setting and falling back to the team default.
func (t *Team) AgentToolsAccess(a AgentSpec) string {
	if a.ToolsAccess != "" {
		return a.ToolsAccess
	}
	if t != nil && t.ToolsAccess != "" {
		return t.ToolsAccess
	}
	return "read_only"
}

// AgentToolBudget resolves the call limit for one agent.
func (t *Team) AgentToolBudget(a AgentSpec) int {
	if a.MaxToolCalls > 0 {
		return a.MaxToolCalls
	}
	if t != nil && t.MaxToolCalls > 0 {
		return t.MaxToolCalls
	}
	return DefaultToolCallBudget
}

// ActiveAgents returns only the enabled agents, preserving order.
func (t *Team) ActiveAgents() []AgentSpec {
	if t == nil {
		return nil
	}
	var out []AgentSpec
	for _, a := range t.Agents {
		if a.Enabled {
			out = append(out, a)
		}
	}
	return out
}

// Validate checks that the team can actually run.
func (t *Team) Validate() error {
	if t == nil {
		return ErrTeamNil
	}
	if t.Name == "" {
		return ErrTeamNoName
	}
	active := t.ActiveAgents()
	if len(active) == 0 {
		return ErrTeamNoAgents
	}
	if t.ConsensusThreshold < 0 || t.ConsensusThreshold > 100 {
		return ErrTeamBadThreshold
	}
	seen := make(map[string]bool)
	for _, a := range active {
		if a.ID == "" {
			return ErrTeamAgentNoID
		}
		if seen[a.ID] {
			return ErrTeamDuplicateAgent
		}
		seen[a.ID] = true
		if a.Model == "" {
			return ErrTeamAgentNoModel
		}
	}
	if t.MaxRounds <= 0 {
		t.MaxRounds = 3
	}
	return nil
}

// ToJSON serialises the team for storage.
func (t *Team) ToJSON() (string, error) {
	b, err := json.Marshal(t)
	return string(b), err
}

// TeamFromJSON parses a stored team.
func TeamFromJSON(s string) (*Team, error) {
	var t Team
	if err := json.Unmarshal([]byte(s), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// BuiltInRoles lists the personas NexusAI ships with. A team may reuse these
// or define its own via Role "custom".
var BuiltInRoles = []AgentRole{
	RoleArchitect,
	RoleCoder,
	RoleCritic,
	RoleSecurity,
	RoleArbiter,
	RoleCustom,
}