import React, { useState } from 'react';
import { X, Key, Cpu, Sliders, Check, Shield, Database, ExternalLink } from 'lucide-react';

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  apiKeys: {
    openai: string;
    openrouter: string;
    anthropic: string;
    gemini: string;
    ollamaUrl: string;
    customBaseUrl: string;
  };
  onSaveApiKeys: (keys: any) => void;
}

export const SettingsModal: React.FC<SettingsModalProps> = ({
  isOpen,
  onClose,
  apiKeys,
  onSaveApiKeys,
}) => {
  const [activeTab, setActiveTab] = useState<'providers' | 'swarm' | 'agents'>('providers');
  const [formData, setFormData] = useState(apiKeys);
  const [savedNote, setSavedNote] = useState(false);

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

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 select-none">
      <div className="bg-[#181818] border border-[#2e2e2e] rounded-xl w-full max-w-2xl h-[520px] flex flex-col shadow-2xl overflow-hidden text-xs">
        {/* Header */}
        <div className="h-12 px-4 border-b border-[#262626] flex items-center justify-between bg-[#151515]">
          <div className="flex items-center gap-2 font-bold text-sm text-zinc-100">
            <Sliders size={16} className="text-indigo-400" />
            <span>NexusAI Settings & API Configuration</span>
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
          <div className="w-48 bg-[#141414] border-r border-[#262626] p-2 space-y-1">
            <button
              onClick={() => setActiveTab('providers')}
              className={`w-full text-left px-3 py-2 rounded-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'providers' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
              }`}
            >
              <Key size={14} />
              <span>API Providers & Keys</span>
            </button>
            <button
              onClick={() => setActiveTab('swarm')}
              className={`w-full text-left px-3 py-2 rounded-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'swarm' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
              }`}
            >
              <Sliders size={14} />
              <span>Swarm & Consensus</span>
            </button>
            <button
              onClick={() => setActiveTab('agents')}
              className={`w-full text-left px-3 py-2 rounded-md font-medium flex items-center gap-2 transition-colors ${
                activeTab === 'agents' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
              }`}
            >
              <Cpu size={14} />
              <span>Models per Agent</span>
            </button>
          </div>

          {/* Form Settings Fields */}
          <form onSubmit={handleSave} className="flex-1 p-6 overflow-y-auto space-y-4">
            {activeTab === 'providers' && (
              <div className="space-y-4">
                <div>
                  <h3 className="font-bold text-zinc-100 text-sm mb-1">LLM Provider Credentials</h3>
                  <p className="text-[11px] text-zinc-500">
                    Keys are stored locally and never sent to any external server other than the respective AI endpoints.
                  </p>
                </div>

                {/* OpenRouter */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">OpenRouter API Key (Supports all models):</label>
                  <input
                    type="password"
                    value={formData.openrouter}
                    onChange={(e) => setFormData({ ...formData, openrouter: e.target.value })}
                    placeholder="sk-or-v1-..."
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono"
                  />
                </div>

                {/* OpenAI */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">OpenAI API Key (GPT-4o, o3, etc.):</label>
                  <input
                    type="password"
                    value={formData.openai}
                    onChange={(e) => setFormData({ ...formData, openai: e.target.value })}
                    placeholder="sk-proj-..."
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono"
                  />
                </div>

                {/* Gemini */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">Google Gemini API Key:</label>
                  <input
                    type="password"
                    value={formData.gemini}
                    onChange={(e) => setFormData({ ...formData, gemini: e.target.value })}
                    placeholder="AIzaSy..."
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono"
                  />
                </div>

                {/* Ollama Local URL */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">Ollama Local Endpoint (Offline):</label>
                  <input
                    type="text"
                    value={formData.ollamaUrl}
                    onChange={(e) => setFormData({ ...formData, ollamaUrl: e.target.value })}
                    placeholder="http://localhost:11434/v1"
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono"
                  />
                </div>

                {/* Custom Base URL */}
                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">Custom OpenAI-Compatible Base URL:</label>
                  <input
                    type="text"
                    value={formData.customBaseUrl}
                    onChange={(e) => setFormData({ ...formData, customBaseUrl: e.target.value })}
                    placeholder="https://api.your-custom-provider.com/v1"
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none font-mono"
                  />
                </div>
              </div>
            )}

            {activeTab === 'swarm' && (
              <div className="space-y-4">
                <div>
                  <h3 className="font-bold text-zinc-100 text-sm mb-1">Swarm & Consensus Parameters</h3>
                  <p className="text-[11px] text-zinc-500">
                    Configure thresholds for how rigorously the Arbiter audits code before approval.
                  </p>
                </div>

                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">Consensus Threshold Score (%):</label>
                  <input
                    type="number"
                    defaultValue={85}
                    min={60}
                    max={99}
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] rounded p-2 text-zinc-200 outline-none"
                  />
                  <div className="text-[10px] text-zinc-500">Default is 85%. Code must reach this score across 5 dimensions to merge.</div>
                </div>

                <div className="space-y-1">
                  <label className="font-semibold text-zinc-300">Maximum Iteration Rounds:</label>
                  <input
                    type="number"
                    defaultValue={5}
                    min={1}
                    max={10}
                    className="w-full bg-[#1f1f1f] border border-[#2e2e2e] rounded p-2 text-zinc-200 outline-none"
                  />
                  <div className="text-[10px] text-zinc-500">Max adversarial round-robin iterations allowed.</div>
                </div>
              </div>
            )}

            {activeTab === 'agents' && (
              <div className="space-y-3">
                <h3 className="font-bold text-zinc-100 text-sm mb-1">Individual Agent Model Selection</h3>
                <p className="text-[11px] text-zinc-500 mb-3">
                  Assign different models to different personas for cost and performance optimization.
                </p>

                {['Architect', 'Lead Coder', 'Critic (Red Team)', 'Security Auditor', 'Arbiter Judge'].map((agentName) => (
                  <div key={agentName} className="flex items-center justify-between p-2 bg-[#1f1f1f] rounded border border-[#2e2e2e]">
                    <span className="font-semibold text-zinc-300">{agentName}:</span>
                    <select className="bg-[#141414] text-zinc-200 p-1 rounded border border-[#333333] outline-none">
                      <option>OpenAI GPT-4o</option>
                      <option>Claude 3.7 Sonnet</option>
                      <option>Gemini 2.5 Flash</option>
                      <option>DeepSeek R1</option>
                      <option>Local Ollama</option>
                    </select>
                  </div>
                ))}
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
                <span>Settings Saved Successfully!</span>
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="px-3 py-1.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-300 font-medium"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              className="px-4 py-1.5 rounded bg-indigo-600 hover:bg-indigo-500 text-white font-medium shadow-sm"
            >
              Save Configuration
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
