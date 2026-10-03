export type AgentPhase = 'design' | 'implement' | 'review' | 'judge' | 'custom';

export type AgentRole =
  | 'architect'
  | 'coder'
  | 'critic'
  | 'security'
  | 'arbiter'
  | 'custom';

export interface AgentSpec {
  id: string;
  name: string;
  role: AgentRole;
  model: string;
  phase?: AgentPhase;
  system_prompt?: string;
  enabled: boolean;
  temperature?: number;
  /**
   * What this agent may do to the project: "read_only" (default), "write"
   * or "full" (includes running the build).
   */
  tools_access?: 'read_only' | 'write' | 'full';
  /** Cap on tool calls per agent per round. */
  max_tool_calls?: number;
}

export interface Team {
  id: string;
  name: string;
  description?: string;
  agents: AgentSpec[];
  consensus_threshold: number;
  max_rounds: number;
  /** Default tool access for the whole team. */
  tools_access?: 'read_only' | 'write' | 'full';
  /** Default tool-call budget per agent. */
  max_tool_calls?: number;
  created_at?: string;
  updated_at?: string;
}

export interface TeamsResponse {
  type: 'teams_data';
  teams: Team[];
  default_team_id: string;
}