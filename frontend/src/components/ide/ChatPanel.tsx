import React, { useState } from 'react';
import { Send, Sparkles, Bot, Shield, CheckCircle, Scale, Code2 } from 'lucide-react';
import { AgentMessage, AgentRole } from '../../types';

interface ChatPanelProps {
  messages: AgentMessage[];
  isRunning: boolean;
  onSendPrompt: (prompt: string) => void;
}

export const ChatPanel: React.FC<ChatPanelProps> = ({
  messages,
  isRunning,
  onSendPrompt,
}) => {
  const [prompt, setPrompt] = useState('');

  const suggestions = [
    "Distributed Auth with Redis & Rate Limiting in Go",
    "Real-time WebSocket Collaborative Whiteboard",
    "E-Commerce Payment Gateway with Idempotency",
    "سیستم احراز هویت توزیع‌شده با توکن JWT و ردیس",
  ];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!prompt.trim() || isRunning) return;
    onSendPrompt(prompt);
    setPrompt('');
  };

  const getAgentBadge = (role: AgentRole) => {
    switch (role) {
      case 'architect':
        return { label: 'Architect', color: 'text-blue-400 bg-blue-950/40 border-blue-800/40', icon: Bot };
      case 'coder':
        return { label: 'Lead Coder', color: 'text-purple-400 bg-purple-950/40 border-purple-800/40', icon: Code2 };
      case 'critic':
        return { label: 'Adversarial Critic', color: 'text-red-400 bg-red-950/40 border-red-800/40', icon: Sparkles };
      case 'security':
        return { label: 'Security Guard', color: 'text-emerald-400 bg-emerald-950/40 border-emerald-800/40', icon: Shield };
      case 'arbiter':
        return { label: 'Arbiter Judge', color: 'text-amber-400 bg-amber-950/40 border-amber-800/40', icon: Scale };
      default:
        return { label: role, color: 'text-zinc-400 bg-zinc-800 border-zinc-700', icon: Bot };
    }
  };

  return (
    <div className="w-80 bg-[#18181b] border-l border-[#27272a] flex flex-col h-full select-text">
      {/* Header */}
      <div className="p-3 border-b border-[#27272a] flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Bot size={16} className="text-indigo-400" />
          <span className="text-xs font-semibold text-zinc-200">Swarm Collaboration</span>
        </div>
        {isRunning && (
          <span className="flex items-center gap-1.5 text-[10px] text-indigo-400 animate-pulse font-medium">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-500"></span>
            Agents Thinking...
          </span>
        )}
      </div>

      {/* Message List */}
      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {messages.length === 0 ? (
          <div className="text-center py-6 px-2 space-y-3">
            <Sparkles size={28} className="mx-auto text-indigo-400 opacity-60" />
            <p className="text-xs text-zinc-400 font-medium">
              Start with a prompt in English or Persian. The 5-agent swarm will build, challenge, audit, and score it.
            </p>
            <div className="space-y-1.5 text-left pt-2">
              <span className="text-[10px] uppercase font-semibold text-zinc-500">Quick Scenarios:</span>
              {suggestions.map((s, idx) => (
                <button
                  key={idx}
                  onClick={() => onSendPrompt(s)}
                  className="w-full text-left text-[11px] p-2 rounded bg-zinc-900/60 hover:bg-zinc-800 text-zinc-300 transition-colors border border-zinc-800/60 line-clamp-1"
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        ) : (
          messages.map((m) => {
            const badge = getAgentBadge(m.from);
            const Icon = badge.icon;
            return (
              <div key={m.id} className="p-2.5 rounded-lg bg-zinc-900/80 border border-zinc-800/80 space-y-1.5 text-xs">
                <div className="flex items-center justify-between">
                  <span className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-medium border ${badge.color}`}>
                    <Icon size={11} />
                    {badge.label}
                  </span>
                  <span className="text-[10px] text-zinc-500 font-mono">Round {m.round}</span>
                </div>
                {m.summary && (
                  <p className="text-zinc-300 font-medium">{m.summary}</p>
                )}
              </div>
            );
          })
        )}
      </div>

      {/* Input Box */}
      <form onSubmit={handleSubmit} className="p-2.5 border-t border-[#27272a] bg-[#18181b]">
        <div className="relative">
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder="Type requirements (Persian or English)..."
            rows={2}
            className="w-full bg-[#141416] text-xs text-zinc-200 placeholder-zinc-500 p-2.5 pr-9 rounded border border-[#27272a] focus:border-indigo-500 outline-none resize-none"
          />
          <button
            type="submit"
            disabled={isRunning || !prompt.trim()}
            className="absolute right-2 bottom-2.5 p-1.5 rounded bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white transition-colors"
          >
            <Send size={13} />
          </button>
        </div>
      </form>
    </div>
  );
};
