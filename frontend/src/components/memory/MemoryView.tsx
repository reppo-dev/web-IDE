import React, { useState } from 'react';
import { Database, Search, Tag, Bot, Plus, Trash2, BrainCircuit } from 'lucide-react';
import { MemoryEntry } from '../../types';

interface MemoryViewProps {
  projectMemories: MemoryEntry[];
  globalMemories: MemoryEntry[];
  onDeleteMemory?: (id: string, isGlobal: boolean) => void;
}

export const MemoryView: React.FC<MemoryViewProps> = ({
  projectMemories,
  globalMemories,
  onDeleteMemory,
}) => {
  const [scope, setScope] = useState<'project' | 'global'>('project');
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedType, setSelectedType] = useState<string>('all');

  const defaultProjectMemories: MemoryEntry[] = [
    {
      id: "mem-1",
      project_id: "nexus-proj",
      agent: "architect",
      type: "decision",
      content: "Adopted Clean Architecture with Repository pattern for testability and isolation.",
      tags: ["architecture", "clean-code", "go"],
      round: 1,
      confidence: 0.95,
      created_at: new Date().toISOString(),
    },
    {
      id: "mem-2",
      project_id: "nexus-proj",
      agent: "critic",
      type: "mistake",
      content: "Caught unhandled nil pointer risk when opening database connection without timeout.",
      tags: ["database", "bug", "critic"],
      round: 2,
      confidence: 0.90,
      created_at: new Date().toISOString(),
    },
    {
      id: "mem-3",
      project_id: "nexus-proj",
      agent: "arbiter",
      type: "insight",
      content: "Consensus reached in Round 3 with 92.4% quality score across 5 agents.",
      tags: ["consensus", "arbiter"],
      round: 3,
      confidence: 1.0,
      created_at: new Date().toISOString(),
    }
  ];

  const defaultGlobalMemories: MemoryEntry[] = [
    {
      id: "gmem-1",
      project_id: "global",
      agent: "architect",
      type: "preference",
      content: "User prefers strict error handling with custom sentinel errors in Go projects.",
      tags: ["style", "preference", "error-handling"],
      confidence: 1.0,
      created_at: new Date().toISOString(),
    },
    {
      id: "gmem-2",
      project_id: "global",
      agent: "coder",
      type: "pattern",
      content: "Always use modernc.org/sqlite (Pure Go) on Windows to eliminate CGO dependencies.",
      tags: ["sqlite", "go", "windows"],
      confidence: 0.98,
      created_at: new Date().toISOString(),
    }
  ];

  const currentList = scope === 'project'
    ? (projectMemories.length > 0 ? projectMemories : defaultProjectMemories)
    : (globalMemories.length > 0 ? globalMemories : defaultGlobalMemories);

  const filtered = currentList.filter(m => {
    const matchesSearch = m.content.toLowerCase().includes(searchTerm.toLowerCase()) ||
      m.tags.some(t => t.toLowerCase().includes(searchTerm.toLowerCase()));
    const matchesType = selectedType === 'all' || m.type === selectedType;
    return matchesSearch && matchesType;
  });

  const getTypeBadge = (type: string) => {
    switch (type) {
      case 'decision':
        return 'text-blue-400 bg-blue-950/40 border-blue-800/40';
      case 'mistake':
        return 'text-red-400 bg-red-950/40 border-red-800/40';
      case 'pattern':
        return 'text-purple-400 bg-purple-950/40 border-purple-800/40';
      case 'preference':
        return 'text-amber-400 bg-amber-950/40 border-amber-800/40';
      default:
        return 'text-emerald-400 bg-emerald-950/40 border-emerald-800/40';
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-[#141416] p-6 space-y-6 overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[#27272a] pb-4">
        <div>
          <h2 className="text-lg font-bold text-zinc-100 flex items-center gap-2">
            <BrainCircuit size={20} className="text-indigo-400" />
            <span>Mem0 Persistent Memory Layer (SQLite)</span>
          </h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Cross-round knowledge base tracking architectural decisions, corrected mistakes, and developer preferences.
          </p>
        </div>

        {/* Scope Switcher */}
        <div className="flex items-center gap-1 bg-[#18181b] p-1 rounded-lg border border-[#27272a]">
          <button
            onClick={() => setScope('project')}
            className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
              scope === 'project' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:text-zinc-200'
            }`}
          >
            Project Memory ({projectMemories.length || defaultProjectMemories.length})
          </button>
          <button
            onClick={() => setScope('global')}
            className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
              scope === 'global' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:text-zinc-200'
            }`}
          >
            Global Cross-Project ({globalMemories.length || defaultGlobalMemories.length})
          </button>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex items-center gap-3">
        <div className="flex-1 relative">
          <Search size={14} className="absolute left-3 top-2.5 text-zinc-500" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search memories by content or tag..."
            className="w-full bg-[#18181b] text-xs text-zinc-200 pl-9 pr-3 py-2 rounded-lg border border-[#27272a] outline-none focus:border-indigo-500"
          />
        </div>

        {/* Type Filter Buttons */}
        <div className="flex items-center gap-1 text-xs">
          {['all', 'decision', 'mistake', 'pattern', 'preference', 'insight'].map((t) => (
            <button
              key={t}
              onClick={() => setSelectedType(t)}
              className={`px-2.5 py-1.5 rounded text-xs capitalize transition-colors ${
                selectedType === t
                  ? 'bg-zinc-800 text-zinc-100 font-semibold'
                  : 'text-zinc-500 hover:text-zinc-300'
              }`}
            >
              {t}
            </button>
          ))}
        </div>
      </div>

      {/* Memory Cards Grid */}
      <div className="flex-1 overflow-y-auto grid grid-cols-2 gap-3">
        {filtered.map((mem) => (
          <div key={mem.id} className="p-4 bg-[#18181b] rounded-lg border border-[#27272a] flex flex-col justify-between space-y-3">
            <div>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className={`text-[10px] font-semibold uppercase px-1.5 py-0.5 rounded border ${getTypeBadge(mem.type)}`}>
                    {mem.type}
                  </span>
                  <span className="text-[11px] text-zinc-400 capitalize font-medium flex items-center gap-1">
                    <Bot size={12} className="text-zinc-500" />
                    {mem.agent}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-[10px] font-mono text-zinc-500">
                    {(mem.confidence * 100).toFixed(0)}% conf
                  </span>
                  {onDeleteMemory && (
                    <button
                      onClick={() => onDeleteMemory(mem.id, scope === 'global')}
                      className="text-zinc-600 hover:text-red-400 transition-colors p-1"
                      title="Delete memory"
                    >
                      <Trash2 size={12} />
                    </button>
                  )}
                </div>
              </div>
              <p className="text-xs text-zinc-200 mt-2 leading-relaxed">{mem.content}</p>
            </div>

            <div className="flex items-center gap-1.5 flex-wrap pt-2 border-t border-zinc-800/80">
              <Tag size={11} className="text-zinc-600" />
              {mem.tags.map((tag, i) => (
                <span key={i} className="text-[10px] font-mono bg-zinc-900 text-indigo-300 px-1.5 py-0.5 rounded">
                  #{tag}
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};
