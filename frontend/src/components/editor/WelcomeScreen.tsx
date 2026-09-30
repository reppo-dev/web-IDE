import React from 'react';
import { 
  Sparkles, 
  FolderPlus, 
  Key, 
  Terminal, 
  BookOpen, 
  Bot, 
  ShieldCheck, 
  Scale 
} from 'lucide-react';

interface WelcomeScreenProps {
  onNewProject: () => void;
  onOpenSettings: () => void;
  onQuickPrompt: (prompt: string) => void;
}

export const WelcomeScreen: React.FC<WelcomeScreenProps> = ({
  onNewProject,
  onOpenSettings,
  onQuickPrompt,
}) => {
  const quickStarters = [
    { title: "High-Performance Redis Cache in Go", desc: "TTL eviction, mutex protection, and metrics." },
    { title: "Microservice Auth with JWT & Rate Limiting", desc: "Clean Architecture with repository pattern." },
    { title: "Real-time WebSocket Hub with Goroutines", desc: "Fan-out messaging and connection pooling." },
    { title: "سیستم کامل ثبت لاگ و احراز هویت توزیع‌شده", desc: "تولید کدهای سازگار با استاندارد امنیتی." },
  ];

  return (
    <div className="flex-1 flex flex-col items-center justify-center bg-[#161616] p-8 overflow-y-auto select-none">
      <div className="max-w-2xl w-full space-y-8">
        {/* Brand Banner */}
        <div className="text-center space-y-3">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-indigo-950/40 border border-indigo-800/40 text-indigo-400 text-xs font-medium">
            <Sparkles size={13} />
            <span>Multi-Agent Adversarial Synthesis</span>
          </div>
          <h1 className="text-3xl font-extrabold text-zinc-100 tracking-tight">
            Nexus<span className="text-indigo-500">AI</span>
          </h1>
          <p className="text-xs text-zinc-400 max-w-md mx-auto leading-relaxed">
            Autonomous software engineering environment powered by 5 adversarial AI agents. Code, critique, audit, and consensus in one unified workspace.
          </p>
        </div>

        {/* Primary Action Buttons */}
        <div className="flex items-center justify-center gap-3">
          <button
            onClick={onNewProject}
            className="flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white rounded-md text-xs font-semibold shadow-sm transition-all"
          >
            <FolderPlus size={14} />
            <span>New or Open Project</span>
          </button>
          <button
            onClick={onOpenSettings}
            className="flex items-center gap-2 px-4 py-2 bg-[#222222] hover:bg-[#282828] text-zinc-300 rounded-md text-xs font-medium border border-[#333333] transition-all"
          >
            <Key size={14} className="text-amber-400" />
            <span>Configure API Keys</span>
          </button>
        </div>

        {/* Quick Swarm Starters */}
        <div className="space-y-3">
          <div className="text-xs font-semibold text-zinc-500 uppercase tracking-wider text-center">
            Or Start Instantly with a Swarm Prompt:
          </div>
          <div className="grid grid-cols-2 gap-3">
            {quickStarters.map((item, idx) => (
              <div
                key={idx}
                onClick={() => onQuickPrompt(item.title)}
                className="p-3 bg-[#1e1e1e] hover:bg-[#252525] border border-[#2b2b2b] hover:border-indigo-500/50 rounded-lg cursor-pointer transition-all group"
              >
                <div className="text-xs font-bold text-zinc-200 group-hover:text-indigo-300 transition-colors">
                  {item.title}
                </div>
                <div className="text-[11px] text-zinc-500 mt-1 leading-normal">
                  {item.desc}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Swarm Agents Mini Roster */}
        <div className="pt-4 border-t border-[#262626] grid grid-cols-5 gap-2 text-center text-[11px] text-zinc-500">
          <div><strong className="text-blue-400">Architect</strong><br />Spec & Blueprint</div>
          <div><strong className="text-purple-400">Lead Coder</strong><br />Clean Code</div>
          <div><strong className="text-red-400">Critic</strong><br />Red Team Bugs</div>
          <div><strong className="text-emerald-400">Security</strong><br />OWASP Audit</div>
          <div><strong className="text-amber-400">Arbiter</strong><br />85% Consensus</div>
        </div>
      </div>
    </div>
  );
};
