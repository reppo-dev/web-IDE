import React, { useEffect, useState } from 'react';
import { X, FolderOpen, FolderPlus, RefreshCw, AlertTriangle } from 'lucide-react';
import { ProjectRef } from '../../types/project';

interface OpenProjectModalProps {
  isOpen: boolean;
  onClose: () => void;
  projects: ProjectRef[];
  activeProjectId: string;
  onOpenProject: (path: string) => Promise<ProjectRef>;
  onSelectProject: (project: ProjectRef) => Promise<void>;
  onForgetProject?: (id: string) => void;
}

/** Browsers cannot read a folder path directly, so we suggest common roots. */
const SUGGESTED_ROOTS = [
  { label: 'درایو G', path: 'G:\\' },
  { label: 'پوشه پروژه‌ها', path: 'G:\\projects' },
  { label: 'Documents', path: 'C:\\Users' },
];

export const OpenProjectModal: React.FC<OpenProjectModalProps> = ({
  isOpen,
  onClose,
  projects,
  activeProjectId,
  onOpenProject,
  onSelectProject,
  onForgetProject,
}) => {
  const [path, setPath] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (isOpen) {
      setError('');
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const open = async (target: string) => {
    const trimmed = target.trim();
    if (!trimmed) {
      setError('مسیر پوشه را وارد کنید.');
      return;
    }
    setBusy(true);
    setError('');
    try {
      const ref = await onOpenProject(trimmed);
      await onSelectProject(ref);
      onClose();
    } catch (e: any) {
      setError(e?.message || 'باز کردن پروژه ناموفق بود.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 select-none">
      <div className="bg-[#181818] border border-[#2e2e2e] rounded-xl w-full max-w-lg flex flex-col shadow-2xl overflow-hidden text-xs text-zinc-200">
        <div className="h-11 px-4 border-b border-[#262626] flex items-center justify-between bg-[#151515] shrink-0">
          <div className="flex items-center gap-2 font-bold text-sm">
            <FolderOpen size={15} className="text-indigo-400" />
            <span>باز کردن پروژه</span>
          </div>
          <button onClick={onClose} className="p-1 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded">
            <X size={15} />
          </button>
        </div>

        <div className="p-4 space-y-3 max-h-[70vh] overflow-y-auto">
          {/* Previously opened */}
          {projects.length > 0 && (
            <div className="space-y-1">
              <div className="text-[10px] uppercase tracking-wider text-zinc-500">پروژه‌های اخیر</div>
              {projects.map((p) => (
                <div key={p.id} className="group flex items-center gap-1">
                  <button
                    onClick={async () => {
                      setBusy(true);
                      try {
                        await onSelectProject(p);
                        onClose();
                      } catch (e: any) {
                        setError(e?.message || 'باز کردن ناموفق بود.');
                      } finally {
                        setBusy(false);
                      }
                    }}
                    className={`flex-1 text-left px-2.5 py-2 rounded flex items-center gap-2 ${
                      activeProjectId === p.id
                        ? 'bg-indigo-600 text-white'
                        : 'bg-[#1e1e1e] hover:bg-[#252525] text-zinc-300'
                    }`}
                  >
                    <FolderOpen size={12} className="shrink-0 opacity-70" />
                    <span className="truncate">{p.name}</span>
                    <span className="ml-auto text-[9px] opacity-60 truncate max-w-[45%]" dir="ltr">
                      {p.root}
                    </span>
                  </button>
                  {onForgetProject && (
                    <button
                      onClick={() => onForgetProject(p.id)}
                      className="p-1.5 text-zinc-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity"
                      title="حذف از فهرست (فایل‌ها را پاک نمی‌کند)"
                    >
                      <X size={11} />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* Type a path */}
          <div className="space-y-1.5">
            <label className="font-semibold text-zinc-300">مسیر پوشه را وارد کنید</label>
            <div className="flex gap-1.5">
              <input
                value={path}
                onChange={(e) => setPath(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') open(path);
                }}
                placeholder="G:\v\IDE"
                dir="ltr"
                className="flex-1 bg-[#1f1f1f] border border-[#2e2e2e] focus:border-indigo-500 rounded px-2.5 py-2 outline-none font-mono text-[11px]"
              />
              <button
                onClick={() => open(path)}
                disabled={busy}
                className="px-3 rounded bg-indigo-600 hover:bg-indigo-500 text-white flex items-center gap-1 disabled:opacity-50"
              >
                {busy ? <RefreshCw size={12} className="animate-spin" /> : <FolderPlus size={12} />}
              </button>
            </div>
            <p className="text-[10px] text-zinc-500 leading-relaxed">
              ایجنت‌ها روی همین پوشه کار می‌کنند و حافظه‌ی پروژه جداگانه برای همین
              مسیر نگه داشته می‌شود.
            </p>
          </div>

          {/* Quick suggestions */}
          <div className="space-y-1">
            <div className="text-[10px] uppercase tracking-wider text-zinc-500">مسیرهای پیشنهادی</div>
            <div className="flex flex-wrap gap-1.5">
              {SUGGESTED_ROOTS.map((r) => (
                <button
                  key={r.path}
                  onClick={() => open(r.path)}
                  disabled={busy}
                  className="px-2 py-1 rounded bg-[#1e1e1e] hover:bg-[#252525] text-zinc-400 text-[10px] font-mono disabled:opacity-50"
                >
                  {r.label}
                </button>
              ))}
            </div>
          </div>

          {error && (
            <div className="flex items-start gap-1.5 text-[10px] bg-red-950/40 border border-red-900/60 text-red-300 px-2 py-1.5 rounded">
              <AlertTriangle size={12} className="mt-px shrink-0" />
              <span>{error}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};