import type { Team } from './types/team';
export type { Team, AgentSpec, AgentPhase } from './types/team';

export type AgentRole = 'architect' | 'coder' | 'critic' | 'security' | 'arbiter' | 'manager' | 'tester';
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

export interface AgentUsage {
  agent: string;
  model?: string;
  prompt_tokens: number;
  output_tokens: number;
  total_tokens: number;
  calls_count: number;
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
  tokens?: AgentUsage;
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

export type TaskState = 'TODO' | 'IN_PROGRESS' | 'DONE' | 'BLOCKED' | 'REWORK' | 'WAITING_FOR_HUMAN' | 'CANCELLED' | 'FAILED';

export interface ProjectTask {
  id: string;
  title: string;
  acceptance?: string;
  state: TaskState;
  files?: string[];
  round?: number;
  /** How many times this task went back for rework before it was handed over. */
  rework_count?: number;
  /** Why the team could not close it, so the next step is not a guess. */
  last_error?: string;
}

export interface ProjectPlan {
  goal: string;
  tasks: ProjectTask[];
}

export interface TestLedgerRecord {
  ran?: string[];
  result?: string;
  duration?: string;
}

export interface ChangeLedgerEntry {
  id?: string;
  task_id?: string;
  file?: string;
  agent?: string;
  commit?: string;
  reason?: string;
  decision_ref?: string;
  constraints?: string[];
  tests?: TestLedgerRecord;
  timestamp: string;
  event?: string;
  details?: string;
}

export interface SwarmEvent {
  type: 'agent_thinking' | 'agent_output' | 'round_started' | 'round_complete' | 'consensus_reached' | 'code_updated' | 'error' | 'teams_data' | 'team_saved' | 'team_deleted' | 'run_started' | 'plan_updated' | 'task_step' | 'locks_changed';
  project_id: string;
  round: number;
  /** The agent that produced this event. "manager" is the orchestrator itself. */
  agent?: AgentRole | 'manager';
  message?: AgentMessage;
  verdict?: ArbiterVerdict;
  files?: CodeFile[];
  payload?: any;
  plan?: ProjectPlan;
  usage?: Record<string, AgentUsage>;
  timestamp: string;
  /** teams_data: every team the user has saved. */
  teams?: Team[];
  /** teams_data: which team runs when a project has no explicit choice. */
  default_team_id?: string;
  /** team_saved: the team that was just written. */
  team?: Team;
  /** team_deleted: the id that was removed. */
  team_id?: string;
  /** task_id: which task this event belongs to. Absent for run-level events. */
  task_id?: string;
  /** task_step: one entry appended to a task's live card. */
  stream?: TaskStreamStep;
  /** task_step: start a fresh card, because the task is being retried. */
  stream_reset?: boolean;
  /** locks_changed: the files currently held by some agent. */
  locks?: FileLockInfo[];
}

/**
 * One entry in a task's live conversation.
 *
 * A task is a single card that fills in as the work happens, rather than a run of
 * separate bubbles. `kind` decides how the row renders, which is what lets the
 * card read as work in progress instead of a wall of prose.
 */
export interface TaskStreamStep {
  task_id: string;
  agent: AgentRole | 'manager' | 'tester';
  /** thought | tool | edit | test | review | decision | lock */
  kind: 'thought' | 'tool' | 'edit' | 'test' | 'review' | 'decision' | 'lock';
  text: string;
  files?: string[];
  at: string;
}

/**
 * A file an agent currently holds.
 *
 * Shown so two agents working the same project are visible rather than merely
 * conflicting — without this the user only discovers the collision by noticing
 * that the two results disagree.
 */
export interface FileLockInfo {
  path: string;
  agent: string;
  task_id: string;
  acquired_at: string;
}
