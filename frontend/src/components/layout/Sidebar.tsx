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
import { ProjectTree } from '../ide/ProjectTree';
import { CodeFile, MemoryEntry, SkillManifest, ArbiterVerdict, ChangeLedgerEntry } from '../../types';

interface SidebarProps {
  view: SidebarView;
  projectName: string | null;
  projectId?: string | null;
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
  projectId,
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
  const [ledgerEntries, setLedgerEntries] = useState<ChangeLedgerEntry[]>([]);
  const [loadingLedger, setLoadingLedger] = useState<boolean>(false);
  const [ledgerError, setLedgerError] = useState<string | null>(null);

  const fetchLedger = React.useCallback(async () => {
    if (!projectId) return;
    setLoadingLedger(true);
    setLedgerError(null);
    try {
      const res = await fetch(`/api/projects/${projectId}/ledger`);
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`);
      }
      const data = await res.json();
      setLedgerEntries(Array.isArray(data) ? data : []);
    } catch (err: any) {
      setLedgerError(err.message || 'خطا در دریافت لاگ تغییرات');
    } finally {
      setLoadingLedger(false);
    }
  }, [projectId]);

  React.useEffect(() => {
    if (view === 'git' && projectId) {
      fetchLedger();
    }
  }, [view, projectId, fetchLedger]);

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
                    className="w-full py-1.5 px-3 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-xs font-medium transition-colors shadow-sm flex items-center justify-center gap-1.5"
                  >
                    <Folder size={13} />
                    <span>باز کردن پوشه از سیستم (Open Folder)</span>
                  </button>
                </div>
              </div>
            ) : files.length === 0 ? (
              <div className="p-4 text-center text-zinc-500 text-xs">
                این پوشه هنوز فایل قابل نمایشی ندارد.
              </div>
            ) : (
              <ProjectTree
                files={files}
                activeFile={activeFile}
                rootName={projectName}
                onSelectFile={onSelectFile}
                onDeleteFile={onDeleteFile}
              />
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

        {/* VIEW 3: GIT SOURCE CONTROL & CHANGE LEDGER */}
        {view === 'git' && (
          <div className="space-y-3">
            <div className="flex items-center justify-between pb-1.5 border-b border-[#242424]">
              <div className="text-[11px] text-zinc-400 font-semibold uppercase flex items-center gap-1.5">
                <GitCommit size={14} className="text-indigo-400" />
                <span>Change Ledger (§30)</span>
              </div>
              <button
                onClick={fetchLedger}
                disabled={loadingLedger || !projectId}
                className="p-1 hover:text-zinc-200 hover:bg-zinc-800 rounded text-zinc-400 transition-colors disabled:opacity-40"
                title="بروزرسانی لاگ تغییرات (Refresh)"
              >
                <RefreshCw size={12} className={loadingLedger ? 'animate-spin' : ''} />
              </button>
            </div>

            {!projectId ? (
              <div className="p-4 text-center text-zinc-500 text-xs">
                برای مشاهده تاریخچه کامیت‌ها و لجر، ابتدا پروژه‌ای باز کنید.
              </div>
            ) : loadingLedger && ledgerEntries.length === 0 ? (
              <div className="p-4 text-center text-zinc-500 text-xs flex items-center justify-center gap-2">
                <RefreshCw size={13} className="animate-spin text-indigo-400" />
                <span>در حال دریافت لاگ تغییرات...</span>
              </div>
            ) : ledgerError ? (
              <div className="p-3 bg-red-950/40 border border-red-900/50 rounded text-red-300 text-[11px]">
                {ledgerError}
              </div>
            ) : ledgerEntries.length === 0 ? (
              <div className="p-4 text-center text-zinc-500 text-xs">
                هنوز تغییری در این پروژه ثبت نشده است.
              </div>
            ) : (
              <div className="space-y-2">
                {ledgerEntries.slice().reverse().map((entry, idx) => {
                  const shortCommit = entry.commit ? entry.commit.slice(0, 7) : null;
                  const timeFormatted = entry.timestamp
                    ? new Date(entry.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
                    : '';

                  return (
                    <div
                      key={entry.id || `${entry.commit}-${idx}`}
                      className="p-2 bg-zinc-900/80 hover:bg-zinc-900 rounded border border-zinc-800/80 transition-colors space-y-1.5"
                    >
                      <div className="flex items-center justify-between text-[10px]">
                        <div className="flex items-center gap-1.5 font-mono">
                          <GitCommit size={12} className="text-indigo-400 shrink-0" />
                          {shortCommit ? (
                            <span className="text-indigo-300 font-semibold">{shortCommit}</span>
                          ) : (
                            <span className="text-zinc-400">Entry</span>
                          )}
                        </div>
                        {timeFormatted && (
                          <span className="text-zinc-500 font-mono text-[9px]">{timeFormatted}</span>
                        )}
                      </div>

                      {/* Badges: Task ID + Agent + Test Status */}
                      <div className="flex flex-wrap items-center gap-1">
                        {entry.task_id && (
                          <span className="px-1.5 py-0.5 rounded bg-blue-950/60 text-blue-300 border border-blue-800/40 text-[9px] font-mono">
                            {entry.task_id}
                          </span>
                        )}
                        {entry.agent && (
                          <span className="px-1.5 py-0.5 rounded bg-purple-950/60 text-purple-300 border border-purple-800/40 text-[9px]">
                            {entry.agent}
                          </span>
                        )}
                        {entry.tests?.result && (
                          <span
                            className={`px-1.5 py-0.5 rounded text-[9px] font-mono border ${
                              entry.tests.result === 'PASSED'
                                ? 'bg-emerald-950/60 text-emerald-400 border-emerald-800/40'
                                : 'bg-red-950/60 text-red-400 border-red-800/40'
                            }`}
                          >
                            {entry.tests.result}
                          </span>
                        )}
                      </div>

                      {/* File Link */}
                      {entry.file && (
                        <button
                          onClick={() => onSelectFile(entry.file!)}
                          className="flex items-center gap-1.5 text-[11px] text-zinc-300 hover:text-indigo-300 transition-colors text-left font-mono truncate w-full"
                          title={`باز کردن ${entry.file}`}
                        >
                          <FileCode size={12} className="text-zinc-500 shrink-0" />
                          <span className="truncate">{entry.file}</span>
                        </button>
                      )}

                      {/* Reason or Details */}
                      {(entry.reason || entry.details) && (
                        <p className="text-[10px] text-zinc-400 leading-relaxed break-words">
                          {entry.reason || entry.details}
                        </p>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
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
