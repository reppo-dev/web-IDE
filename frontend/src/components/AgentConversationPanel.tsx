import React, { useMemo } from 'react';
import { AgentConversation, TaskStreamStep } from '../types';

/**
 * One conversation per agent, each showing every task that agent worked on.
 *
 * The earlier layout was one card per task. It looked reasonable while runs were
 * serial, and fell apart the moment work overlapped: a coder handling three tasks
 * produced three cards with three headers, and reading them side by side meant
 * assembling one agent's reasoning by hand from fragments.
 *
 * Grouping by agent fixes that. Each agent has a single thread, and every line
 * names the task it belongs to, so the sequence of its reasoning survives while
 * the tasks it worked on stay legible.
 */

const AGENT_LABELS: Record<string, string> = {
  manager: 'مدیر',
  architect: 'معمار',
  coder: 'کدنویس',
  critic: 'بازبین',
  security: 'امنیت',
  arbiter: 'داور',
  tester: 'تستر',
};

const AGENT_COLORS: Record<string, string> = {
  manager: '#8b5cf6',
  architect: '#3b82f6',
  coder: '#10b981',
  critic: '#f59e0b',
  security: '#ef4444',
  arbiter: '#06b6d4',
  tester: '#22c55e',
};

const KIND_STYLES: Record<string, { icon: string; tint: string }> = {
  thought: { icon: '💭', tint: '#94a3b8' },
  tool: { icon: '🔧', tint: '#38bdf8' },
  edit: { icon: '✏️', tint: '#10b981' },
  test: { icon: '✓', tint: '#22c55e' },
  review: { icon: '⚠', tint: '#f59e0b' },
  decision: { icon: '◆', tint: '#8b5cf6' },
  lock: { icon: '🔒', tint: '#f97316' },
};

/** Parallel workers share a role, so their names are disambiguated by index. */
function agentLabel(agent: string): string {
  const match = agent.match(/^(coder|architect|critic|tester)-(\d+)$/);
  if (match) {
    return `${AGENT_LABELS[match[1]] ?? match[1]} ${match[2]}`;
  }
  return AGENT_LABELS[agent] ?? agent;
}

function agentColor(agent: string): string {
  const base = agent.replace(/-\d+$/, '');
  return AGENT_COLORS[base] ?? '#94a3b8';
}

interface AgentConversationPanelProps {
  conversations: AgentConversation[];
}

/**
 * groupStepsByAgent turns the flat per-task stream into per-agent threads.
 *
 * This lives outside the component on purpose: it is the actual logic of the
 * feature, and a grouping rule buried in render code is impossible to test and
 * easy to get subtly wrong.
 */
export function groupStepsByAgent(
  steps: TaskStreamStep[],
  titles: Record<string, string>,
  activeTaskId: string | null,
  locks: { agent: string; path: string }[],
): AgentConversation[] {
  const byAgent = new Map<string, TaskStreamStep[]>();
  const taskIdsByAgent = new Map<string, string[]>();

  // Steps arrive in the order events were emitted, which is the order the work
  // actually happened. Preserving it is what keeps an agent's reasoning
  // readable; sorting by task would undo that.
  for (const step of steps) {
    const key = step.as || step.agent;
    const existing = byAgent.get(key);
    if (existing) {
      existing.push(step);
    } else {
      byAgent.set(key, [step]);
    }

    const agentTasks = taskIdsByAgent.get(key);
    if (agentTasks && !agentTasks.includes(step.task_id)) {
      agentTasks.push(step.task_id);
    } else if (!agentTasks) {
      taskIdsByAgent.set(key, [step.task_id]);
    }
  }

  return Array.from(byAgent.entries()).map(([agent, agentSteps]) => {
    const taskIds = taskIdsByAgent.get(agent) ?? [];
    const taskTitles: Record<string, string> = {};
    for (const id of taskIds) {
      if (titles[id]) taskTitles[id] = titles[id];
    }
    return {
      agent,
      steps: agentSteps,
      taskIds,
      taskTitles,
      // Active means this agent touched the task currently in progress. Under a
      // parallel run several agents can be active at once, which is the point.
      active: agentSteps.some((s) => s.task_id === activeTaskId),
      lockedFiles: locks.filter((l) => l.agent === agent).map((l) => l.path),
    };
  });
}

export function AgentConversationPanel({ conversations }: AgentConversationPanelProps) {
  // Active agents first, then by recency of activity, so the thread the user
  // came to watch is the one at the top.
  const ordered = useMemo(() => {
    return [...conversations].sort((a, b) => {
      if (a.active !== b.active) return a.active ? -1 : 1;
      return b.steps.length - a.steps.length;
    });
  }, [conversations]);

  if (ordered.length === 0) return null;

  return (
    <div className="agent-conversations">
      {ordered.map((conv) => {
        const color = agentColor(conv.agent);
        const taskLabel =
          conv.taskIds.length === 1
            ? conv.taskTitles[conv.taskIds[0]] ?? conv.taskIds[0]
            : `${conv.taskIds.length} تسک`;

        return (
          <div className="agent-conversation" key={conv.agent}>
            <div className="agent-conversation-header">
              <span className="agent-conversation-dot" style={{ background: color }} />
              <span className="agent-conversation-name" style={{ color }}>
                {agentLabel(conv.agent)}
              </span>
              <span className="agent-conversation-tasks">{taskLabel}</span>
              <span className="agent-conversation-count">{conv.steps.length} گام</span>
              {conv.lockedFiles.length > 0 && (
                <span
                  className="agent-conversation-locks"
                  title={conv.lockedFiles.join('، ')}
                >
                  🔒 {conv.lockedFiles.length}
                </span>
              )}
              {conv.active && <span className="agent-conversation-pulse" />}
            </div>

            <div className="agent-conversation-body">
              {conv.steps.map((step, i) => {
                const style = KIND_STYLES[step.kind] ?? KIND_STYLES.thought;
                // The task label is only shown once it changes. Repeating it on
                // every line would bury the actual content.
                const prev = i > 0 ? conv.steps[i - 1] : null;
                const showTask = !prev || prev.task_id !== step.task_id;

                return (
                  <div className="agent-conversation-step" key={`${step.task_id}-${i}`}>
                    {showTask && (
                      <div className="agent-conversation-tasktag">
                        {step.task_id}
                        {conv.taskTitles[step.task_id]
                          ? ` · ${conv.taskTitles[step.task_id]}`
                          : ''}
                      </div>
                    )}
                    <div className="agent-conversation-line">
                      <span className="agent-conversation-kind" style={{ color: style.tint }}>
                        {style.icon}
                      </span>
                      <span className="agent-conversation-text">{step.text}</span>
                      {step.files && step.files.length > 0 && (
                        <span className="agent-conversation-files">
                          {step.files.slice(0, 2).join('، ')}
                          {step.files.length > 2 ? ` +${step.files.length - 2}` : ''}
                        </span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export default AgentConversationPanel;