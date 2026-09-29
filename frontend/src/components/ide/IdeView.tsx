import React, { useState } from 'react';
import { FileExplorer } from './FileExplorer';
import { CodeEditor } from './CodeEditor';
import { TerminalView } from './TerminalView';
import { ChatPanel } from './ChatPanel';
import { CodeFile, AgentMessage, ReviewIssue } from '../../types';

interface IdeViewProps {
  files: CodeFile[];
  messages: AgentMessage[];
  criticIssues?: ReviewIssue[];
  securityVulns?: ReviewIssue[];
  isRunning: boolean;
  onSendPrompt: (prompt: string) => void;
  onUpdateFileContent?: (path: string, content: string) => void;
}

export const IdeView: React.FC<IdeViewProps> = ({
  files,
  messages,
  criticIssues = [],
  securityVulns = [],
  isRunning,
  onSendPrompt,
  onUpdateFileContent,
}) => {
  const [activeFilePath, setActiveFilePath] = useState<string>(files[0]?.path || '');
  const [terminalLogs, setTerminalLogs] = useState<string[]>([
    "NexusAI IDE initialized.",
    "Connected to Go daemon orchestrator on ws://localhost:8080/ws",
    "Ready for multi-agent synthesis."
  ]);

  const activeFile = files.find(f => f.path === activeFilePath) || files[0] || null;

  const handleCommand = (cmd: string) => {
    setTerminalLogs(prev => [...prev, cmd, `[exec] Simulated output for: ${cmd}`, "Done (exit code 0)."]);
  };

  return (
    <div className="flex-1 flex h-full overflow-hidden">
      {/* Left: File Tree */}
      <FileExplorer
        files={files}
        activeFile={activeFile?.path || ''}
        onSelectFile={(p) => setActiveFilePath(p)}
      />

      {/* Center: Monaco Editor & Terminal */}
      <div className="flex-1 flex flex-col h-full overflow-hidden">
        <CodeEditor
          file={activeFile}
          criticIssues={criticIssues}
          securityVulns={securityVulns}
          onChange={(newContent) => {
            if (activeFile && onUpdateFileContent) {
              onUpdateFileContent(activeFile.path, newContent);
            }
          }}
        />
        <TerminalView
          logs={terminalLogs}
          onClear={() => setTerminalLogs([])}
          onRunCommand={handleCommand}
        />
      </div>

      {/* Right: Integrated Multi-Agent Chat */}
      <ChatPanel
        messages={messages}
        isRunning={isRunning}
        onSendPrompt={onSendPrompt}
      />
    </div>
  );
};
