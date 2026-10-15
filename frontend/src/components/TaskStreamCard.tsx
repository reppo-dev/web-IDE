import React, { useEffect, useRef, useState } from 'react';
import { TaskStreamStep } from '../types';

/**
 * One card per task, filling in as the work happens.
 *
 * The previous design gave every agent utterance its own chat bubble, so a
 * twenty-five task project produced a wall of bubbles the user had to scroll back
 * through to work out what had actually been done. Each task is one conversation
 * here: steps append to it, and the newest is always in view.
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

/** Per-kind accent, so a test result is readable as a test result at a glance. */
const KIND_STYLES: Record<string, { icon: string; tint: string }> = {
  thought: { icon: '💭', tint: '#94a3b8' },
  tool: { icon: '🔧', tint: '#38bdf8' },
  edit: { icon: '✏️', tint: '#10b981' },
  test: { icon: '✓', tint: '#22c55e' },
  review: { icon: '⚠', tint: '#f59e0b' },
  decision: { icon: '◆', tint: '#8b5cf6' },
  lock: { icon: '🔒', tint: '#f97316' },
};

interface TaskStreamCardProps {
  taskId: string;
  title: string;
  steps: TaskStreamStep[];
  running: boolean;
}

export function TaskStreamCard({ taskId, title, steps, running }: TaskStreamCardProps) {
  const [expanded, setExpanded] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const pinnedToBottom = useRef(true);
  const latest = steps.length > 0 ? steps[steps.length - 1] : null;

  // Follow the newest step, but stop following the moment the user scrolls up.
  // Auto-scrolling unconditionally would yank them away from something they had
  // deliberately gone back to read.
  useEffect(() => {
    const el = scrollRef.current;
    if (el && pinnedToBottom.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [steps.length, latest?.text]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    pinnedToBottom.current = distanceFromBottom < 40;
  };

  const jumpToLatest = () => {
    const el = scrollRef.current;
    if (el) {
      el.scrollTop = el.scrollHeight;
      pinnedToBottom.current = true;
    }
  };

  return (
    <div className="task-stream-card" data-task-id={taskId}>
      <button className="task-stream-header" onClick={() => setExpanded((v) => !v)}>
        <span className={`task-stream-caret ${expanded ? 'open' : ''}`}>▸</span>
        <span className="task-stream-id">{taskId}</span>
        <span className="task-stream-title">{title}</span>
        <span className="task-stream-count">{steps.length} گام</span>
        {running && <span className="task-stream-pulse" aria-label="در حال اجرا" />}
      </button>

      {expanded && (
        <div className="task-stream-body">
          <div className="task-stream-scroll" ref={scrollRef} onScroll={handleScroll}>
            {steps.length === 0 && (
              <div className="task-stream-empty">در انتظار اولین گام…</div>
            )}

            {steps.map((step, i) => {
              const style = KIND_STYLES[step.kind] ?? KIND_STYLES.thought;
              const color = AGENT_COLORS[step.agent] ?? '#94a3b8';
              return (
                <div className="task-stream-step" key={`${step.task_id}-${i}`}>
                  <span className="task-stream-dot" style={{ background: color }} />
                  <span className="task-stream-kind" style={{ color: style.tint }}>
                    {style.icon}
                  </span>
                  <span className="task-stream-agent" style={{ color }}>
                    {AGENT_LABELS[step.agent] ?? step.agent}
                  </span>
                  <span className="task-stream-text">{step.text}</span>
                  {step.files && step.files.length > 0 && (
                    <span className="task-stream-files">
                      {step.files.slice(0, 3).join('، ')}
                      {step.files.length > 3 ? ` +${step.files.length - 3}` : ''}
                    </span>
                  )}
                </div>
              );
            })}
          </div>

          {/* Only offered when the user has scrolled away, so it never covers
              content they are reading. */}
          {!pinnedToBottom.current && latest && (
            <button className="task-stream-jump" onClick={jumpToLatest}>
              ↓ جدیدترین گام
            </button>
          )}
        </div>
      )}
    </div>
  );
}

export default TaskStreamCard;
