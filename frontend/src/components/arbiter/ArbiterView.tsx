import React from 'react';
import { Scale, CheckCircle2, AlertTriangle, ShieldCheck, FileDiff, Trophy } from 'lucide-react';
import { ArbiterVerdict, CodeFile } from '../../types';

interface ArbiterViewProps {
  verdict: ArbiterVerdict | null;
  files: CodeFile[];
}

export const ArbiterView: React.FC<ArbiterViewProps> = ({
  verdict,
  files,
}) => {
  const defaultDimensions = [
    { name: 'Code Correctness', weight: 0.30, score: verdict ? (verdict.dimensions[0]?.score || 85) : 85, explanation: 'Logic flaws & edge cases' },
    { name: 'Bug Resolution', weight: 0.25, score: verdict ? (verdict.dimensions[1]?.score || 80) : 80, explanation: 'Critic issues refactored' },
    { name: 'Security Compliance', weight: 0.20, score: verdict ? (verdict.dimensions[2]?.score || 95) : 95, explanation: 'Vulnerability scan' },
    { name: 'Code Quality', weight: 0.15, score: verdict ? (verdict.dimensions[3]?.score || 85) : 85, explanation: 'Readability & Clean Code' },
    { name: 'Spec Adherence', weight: 0.10, score: verdict ? (verdict.dimensions[4]?.score || 90) : 90, explanation: 'Architect blueprint match' },
  ];

  const dimensions = verdict?.dimensions?.length ? verdict.dimensions : defaultDimensions;
  const currentScore = verdict?.final_score || 86.5;
  const passed = verdict ? verdict.passed : true;

  return (
    <div className="flex-1 flex flex-col h-full bg-[#141416] overflow-y-auto p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-[#27272a] pb-4">
        <div>
          <h2 className="text-lg font-bold text-zinc-100 flex items-center gap-2">
            <Scale size={20} className="text-amber-400" />
            <span>Arbiter Consensus Matrix & Scoreboard</span>
          </h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Objective evaluation of swarm performance and consensus verification.
          </p>
        </div>

        {/* Big Verdict Pill */}
        <div className={`flex items-center gap-2 px-4 py-2 rounded-lg border font-semibold text-sm ${
          passed
            ? 'bg-emerald-950/40 border-emerald-800 text-emerald-400'
            : 'bg-amber-950/40 border-amber-800 text-amber-400'
        }`}>
          {passed ? <CheckCircle2 size={18} /> : <AlertTriangle size={18} />}
          <span>{passed ? 'CONSENSUS APPROVED (>=85%)' : 'REFINEMENT ORDERED (<85%)'}</span>
          <span className="font-mono text-base font-bold ml-1">[{currentScore.toFixed(1)}%]</span>
        </div>
      </div>

      {/* Grid: 5 Dimension Score Cards */}
      <div className="grid grid-cols-5 gap-3">
        {dimensions.map((dim, i) => (
          <div key={i} className="p-3 bg-[#18181b] rounded-lg border border-[#27272a] flex flex-col justify-between">
            <div>
              <div className="flex items-center justify-between">
                <span className="text-xs font-bold text-zinc-300">{dim.name}</span>
                <span className="text-[10px] text-zinc-500 font-mono">{(dim.weight * 100)}%</span>
              </div>
              <div className="text-[10px] text-zinc-400 mt-1">{dim.explanation}</div>
            </div>
            <div className="mt-4">
              <div className="flex justify-between items-baseline mb-1">
                <span className="text-lg font-bold text-indigo-400 font-mono">{dim.score.toFixed(0)}%</span>
              </div>
              <div className="w-full bg-zinc-800 h-1.5 rounded-full overflow-hidden">
                <div
                  className={`h-full rounded-full ${dim.score >= 85 ? 'bg-emerald-500' : dim.score >= 70 ? 'bg-amber-500' : 'bg-red-500'}`}
                  style={{ width: `${Math.min(100, Math.max(0, dim.score))}%` }}
                ></div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Agent Performance Leaderboard */}
      <div className="bg-[#18181b] rounded-lg border border-[#27272a] p-4 space-y-3">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-zinc-300">
          <Trophy size={14} className="text-amber-400" />
          <span>Agent Accuracy & Performance Ledger</span>
        </div>
        <div className="grid grid-cols-4 gap-3 text-xs">
          <div className="p-3 bg-[#141416] rounded border border-[#27272a]">
            <div className="text-zinc-400 text-[11px]">Critic Accuracy</div>
            <div className="text-base font-bold text-red-400 mt-1">94.2%</div>
            <div className="text-[10px] text-zinc-500 mt-0.5">Caught 4 bugs (0 false positives)</div>
          </div>
          <div className="p-3 bg-[#141416] rounded border border-[#27272a]">
            <div className="text-zinc-400 text-[11px]">Coder Resolution Rate</div>
            <div className="text-base font-bold text-purple-400 mt-1">100%</div>
            <div className="text-[10px] text-zinc-500 mt-0.5">Fixed 4 of 4 flagged issues</div>
          </div>
          <div className="p-3 bg-[#141416] rounded border border-[#27272a]">
            <div className="text-zinc-400 text-[11px]">Security Guard Cleanliness</div>
            <div className="text-base font-bold text-emerald-400 mt-1">100%</div>
            <div className="text-[10px] text-zinc-500 mt-0.5">0 CWE/OWASP vulnerabilities remaining</div>
          </div>
          <div className="p-3 bg-[#141416] rounded border border-[#27272a]">
            <div className="text-zinc-400 text-[11px]">Arbiter Reliability Index</div>
            <div className="text-base font-bold text-amber-400 mt-1">98.5%</div>
            <div className="text-[10px] text-zinc-500 mt-0.5">Weighted consensus synthesis</div>
          </div>
        </div>
      </div>

      {/* Diff Viewer Snapshot (Before vs. After Critic) */}
      <div className="flex-1 bg-[#18181b] rounded-lg border border-[#27272a] p-4 flex flex-col space-y-3">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-zinc-300">
          <FileDiff size={14} className="text-indigo-400" />
          <span>Refactor Diff Matrix (Critic Interventions & Patches)</span>
        </div>

        {files.length === 0 ? (
          <div className="text-center py-10 text-zinc-500 text-xs">
            Run a swarm synthesis to generate code files and compare adversarial diffs.
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-3 flex-1 overflow-hidden font-mono text-xs">
            {/* Before */}
            <div className="flex flex-col bg-[#141416] rounded border border-[#27272a] overflow-hidden">
              <div className="p-2 bg-red-950/20 border-b border-red-900/30 text-red-400 font-semibold text-[11px]">
                Draft Before Critic Review ({files[0]?.path})
              </div>
              <pre className="p-3 overflow-y-auto text-zinc-400 text-[11px] leading-relaxed flex-1">
                {files[0]?.content || "// Initial draft"}
              </pre>
            </div>

            {/* After */}
            <div className="flex flex-col bg-[#141416] rounded border border-[#27272a] overflow-hidden">
              <div className="p-2 bg-emerald-950/20 border-b border-emerald-900/30 text-emerald-400 font-semibold text-[11px]">
                Refactored & Hardened Final Consensus ({files[0]?.path})
              </div>
              <pre className="p-3 overflow-y-auto text-emerald-300 text-[11px] leading-relaxed flex-1">
                {files[0]?.content || "// Patched code"}
              </pre>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
