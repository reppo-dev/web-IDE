import React, { useState } from 'react';
import { 
  Send, 
  Sparkles, 
  Bot, 
  Code2, 
  Shield, 
  Scale, 
  ChevronDown, 
  ChevronUp,
  Check, 
  RotateCcw, 
  Users,
  Wand2,
  CheckCircle2,
  ArrowRight,
  MessageSquare,
  FileCode,
  Copy,
  CheckCheck,
  WifiOff,
  AlertTriangle,
  Settings2,
  ListTodo,
  Clock,
  AlertCircle
} from 'lucide-react';
import { AgentMessage, ArbiterVerdict, ExecutionMode, AgentRole, ProjectPlan } from '../../types';
import { AVAILABLE_MODELS, SWARM_PRESETS, SwarmPreset } from '../../config/models';
import { Team } from '../../types/team';

interface ComposerPanelProps {
  messages: AgentMessage[];
  isRunning: boolean;
  verdict: ArbiterVerdict | null;
  executionMode: ExecutionMode;
  onSetExecutionMode: (mode: ExecutionMode) => void;
  agentModels: Record<string, string>;
  onUpdateAgentModel: (role: string, model: string) => void;
  onApplyPreset: (preset: SwarmPreset) => void;
  onSendPrompt: (prompt: string) => void;
  onAcceptChanges?: () => void;
  thinkingAgent?: { role: string; status: string } | null;
  /** True when no swarm event has arrived for several minutes. */
  stalled?: boolean;
  /** False when the daemon WebSocket is not connected. */
  connected?: boolean;
  /** User-built teams available for this project. */
  teams?: Team[];
  /** Which team runs when the user sends a prompt. */
  selectedTeamId?: string;
  onSelectTeam?: (teamId: string) => void;
  onOpenTeamBuilder?: () => void;
  /** Active project plan and tasks. */
  plan?: ProjectPlan | null;
}

// Helper to render simple markdown cleanly without heavy external libs
const SimpleMarkdown: React.FC<{ text: string }> = ({ text }) => {
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  if (!text) return null;

  const copyToClipboard = (code: string, idx: number) => {
    navigator.clipboard.writeText(code);
    setCopiedIndex(idx);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  // Split by code blocks
  const parts = text.split(/(```[\s\S]*?```)/g);

  return (
    <div className="space-y-2 text-zinc-200 leading-relaxed text-xs break-words font-sans" dir="auto">
      {parts.map((part, pIdx) => {
        if (part.startsWith('```')) {
          const lines = part.slice(3, -3).trim().split('\n');
          const firstLine = lines[0].trim();
          const hasLang = !firstLine.includes(' ') && firstLine.length < 15;
          const lang = hasLang ? firstLine : '';
          const codeContent = hasLang ? lines.slice(1).join('\n') : lines.join('\n');

          return (
            <div key={pIdx} className="my-2 rounded-lg bg-[#111111] border border-zinc-800 overflow-hidden font-mono text-[11px] text-left" dir="ltr">
              <div className="flex items-center justify-between px-3 py-1 bg-[#181818] border-b border-zinc-800 text-[10px] text-zinc-400">
                <span>{lang || 'code'}</span>
                <button
                  type="button"
                  onClick={() => copyToClipboard(codeContent, pIdx)}
                  className="flex items-center gap-1 hover:text-zinc-200 transition-colors"
                >
                  {copiedIndex === pIdx ? (
                    <>
                      <CheckCheck size={11} className="text-emerald-400" />
                      <span className="text-emerald-400">کپی شد</span>
                    </>
                  ) : (
                    <>
                      <Copy size={11} />
                      <span>کپی</span>
                    </>
                  )}
                </button>
              </div>
              <pre className="p-3 overflow-x-auto text-zinc-300 whitespace-pre">
                <code>{codeContent}</code>
              </pre>
            </div>
          );
        }

        // Standard paragraphs / lines
        return (
          <div key={pIdx} className="space-y-1.5 whitespace-pre-wrap">
            {part.split('\n\n').map((paragraph, paraIdx) => {
              const trimmed = paragraph.trim();
              if (!trimmed) return null;

              if (trimmed.startsWith('### ')) {
                return (
                  <h4 key={paraIdx} className="text-sm font-bold text-zinc-100 mt-2 mb-1 flex items-center gap-1">
                    {trimmed.replace('### ', '')}
                  </h4>
                );
              }
              if (trimmed.startsWith('## ')) {
                return (
                  <h3 key={paraIdx} className="text-sm font-bold text-indigo-300 mt-2 mb-1">
                    {trimmed.replace('## ', '')}
                  </h3>
                );
              }
              if (trimmed.startsWith('# ')) {
                return (
                  <h2 key={paraIdx} className="text-base font-extrabold text-indigo-200 mt-3 mb-1">
                    {trimmed.replace('# ', '')}
                  </h2>
                );
              }

              return (
                <p key={paraIdx} className="leading-relaxed">
                  {trimmed}
                </p>
              );
            })}
          </div>
        );
      })}
    </div>
  );
};

export const ComposerPanel: React.FC<ComposerPanelProps> = ({
  messages,
  isRunning,
  verdict,
  executionMode,
  onSetExecutionMode,
  agentModels,
  onUpdateAgentModel,
  onApplyPreset,
  onSendPrompt,
  onAcceptChanges,
  thinkingAgent,
  stalled,
  connected,
  teams,
  selectedTeamId,
  onSelectTeam,
  onOpenTeamBuilder,
  plan,
}) => {
  const [prompt, setPrompt] = useState('');
  const [showRoster, setShowRoster] = useState(false);
  const [showPlanBoard, setShowPlanBoard] = useState(true);
  const [customModelInputs, setCustomModelInputs] = useState<Record<string, string>>({});
  // Track open/collapsed deliberation accordions by turn ID
  const [collapsedTurns, setCollapsedTurns] = useState<Record<string, boolean>>({});

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!prompt.trim() || isRunning) return;
    onSendPrompt(prompt);
    setPrompt('');
  };

  const getAgentTheme = (role: AgentRole | string) => {
    switch (role) {
      case 'architect':
        return { 
          label: 'Architect', 
          faLabel: 'معمار سیستم', 
          stepTitle: 'گام ۱: طرح معماری و تحلیل اولیه',
          replyPrefix: 'طرح معماری و چارچوب اولیه:',
          color: 'text-blue-400 bg-blue-950/30 border-blue-800/40', 
          dotColor: 'bg-blue-400 shadow-blue-500/50',
          icon: Bot 
        };
      case 'coder':
        return { 
          label: 'Lead Coder', 
          faLabel: 'برنامه‌نویس ارشد', 
          stepTitle: 'گام ۲: پاسخ به معمار • ارزیابی فنی و کد',
          replyPrefix: 'در پاسخ به معمار (بررسی فنی و کد):',
          color: 'text-purple-400 bg-purple-950/30 border-purple-800/40', 
          dotColor: 'bg-purple-400 shadow-purple-500/50',
          icon: Code2 
        };
      case 'critic':
        return { 
          label: 'Critic Red-Team', 
          faLabel: 'منتقد بی‌رحم', 
          stepTitle: 'گام ۳: نقد تیمی • چالش فرضیات و اشکالات',
          replyPrefix: 'در پاسخ به تیم (نقد و ارزیابی موشکافانه):',
          color: 'text-rose-400 bg-rose-950/30 border-rose-800/40', 
          dotColor: 'bg-rose-400 shadow-rose-500/50',
          icon: Sparkles 
        };
      case 'security':
        return { 
          label: 'Security Auditor', 
          faLabel: 'ممیز امنیتی', 
          stepTitle: 'گام ۴: ممیزی امنیت • پایداری و تاب‌آوری',
          replyPrefix: 'در پاسخ به تیم (ممیزی امنیتی و داده‌ها):',
          color: 'text-emerald-400 bg-emerald-950/30 border-emerald-800/40', 
          dotColor: 'bg-emerald-400 shadow-emerald-500/50',
          icon: Shield 
        };
      case 'arbiter':
        return { 
          label: 'Arbiter Judge', 
          faLabel: 'قاضی نهایی اجماع', 
          stepTitle: 'گام ۵: داوری نهایی • حل تعارض و جمع‌بندی',
          replyPrefix: 'جمع‌بندی و داوری نهایی:',
          color: 'text-amber-400 bg-amber-950/30 border-amber-800/40', 
          dotColor: 'bg-amber-400 shadow-amber-500/50',
          icon: Scale 
        };
      default:
        return { 
          label: role, 
          faLabel: role, 
          stepTitle: `بررسی ${role}`,
          replyPrefix: 'دیدگاه این نقش:',
          color: 'text-zinc-400 bg-zinc-900 border-zinc-800', 
          dotColor: 'bg-zinc-400',
          icon: Bot 
        };
    }
  };

  const getModelDisplayName = (modelId: string) => {
    if (!modelId) return 'Default Model';
    const found = AVAILABLE_MODELS.find(m => m.id === modelId);
    if (found) return found.name;
    const parts = modelId.split('/');
    return parts[parts.length - 1];
  };

  const roles: AgentRole[] = ['architect', 'coder', 'critic', 'security', 'arbiter'];

  // Group messages into conversational turns:
  // Each turn contains:
  // 1. userPrompt (if any)
  // 2. deliberationSteps (intermediate thinking/dialogue from agents)
  // 3. finalDecision (the overarching synthesis / decision from Arbiter or last agent)
  interface Turn {
    id: string;
    userMessage?: AgentMessage;
    deliberationSteps: AgentMessage[];
    finalDecision?: AgentMessage;
    isFinished: boolean;
  }

  const turns: Turn[] = [];
  let currentTurn: Turn | null = null;

  for (let idx = 0; idx < messages.length; idx++) {
    const m = messages[idx];
    if (m.type === 'user_prompt') {
      if (currentTurn) {
        turns.push(currentTurn);
      }
      currentTurn = {
        id: `turn-${m.id || idx}`,
        userMessage: m,
        deliberationSteps: [],
        finalDecision: undefined,
        isFinished: false,
      };
    } else {
      if (!currentTurn) {
        currentTurn = {
          id: `turn-init-${idx}`,
          userMessage: undefined,
          deliberationSteps: [],
          finalDecision: undefined,
          isFinished: false,
        };
      }

      // Check if message is the final arbiter synthesis or consensus message
      if (m.from === 'arbiter') {
        currentTurn.deliberationSteps.push(m);
        currentTurn.finalDecision = m;
        currentTurn.isFinished = true;
      } else {
        currentTurn.deliberationSteps.push(m);
      }
    }
  }

  if (currentTurn) {
    const lastTurn: Turn = currentTurn;
    if (!isRunning && lastTurn.deliberationSteps.length > 0) {
      lastTurn.isFinished = true;
      // If no arbiter message arrived, pick the last agent message as the final output
      if (!lastTurn.finalDecision) {
        lastTurn.finalDecision = lastTurn.deliberationSteps[lastTurn.deliberationSteps.length - 1];
      }
    }
    turns.push(lastTurn);
  }

  const toggleTurnAccordion = (turnId: string, currentlyCollapsed: boolean) => {
    setCollapsedTurns(prev => ({
      ...prev,
      [turnId]: !currentlyCollapsed
    }));
  };

  return (
    <aside className="w-80 md:w-96 bg-[#181818] border-l border-[#242424] flex flex-col h-full select-text z-20 shrink-0 text-xs">
      {/* Top Header: Composer Title, Mode Pill, and Swarm Team Roster Toggle */}
      <div className="p-3 border-b border-[#242424] space-y-2 bg-[#161616]">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5 font-bold text-zinc-100 text-xs">
            <Sparkles size={14} className="text-indigo-400" />
            <span>NexusAI Composer</span>
          </div>

          {/* Mode Pill */}
          <select
            value={executionMode}
            onChange={(e) => onSetExecutionMode(e.target.value as ExecutionMode)}
            className="bg-[#202020] text-indigo-300 font-medium text-[11px] px-2 py-0.5 rounded border border-[#303030] outline-none cursor-pointer"
          >
            <option value="rolebased">Role-Based (Phased Relay)</option>
            <option value="concurrent">Concurrent (Channels)</option>
            <option value="sequential">Sequential (Linear)</option>
          </select>
        </div>

        {/* Connection / stall warnings: without these the composer looks
            identical whether the daemon is live, unreachable, or wedged. */}
        {connected === false && (
          <div className="flex items-start gap-1.5 text-[10px] bg-rose-950/40 border border-rose-900/60 text-rose-300 px-2 py-1.5 rounded">
            <WifiOff size={12} className="mt-px shrink-0" />
            <span>
              دیمون <span className="font-mono">nexusd</span> روی پورت ۸۰۸۰ در دسترس نیست.
              پیام‌ها ارسال نمی‌شوند. با <span className="font-mono">run-dev.ps1</span> دیمون را اجرا کنید.
            </span>
          </div>
        )}

        {stalled && connected !== false && (
          <div className="flex items-start gap-1.5 text-[10px] bg-amber-950/40 border border-amber-900/60 text-amber-300 px-2 py-1.5 rounded">
            <AlertTriangle size={12} className="mt-px shrink-0" />
            <span>
              بیش از ۳ دقیقه از swarm خبری نرسیده. ممکن است دیمون در حال اجرای یک تسک
              طولانی باشد. اگر تا ۱۵ دقیقه ادامه داشت، دیمون را بررسی کنید.
            </span>
          </div>
        )}
        {/* Team picker: the user chooses which saved team runs this project. */}
        {onSelectTeam && (
          <div className="flex items-center gap-1.5">
            <select
              value={selectedTeamId || 'default'}
              onChange={(e) => onSelectTeam(e.target.value)}
              className="flex-1 bg-[#1e1e1e] text-zinc-200 text-[11px] px-2 py-1.5 rounded border border-[#2a2a2a] outline-none cursor-pointer"
            >
              <option value="default">
                تیم پیش‌فرض ۵ نفره
              </option>
              {(teams || [])
                .filter((t) => t.id !== 'default')
                .map((t) => (
                  <option key={t.id} value={t.id}>
                    {t.name} ({t.agents.length} ایجنت)
                  </option>
                ))}
            </select>
            {onOpenTeamBuilder && (
              <button
                onClick={onOpenTeamBuilder}
                className="p-1.5 bg-[#1e1e1e] hover:bg-[#252525] text-zinc-400 hover:text-zinc-200 rounded border border-[#2a2a2a]"
                title="ساخت تیم دلخواه"
              >
                <Settings2 size={13} />
              </button>
            )}
          </div>
        )}

        <button
          onClick={() => setShowRoster(!showRoster)}
          className="w-full flex items-center justify-between bg-[#1e1e1e] hover:bg-[#252525] px-2.5 py-1.5 rounded border border-[#2a2a2a] transition-colors text-left"
        >
          <div className="flex items-center gap-1.5 text-zinc-300 font-medium text-[11px]">
            <Users size={13} className="text-indigo-400" />
            <span>تیم هوش مصنوعی (۵ مدل همزمان)</span>
          </div>
          <div className="flex items-center gap-1 text-zinc-400 text-[10px]">
            <span className="text-zinc-500">تنظیم مدل‌ها</span>
            {showRoster ? <ChevronUp size={13} /> : <ChevronDown size={13} />}
          </div>
        </button>

        {/* Collapsible Swarm Team Roster Config Panel */}
        {showRoster && (
          <div className="bg-[#141414] border border-[#2a2a2a] rounded-lg p-2.5 space-y-2.5 animate-in fade-in duration-200">
            {/* Quick Presets */}
            <div className="space-y-1">
              <span className="text-[10px] text-zinc-400 font-semibold block">الگوهای آماده تیم (Quick Presets):</span>
              <div className="grid grid-cols-2 gap-1">
                {SWARM_PRESETS.map((p) => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => onApplyPreset(p)}
                    className="p-1 rounded bg-[#202020] hover:bg-indigo-950/40 hover:border-indigo-700/60 border border-[#2d2d2d] text-[10px] text-zinc-300 text-left transition-colors truncate"
                    title={p.description}
                  >
                    {p.nameFa}
                  </button>
                ))}
              </div>
            </div>

            {/* Individual Agent Selectors */}
            <div className="space-y-1.5 pt-1 border-t border-[#222222]">
              <span className="text-[10px] text-zinc-400 font-semibold block">مدل اختصاصی هر عضو تیم:</span>
              {roles.map((role) => {
                const theme = getAgentTheme(role);
                const currentModel = agentModels[role] || '';
                const isCustom = !AVAILABLE_MODELS.some(m => m.id === currentModel);

                return (
                  <div key={role} className="space-y-0.5">
                    <div className="flex items-center justify-between text-[10px]">
                      <span className="text-zinc-300 font-medium flex items-center gap-1">
                        <span className={`w-1.5 h-1.5 rounded-full ${
                          role === 'architect' ? 'bg-blue-400' :
                          role === 'coder' ? 'bg-purple-400' :
                          role === 'critic' ? 'bg-rose-400' :
                          role === 'security' ? 'bg-emerald-400' : 'bg-amber-400'
                        }`} />
                        {theme.label} ({theme.faLabel})
                      </span>
                    </div>

                    <select
                      value={isCustom ? 'custom' : currentModel}
                      onChange={(e) => {
                        const val = e.target.value;
                        if (val === 'custom') {
                          onUpdateAgentModel(role, customModelInputs[role] || '');
                        } else {
                          onUpdateAgentModel(role, val);
                        }
                      }}
                      className="w-full bg-[#1e1e1e] border border-[#2d2d2d] text-zinc-200 text-[10px] rounded px-1.5 py-1 outline-none cursor-pointer"
                    >
                      <option value="" disabled className="text-zinc-500">
                        -- انتخاب مدل الزامی است --
                      </option>
                      {AVAILABLE_MODELS.map((m) => (
                        <option key={m.id} value={m.id} className="bg-zinc-900 text-zinc-200">
                          {m.name} {m.isFree ? '⚡ رایگان' : ''}
                        </option>
                      ))}
                      <option value="custom" className="bg-zinc-900 text-indigo-300">
                        ✍️ مدل سفارشی دیگر...
                      </option>
                    </select>

                    {isCustom && (
                      <input
                        type="text"
                        placeholder="نام مدل اوپن‌روتر، مثلا: meta-llama/llama-3.1-405b-instruct"
                        value={currentModel}
                        onChange={(e) => {
                          const val = e.target.value;
                          setCustomModelInputs(prev => ({ ...prev, [role]: val }));
                          onUpdateAgentModel(role, val);
                        }}
                        className="w-full bg-[#191919] border border-indigo-700/50 text-indigo-200 text-[10px] rounded px-1.5 py-0.5 outline-none font-mono"
                      />
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>

      {/* Consensus Banner if Verdict Reached */}
      {verdict && (
        <div className={`px-3 py-2 border-b flex items-center justify-between ${
          verdict.passed 
            ? 'bg-emerald-950/20 border-emerald-900/30 text-emerald-300' 
            : 'bg-amber-950/20 border-amber-900/30 text-amber-300'
        }`}>
          <div className="flex items-center gap-1.5 text-[11px] font-semibold">
            <Scale size={13} />
            <span>کیفیت اجماع: {verdict.final_score.toFixed(1)}%</span>
            <span className="text-[10px] font-normal text-zinc-400">({verdict.passed ? 'تایید شد >= ۸۵٪' : 'در حال اصلاح'})</span>
          </div>
          {onAcceptChanges && verdict.passed && (
            <button
              onClick={onAcceptChanges}
              className="flex items-center gap-1 px-2 py-0.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-[10px] font-semibold transition-colors"
            >
              <Check size={11} />
              <span>پذیرش تغییرات</span>
            </button>
          )}
        </div>
      )}

      {/* Project Plan Task Tracker Board */}
      {plan && plan.tasks && plan.tasks.length > 0 && (() => {
        const planTasks = plan.tasks;
        const doneTasks = planTasks.filter((t) => t.state === 'DONE').length;
        const planPercent = Math.round((doneTasks / planTasks.length) * 100);

        return (
          <div className="border-b border-[#222222] bg-[#141414]/90 px-3 py-2 space-y-1.5 transition-all">
            <div className="flex items-center justify-between text-[11px]">
              <button 
                type="button" 
                onClick={() => setShowPlanBoard(!showPlanBoard)}
                className="flex items-center gap-1.5 font-semibold text-zinc-200 hover:text-white transition-colors"
              >
                <ListTodo size={13} className="text-indigo-400" />
                <span>برنامه و تسک‌های پروژه</span>
                <span className="text-[10px] text-zinc-400 font-normal">
                  ({doneTasks}/{planTasks.length} انجام شده — {planPercent}٪)
                </span>
                {showPlanBoard ? <ChevronUp size={12} className="text-zinc-500" /> : <ChevronDown size={12} className="text-zinc-500" />}
              </button>
              <div className="w-20 bg-zinc-800 h-1.5 rounded-full overflow-hidden">
                <div 
                  className="bg-indigo-500 h-full rounded-full transition-all duration-300"
                  style={{ width: `${planPercent}%` }}
                />
              </div>
            </div>

            {showPlanBoard && (
              <div className="space-y-1 pt-1 max-h-40 overflow-y-auto pr-1">
                {plan.tasks.map((task) => {
                  let badgeColor = 'bg-zinc-800/70 text-zinc-400 border-zinc-700/60';
                  let stateLabel = 'در انتظار';
                  let stateIcon = <Clock size={11} className="text-zinc-500" />;

                  if (task.state === 'DONE') {
                    badgeColor = 'bg-emerald-950/60 text-emerald-400 border-emerald-800/40';
                    stateLabel = 'تکمیل شد';
                    stateIcon = <CheckCircle2 size={11} className="text-emerald-400" />;
                  } else if (task.state === 'IN_PROGRESS') {
                    badgeColor = 'bg-indigo-950/60 text-indigo-300 border-indigo-700/50 animate-pulse';
                    stateLabel = 'در حال انجام';
                    stateIcon = <RotateCcw size={11} className="text-indigo-400 animate-spin" />;
                  } else if (task.state === 'REWORK') {
                    badgeColor = 'bg-amber-950/60 text-amber-300 border-amber-700/50';
                    stateLabel = 'بازبینی / اصلاح';
                    stateIcon = <RotateCcw size={11} className="text-amber-400" />;
                  } else if (task.state === 'BLOCKED') {
                    badgeColor = 'bg-rose-950/60 text-rose-300 border-rose-800/50';
                    stateLabel = 'مسدود';
                    stateIcon = <AlertTriangle size={11} className="text-rose-400" />;
                  } else if (task.state === 'WAITING_FOR_HUMAN') {
                    badgeColor = 'bg-purple-950/60 text-purple-300 border-purple-800/50';
                    stateLabel = 'نیاز به کاربر';
                    stateIcon = <AlertCircle size={11} className="text-purple-400" />;
                  }

                  return (
                    <div key={task.id} className="flex items-center justify-between p-1.5 rounded bg-[#1a1a1a] border border-[#262626] text-[10px]">
                      <div className="flex items-center gap-1.5 truncate">
                        {stateIcon}
                        <span className="font-mono text-zinc-400 shrink-0">{task.id}</span>
                        <span className="text-zinc-200 truncate">{task.title}</span>
                      </div>
                      <span className={`px-1.5 py-0.5 rounded border text-[9px] font-medium shrink-0 flex items-center gap-1 ${badgeColor}`}>
                        {stateLabel}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })()}


      {/* Messages stream */}
      <div className="flex-1 overflow-y-auto p-3 space-y-4">
        {messages.length === 0 ? (
          <div className="p-4 text-center text-zinc-500 space-y-3 pt-8">
            <Bot size={32} className="mx-auto text-zinc-600 opacity-80" />
            <p className="text-zinc-200 font-semibold text-xs">
              تیم ۵ نفره هوش مصنوعی آماده همکاری و مباحثه
            </p>
            <p className="text-[11px] text-zinc-400 leading-relaxed" dir="auto">
              هر پیامی بنویسید، ۵ هوش مصنوعی در یک زنجیره پله‌به‌پله با یکدیگر گفتگو کرده، ایده‌های یکدیگر را نقد و تکمیل می‌کنند و در نهایت یک خروجی واحد و جامع ارائه می‌دهند.
            </p>

            {/* Current Active Roster Badges */}
            <div className="bg-[#1a1a1a] border border-[#2a2a2a] rounded-lg p-2 text-left space-y-1">
              <span className="text-[10px] font-bold text-zinc-400 block mb-1">مدل‌های فعال کنونی در این چت:</span>
              {roles.map(r => {
                const theme = getAgentTheme(r);
                const mName = getModelDisplayName(agentModels[r]);
                return (
                  <div key={r} className="flex items-center justify-between text-[10px] text-zinc-300">
                    <span className="flex items-center gap-1">
                      <theme.icon size={11} className={theme.color.split(' ')[0]} />
                      <span>{theme.label}:</span>
                    </span>
                    <span className="text-indigo-400 font-mono text-[9px] truncate max-w-[150px]">{mName}</span>
                  </div>
                );
              })}
            </div>
          </div>
        ) : (
          turns.map((turn, tIdx) => {
            const isTurnActive = isRunning && tIdx === turns.length - 1;
            // Default to open (expanded) so the user immediately sees the thinking process
            const isCollapsed = collapsedTurns[turn.id] ?? false;
            const stepCount = turn.deliberationSteps.length;

            return (
              <div key={turn.id} className="space-y-2.5">
                {/* 1. User Message */}
                {turn.userMessage && (
                  <div className="bg-gradient-to-r from-zinc-800/90 to-zinc-800/60 p-2.5 rounded-lg border border-zinc-700/60 text-zinc-200">
                    <div className="flex items-center justify-between text-[10px] text-zinc-400 mb-1 font-semibold">
                      <span className="flex items-center gap-1">
                        <span className="w-1.5 h-1.5 rounded-full bg-indigo-400" />
                        <span>شما (کاربر)</span>
                      </span>
                      <span className="text-zinc-500 font-mono text-[9px]">
                        {new Date(turn.userMessage.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                      </span>
                    </div>
                    <div className="leading-relaxed font-sans text-xs" dir="auto">
                      {turn.userMessage.content}
                    </div>
                  </div>
                )}

                {/* 2. Step-by-Step Deliberation & Reasoning Stepper */}
                {stepCount > 0 && (
                  <div className="bg-[#191919] border border-[#292929] rounded-xl overflow-hidden shadow-sm transition-all duration-200">
                    {/* Deliberation Accordion Header */}
                    <button
                      type="button"
                      onClick={() => toggleTurnAccordion(turn.id, isCollapsed)}
                      className="w-full flex items-center justify-between p-2.5 bg-[#161616] hover:bg-[#1f1f1f] border-b border-[#242424] transition-colors text-left"
                    >
                      <div className="flex items-center gap-2">
                        {isTurnActive ? (
                          <span className="relative flex h-2 w-2">
                            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-indigo-400 opacity-75"></span>
                            <span className="relative inline-flex rounded-full h-2 w-2 bg-indigo-500"></span>
                          </span>
                        ) : (
                          <CheckCircle2 size={13} className="text-emerald-400" />
                        )}
                        <span className="font-semibold text-zinc-200 text-xs flex items-center gap-1.5">
                          <span>💭 روند تفکر و گفتگوی هوش‌های مصنوعی</span>
                          <span className="text-[10px] text-indigo-400 bg-indigo-950/60 border border-indigo-800/50 px-1.5 py-0.2 rounded-full font-mono">
                            {stepCount} مرحله
                          </span>
                        </span>
                      </div>

                      <div className="flex items-center gap-1 text-[10px] text-zinc-400">
                        {isTurnActive ? (
                          <span className="text-indigo-400 animate-pulse text-[10px]">در حال تبادل نظر...</span>
                        ) : (
                          <span className="text-zinc-500">
                            {isCollapsed ? 'نمایش گفتگوی ۵ مدل' : 'بستن مراحل تفکر'}
                          </span>
                        )}
                        {isCollapsed ? <ChevronDown size={14} /> : <ChevronUp size={14} />}
                      </div>
                    </button>

                    {/* Stepper Timeline Body */}
                    {!isCollapsed && (
                      <div className="p-3 space-y-3 bg-[#131313]/90 relative">
                        {/* Vertical connecting line */}
                        <div className="absolute right-5 top-4 bottom-4 w-0.5 bg-gradient-to-b from-indigo-600/40 via-purple-600/30 to-emerald-600/30" />

                        {turn.deliberationSteps.map((m, sIdx) => {
                          const theme = getAgentTheme(m.from);
                          const Icon = theme.icon;
                          const modelUsed = agentModels[m.from] ? getModelDisplayName(agentModels[m.from]) : null;

                          // Extract reply / dialogue and thought
                          let replyText = m.summary || '';
                          let thoughtText = m.reasoning || '';

                          if (typeof m.content === 'object' && m.content !== null) {
                            if (m.content.reply && typeof m.content.reply === 'string') {
                              replyText = m.content.reply;
                            }
                            if (m.content.thought && typeof m.content.thought === 'string') {
                              thoughtText = m.content.thought;
                            }
                          } else if (typeof m.content === 'string' && m.content) {
                            if (!replyText || replyText.startsWith('[')) {
                              replyText = m.content;
                            }
                          }

                          // Guard: Ensure user prompt never leaks as the AI's internal thought or reply
                          const userContent = turn.userMessage?.content?.trim() || '';
                          if (userContent) {
                            if (thoughtText.trim() === userContent) {
                              thoughtText = '';
                            }
                            if (replyText.trim() === userContent) {
                              replyText = 'درود! پیام شما دریافت شد و بررسی درخواست در حال انجام است.';
                            }
                          }

                          return (
                            <div key={m.id || sIdx} className="relative pr-6 space-y-1.5 group">
                              {/* Step Timeline Node Dot */}
                              <div className={`absolute right-[-4px] top-1.5 w-3 h-3 rounded-full border-2 border-[#131313] ${theme.dotColor} shadow-md flex items-center justify-center`} />

                              {/* Step Card */}
                              <div className="bg-[#1b1b1b] border border-[#272727] hover:border-zinc-700/80 rounded-lg p-2.5 space-y-1.5 transition-colors">
                                {/* Header: Role Badge + Model Tag + Inter-agent context */}
                                <div className="flex items-center justify-between border-b border-[#262626] pb-1.5">
                                  <div className="flex items-center gap-1.5 flex-wrap">
                                    <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold border ${theme.color}`}>
                                      <Icon size={11} />
                                      <span>{theme.faLabel}</span>
                                      <span className="opacity-75 font-mono text-[9px]">({theme.label})</span>
                                    </span>
                                    {modelUsed && (
                                      <span className="text-[9px] font-mono text-zinc-400 bg-[#222222] px-1 py-0.5 rounded border border-[#2f2f2f]">
                                        {modelUsed}
                                      </span>
                                    )}
                                  </div>
                                  <span className="text-[9px] text-zinc-500 font-mono">
                                    گام {sIdx + 1} از ۵
                                  </span>
                                </div>

                                {/* Step Purpose / Dialogue Arrow */}
                                <div className="text-[10px] text-indigo-300/90 font-medium flex items-center gap-1" dir="auto">
                                  <MessageSquare size={10} className="text-indigo-400" />
                                  <span>{theme.stepTitle}</span>
                                </div>

                                {/* Thought Process (تفکر پله‌به‌پله مدل) */}
                                {thoughtText && (
                                  <div className="bg-[#121212] border border-[#242424] rounded p-2 space-y-1 text-left font-mono" dir="auto">
                                    <div className="text-[9px] text-amber-400/90 flex items-center gap-1 font-sans">
                                      <Wand2 size={10} />
                                      <span>💭 تفکر درون‌ذهنی مدل:</span>
                                    </div>
                                    <p className="text-[10px] text-zinc-400 leading-relaxed whitespace-pre-wrap font-sans">
                                      {thoughtText}
                                    </p>
                                  </div>
                                )}

                                {/* Spoken Inter-Agent Dialogue */}
                                {replyText && (
                                  <div className="text-zinc-200 text-[11px] leading-relaxed pt-0.5" dir="auto">
                                    <span className="text-zinc-400 font-semibold block text-[10px] mb-0.5">
                                      {theme.replyPrefix}
                                    </span>
                                    <div className="text-zinc-300">
                                      <SimpleMarkdown text={replyText} />
                                    </div>
                                  </div>
                                )}
                              </div>
                            </div>
                          );
                        })}

                        {/* Active Live Thinking Node */}
                        {isTurnActive && thinkingAgent && (
                          <div className="relative pr-6 space-y-1">
                            <div className="absolute right-[-4px] top-1.5 w-3 h-3 rounded-full bg-indigo-500 shadow-indigo-500/80 shadow-md animate-ping" />
                            <div className="bg-indigo-950/20 border border-indigo-900/50 rounded-lg p-2.5 flex items-center gap-2 text-indigo-300 text-xs">
                              <span className="w-2 h-2 rounded-full bg-indigo-400 animate-pulse" />
                              <span className="font-semibold">{getAgentTheme(thinkingAgent.role).faLabel}:</span>
                              <span className="text-[11px] text-indigo-200 opacity-90">{thinkingAgent.status}</span>
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}

                {/* 3. The Overarching Final Consensus Decision Card */}
                {turn.finalDecision && (
                  <div className="rounded-xl border border-indigo-700/50 bg-gradient-to-b from-[#1c1d29] to-[#161622] p-3 shadow-lg space-y-2">
                    {/* Header */}
                    <div className="flex items-center justify-between border-b border-indigo-800/40 pb-2">
                      <div className="flex items-center gap-1.5 text-indigo-200 font-bold text-xs">
                        <Sparkles size={14} className="text-amber-400 animate-pulse" />
                        <span>✨ تصمیم و پاسخ نهایی تیم هوش مصنوعی (NexusAI Consensus)</span>
                      </div>
                      <span className="text-[9px] px-2 py-0.5 rounded-full bg-indigo-900/60 border border-indigo-700/60 text-indigo-300 font-medium">
                        خرد جمعی ۵ مدل
                      </span>
                    </div>

                    {/* Final Overarching Response Content */}
                    <div className="py-1">
                      {(() => {
                        let finalContent = turn.finalDecision.summary || '';
                        if (typeof turn.finalDecision.content === 'object' && turn.finalDecision.content !== null) {
                          if (turn.finalDecision.content.reply) {
                            finalContent = turn.finalDecision.content.reply;
                          } else if (turn.finalDecision.content.explanation) {
                            finalContent = turn.finalDecision.content.explanation;
                          }
                        } else if (typeof turn.finalDecision.content === 'string' && turn.finalDecision.content) {
                          finalContent = turn.finalDecision.content;
                        }

                        return <SimpleMarkdown text={finalContent || 'پاسخ تیم هوش مصنوعی آماده شد.'} />;
                      })()}
                    </div>

                    {/* Footer / Verdict / Actions */}
                    {verdict && (
                      <div className="pt-2 border-t border-indigo-900/40 flex items-center justify-between text-[11px]">
                        <div className="flex items-center gap-1.5 text-zinc-400">
                          <Scale size={12} className="text-amber-400" />
                          <span>امتیاز داوری کیفیت:</span>
                          <span className="font-bold text-indigo-300 font-mono">{verdict.final_score.toFixed(1)}%</span>
                        </div>
                        {onAcceptChanges && verdict.passed && (
                          <button
                            type="button"
                            onClick={onAcceptChanges}
                            className="flex items-center gap-1 px-2.5 py-1 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-[10px] font-semibold transition-colors shadow-sm"
                          >
                            <Check size={12} />
                            <span>تایید و ادغام کد</span>
                          </button>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })
        )}

        {/* Global isRunning indicator when turn has not yet started deliberation */}
        {isRunning && (!thinkingAgent && messages.length > 0 && turns[turns.length - 1]?.deliberationSteps.length === 0) && (
          <div className="flex items-center gap-2 p-2.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-400 text-xs">
            <span className="w-2 h-2 rounded-full bg-indigo-500 animate-pulse"></span>
            <span>در حال آغاز مباحثه و هم‌اندیشی ۵ هوش مصنوعی...</span>
          </div>
        )}
      </div>

      {/* Input box (Cursor Composer style) */}
      <form onSubmit={handleSubmit} className="p-3 border-t border-[#242424] bg-[#161616]">
        <div className="bg-[#1e1e1e] border border-[#2c2c2c] focus-within:border-indigo-500/80 rounded-lg p-2 transition-colors">
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                if (prompt.trim() && !isRunning) {
                  handleSubmit(e);
                }
              }
            }}
            placeholder="دستور، سؤال یا ویژگی مورد نظر خود را بنویسید (فارسی / انگلیسی)..."
            rows={3}
            className="w-full bg-transparent text-xs text-zinc-100 placeholder-zinc-500 outline-none resize-none leading-relaxed"
            dir="auto"
          />
          <div className="flex items-center justify-between pt-1 border-t border-zinc-800/80 mt-1">
            <div className="flex items-center gap-2 text-[10px] text-zinc-500">
              <span className="hover:text-zinc-300 cursor-pointer">@ فایل‌ها</span>
              <span className="hover:text-zinc-300 cursor-pointer"># مهارت‌ها</span>
              <span className="text-indigo-400 text-[9px] font-mono">۵ هوش مصنوعی آماده</span>
            </div>
            <button
              type="submit"
              disabled={isRunning || !prompt.trim()}
              className="px-2.5 py-1 rounded bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white font-medium text-xs flex items-center gap-1 transition-colors"
            >
              <span>ارسال</span>
              <Send size={11} />
            </button>
          </div>
        </div>
      </form>
    </aside>
  );
};
