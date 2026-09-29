import React, { useState } from 'react';
import { Bot, Code2, Sparkles, Shield, Scale, ArrowRight, CheckCircle2, ChevronRight } from 'lucide-react';
import { AgentMessage, ArbiterVerdict, AgentRole } from '../../types';

interface AgentArenaViewProps {
  messages: AgentMessage[];
  verdict: ArbiterVerdict | null;
  currentRound: number;
}

export const AgentArenaView: React.FC<AgentArenaViewProps> = ({
  messages,
  verdict,
  currentRound,
}) => {
  const [selectedRound, setSelectedRound] = useState<number>(currentRound || 1);

  const agents: { role: AgentRole; label: string; desc: string; icon: any; color: string; bg: string }[] = [
    { role: 'architect', label: 'Architect', desc: 'System Blueprint & File Tree', icon: Bot, color: 'text-blue-400', bg: 'border-blue-800/40 bg-blue-950/20' },
    { role: 'coder', label: 'Lead Coder', desc: 'Code Generation & Refactoring', icon: Code2, color: 'text-purple-400', bg: 'border-purple-800/40 bg-purple-950/20' },
    { role: 'critic', label: 'Adversarial Critic', desc: 'Logic Flaws & Red Team Bugs', icon: Sparkles, color: 'text-red-400', bg: 'border-red-800/40 bg-red-950/20' },
    { role: 'security', label: 'Security Auditor', desc: 'Vulnerability Scan & Attack Surface', icon: Shield, color: 'text-emerald-400', bg: 'border-emerald-800/40 bg-emerald-950/20' },
    { role: 'arbiter', label: 'Arbiter Judge', desc: 'Consensus & Quality Scoring', icon: Scale, color: 'text-amber-400', bg: 'border-amber-800/40 bg-amber-950/20' },
  ];

  // Filter messages for selected round
  const roundMessages = messages.filter(m => m.round === selectedRound);

  return (
    <div className="flex-1 flex flex-col h-full bg-[#141416] overflow-hidden p-6 space-y-6">
      {/* Top Swarm Header & Round Switcher */}
      <div className="flex items-center justify-between border-b border-[#27272a] pb-4">
        <div>
          <h2 className="text-lg font-bold text-zinc-100 flex items-center gap-2">
            <span>Agent Arena & Debate Flow</span>
            {verdict?.passed && (
              <span className="flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-emerald-950/60 border border-emerald-800/60 text-emerald-400 font-medium">
                <CheckCircle2 size={12} />
                Consensus Reached ({verdict.final_score.toFixed(1)}%)
              </span>
            )}
          </h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Adversarial cross-examination loop between generative, critical, and auditing agents.
          </p>
        </div>

        {/* Round Switcher */}
        <div className="flex items-center gap-1 bg-[#18181b] p-1 rounded-lg border border-[#27272a]">
          {[1, 2, 3].map((r) => (
            <button
              key={r}
              onClick={() => setSelectedRound(r)}
              className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                selectedRound === r
                  ? 'bg-indigo-600 text-white'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              Round {r}
            </button>
          ))}
        </div>
      </div>

      {/* 5 Agent Persona Cards */}
      <div className="grid grid-cols-5 gap-3">
        {agents.map((ag) => {
          const Icon = ag.icon;
          const hasMessage = roundMessages.some(m => m.from === ag.role);
          return (
            <div
              key={ag.role}
              className={`p-3 rounded-lg border flex flex-col justify-between ${ag.bg} transition-all`}
            >
              <div className="flex items-center justify-between">
                <Icon size={18} className={ag.color} />
                <span className={`w-2 h-2 rounded-full ${hasMessage ? 'bg-emerald-400 animate-pulse' : 'bg-zinc-600'}`}></span>
              </div>
              <div className="mt-3">
                <div className={`text-xs font-bold ${ag.color}`}>{ag.label}</div>
                <div className="text-[10px] text-zinc-400 leading-tight mt-0.5">{ag.desc}</div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Debate Stream and Arguments */}
      <div className="flex-1 bg-[#18181b] rounded-lg border border-[#27272a] p-4 overflow-y-auto space-y-4">
        <div className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">
          Round {selectedRound} Argument Ledger:
        </div>

        {roundMessages.length === 0 ? (
          <div className="text-center py-12 text-zinc-500 text-xs">
            No agent arguments recorded in Round {selectedRound} yet.
          </div>
        ) : (
          roundMessages.map((msg, i) => {
            const agentCfg = agents.find(a => a.role === msg.from);
            const Icon = agentCfg?.icon || Bot;
            return (
              <div key={i} className="flex gap-3 text-xs">
                <div className="shrink-0 mt-1">
                  <div className={`w-7 h-7 rounded-md flex items-center justify-center border ${agentCfg?.bg}`}>
                    <Icon size={14} className={agentCfg?.color} />
                  </div>
                </div>
                <div className="flex-1 bg-[#141416] p-3 rounded-lg border border-[#27272a] space-y-1.5">
                  <div className="flex items-center justify-between">
                    <span className={`font-bold capitalize ${agentCfg?.color}`}>{msg.from}</span>
                    <span className="text-[10px] text-zinc-500 font-mono">
                      {new Date(msg.timestamp).toLocaleTimeString()}
                    </span>
                  </div>
                  {msg.summary && (
                    <div className="text-zinc-200 font-medium">{msg.summary}</div>
                  )}
                  {msg.content && (
                    <div className="bg-zinc-900/80 p-2 rounded text-[11px] font-mono text-zinc-300 overflow-x-auto max-h-48">
                      {typeof msg.content === 'string'
                        ? msg.content
                        : JSON.stringify(msg.content, null, 2)}
                    </div>
                  )}
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};
