import React, { useState } from 'react';
import { 
  FolderPlus, 
  FilePlus, 
  RefreshCw, 
  ChevronRight, 
  ChevronDown, 
  FileCode, 
  Folder, 
  Trash2, 
  GitCommit, 
  Package, 
  BrainCircuit, 
  Scale, 
  Check, 
  Plus, 
  Search 
} from 'lucide-react';
import { SidebarView } from './ActivityBar';
import { CodeFile, MemoryEntry, SkillManifest, ArbiterVerdict } from '../../types';

interface SidebarProps {
  view: SidebarView;
  projectName: string | null;
  files: CodeFile[];
  activeFile: string;
  onSelectFile: (path: string) => void;
  onNewFile: () => void;
  onNewProject: () => void;
  onDeleteFile?: (path: string) => void;
  memories: MemoryEntry[];
  skills: SkillManifest[];
  verdict: ArbiterVerdict | null;
}

export const Sidebar: React.FC<SidebarProps> = ({
  view,
  projectName,
  files,
  activeFile,
  onSelectFile,
  onNewFile,
  onNewProject,
  onDeleteFile,
  memories,
  skills,
  verdict,
}) => {
  const [memorySearch, setMemorySearch] = useState('');

  if (!view) return null;

  return (
    <aside className="w-64 bg-[#181818] border-r border-[#242424] flex flex-col h-full select-none shrink-0 overflow-hidden text-xs">
      {/* Sidebar Header */}
      <div className="h-9 px-3 border-b border-[#242424] flex items-center justify-between text-zinc-400 font-semibold tracking-wider text-[11px] uppercase">
        <span className="truncate">
          {view === 'explorer' && (projectName ? projectName : 'Explorer')}
          {view === 'search' && 'Search'}
          {view === 'git' && 'Source Control'}
          {view === 'skills' && 'Skills & MCP'}
          {view === 'memory' && 'Mem0 Memory'}
          {view === 'arbiter' && 'Arbiter Audit'}
        </span>

        {view === 'explorer' && projectName && (
          <div className="flex items-center gap-1">
            <button onClick={onNewFile} className="p-1 hover:text-zinc-200 hover:bg-zinc-800 rounded" title="New File">
              <FilePlus size={13} />
            </button>
            <button onClick={onNewProject} className="p-1 hover:text-zinc-200 hover:bg-zinc-800 rounded" title="Switch Project">
              <FolderPlus size={13} />
            </button>
          </div>
        )}
      </div>

      {/* View Content */}
      <div className="flex-1 overflow-y-auto p-2">
        {/* VIEW 1: EXPLORER */}
        {view === 'explorer' && (
          <div>
            {!projectName ? (
              // Empty State - Cursor Style
              <div className="p-4 flex flex-col items-center text-center space-y-3 pt-8">
                <div className="w-10 h-10 rounded-full bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-500">
                  <Folder size={18} />
                </div>
                <div className="text-zinc-300 font-medium text-xs">No Folder Opened</div>
                <p className="text-[11px] text-zinc-500 leading-relaxed">
                  You have not opened a folder or created a project yet.
                </p>
                <div className="w-full space-y-2 pt-2">
                  <button
                    onClick={onNewProject}
                    className="w-full py-1.5 px-3 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-xs font-medium transition-colors shadow-sm"
                  >
                    Open or Create Project
                  </button>
                </div>
              </div>
            ) : files.length === 0 ? (
              <div className="p-4 text-center text-zinc-500 text-xs">
                No files created yet.<br />Use Cursor Composer (right panel) to let the swarm generate files.
              </div>
            ) : (
              <div className="space-y-0.5">
                <div className="flex items-center gap-1 px-2 py-1 text-zinc-400 font-semibold text-[11px]">
                  <ChevronDown size={13} />
                  <span>SRC</span>
                </div>
                {files.map((file) => {
                  const isActive = activeFile === file.path;
                  return (
                    <div
                      key={file.path}
                      onClick={() => onSelectFile(file.path)}
                      className={`flex items-center justify-between group px-2 py-1.5 rounded cursor-pointer transition-colors ${
                        isActive
                          ? 'bg-zinc-800 text-zinc-100 font-medium border-l-2 border-indigo-500'
                          : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200'
                      }`}
                    >
                      <div className="flex items-center gap-1.5 truncate">
                        <FileCode size={13} className={isActive ? 'text-indigo-400' : 'text-zinc-500'} />
                        <span className="truncate">{file.path}</span>
                      </div>
                      {onDeleteFile && (
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            onDeleteFile(file.path);
                          }}
                          className="opacity-0 group-hover:opacity-100 p-0.5 text-zinc-500 hover:text-red-400 transition-opacity"
                        >
                          <Trash2 size={11} />
                        </button>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {/* VIEW 2: SEARCH */}
        {view === 'search' && (
          <div className="space-y-2 p-1">
            <input
              type="text"
              placeholder="Search across files..."
              className="w-full bg-[#1e1e1e] text-xs text-zinc-200 px-2.5 py-1.5 rounded border border-[#2a2a2a] outline-none focus:border-indigo-500"
            />
            <div className="text-[11px] text-zinc-500 text-center py-4">No search results yet.</div>
          </div>
        )}

        {/* VIEW 3: GIT SOURCE CONTROL */}
        {view === 'git' && (
          <div className="space-y-3">
            <div className="text-[11px] text-zinc-500 font-semibold uppercase">Swarm Auto-Commit Log</div>
            <div className="space-y-2 font-mono text-[11px]">
              <div className="p-2 bg-zinc-900/80 rounded border border-zinc-800/80 space-y-1">
                <div className="flex items-center gap-1.5 text-indigo-400 font-medium">
                  <GitCommit size={13} />
                  <span>Round 1 Consensus</span>
                </div>
                <div className="text-zinc-400 text-[10px]">Score: 91.5% | Passed threshold</div>
              </div>
            </div>
          </div>
        )}

        {/* VIEW 4: SKILLS & MCP */}
        {view === 'skills' && (
          <div className="space-y-3">
            <div className="text-[11px] text-zinc-500 font-semibold uppercase">Installed Swarm Skills</div>
            {skills.map((s) => (
              <div key={s.name} className="p-2.5 bg-zinc-900/60 rounded border border-zinc-800/80 space-y-1">
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-zinc-200">{s.name}</span>
                  <span className="text-[9px] px-1 bg-emerald-950/60 text-emerald-400 rounded">Active</span>
                </div>
                <p className="text-[11px] text-zinc-400 leading-tight">{s.description}</p>
              </div>
            ))}
          </div>
        )}

        {/* VIEW 5: MEMORY (MEM0) */}
        {view === 'memory' && (
          <div className="space-y-3">
            <div className="relative">
              <Search size={12} className="absolute left-2.5 top-2 text-zinc-500" />
              <input
                type="text"
                value={memorySearch}
                onChange={(e) => setMemorySearch(e.target.value)}
                placeholder="Filter memories..."
                className="w-full bg-[#1e1e1e] text-xs text-zinc-200 pl-7 pr-2 py-1.5 rounded border border-[#2a2a2a] outline-none focus:border-indigo-500"
              />
            </div>
            <div className="space-y-2">
              {memories.map((m) => (
                <div key={m.id} className="p-2 bg-zinc-900/70 rounded border border-zinc-800/80 space-y-1">
                  <div className="flex items-center justify-between text-[10px]">
                    <span className="font-semibold text-indigo-400 uppercase">{m.type}</span>
                    <span className="text-zinc-500 font-mono">{(m.confidence * 100).toFixed(0)}%</span>
                  </div>
                  <p className="text-[11px] text-zinc-300 leading-tight">{m.content}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* VIEW 6: ARBITER AUDIT */}
        {view === 'arbiter' && (
          <div className="space-y-3 p-1">
            <div className="text-[11px] text-zinc-500 font-semibold uppercase">Consensus Verification</div>
            <div className="p-3 bg-zinc-900/80 rounded border border-zinc-800/80 space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-zinc-300 font-medium">Quality Score</span>
                <span className="font-mono font-bold text-emerald-400 text-sm">
                  {verdict ? verdict.final_score.toFixed(1) : '91.5'}%
                </span>
              </div>
              <div className="w-full bg-zinc-800 h-1.5 rounded-full overflow-hidden">
                <div className="bg-emerald-500 h-full w-[91%]"></div>
              </div>
              <div className="text-[10px] text-zinc-400 pt-1">
                Threshold: 85.0% (Status: Approved for Merge)
              </div>
            </div>
          </div>
        )}
      </div>
    </aside>
  );
};
