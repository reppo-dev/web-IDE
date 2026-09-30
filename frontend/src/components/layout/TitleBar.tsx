import React from 'react';
import { 
  Minus, 
  Square, 
  X, 
  Search, 
  Settings, 
  PanelLeft, 
  PanelBottom, 
  PanelRight, 
  Bot 
} from 'lucide-react';

interface TitleBarProps {
  projectName: string;
  onOpenSettings: () => void;
  showLeftSidebar: boolean;
  onToggleLeftSidebar: () => void;
  showBottomPanel: boolean;
  onToggleBottomPanel: () => void;
  showComposer: boolean;
  onToggleComposer: () => void;
  onOpenCommandPalette?: () => void;
}

export const TitleBar: React.FC<TitleBarProps> = ({
  projectName,
  onOpenSettings,
  showLeftSidebar,
  onToggleLeftSidebar,
  showBottomPanel,
  onToggleBottomPanel,
  showComposer,
  onToggleComposer,
  onOpenCommandPalette,
}) => {
  return (
    <header className="h-9 bg-[#141414] border-b border-[#242424] flex items-center justify-between px-3 select-none z-30 text-xs text-zinc-400">
      {/* Left: Window Menu & Logo */}
      <div className="flex items-center gap-3">
        <div className="flex items-center gap-1.5 font-semibold text-zinc-200">
          <div className="w-4 h-4 rounded bg-indigo-600 flex items-center justify-center text-[10px] font-black text-white shadow-sm">
            N
          </div>
          <span className="text-zinc-300 font-medium">NexusAI</span>
          <span className="text-[10px] text-zinc-600 font-mono">Cursor Edition</span>
        </div>

        <div className="hidden md:flex items-center gap-3 text-[11px] text-zinc-400 ml-2">
          <span className="hover:text-zinc-200 cursor-pointer">File</span>
          <span className="hover:text-zinc-200 cursor-pointer">Edit</span>
          <span className="hover:text-zinc-200 cursor-pointer">Selection</span>
          <span className="hover:text-zinc-200 cursor-pointer">View</span>
          <span className="hover:text-zinc-200 cursor-pointer">Swarm</span>
          <span className="hover:text-zinc-200 cursor-pointer">Terminal</span>
        </div>
      </div>

      {/* Center: Command Palette / Search Pill (Cursor Style) */}
      <div 
        onClick={onOpenCommandPalette}
        className="flex items-center gap-2 bg-[#1e1e1e] hover:bg-[#252525] border border-[#2d2d2d] px-4 py-1 rounded-md cursor-pointer text-zinc-400 text-[11px] transition-colors w-72 md:w-96 justify-center shadow-inner"
      >
        <Search size={12} className="text-zinc-500" />
        <span className="truncate">{projectName ? `${projectName} — ` : ''}Search files, commands (Ctrl + P)</span>
      </div>

      {/* Right: Layout toggles & Settings & Window controls */}
      <div className="flex items-center gap-2">
        <div className="flex items-center gap-1 bg-[#1e1e1e] p-0.5 rounded border border-[#282828]">
          <button
            onClick={onToggleLeftSidebar}
            className={`p-1 rounded hover:text-zinc-200 transition-colors ${showLeftSidebar ? 'text-indigo-400 bg-zinc-800' : 'text-zinc-500'}`}
            title="Toggle Primary Sidebar"
          >
            <PanelLeft size={13} />
          </button>
          <button
            onClick={onToggleBottomPanel}
            className={`p-1 rounded hover:text-zinc-200 transition-colors ${showBottomPanel ? 'text-indigo-400 bg-zinc-800' : 'text-zinc-500'}`}
            title="Toggle Bottom Terminal Panel"
          >
            <PanelBottom size={13} />
          </button>
          <button
            onClick={onToggleComposer}
            className={`p-1 rounded hover:text-zinc-200 transition-colors ${showComposer ? 'text-indigo-400 bg-zinc-800' : 'text-zinc-500'}`}
            title="Toggle Cursor Composer Panel"
          >
            <PanelRight size={13} />
          </button>
        </div>

        <button
          onClick={onOpenSettings}
          className="p-1 hover:bg-zinc-800 rounded text-zinc-400 hover:text-zinc-200 transition-colors"
          title="Settings (API Keys, Models, Swarm)"
        >
          <Settings size={14} />
        </button>

        <div className="hidden lg:flex items-center gap-1 ml-2 border-l border-[#262626] pl-2 text-zinc-500">
          <button className="p-1 hover:text-zinc-200 hover:bg-zinc-800 rounded"><Minus size={11} /></button>
          <button className="p-1 hover:text-zinc-200 hover:bg-zinc-800 rounded"><Square size={10} /></button>
          <button className="p-1 hover:text-red-400 hover:bg-zinc-800 rounded"><X size={12} /></button>
        </div>
      </div>
    </header>
  );
};
