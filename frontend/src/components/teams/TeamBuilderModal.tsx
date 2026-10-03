import React, { useEffect, useState } from 'react';
import { X, Plus, Trash2, Users, Save, AlertTriangle, Cpu } from 'lucide-react';
import { AgentPhase, AgentRole, AgentSpec, Team } from '../../types/team';
import { AVAILABLE_MODELS } from '../../config/models';

interface TeamBuilderModalProps {
  isOpen: boolean;
  onClose: () => void;
  teams: Team[];
  selectedTeamId: string;
  onSelectTeam: (teamId: string) => void;
  onSaveTeam: (team: Team) => Promise<void>;
  onDeleteTeam: (teamId: string) => Promise<void>;
}

const PHASE_LABELS: Record<AgentPhase, string> = {
  design: 'طراحی',
  implement: 'پیاده‌سازی',
  review: 'بازبینی',
  judge: 'داوری',
  custom: 'سفارشی',
};

const ROLE_OPTIONS: { value: AgentRole; label: string }[] = [
  { value: 'architect', label: 'معمار' },
  { value: 'coder', label: 'کدنویس' },
  { value: 'critic', label: 'منتقد' },
  { value: 'security', label: 'امنیت' },
  { value: 'arbiter', label: 'داور' },
  { value: 'custom', label: 'سفارشی' },
];

function blankAgent(index: number): AgentSpec {
  return {
    id: `agent-${Date.now()}-${index}`,
    name: '',
    role: 'custom',
    model: 'openrouter/free',
    phase: 'implement',
    system_prompt: '',
    enabled: true,
    temperature: 0.2,
    tools_access: 'write',
    max_tool_calls: 12,
  };
}

function blankTeam(): Team {
  return {
    id: '',
    name: '',
    description: '',
    agents: [blankAgent(0)],
    consensus_threshold: 85,
    max_rounds: 3,
  };
}

export const TeamBuilderModal: React.FC<TeamBuilderModalProps> = ({
  isOpen,
  onClose,
  teams,
  selectedTeamId,
  onSelectTeam,
  onSaveTeam,
  onDeleteTeam,
}) => {
  const [draft, setDraft] = useState<Team>(blankTeam);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  // Load the selected team whenever the modal opens so edits start from truth.
  useEffect(() => {
    if (!isOpen) return;
    const found = teams.find((t) => t.id === selectedTeamId);
    setDraft(found ? JSON.parse(JSON.stringify(found)) : blankTeam());
    setError('');
  }, [isOpen, selectedTeamId, teams]);

  if (!isOpen) return null;

  const updateAgent = (index: number, patch: Partial<AgentSpec>) => {
    setDraft((prev) => ({
      ...prev,
      agents: prev.agents.map((a, i) => (i === index ? { ...a, ...patch } : a)),
    }));
  };

  const addAgent = () => {
    setDraft((prev) => ({ ...prev, agents: [...prev.agents, blankAgent(prev.agents.length)] }));
  };

  const removeAgent = (index: number) => {
    setDraft((prev) => ({
      ...prev,
      agents: prev.agents.filter((_, i) => i !== index),
    }));
  };

  const handleSave = async () => {
    setError('');

    const missingName = draft.agents.findIndex((a) => !a.name.trim());
    if (missingName >= 0) {
      setError(`ایجنت شماره ${missingName + 1} نام ندارد.`);
      return;
    }
    const noModel = draft.agents.findIndex((a) => !a.model);
    if (noModel >= 0) {
      setError(`ایجنت شماره ${noModel + 1} مدل ندارد.`);
      return;
    }
    // If no explicit judge is configured, the backend automatically uses consensus/last agent.

    setSaving(true);
    try {
      await onSaveTeam(draft);
      onClose();
    } catch (e: any) {
      setError(e?.message || 'ذخیره‌سازی تیم ناموفق بود.');
    } finally {
      setSaving(false);
    }
  };

  const activeCount = draft.agents.filter((a) => a.enabled).length;

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 select-none">
      <div className="bg-[#181818] border border-[#2e2e2e] rounded-xl w-full max-w-4xl h-[680px] flex flex-col shadow-2xl overflow-hidden text-xs text-zinc-200">
        {/* Header */}
        <div className="h-12 px-4 border-b border-[#262626] flex items-center justify-between bg-[#151515] shrink-0">
          <div className="flex items-center gap-2 font-bold text-sm">
            <Users size={16} className="text-indigo-400" />
            <span>ساخت تیم هوش مصنوعی</span>
            <span className="text-[10px] font-normal text-zinc-500">
              ({activeCount} ایجنت فعال از {draft.agents.length})
            </span>
          </div>
          <button onClick={onClose} className="p-1 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 flex overflow-hidden">
          {/* Saved teams */}
          <div className="w-56 bg-[#141414] border-r border-[#262626] p-2 space-y-1 overflow-y-auto shrink-0">
            <div className="text-[10px] uppercase tracking-wider text-zinc-500 px-1 pb-1">تیم‌های ذخیره‌شده</div>
            <button
              onClick={() => setDraft(blankTeam())}
              className="w-full text-left px-2 py-1.5 rounded text-emerald-300 hover:bg-zinc-800 flex items-center gap-1.5"
            >
              <Plus size={12} />
              <span>تیم جدید</span>
            </button>
            {teams.map((t) => (
              <div key={t.id} className="group flex items-center gap-1">
                <button
                  onClick={() => onSelectTeam(t.id)}
                  className={`flex-1 text-left px-2 py-1.5 rounded truncate ${
                    selectedTeamId === t.id ? 'bg-indigo-600 text-white' : 'text-zinc-300 hover:bg-zinc-800'
                  }`}
                  title={t.name}
                >
                  {t.name}
                  <span className="block text-[9px] opacity-70">{t.agents.length} ایجنت</span>
                </button>
                {t.id !== 'default' && (
                  <button
                    onClick={() => onDeleteTeam(t.id)}
                    className="p-1 text-zinc-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity"
                    title="حذف تیم"
                  >
                    <Trash2 size={11} />
                  </button>
                )}
              </div>
            ))}
          </div>

          {/* Editor */}
          <div className="flex-1 flex flex-col overflow-hidden">
            <div className="flex-1 overflow-y-auto p-3 space-y-3">
              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">نام تیم</label>
                  <input
                    value={draft.name}
                    onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                    placeholder="مثلاً: تیم بک‌اند"
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 outline-none"
                  />
                </div>
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">توضیح</label>
                  <input
                    value={draft.description}
                    onChange={(e) => setDraft({ ...draft, description: e.target.value })}
                    placeholder="این تیم برای چه کاری است؟"
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 outline-none"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">
                    آستانه اجماع: {draft.consensus_threshold}%
                  </label>
                  <input
                    type="range"
                    min={0}
                    max={100}
                    step={5}
                    value={draft.consensus_threshold}
                    onChange={(e) => setDraft({ ...draft, consensus_threshold: Number(e.target.value) })}
                    className="w-full accent-indigo-500"
                  />
                </div>
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">حداکثر دور: {draft.max_rounds}</label>
                  <input
                    type="range"
                    min={1}
                    max={10}
                    value={draft.max_rounds}
                    onChange={(e) => setDraft({ ...draft, max_rounds: Number(e.target.value) })}
                    className="w-full accent-indigo-500"
                  />
                </div>
              </div>

              {/* Agents */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <label className="font-semibold text-zinc-300">اعضای تیم</label>
                  <button
                    onClick={addAgent}
                    className="px-2 py-1 rounded bg-indigo-600/20 text-indigo-300 hover:bg-indigo-600/40 text-[10px] flex items-center gap-1"
                  >
                    <Plus size={11} />
                    افزودن ایجنت
                  </button>
                </div>

                {draft.agents.map((agent, i) => (
                  <div key={agent.id} className="border border-[#2a2a2a] rounded-lg p-2.5 space-y-2 bg-[#151515]">
                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        checked={agent.enabled}
                        onChange={(e) => updateAgent(i, { enabled: e.target.checked })}
                        className="accent-indigo-500"
                        title="فعال"
                      />
                      <input
                        value={agent.name}
                        onChange={(e) => updateAgent(i, { name: e.target.value })}
                        placeholder={`نام ایجنت ${i + 1}`}
                        className="flex-1 bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded px-2 py-1 outline-none"
                      />
                      <button
                        onClick={() => removeAgent(i)}
                        className="p-1 text-zinc-600 hover:text-red-400"
                        title="حذف"
                      >
                        <Trash2 size={12} />
                      </button>
                    </div>

                    <div className="grid grid-cols-3 gap-2">
                      <select
                        value={agent.role}
                        onChange={(e) => updateAgent(i, { role: e.target.value as AgentRole })}
                        className="bg-[#1f1f1f] border border-[#2e2e2e] rounded px-2 py-1 outline-none text-[11px]"
                      >
                        {ROLE_OPTIONS.map((r) => (
                          <option key={r.value} value={r.value}>
                            {r.label}
                          </option>
                        ))}
                      </select>

                      <select
                        value={agent.model}
                        onChange={(e) => updateAgent(i, { model: e.target.value })}
                        className="bg-[#1f1f1f] border border-[#2e2e2e] rounded px-2 py-1 outline-none text-[11px]"
                      >
                        {AVAILABLE_MODELS.map((m) => (
                          <option key={m.id} value={m.id}>
                            {m.name}
                          </option>
                        ))}
                      </select>

                      <select
                        value={agent.phase || 'implement'}
                        onChange={(e) => updateAgent(i, { phase: e.target.value as AgentPhase })}
                        className="bg-[#1f1f1f] border border-[#2e2e2e] rounded px-2 py-1 outline-none text-[11px]"
                      >
                        {(Object.keys(PHASE_LABELS) as AgentPhase[]).map((p) => (
                          <option key={p} value={p}>
                            {PHASE_LABELS[p]}
                          </option>
                        ))}
                      </select>
                    </div>

                    {agent.role === 'custom' && (
                      <textarea
                        value={agent.system_prompt || ''}
                        onChange={(e) => updateAgent(i, { system_prompt: e.target.value })}
                        placeholder="این ایجنت دقیقاً چه کاری باید بکند؟ مثال: فقط بهینه‌سازی کوئری‌های PostgreSQL را بررسی کن."
                        rows={3}
                        className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 outline-none text-[11px] resize-y"
                      />
                    )}

                    {/* Tool access: what this agent may do to the project. */}
                    <div className="flex items-center gap-2">
                      <span className="text-[10px] text-zinc-500 w-20 shrink-0">دسترسی ابزار</span>
                      <select
                        value={agent.tools_access || 'read_only'}
                        onChange={(e) =>
                          updateAgent(i, {
                            tools_access: e.target.value as AgentSpec['tools_access'],
                          })
                        }
                        className="bg-[#1f1f1f] border border-[#2e2e2e] rounded px-2 py-1 outline-none text-[11px] flex-1"
                      >
                        <option value="read_only">فقط خواندن — امن</option>
                        <option value="write">خواندن و نوشتن</option>
                        <option value="full">خواندن، نوشتن و اجرای تست</option>
                      </select>
                    </div>

                    {agent.tools_access !== 'read_only' && (
                      <div className="flex items-start gap-1.5 text-[10px] bg-amber-950/30 border border-amber-900/50 text-amber-300/90 px-2 py-1.5 rounded">
                        <AlertTriangle size={11} className="mt-px shrink-0" />
                        <span>
                          این ایجنت می‌تواند فایل‌های پروژه را تغییر دهد. تمام تغییرات
                          در git ثبت می‌شود.
                        </span>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>

            {/* Footer */}
            <div className="h-12 border-t border-[#262626] flex items-center justify-between px-3 bg-[#151515] shrink-0">
              <div className="flex items-center gap-2">
                {error && (
                  <span className="flex items-center gap-1 text-red-400">
                    <AlertTriangle size={12} />
                    {error}
                  </span>
                )}
                {!error && (
                  <span className="flex items-center gap-1 text-zinc-500">
                    <Cpu size={12} />
                    {activeCount} ایجنت اجرا خواهد شد
                  </span>
                )}
              </div>
              <div className="flex gap-2">
                <button onClick={onClose} className="px-3 py-1.5 rounded text-zinc-400 hover:bg-zinc-800">
                  انصراف
                </button>
                <button
                  onClick={handleSave}
                  disabled={saving}
                  className="px-3 py-1.5 rounded bg-indigo-600 hover:bg-indigo-500 text-white flex items-center gap-1 disabled:opacity-50"
                >
                  <Save size={12} />
                  {saving ? 'در حال ذخیره...' : 'ذخیره تیم'}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};