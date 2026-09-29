import React, { useState } from 'react';
import { Package, Plug, Plus, Check, Power, Terminal, ExternalLink } from 'lucide-react';
import { SkillManifest } from '../../types';

interface SkillsViewProps {
  skills: SkillManifest[];
  onToggleSkill?: (name: string, enabled: boolean) => void;
}

export const SkillsView: React.FC<SkillsViewProps> = ({
  skills,
  onToggleSkill,
}) => {
  const [activeTab, setActiveTab] = useState<'skills' | 'mcp'>('skills');

  const defaultSkills: SkillManifest[] = [
    {
      name: "docker-deploy",
      description: "Generates production-grade multi-stage Dockerfiles and compose setups.",
      version: "1.0.0",
      agents: ["coder", "architect"],
      triggers: ["docker", "deploy", "container"],
      enabled: true,
    },
    {
      name: "api-designer",
      description: "Applies OpenAPI 3.0 and gRPC contracts best practices and validation.",
      version: "1.0.0",
      agents: ["architect", "coder"],
      triggers: ["api", "rest", "grpc", "endpoint"],
      enabled: true,
    },
    {
      name: "test-generator",
      description: "Creates comprehensive table-driven unit and integration tests.",
      version: "1.0.0",
      agents: ["coder", "critic"],
      triggers: ["test", "coverage", "benchmark"],
      enabled: true,
    },
    {
      name: "code-review",
      description: "Deep AST inspection for concurrency hazards, leaks, and style alignment.",
      version: "1.0.0",
      agents: ["critic"],
      triggers: ["review", "audit"],
      enabled: true,
    }
  ];

  const mcpServers = [
    { name: "github", command: "npx @github/mcp-server", status: "Connected", toolsCount: 8 },
    { name: "postgres-local", command: "npx @mcp/postgres-server", status: "Active", toolsCount: 5 },
    { name: "snyk-security", command: "snyk-mcp --port 4000", status: "Active", toolsCount: 3 },
  ];

  const displaySkills = skills.length > 0 ? skills : defaultSkills;

  return (
    <div className="flex-1 flex flex-col h-full bg-[#141416] p-6 space-y-6 overflow-y-auto">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[#27272a] pb-4">
        <div>
          <h2 className="text-lg font-bold text-zinc-100 flex items-center gap-2">
            <Package size={20} className="text-indigo-400" />
            <span>Skills & MCP Ecosystem</span>
          </h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Extend your swarm with specialized developer skill-packs and Model Context Protocol servers.
          </p>
        </div>

        {/* Tab switch */}
        <div className="flex items-center gap-1 bg-[#18181b] p-1 rounded-lg border border-[#27272a]">
          <button
            onClick={() => setActiveTab('skills')}
            className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
              activeTab === 'skills' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <Package size={13} />
            <span>Installed Skills ({displaySkills.length})</span>
          </button>
          <button
            onClick={() => setActiveTab('mcp')}
            className={`flex items-center gap-1.5 px-3 py-1 rounded text-xs font-medium transition-colors ${
              activeTab === 'mcp' ? 'bg-indigo-600 text-white' : 'text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <Plug size={13} />
            <span>MCP Servers ({mcpServers.length})</span>
          </button>
        </div>
      </div>

      {/* Skills Tab Content */}
      {activeTab === 'skills' && (
        <div className="grid grid-cols-2 gap-4">
          {displaySkills.map((s) => (
            <div key={s.name} className="p-4 bg-[#18181b] rounded-lg border border-[#27272a] flex flex-col justify-between space-y-3">
              <div>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="font-bold text-sm text-zinc-200">{s.name}</span>
                    <span className="text-[10px] font-mono px-1.5 py-0.2 rounded bg-zinc-800 text-zinc-400">v{s.version}</span>
                  </div>
                  <button
                    onClick={() => onToggleSkill && onToggleSkill(s.name, !s.enabled)}
                    className={`flex items-center gap-1 text-[11px] px-2 py-0.5 rounded font-medium transition-colors ${
                      s.enabled ? 'bg-emerald-950/60 text-emerald-400 border border-emerald-800/60' : 'bg-zinc-800 text-zinc-500'
                    }`}
                  >
                    <Power size={11} />
                    <span>{s.enabled ? 'Active' : 'Disabled'}</span>
                  </button>
                </div>
                <p className="text-xs text-zinc-400 mt-1.5 leading-relaxed">{s.description}</p>
              </div>

              <div className="space-y-2 pt-2 border-t border-zinc-800/80">
                <div className="flex items-center gap-1.5 flex-wrap">
                  <span className="text-[10px] text-zinc-500 font-semibold">Triggers:</span>
                  {s.triggers.map((trig, idx) => (
                    <span key={idx} className="text-[10px] bg-zinc-900 text-indigo-300 px-1.5 py-0.5 rounded font-mono">
                      #{trig}
                    </span>
                  ))}
                </div>
                <div className="flex items-center gap-1.5 text-[10px] text-zinc-400">
                  <span className="text-zinc-500 font-semibold">Authorized Agents:</span>
                  <span className="capitalize">{s.agents.join(', ')}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* MCP Tab Content */}
      {activeTab === 'mcp' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center bg-[#18181b] p-3 rounded-lg border border-[#27272a]">
            <span className="text-xs text-zinc-400">
              NexusAI automatically connects to local and stdio Model Context Protocol tool providers.
            </span>
            <button className="flex items-center gap-1 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded text-xs font-medium">
              <Plus size={13} />
              <span>Connect MCP Server</span>
            </button>
          </div>

          <div className="grid grid-cols-1 gap-3">
            {mcpServers.map((srv, idx) => (
              <div key={idx} className="p-4 bg-[#18181b] rounded-lg border border-[#27272a] flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <div className="w-9 h-9 rounded-lg bg-zinc-900 border border-zinc-800 flex items-center justify-center text-indigo-400">
                    <Plug size={18} />
                  </div>
                  <div>
                    <div className="font-bold text-sm text-zinc-200">{srv.name}</div>
                    <div className="font-mono text-xs text-zinc-400 mt-0.5">{srv.command}</div>
                  </div>
                </div>
                <div className="flex items-center gap-4 text-xs">
                  <span className="text-zinc-400 font-medium">{srv.toolsCount} Tools Exposed</span>
                  <span className="px-2 py-0.5 rounded bg-emerald-950/40 text-emerald-400 border border-emerald-800/40 font-medium flex items-center gap-1">
                    <Check size={12} />
                    {srv.status}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
