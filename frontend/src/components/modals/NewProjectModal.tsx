import React, { useState, useRef } from 'react';
import { X, FolderOpen, FolderPlus, Sparkles, Code, Check, HardDrive, ArrowRight } from 'lucide-react';

interface NewProjectModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreateProject: (name: string, template: string) => void;
  onOpenFolder: (path: string) => void;
}

export const NewProjectModal: React.FC<NewProjectModalProps> = ({
  isOpen,
  onClose,
  onCreateProject,
  onOpenFolder,
}) => {
  const [activeTab, setActiveTab] = useState<'open' | 'create'>('open');
  const [folderPath, setFolderPath] = useState('g:\\v\\IDE');
  const [projectName, setProjectName] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('go-microservice');
  const fileInputRef = useRef<HTMLInputElement>(null);

  if (!isOpen) return null;

  const templates = [
    { id: 'go-microservice', name: 'Go Microservice API', desc: 'Clean Architecture with HTTP server, middleware & tests.' },
    { id: 'react-app', name: 'React + TypeScript Webapp', desc: 'Vite, Tailwind CSS and component structure.' },
    { id: 'python-service', name: 'Python FastAPI Service', desc: 'Pydantic models, async endpoints and SQLite repository.' },
    { id: 'empty', name: 'Empty Project', desc: 'Start completely from scratch with Swarm prompts.' },
  ];

  const handleOpenFolderSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!folderPath.trim()) return;
    onOpenFolder(folderPath.trim());
    onClose();
  };

  const handleCreateSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const finalName = projectName.trim() || 'my-nexus-project';
    onCreateProject(finalName, selectedTemplate);
    onClose();
  };

  const handleNativeFolderSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (files && files.length > 0) {
      // On webkitdirectory, files[0].webkitRelativePath has "foldername/file"
      const firstPath = files[0].webkitRelativePath || '';
      const topDir = firstPath.split('/')[0] || '';
      if (topDir) {
        setFolderPath(topDir);
        onOpenFolder(topDir);
        onClose();
      }
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 select-none">
      <div className="bg-[#181818] border border-[#2e2e2e] rounded-xl w-full max-w-lg shadow-2xl overflow-hidden text-xs">
        {/* Header */}
        <div className="h-12 px-4 border-b border-[#262626] flex items-center justify-between bg-[#151515]">
          <div className="flex items-center gap-2 font-bold text-sm text-zinc-100">
            <FolderOpen size={16} className="text-indigo-400" />
            <span>مدیریت و باز کردن پروژه (Project Workspace)</span>
          </div>
          <button onClick={onClose} className="p-1 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded">
            <X size={16} />
          </button>
        </div>

        {/* Tab Switcher */}
        <div className="flex border-b border-[#262626] bg-[#141414] px-4 pt-2 gap-2">
          <button
            type="button"
            onClick={() => setActiveTab('open')}
            className={`flex items-center gap-1.5 pb-2 px-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'open'
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <HardDrive size={13} />
            <span>باز کردن پوشه از سیستم (Open Local Folder)</span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('create')}
            className={`flex items-center gap-1.5 pb-2 px-2 text-xs font-semibold border-b-2 transition-colors ${
              activeTab === 'create'
                ? 'border-indigo-500 text-indigo-400'
                : 'border-transparent text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <FolderPlus size={13} />
            <span>ساخت پروژه جدید (New Project)</span>
          </button>
        </div>

        {/* Body */}
        {activeTab === 'open' ? (
          <form onSubmit={handleOpenFolderSubmit} className="p-6 space-y-4">
            <div className="space-y-1">
              <label className="font-semibold text-zinc-300 block">
                مسیر پوشه روی سیستم (Path):
              </label>
              <input
                type="text"
                value={folderPath}
                onChange={(e) => setFolderPath(e.target.value)}
                placeholder="مثال: g:\v\IDE یا C:\Projects\my-app"
                className="w-full bg-[#1e1e1e] border border-[#2a2a2a] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none text-xs font-mono"
                autoFocus
              />
              <p className="text-[11px] text-zinc-500 leading-relaxed pt-1">
                تیم هوش مصنوعی فایل‌های موجود در این پوشه را بررسی، درک و کدهای جدید را روی آن اعمال می‌کنند.
              </p>
            </div>

            {/* Quick Paths */}
            <div className="space-y-1.5 pt-1">
              <span className="text-[10px] text-zinc-500 font-semibold block">مسیرهای سریع:</span>
              <div className="flex flex-wrap gap-1.5">
                {['g:\\v\\IDE', '.', 'workspaces/demo'].map((p) => (
                  <button
                    key={p}
                    type="button"
                    onClick={() => setFolderPath(p)}
                    className="px-2 py-1 rounded bg-[#222222] hover:bg-zinc-700 text-zinc-300 text-[10px] font-mono border border-zinc-700/60"
                  >
                    {p}
                  </button>
                ))}
              </div>
            </div>

            {/* Native Folder Browser Hidden Input */}
            <input
              type="file"
              ref={fileInputRef}
              onChange={handleNativeFolderSelect}
              // @ts-ignore
              webkitdirectory=""
              directory=""
              className="hidden"
            />

            <div className="pt-3 border-t border-[#262626] flex items-center justify-between">
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="px-3 py-1.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-300 flex items-center gap-1.5 transition-colors"
              >
                <FolderOpen size={13} className="text-zinc-400" />
                <span>جستجو در سیستم (Browse...)</span>
              </button>

              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={onClose}
                  className="px-3 py-1.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-400"
                >
                  انصراف
                </button>
                <button
                  type="submit"
                  disabled={!folderPath.trim()}
                  className="px-4 py-1.5 rounded bg-indigo-600 hover:bg-indigo-500 text-white font-medium flex items-center gap-1.5 disabled:opacity-40"
                >
                  <span>باز کردن این پروژه</span>
                  <ArrowRight size={13} />
                </button>
              </div>
            </div>
          </form>
        ) : (
          <form onSubmit={handleCreateSubmit} className="p-6 space-y-4">
            <div className="space-y-1">
              <label className="font-semibold text-zinc-300">نام پروژه:</label>
              <input
                type="text"
                value={projectName}
                onChange={(e) => setProjectName(e.target.value)}
                placeholder="مثال: distributed-auth-service"
                className="w-full bg-[#1e1e1e] border border-[#2a2a2a] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none text-xs"
                autoFocus
              />
            </div>

            <div className="space-y-2">
              <label className="font-semibold text-zinc-300">قالب اولیه (Starter Template):</label>
              <div className="space-y-2">
                {templates.map((tpl) => (
                  <div
                    key={tpl.id}
                    onClick={() => setSelectedTemplate(tpl.id)}
                    className={`p-3 rounded-lg border cursor-pointer transition-all flex items-start justify-between ${
                      selectedTemplate === tpl.id
                        ? 'bg-indigo-950/30 border-indigo-500/80 text-zinc-100'
                        : 'bg-[#1e1e1e] border-[#292929] hover:bg-zinc-800 text-zinc-400'
                    }`}
                  >
                    <div>
                      <div className="font-bold text-zinc-200">{tpl.name}</div>
                      <div className="text-[11px] text-zinc-500 mt-0.5">{tpl.desc}</div>
                    </div>
                    {selectedTemplate === tpl.id && (
                      <Check size={14} className="text-indigo-400 mt-1 shrink-0" />
                    )}
                  </div>
                ))}
              </div>
            </div>

            <div className="pt-2 flex justify-end gap-2">
              <button
                type="button"
                onClick={onClose}
                className="px-3 py-1.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-300"
              >
                انصراف
              </button>
              <button
                type="submit"
                className="px-4 py-1.5 rounded bg-indigo-600 hover:bg-indigo-500 text-white font-medium"
              >
                ساخت و باز کردن پروژه
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
};
