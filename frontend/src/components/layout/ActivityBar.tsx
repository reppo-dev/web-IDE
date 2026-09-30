import React from 'react';
import { 
  Files, 
  Search, 
  GitBranch, 
  Package, 
  BrainCircuit, 
  Scale, 
  Settings, 
  User 
} from 'lucide-react';

export type SidebarView = 'explorer' | 'search' | 'git' | 'skills' | 'memory' | 'arbiter' | null;

interface ActivityBarProps {
  activeView: SidebarView;
  onSelectView: (view: SidebarView) => void;
  onOpenSettings: () => void;
}

export const ActivityBar: React.FC<ActivityBarProps> = ({
  activeView,
  onSelectView,
  onOpenSettings,
}) => {
  const topButtons: { id: SidebarView; label: string; icon: any }[] = [
    { id: 'explorer', label: 'Explorer (Ctrl+Shift+E)', icon: Files },
    { id: 'search', label: 'Search (Ctrl+Shift+F)', icon: Search },
    { id: 'git', label: 'Source Control & Swarm Commits', icon: GitBranch },
    { id: 'skills', label: 'Skills & MCP Tools', icon: Package },
    { id: 'memory', label: 'Mem0 Persistent Memory', icon: BrainCircuit },
    { id: 'arbiter', label: 'Arbiter Consensus & Quality Matrix', icon: Scale },
  ];

  return (
    <aside className="w-12 bg-[#121212] border-r border-[#202020] flex flex-col justify-between items-center py-2 select-none z-20 shrink-0">
      {/* Top Activity Icons */}
      <div className="flex flex-col gap-1 w-full items-center">
        {topButtons.map((btn) => {
          const Icon = btn.icon;
          const isActive = activeView === btn.id;
          return (
            <button
              key={btn.id}
              onClick={() => onSelectView(isActive ? null : btn.id)}
              className={`w-9 h-9 rounded flex items-center justify-center transition-all relative group ${
                isActive
                  ? 'text-white bg-zinc-800/80'
                  : 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/40'
              }`}
              title={btn.label}
            >
              {isActive && (
                <div className="absolute left-0 top-1.5 bottom-1.5 w-0.5 bg-indigo-500 rounded-r"></div>
              )}
              <Icon size={18} strokeWidth={isActive ? 2 : 1.75} />
            </button>
          );
        })}
      </div>

      {/* Bottom Icons (Settings & Account) */}
      <div className="flex flex-col gap-1 w-full items-center">
        <button
          onClick={onOpenSettings}
          className="w-9 h-9 rounded flex items-center justify-center text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/40 transition-colors"
          title="Settings (API Keys, LLM Providers, Swarm Modes)"
        >
          <Settings size={18} strokeWidth={1.75} />
        </button>
        <button
          className="w-9 h-9 rounded flex items-center justify-center text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/40 transition-colors"
          title="Account / Swarm Status"
        >
          <User size={18} strokeWidth={1.75} />
        </button>
      </div>
    </aside>
  );
};
