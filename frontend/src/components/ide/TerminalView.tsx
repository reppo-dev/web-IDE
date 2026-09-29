import React, { useState } from 'react';
import { Terminal as TermIcon, Play, Trash2 } from 'lucide-react';

interface TerminalViewProps {
  logs: string[];
  onClear?: () => void;
  onRunCommand?: (cmd: string) => void;
}

export const TerminalView: React.FC<TerminalViewProps> = ({
  logs,
  onClear,
  onRunCommand,
}) => {
  const [inputCmd, setInputCmd] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!inputCmd.trim()) return;
    if (onRunCommand) onRunCommand(inputCmd);
    setInputCmd('');
  };

  return (
    <div className="h-44 bg-[#141416] border-t border-[#27272a] flex flex-col font-mono text-xs select-text">
      {/* Header */}
      <div className="h-7 bg-[#18181b] border-b border-[#27272a] px-3 flex items-center justify-between text-zinc-400">
        <div className="flex items-center gap-1.5 font-medium">
          <TermIcon size={13} className="text-zinc-400" />
          <span>Terminal & Output</span>
        </div>
        <div className="flex items-center gap-2">
          {onClear && (
            <button onClick={onClear} className="hover:text-zinc-200" title="Clear Terminal">
              <Trash2 size={13} />
            </button>
          )}
        </div>
      </div>

      {/* Logs stream */}
      <div className="flex-1 p-2 overflow-y-auto space-y-1 text-zinc-300">
        {logs.length === 0 ? (
          <div className="text-zinc-600 italic">Terminal ready. Type a command or run swarm.</div>
        ) : (
          logs.map((log, i) => (
            <div key={i} className="leading-relaxed whitespace-pre-wrap">
              <span className="text-indigo-400 mr-2">$</span>
              {log}
            </div>
          ))
        )}
      </div>

      {/* Input */}
      <form onSubmit={handleSubmit} className="h-8 bg-[#18181b] border-t border-[#27272a] flex items-center px-2">
        <span className="text-indigo-400 font-bold mr-2">&gt;</span>
        <input
          type="text"
          value={inputCmd}
          onChange={(e) => setInputCmd(e.target.value)}
          placeholder="Execute shell or go command (e.g. go run main.go)..."
          className="flex-1 bg-transparent text-zinc-200 outline-none text-xs"
        />
        <button type="submit" className="p-1 text-zinc-400 hover:text-zinc-100">
          <Play size={12} />
        </button>
      </form>
    </div>
  );
};
