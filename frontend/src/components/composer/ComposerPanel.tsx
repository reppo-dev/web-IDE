import React, { useState } from 'react';
import { 
  Send, 
  Sparkles, 
  Bot, 
  Code2, 
  Shield, 
  Scale, 
  ChevronDown, 
  Check, 
  RotateCcw, 
  Cpu, 
  Zap, 
  FileText 
} from 'lucide-react';
import { AgentMessage, ArbiterVerdict, ExecutionMode, AgentRole } from '../../types';

interface ComposerPanelProps {
  messages: AgentMessage[];
  isRunning: boolean;
  verdict: ArbiterVerdict | null;
  executionMode: ExecutionMode;
  onSetExecutionMode: (mode: ExecutionMode) => void;
  selectedModel: string;
  onSelectModel: (model: string) => void;
  onSendPrompt: (prompt: string) => void;
  onAcceptChanges?: () => void;
}

export const ComposerPanel: React.FC<ComposerPanelProps> = ({
  messages,
  isRunning,
  verdict,
  executionMode,
  onSetExecutionMode,
  selectedModel,
  onSelectModel,
  onSendPrompt,
  onAcceptChanges,
}) => {
  const [prompt, setPrompt] = useState('');

  const models = [
    { id: "gpt-4o", name: "GPT-4o (OpenAI)" },
    { id: "claude-3-7-sonnet", name: "Claude 3.7 Sonnet (Anthropic)" },
    { id: "gemini-2.5-flash", name: "Gemini 2.5 Flash (Google)" },
    { id: "deepseek-r1", name: "DeepSeek R1 (Reasoning)" },
    { id: "ollama-local", name: "Ollama (Local / Offline)" },
  ];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!prompt.trim() || isRunning) return;
    onSendPrompt(prompt);
    setPrompt('');
  };

  const getAgentTheme = (role: AgentRole) => {
    switch (role) {
      case 'architect':
        return { label: 'Architect', color: 'text-blue-400 bg-blue-950/30 border-blue-800/40', icon: Bot };
      case 'coder':
        return { label: 'Lead Coder', color: 'text-purple-400 bg-purple-950/30 border-purple-800/40', icon: Code2 };
      case 'critic':
        return { label: 'Critic Red-Team', color: 'text-red-400 bg-red-950/30 border-red-800/40', icon: Sparkles };
      case 'security':
        return { label: 'Security Guard', color: 'text-emerald-400 bg-emerald-950/30 border-emerald-800/40', icon: Shield };
      case 'arbiter':
        return { label: 'Arbiter Judge', color: 'text-amber-400 bg-amber-950/30 border-amber-800/40', icon: Scale };
      default:
        return { label: role, color: 'text-zinc-400 bg-zinc-900 border-zinc-800', icon: Bot };
    }
  };

  return (
    <aside className="w-80 md:w-96 bg-[#181818] border-l border-[#242424] flex flex-col h-full select-text z-20 shrink-0 text-xs">
      {/* Top Header: Composer Title, Model Selector, Mode Pill */}
      <div className="p-3 border-b border-[#242424] space-y-2 bg-[#161616]">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5 font-bold text-zinc-100 text-xs">
            <Sparkles size={14} className="text-indigo-400" />
            <span>Cursor Composer</span>
          </div>

          {/* Mode Pill */}
          <select
            value={executionMode}
            onChange={(e) => onSetExecutionMode(e.target.value as ExecutionMode)}
            className="bg-[#202020] text-indigo-300 font-medium text-[11px] px-2 py-0.5 rounded border border-[#303030] outline-none cursor-pointer"
          >
            <option value="rolebased">Role-Based (Phased)</option>
            <option value="concurrent">Concurrent (Channels)</option>
            <option value="sequential">Sequential (Linear)</option>
          </select>
        </div>

        {/* Model Selector dropdown */}
        <div className="flex items-center justify-between bg-[#1e1e1e] px-2 py-1.5 rounded border border-[#2a2a2a]">
          <span className="text-zinc-400 text-[11px] flex items-center gap-1">
            <Cpu size={12} className="text-zinc-500" />
            <span>Model:</span>
          </span>
          <select
            value={selectedModel}
            onChange={(e) => onSelectModel(e.target.value)}
            className="bg-transparent text-zinc-200 font-medium text-[11px] outline-none cursor-pointer"
          >
            {models.map(m => (
              <option key={m.id} value={m.id} className="bg-zinc-900 text-zinc-200">
                {m.name}
              </option>
            ))}
          </select>
        </div>
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
            <span>Arbiter Score: {verdict.final_score.toFixed(1)}%</span>
            <span className="text-[10px] font-normal text-zinc-400">({verdict.passed ? 'Consensus >= 85%' : 'Refining'})</span>
          </div>
          {onAcceptChanges && verdict.passed && (
            <button
              onClick={onAcceptChanges}
              className="flex items-center gap-1 px-2 py-0.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-[10px] font-semibold transition-colors"
            >
              <Check size={11} />
              <span>Accept</span>
            </button>
          )}
        </div>
      )}

      {/* Messages stream */}
      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {messages.length === 0 ? (
          <div className="p-4 text-center text-zinc-500 space-y-3 pt-12">
            <Bot size={32} className="mx-auto text-zinc-600 opacity-80" />
            <p className="text-zinc-300 font-medium text-xs">
              What do you want to build?
            </p>
            <p className="text-[11px] text-zinc-500 leading-relaxed">
              NexusAI pits 5 AI agents against each other to generate, review, audit, and score your codebase before applying changes.
            </p>
          </div>
        ) : (
          messages.map((m) => {
            const isUser = m.type === 'user_prompt';
            const theme = getAgentTheme(m.from);
            const Icon = theme.icon;

            if (isUser) {
              return (
                <div key={m.id} className="bg-zinc-800/80 p-2.5 rounded-lg border border-zinc-700/60 text-zinc-200">
                  <div className="text-[10px] font-semibold text-zinc-400 mb-1">You</div>
                  <div className="leading-relaxed font-sans">{m.content}</div>
                </div>
              );
            }

            return (
              <div key={m.id} className="bg-[#1e1e1e] p-2.5 rounded-lg border border-[#2b2b2b] space-y-1.5">
                <div className="flex items-center justify-between">
                  <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium border ${theme.color}`}>
                    <Icon size={11} />
                    {theme.label}
                  </span>
                  <span className="text-[10px] text-zinc-500 font-mono">Round {m.round}</span>
                </div>
                {m.summary && (
                  <div className="text-zinc-300 font-medium leading-relaxed">{m.summary}</div>
                )}
              </div>
            );
          })
        )}

        {isRunning && (
          <div className="flex items-center gap-2 p-2.5 rounded-lg bg-indigo-950/20 border border-indigo-900/40 text-indigo-400 text-xs">
            <span className="w-2 h-2 rounded-full bg-indigo-500 animate-ping"></span>
            <span>Swarm agents cross-evaluating and synthesizing...</span>
          </div>
        )}
      </div>

      {/* Input box (Cursor Composer style) */}
      <form onSubmit={handleSubmit} className="p-3 border-t border-[#242424] bg-[#161616]">
        <div className="bg-[#1e1e1e] border border-[#2c2c2c] focus-within:border-indigo-500/80 rounded-lg p-2 transition-colors">
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="Ask Swarm or describe a feature in Persian / English..."
            rows={3}
            className="w-full bg-transparent text-xs text-zinc-100 placeholder-zinc-500 outline-none resize-none leading-relaxed"
          />
          <div className="flex items-center justify-between pt-1 border-t border-zinc-800/80 mt-1">
            <div className="flex items-center gap-2 text-[10px] text-zinc-500">
              <span className="hover:text-zinc-300 cursor-pointer">@ files</span>
              <span className="hover:text-zinc-300 cursor-pointer"># skills</span>
            </div>
            <button
              type="submit"
              disabled={isRunning || !prompt.trim()}
              className="px-2.5 py-1 rounded bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white font-medium text-xs flex items-center gap-1 transition-colors"
            >
              <span>Generate</span>
              <Send size={11} />
            </button>
          </div>
        </div>
      </form>
    </aside>
  );
};
