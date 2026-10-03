import React from 'react';
import { Plus } from 'lucide-react';
import { CodeFile } from '../../types';
import { ProjectTree } from './ProjectTree';

interface FileExplorerProps {
  files: CodeFile[];
  activeFile: string;
  onSelectFile: (path: string) => void;
  onNewFile?: () => void;
  onDeleteFile?: (path: string) => void;
  /** Root folder name, shown as the tree's top node. */
  rootName?: string;
}

/**
 * FileExplorer is a thin framed wrapper around ProjectTree. The tree itself is
 * shared with the sidebar so both views collapse and sort identically.
 */
export const FileExplorer: React.FC<FileExplorerProps> = ({
  files,
  activeFile,
  onSelectFile,
  onNewFile,
  onDeleteFile,
  rootName,
}) => {
  return (
    <div className="w-64 bg-[#18181b] border-r border-[#27272a] flex flex-col h-full select-none shrink-0">
      <div className="p-3 border-b border-[#27272a] flex items-center justify-between">
        <span className="text-xs font-semibold uppercase tracking-wider text-zinc-400 truncate">
          {rootName || 'Project'}
        </span>
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

      <div className="flex-1 overflow-y-auto px-1.5">
        {files.length === 0 ? (
          <div className="p-4 text-center text-xs text-zinc-500">
            پروژه‌ای باز نیست یا هنوز فایلی ندارد.
          </div>
        ) : (
          <ProjectTree
            files={files}
            activeFile={activeFile}
            onSelectFile={onSelectFile}
            onDeleteFile={onDeleteFile}
          />
        )}
      </div>
    </div>
  );
};