package domain

import (
	"encoding/json"
	"time"
)

// AgentRole defines the role of an AI agent in the swarm.
type AgentRole string

const (
	RoleArchitect AgentRole = "architect"
	RoleCoder     AgentRole = "coder"
	RoleCritic    AgentRole = "critic"
	RoleSecurity  AgentRole = "security"
	RoleArbiter   AgentRole = "arbiter"
	// RoleManager is the orchestrator when it speaks as an agent: opening a task,
	// handing work out, or reporting a blocked file. PROJECT_VISION §10 lists
	// Manager/Orchestrator as a role, but it had no constant — so everything the
	// orchestrator wanted to say had to be mislabelled as some other agent's
	// output, which made the log read as though the coder had decided policy.
	RoleManager AgentRole = "manager"
	// RoleTester is the agent that produces verification evidence. It is distinct
	// from the critic, who reads the code: conflating them let a review opinion
	// stand in for a passing test, which is the exact gap the verification gate
	// was built to close.
	RoleTester AgentRole = "tester"
	// RoleCustom marks an agent whose behaviour comes entirely from the user's
	// prompt. It carries no built-in persona, which is what lets a team grow
	// beyond the shipped roles without new Go code.
	RoleCustom AgentRole = "custom"
)

// ExecutionMode defines how the swarm coordinates agent operations.
type ExecutionMode string

const (
	ModeSequential ExecutionMode = "sequential"
	ModeConcurrent ExecutionMode = "concurrent"
	ModeRoleBased  ExecutionMode = "rolebased"
)

// MessageType classifies the payload sent between agents.
type MessageType string

const (
	MsgBlueprint         MessageType = "blueprint"
	MsgCodeDraft         MessageType = "code_draft"
	MsgCriticReview      MessageType = "critic_review"
	MsgSecurityReport    MessageType = "security_report"
	MsgArbiterVerdict    MessageType = "arbiter_verdict"
	MsgFeedback          MessageType = "feedback"
	MsgInterAgentDebate  MessageType = "inter_agent_debate"
	MsgAgentReport       MessageType = "agent_report"
)

// AgentMessage represents an event or message exchanged inside the swarm.
type AgentMessage struct {
	ID        string          `json:"id"`
	From      AgentRole       `json:"from"`
	To        AgentRole       `json:"to,omitempty"` // Empty means broadcast to all
	Round     int             `json:"round"`
	Type      MessageType     `json:"type"`
	Content   json.RawMessage `json:"content"`
	Summary   string          `json:"summary,omitempty"`
	Reasoning string          `json:"reasoning,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Tokens    *AgentUsage     `json:"tokens,omitempty"`
}

// MemoryType classifies a persistent memory item.
type MemoryType string

const (
	MemoryDecision   MemoryType = "decision"
	MemoryMistake    MemoryType = "mistake"
	MemoryPattern    MemoryType = "pattern"
	MemoryPreference MemoryType = "preference"
	MemoryInsight    MemoryType = "insight"
	MemoryInvariant  MemoryType = "invariant"
	MemoryFact       MemoryType = "fact"
	MemoryWarning     MemoryType = "warning"
	MemoryProposal   MemoryType = "proposal"
	MemoryContract   MemoryType = "contract"
)

// MemoryEntry represents a persistent Mem0-style memory block.
type MemoryEntry struct {
	ID         string     `json:"id"`
	ProjectID  string     `json:"project_id"`
	Agent      AgentRole  `json:"agent"`
	Type       MemoryType `json:"type"`
	Content    string     `json:"content"`
	Tags       []string   `json:"tags"`
	Round      int        `json:"round,omitempty"`
	Confidence float64    `json:"confidence"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// CodeFile represents a file generated or modified by the swarm.
type CodeFile struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Content  string `json:"content"`
	Diff     string `json:"diff,omitempty"`
}

// BlueprintPayload represents the architect's decomposition.
type BlueprintPayload struct {
	ProjectOverview string     `json:"project_overview"`
	FileTree        []string   `json:"file_tree"`
	KeyComponents   []string   `json:"key_components"`
	APIContracts    []string   `json:"api_contracts"`
	Files           []CodeFile `json:"files,omitempty"`
}

// IntentKind classifies what the user actually asked for, so a greeting or a
// question is answered conversationally instead of triggering a build.
type IntentKind string

const (
	// IntentConversation covers greetings, thanks and small talk.
	IntentConversation IntentKind = "conversation"
	// IntentQuestion covers information requests that need no code changes.
	IntentQuestion IntentKind = "question"
	// IntentBuild covers requests to create or modify software.
	IntentBuild IntentKind = "build"
)

// IntentRoute is the architect's triage decision for a user message.
type IntentRoute struct {
	Kind     IntentKind `json:"kind"`
	Reply    string     `json:"reply"`
	Reason   string     `json:"reason"`
	Reworded string     `json:"reworded,omitempty"`
}

// ReviewIssue represents a bug or edge-case flagged by the Critic or Security auditor.
type ReviewIssue struct {
	Severity    string `json:"severity"` // "critical", "major", "minor"
	File        string `json:"file"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
}

// CriticPayload contains the code review findings.
type CriticPayload struct {
	GeneralReview string        `json:"general_review"`
	Issues        []ReviewIssue `json:"issues"`
	RefactorPlan  string        `json:"refactor_plan"`
}

// SecurityPayload contains the vulnerability assessment.
type SecurityPayload struct {
	Vulnerabilities []ReviewIssue `json:"vulnerabilities"`
	AttackSurface   string        `json:"attack_surface"`
	IsClean         bool          `json:"is_clean"`
}

// ScoringDimension represents one of the 5 weighted quality dimensions.
type ScoringDimension struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Score       float64 `json:"score"` // 0.0 to 100.0
	Explanation string  `json:"explanation"`
}

// ArbiterVerdict contains the round judgment and consensus decision.
type ArbiterVerdict struct {
	Round           int                `json:"round"`
	FinalScore      float64            `json:"final_score"` // 0.0 to 100.0
	Threshold       float64            `json:"threshold"`   // default 85.0
	Passed          bool               `json:"passed"`
	NeedsRefinement bool               `json:"needs_refinement"`
	Dimensions      []ScoringDimension `json:"dimensions"`
	FeedbackToCoder string             `json:"feedback_to_coder"`
	Summary         string             `json:"summary"`
}

// SwarmStatus tracks current execution state of the IDE swarm.
type SwarmStatus struct {
	ProjectID        string        `json:"project_id"`
	Mode             ExecutionMode `json:"mode"`
	ActiveRound      int           `json:"active_round"`
	MaxRounds        int           `json:"max_rounds"`
	ActiveAgents     []AgentRole   `json:"active_agents"`
	ConsensusReached bool          `json:"consensus_reached"`
	CurrentScore     float64       `json:"current_score"`
}

// AgentUsage records token consumption and invocation counts per agent (PROJECT_VISION §37).
type AgentUsage struct {
	Agent        string `json:"agent"`
	Model        string `json:"model,omitempty"`
	PromptTokens int    `json:"prompt_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
	CallsCount   int    `json:"calls_count"`
}

