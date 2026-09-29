import React from 'react';
import { FileCode, Folder, ChevronRight, ChevronDown, Plus, Trash2 } from 'lucide-react';
import { CodeFile } from '../../types';

interface FileExplorerProps {
  files: CodeFile[];
  activeFile: string;
  onSelectFile: (path: string) => void;
  onNewFile?: () => void;
  onDeleteFile?: (path: string) => void;
}

export const FileExplorer: React.FC<FileExplorerProps> = ({
  files,
  activeFile,
  onSelectFile,
  onNewFile,
  onDeleteFile,
}) => {
  return (
    <div className="w-64 bg-[#18181b] border-r border-[#27272a] flex flex-col h-full select-none">
      <div className="p-3 border-b border-[#27272a] flex items-center justify-between">
        <span className="text-xs font-semibold uppercase tracking-wider text-zinc-400">Project Files</span>
        {onNewFile && (
          <button
            onClick={onNewFile}
            className="p-1 hover:bg-zinc-800 rounded text-zinc-400 hover:text-zinc-200 transition-colors"
            title="New File"
          >
            <Plus size={14} />
          </button>
        )}
      </div>

      <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
        <div className="flex items-center gap-1.5 px-2 py-1 text-xs text-zinc-400 font-medium">
          <ChevronDown size={14} />
          <Folder size={14} className="text-indigo-400" />
          <span>src</span>
        </div>

        {files.length === 0 ? (
          <div className="p-4 text-center text-xs text-zinc-500">
            No files generated yet.<br />Run a swarm prompt to create files.
          </div>
        ) : (
          files.map((file) => {
            const isActive = activeFile === file.path;
            return (
              <div
                key={file.path}
                onClick={() => onSelectFile(file.path)}
                className={`flex items-center justify-between group px-2 py-1.5 rounded text-xs cursor-pointer transition-colors ${
                  isActive
                    ? 'bg-indigo-600/20 text-indigo-300 font-medium'
                    : 'text-zinc-300 hover:bg-zinc-800/60 hover:text-zinc-100'
                }`}
              >
                <div className="flex items-center gap-2 truncate">
                  <FileCode size={14} className={isActive ? 'text-indigo-400' : 'text-zinc-400'} />
                  <span className="truncate">{file.path}</span>
                </div>
                {onDeleteFile && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      onDeleteFile(file.path);
                    }}
                    className="opacity-0 group-hover:opacity-100 p-0.5 hover:text-red-400 text-zinc-500 transition-all"
                  >
                    <Trash2 size={12} />
                  </button>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};
