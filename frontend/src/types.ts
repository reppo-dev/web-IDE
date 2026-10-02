export type AgentRole = 'architect' | 'coder' | 'critic' | 'security' | 'arbiter';
export type ExecutionMode = 'sequential' | 'concurrent' | 'rolebased';

export interface CodeFile {
  path: string;
  language: string;
  content: string;
  diff?: string;
}

export interface ReviewIssue {
  severity: 'critical' | 'major' | 'minor';
  file: string;
  line?: number;
  description: string;
  suggested_fix?: string;
}

export interface ScoringDimension {
  name: string;
  weight: number;
  score: number;
  explanation: string;
}

export interface ArbiterVerdict {
  round: number;
  final_score: number;
  threshold: number;
  passed: boolean;
  needs_refinement: boolean;
  dimensions: ScoringDimension[];
  feedback_to_coder: string;
  summary: string;
}

export interface AgentMessage {
  id: string;
  from: AgentRole;
  to?: AgentRole;
  round: number;
  type: string;
  content: any;
  summary?: string;
  reasoning?: string;
  timestamp: string;
}

export interface MemoryEntry {
  id: string;
  project_id: string;
  agent: AgentRole;
  type: 'decision' | 'mistake' | 'pattern' | 'preference' | 'insight';
  content: string;
  tags: string[];
  round?: number;
  confidence: number;
  created_at: string;
  updated_at?: string;
}

export interface SkillManifest {
  name: string;
  description: string;
  version: string;
  agents: AgentRole[];
  triggers: string[];
  requires?: string[];
  prompt?: string;
  enabled: boolean;
}

export interface SwarmEvent {
  type: 'agent_thinking' | 'agent_output' | 'round_started' | 'round_complete' | 'consensus_reached' | 'code_updated' | 'error';
  project_id: string;
  round: number;
  agent?: AgentRole;
  message?: AgentMessage;
  verdict?: ArbiterVerdict;
  files?: CodeFile[];
  payload?: any;
  timestamp: string;
}
