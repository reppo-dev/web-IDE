package domain

import "errors"

// Team validation errors. These are returned to the API layer so the frontend
// can show a precise message instead of a generic failure.
var (
	ErrTeamNil           = errors.New("team is nil")
	ErrTeamNoName        = errors.New("team must have a name")
	ErrTeamNoAgents      = errors.New("team must have at least one enabled agent")
	ErrTeamBadThreshold  = errors.New("consensus threshold must be between 0 and 100")
	ErrTeamAgentNoID     = errors.New("every agent needs an id")
	ErrTeamDuplicateAgent = errors.New("agent ids must be unique within a team")
	ErrTeamAgentNoModel  = errors.New("every agent needs a model")
	ErrTeamNotFound      = errors.New("team not found")
	ErrTeamNoJudge       = errors.New("team has no judging agent; add one with role arbiter or phase judge")
)

// PhaseOrder gives agents a stable execution sequence regardless of the order
// the user listed them.
func PhaseOrder(p AgentPhase) int {
	switch p {
	case PhaseDesign:
		return 0
	case PhaseImplement:
		return 1
	case PhaseReview:
		return 2
	case PhaseJudge:
		return 3
	case PhaseCustom:
		return 1
	default:
		return 1
	}
}

// IsJudgeRole reports whether an agent should act as the final judge. A team
// needs at least one or there is nobody to declare consensus.
func IsJudgeRole(r AgentRole, p AgentPhase) bool {
	return r == RoleArbiter || p == PhaseJudge
}