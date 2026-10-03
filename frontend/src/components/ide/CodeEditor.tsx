import React from 'react';
import Editor from '@monaco-editor/react';
import { AlertCircle, ShieldAlert, Sparkles, CheckCircle2 } from 'lucide-react';
import { CodeFile, ReviewIssue } from '../../types';

interface CodeEditorProps {
  file: CodeFile | null;
  criticIssues?: ReviewIssue[];
  securityVulns?: ReviewIssue[];
  onChange?: (newContent: string) => void;
}

export const CodeEditor: React.FC<CodeEditorProps> = ({
  file,
  criticIssues = [],
  securityVulns = [],
  onChange,
}) => {
  if (!file) {
    return (
      <div className="flex-1 flex flex-col items-center justify-center bg-[#141416] text-zinc-500">
        <Sparkles size={36} className="text-zinc-600 mb-3 animate-pulse" />
        <p className="text-sm font-medium">Select a file from the explorer or trigger a Swarm run</p>
        <p className="text-xs text-zinc-600 mt-1">NexusAI agents collaborate in real-time</p>
      </div>
    );
  }

  // Filter issues for current active file
  const fileCriticIssues = criticIssues.filter(i => i.file === file.path || file.path.endsWith(i.file));
  const fileSecIssues = securityVulns.filter(v => v.file === file.path || file.path.endsWith(v.file));

  const mapLanguage = (lang: string) => {
    switch (lang.toLowerCase()) {
      case 'go':
      case 'golang':
        return 'go';
      case 'ts':
      case 'typescript':
        return 'typescript';
      case 'js':
      case 'javascript':
        return 'javascript';
      case 'py':
      case 'python':
        return 'python';
      case 'json':
        return 'json';
      case 'yaml':
      case 'yml':
        return 'yaml';
      default:
        return 'plaintext';
    }
  };

  return (
    <div className="flex-1 flex flex-col h-full bg-[#141416] relative overflow-hidden">
      {/* File Tab Header */}
      <div className="h-9 bg-[#18181b] border-b border-[#27272a] flex items-center justify-between px-3">
        <div className="flex items-center gap-2 text-xs font-mono text-zinc-300">
          <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
          <span>{file.path}</span>
          <span className="text-zinc-500">({file.language})</span>
        </div>

        {/* Issue Badges */}
        <div className="flex items-center gap-2 text-xs">
          {fileCriticIssues.length > 0 && (
            <span className="flex items-center gap-1 text-red-400 bg-red-950/40 px-2 py-0.5 rounded border border-red-800/40">
              <AlertCircle size={12} />
              <span>{fileCriticIssues.length} Critic Flag(s)</span>
            </span>
          )}
          {fileSecIssues.length > 0 && (
            <span className="flex items-center gap-1 text-amber-400 bg-amber-950/40 px-2 py-0.5 rounded border border-amber-800/40">
              <ShieldAlert size={12} />
              <span>{fileSecIssues.length} امنیتی</span>
            </span>
          )}
          {fileCriticIssues.length === 0 && fileSecIssues.length === 0 && (
            <span className="flex items-center gap-1 text-zinc-400 text-[11px]">
              <CheckCircle2 size={12} className="text-emerald-500" />
              <span>Verified Clean</span>
            </span>
          )}
        </div>
      </div>

      {/* Monaco Editor */}
      <div className="flex-1 relative">
        <Editor
          height="100%"
          theme="vs-dark"
          language={mapLanguage(file.language)}
          value={file.content}
          onChange={(val) => onChange && onChange(val || '')}
          options={{
            fontSize: 13,
            fontFamily: "'Fira Code', monospace",
            minimap: { enabled: true },
            scrollBeyondLastLine: false,
            automaticLayout: true,
            tabSize: 2,
            lineNumbers: 'on',
            renderWhitespace: 'selection',
            bracketPairColorization: { enabled: true },
          }}
        />
      </div>

      {/* Bottom Annotations Drawer if issues exist */}
      {(fileCriticIssues.length > 0 || fileSecIssues.length > 0) && (
        <div className="bg-[#18181b]/95 border-t border-[#27272a] max-h-40 overflow-y-auto p-3 text-xs space-y-2">
          <div className="font-semibold text-zinc-400 uppercase tracking-wider text-[10px]">
            Agent Review Contention on this file:
          </div>
          {fileCriticIssues.map((issue, idx) => (
            <div key={`crit-${idx}`} className="flex items-start gap-2 text-red-300 bg-red-950/20 p-2 rounded border border-red-900/30">
              <AlertCircle size={14} className="shrink-0 mt-0.5 text-red-400" />
              <div>
                <span className="font-semibold capitalize">Critic [{issue.severity}]:</span> {issue.description}
                {issue.suggested_fix && (
                  <div className="mt-1 font-mono text-[11px] text-zinc-400 bg-zinc-900/80 p-1.5 rounded">
                    Fix: {issue.suggested_fix}
                  </div>
                )}
              </div>
            </div>
          ))}
          {fileSecIssues.map((vuln, idx) => (
            <div key={`sec-${idx}`} className="flex items-start gap-2 text-amber-300 bg-amber-950/20 p-2 rounded border border-amber-900/30">
              <ShieldAlert size={14} className="shrink-0 mt-0.5 text-amber-400" />
              <div>
                <span className="font-semibold capitalize">Security Auditor [{vuln.severity}]:</span> {vuln.description}
                {vuln.suggested_fix && (
                  <div className="mt-1 font-mono text-[11px] text-zinc-400 bg-zinc-900/80 p-1.5 rounded">
                    Patch: {vuln.suggested_fix}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
