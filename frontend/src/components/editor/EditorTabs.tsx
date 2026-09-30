import React from 'react';
import { X, FileCode, Split, Diff } from 'lucide-react';
import { CodeFile } from '../../types';

interface EditorTabsProps {
  openFiles: string[];
  activeFile: string;
  files: CodeFile[];
  onSelectTab: (path: string) => void;
  onCloseTab: (path: string) => void;
  showDiff: boolean;
  onToggleDiff: () => void;
}

export const EditorTabs: React.FC<EditorTabsProps> = ({
  openFiles,
  activeFile,
  files,
  onSelectTab,
  onCloseTab,
  showDiff,
  onToggleDiff,
}) => {
  return (
    <div className="h-9 bg-[#181818] border-b border-[#242424] flex items-center justify-between select-none z-10 shrink-0">
      {/* Tabs list */}
      <div className="flex items-center h-full overflow-x-auto">
        {openFiles.map((path) => {
          const isActive = activeFile === path;
          const fileName = path.split('/').pop() || path;
          return (
            <div
              key={path}
              onClick={() => onSelectTab(path)}
              className={`h-full flex items-center gap-2 px-3 border-r border-[#242424] cursor-pointer text-xs font-mono transition-colors group ${
                isActive
                  ? 'bg-[#1e1e1e] text-zinc-100 border-t-2 border-t-indigo-500'
                  : 'bg-[#141414] text-zinc-500 hover:bg-[#191919] hover:text-zinc-300'
              }`}
            >
              <FileCode size={13} className={isActive ? 'text-indigo-400' : 'text-zinc-500'} />
              <span className="truncate max-w-[140px]">{fileName}</span>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  onCloseTab(path);
                }}
                className="opacity-0 group-hover:opacity-100 hover:text-zinc-100 hover:bg-zinc-700/60 p-0.5 rounded transition-opacity"
              >
                <X size={11} />
              </button>
            </div>
          );
        })}
      </div>

      {/* Action buttons (Diff toggle / Split) */}
      <div className="flex items-center gap-1 px-2 text-zinc-500">
        <button
          onClick={onToggleDiff}
          className={`flex items-center gap-1 text-[11px] px-2 py-0.5 rounded transition-colors ${
            showDiff ? 'bg-indigo-600/30 text-indigo-400 font-medium' : 'hover:text-zinc-200'
          }`}
          title="Toggle Adversarial Diff View"
        >
          <Diff size={13} />
          <span className="hidden sm:inline">Diff</span>
        </button>
      </div>
    </div>
  );
};
