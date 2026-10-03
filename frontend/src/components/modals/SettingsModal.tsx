import React, { useState, useEffect } from 'react';
import { X, Key, Cpu, Sliders, Check, Shield, Database, ExternalLink } from 'lucide-react';
import { AVAILABLE_MODELS, SWARM_PRESETS, SwarmPreset } from '../../config/models';
import { AgentRole } from '../../types';

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  apiKeys: {
    openai: string;
    openrouter: string;
    anthropic: string;
    gemini: string;
    ninerouter: string;
    ollamaUrl: string;
    customBaseUrl: string;
  };
  onSaveApiKeys: (keys: any) => void;
  agentModels: Record<string, string>;
  onUpdateAgentModel: (role: string, model: string) => void;
  onApplyPreset: (preset: SwarmPreset) => void;
}

export const SettingsModal: React.FC<SettingsModalProps> = ({
  isOpen,
  onClose,
  apiKeys,
  onSaveApiKeys,
  agentModels,
  onUpdateAgentModel,
  onApplyPreset,
}) => {
  const [activeTab, setActiveTab] = useState<'providers' | 'swarm' | 'agents'>('providers');
  const [formData, setFormData] = useState(apiKeys);
  const [savedNote, setSavedNote] = useState(false);

  // Re-sync the form whenever the modal opens. Without this the fields kept
// showing whatever was typed the first time the modal was mounted, so a key
// that changed elsewhere was never reflected here.
useEffect(() => {
  if (isOpen) {
    setFormData(apiKeys);
  }
}, [isOpen, apiKeys]);

if (!isOpen) return null;

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    onSaveApiKeys(formData);
    setSavedNote(true);
    setTimeout(() => {
      setSavedNote(false);
      onClose();
    }, 800);
  };

  const roles: { role: AgentRole; label: string; faLabel: string; desc: string }[] = [
    { role: 'architect', label: 'Architect', faLabel: 'معمار سیستم', desc: 'طراحی ساختار، معماری فایل‌ها و قراردادهای API' },
    { role: 'coder', label: 'Lead Coder', faLabel: 'برنامه‌نویس ارشد', desc: 'تولید کدهای با کیفیت و بدون placeholder' },
    { role: 'critic', label: 'Critic Red-Team', faLabel: 'منتقد بی‌رحم کد', desc: 'کشف باگ‌ها، رفتارهای نامشخص و اشکالات ساختاری' },
    { role: 'security', label: 'Security Auditor', faLabel: 'ممیز امنیت', desc: 'بررسی آسیب‌پذیری‌ها، تزریق کد، و اعتبارسنجی ورودی' },
    { role: 'arbiter', label: 'Arbiter Judge', faLabel: 'قاضی نهایی اجماع', desc: 'ارزیابی ۵ بعدی و قضاوت بر اساس حداقل ۸۵٪ امتیاز' },
  ];

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 select-none">
      <div className="bg-[#181818] border border-[#2e2e2e] rounded-xl w-full max-w-2xl h-[540px] flex flex-col shadow-2xl overflow-hidden text-xs">
        {/* Header */}
        <div className="h-12 px-4 border-b border-[#262626] flex items-center justify-between bg-[#151515]">
          <div className="flex items-center gap-2 font-bold text-sm text-zinc-100">
            <Sliders size={16} className="text-indigo-400" />
            <span>تنظیمات NexusAI و کلیدهای API</span>
          </div>
          <button
            onClick={onClose}
            className="p-1 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded transition-colors"
          >
            <X size={16} />
          </button>
        </div>

        {/* Content Body: Sidebar + Main Settings */}
        <div className="flex-1 flex overflow-hidden">
          {/* Settings Navigation Tabs */}
          <div className="w-52 bg-[#141414] border-r border-[#262626] p-2 space-y-1">
            <button
              onClick={() => setActiveTab('providers')}
              className={`w-full text-left px-3 py-2 rounded-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'providers' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
              }`}
            >
              <Key size={14} />
              <span>کلیدهای API (Providers)</span>
            </button>
            <button
              onClick={() => setActiveTab('agents')}
              className={`w-full text-left px-3 py-2 rounded-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'agents' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
              }`}
            >
              <Cpu size={14} />
              <span>مدل‌های اعضای تیم (Team Models)</span>
            </button>
            <button
              onClick={() => setActiveTab('swarm')}
              className={`w-full text-left px-3 py-2 rounded-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'swarm' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
              }`}
            >
              <Sliders size={14} />
              <span>پارامترهای اجماع (Consensus)</span>
            </button>
          </div>

          {/* Form Settings Fields */}
          <form onSubmit={handleSave} className="flex-1 p-5 overflow-y-auto space-y-4">
            {activeTab === 'providers' && (
              <div className="space-y-4">
                <div>
                  <h3 className="font-bold text-zinc-100 text-sm mb-1">کلیدهای دسترسی به هوش مصنوعی</h3>
                  <p className="text-[11px] text-zinc-400">
                    کلید اوپن‌روتر شما مستقیماً در فایل <code className="text-indigo-300">.env</code> ذخیره شده و از تمام مدل‌های دیپ‌سیک، لاما، جمنای و کلاد به صورت یکپارچه پشتیبانی می‌کند.
                  </p>
                </div>

                {/* OpenRouter */}
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="font-semibold text-zinc-200">OpenRouter API Key:</label>
                    <span className="text-[10px] text-emerald-400 font-medium">✓ متصل و فعال (.env)</span>
                  </div>
                  <input
                    type="password"
                    value={formData.openrouter}
                    onChange={(e) => setFormData({ ...formData, openrouter: e.target.value })}
                    placeholder="sk-or-v1-..."
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono text-xs"
                  />
                  <span className="text-[10px] text-zinc-500">پشتیبانی رایگان از Llama 3.3, DeepSeek Chat/R1, Gemini 2.0 Flash و Qwen 2.5 Coder</span>
                </div>

                {/* NineRouter */}
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <label className="font-semibold text-zinc-200">NineRouter API Key:</label>
                  </div>
                  <input
                    type="password"
                    value={formData.ninerouter || ''}
                    onChange={(e) => setFormData({ ...formData, ninerouter: e.target.value })}
                    placeholder="sk-..."
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono text-xs"
                  />
                </div>

                {/* Gemini */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">Google Gemini API Key (مستقیم اختیاری):</label>
                  <input
                    type="password"
                    value={formData.gemini}
                    onChange={(e) => setFormData({ ...formData, gemini: e.target.value })}
                    placeholder="AIzaSy..."
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono text-xs"
                  />
                </div>

                {/* OpenAI */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">OpenAI API Key (اختیاری):</label>
                  <input
                    type="password"
                    value={formData.openai}
                    onChange={(e) => setFormData({ ...formData, openai: e.target.value })}
                    placeholder="sk-proj-..."
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono text-xs"
                  />
                </div>

                {/* Ollama Local URL */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">Ollama Endpoint (آفلاین محلی):</label>
                  <input
                    type="text"
                    value={formData.ollamaUrl}
                    onChange={(e) => setFormData({ ...formData, ollamaUrl: e.target.value })}
                    placeholder="http://localhost:11434/v1"
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono text-xs"
                  />
                </div>
              </div>
            )}

            {activeTab === 'agents' && (
              <div className="space-y-3">
                <div>
                  <h3 className="font-bold text-zinc-100 text-sm mb-1">تخصیص مدل به هر عضو تیم هوش مصنوعی</h3>
                  <p className="text-[11px] text-zinc-400">
                    می‌توانید برای هر نقش یک مدل هوش مصنوعی مجزا انتخاب کنید تا همزمان با هم همکاری و کدهای یکدیگر را نقد کنند.
                  </p>
                </div>

                {/* Presets */}
                <div className="space-y-1 bg-[#151515] p-2.5 rounded border border-[#262626]">
                  <span className="text-[10px] text-zinc-400 font-semibold block">انتخاب سریع الگو:</span>
                  <div className="grid grid-cols-2 gap-1.5">
                    {SWARM_PRESETS.map((p) => (
                      <button
                        key={p.id}
                        type="button"
                        onClick={() => onApplyPreset(p)}
                        className="p-1.5 rounded bg-[#202020] hover:bg-indigo-950/40 hover:border-indigo-700/60 border border-[#2d2d2d] text-[10px] text-zinc-200 text-left transition-colors truncate"
                      >
                        {p.nameFa}
                      </button>
                    ))}
                  </div>
                </div>

                {roles.map(({ role, label, faLabel, desc }) => {
                  const currentModel = agentModels[role] || 'deepseek/deepseek-chat:free';
                  return (
                    <div key={role} className="p-2 bg-[#1f1f1f] rounded border border-[#2e2e2e] space-y-1">
                      <div className="flex items-center justify-between">
                        <div>
                          <span className="font-bold text-zinc-200 text-xs">{label} ({faLabel})</span>
                          <span className="block text-[10px] text-zinc-500">{desc}</span>
                        </div>
                        <select
                          value={currentModel}
                          onChange={(e) => onUpdateAgentModel(role, e.target.value)}
                          className="bg-[#141414] text-zinc-200 p-1 rounded border border-[#333333] outline-none text-xs"
                        >
                          {AVAILABLE_MODELS.map((m) => (
                            <option key={m.id} value={m.id}>
                              {m.name} {m.isFree ? '(رایگان)' : ''}
                            </option>
                          ))}
                        </select>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}

            {activeTab === 'swarm' && (
              <div className="space-y-4">
                <div>
                  <h3 className="font-bold text-zinc-100 text-sm mb-1">پارامترهای بازبینی و اجماع</h3>
                  <p className="text-[11px] text-zinc-500">
                    تنظیم دقت قاضی برای تایید نهایی خروجی‌های تیم.
                  </p>
                </div>

                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">حداقل امتیاز اجماع (Consensus Threshold %):</label>
                  <input
                    type="number"
                    defaultValue={85}
                    min={60}
                    max={99}
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] rounded p-2 text-zinc-200 outline-none"
                  />
                  <div className="text-[10px] text-zinc-500">پیش‌فرض ۸۵٪ است. کدها باید در تمام ۵ بعد این امتیاز را کسب کنند.</div>
                </div>

                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">حداکثر دورهای بازبینی (Max Iteration Rounds):</label>
                  <input
                    type="number"
                    defaultValue={5}
                    min={1}
                    max={10}
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] rounded p-2 text-zinc-200 outline-none"
                  />
                  <div className="text-[10px] text-zinc-500">حداکثر دفعاتی که هوش مصنوعی‌ها مجازند کدهای یکدیگر را بازنویسی و اصلاح کنند.</div>
                </div>
              </div>
            )}
          </form>
        </div>

        {/* Footer */}
        <div className="h-12 px-4 border-t border-[#262626] flex items-center justify-between bg-[#151515]">
          <div>
            {savedNote && (
              <span className="flex items-center gap-1 text-emerald-400 font-medium text-xs">
                <Check size={13} />
                <span>تنظیمات با موفقیت ذخیره شد!</span>
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="px-3 py-1.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-300 font-medium"
            >
              بستن
            </button>
            <button
              onClick={handleSave}
              className="px-4 py-1.5 rounded bg-indigo-600 hover:bg-indigo-500 text-white font-medium shadow-sm"
            >
              ذخیره تنظیمات
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
