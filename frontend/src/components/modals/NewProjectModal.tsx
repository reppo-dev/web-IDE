import React, { useState } from 'react';
import { X, FolderPlus, Sparkles, Code, Check } from 'lucide-react';

interface NewProjectModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreateProject: (name: string, template: string) => void;
}

export const NewProjectModal: React.FC<NewProjectModalProps> = ({
  isOpen,
  onClose,
  onCreateProject,
}) => {
  const [projectName, setProjectName] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('go-microservice');

  if (!isOpen) return null;

  const templates = [
    { id: 'go-microservice', name: 'Go Microservice API', desc: 'Clean Architecture with HTTP server, middleware & tests.' },
    { id: 'react-app', name: 'React + TypeScript Webapp', desc: 'Vite, Tailwind CSS and component structure.' },
    { id: 'python-service', name: 'Python FastAPI Service', desc: 'Pydantic models, async endpoints and SQLite repository.' },
    { id: 'empty', name: 'Empty Project', desc: 'Start completely from scratch with Swarm prompts.' },
  ];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const finalName = projectName.trim() || 'my-nexus-project';
    onCreateProject(finalName, selectedTemplate);
    onClose();
  };

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4 select-none">
      <div className="bg-[#181818] border border-[#2e2e2e] rounded-xl w-full max-w-lg shadow-2xl overflow-hidden text-xs">
        {/* Header */}
        <div className="h-12 px-4 border-b border-[#262626] flex items-center justify-between bg-[#151515]">
          <div className="flex items-center gap-2 font-bold text-sm text-zinc-100">
            <FolderPlus size={16} className="text-indigo-400" />
            <span>Open or Create Project</span>
          </div>
          <button onClick={onClose} className="p-1 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded">
            <X size={16} />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div className="space-y-1">
            <label className="font-semibold text-zinc-300">Project Name:</label>
            <input
              type="text"
              value={projectName}
              onChange={(e) => setProjectName(e.target.value)}
              placeholder="e.g. distributed-auth-service"
              className="w-full bg-[#1e1e1e] border border-[#2a2a2a] focus:border-indigo-500 rounded p-2 text-zinc-200 outline-none text-xs"
              autoFocus
            />
          </div>

          <div className="space-y-2">
            <label className="font-semibold text-zinc-300">Starter Template:</label>
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
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-1.5 rounded bg-indigo-600 hover:bg-indigo-500 text-white font-medium"
            >
              Create & Open Project
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
