import React, { useState, useEffect } from 'react';
import { TitleBar } from './components/layout/TitleBar';
import { ActivityBar, SidebarView } from './components/layout/ActivityBar';
import { Sidebar } from './components/layout/Sidebar';
import { EditorTabs } from './components/editor/EditorTabs';
import { WelcomeScreen } from './components/editor/WelcomeScreen';
import { CodeEditor } from './components/ide/CodeEditor';
import { TerminalView } from './components/ide/TerminalView';
import { ComposerPanel } from './components/composer/ComposerPanel';
import { SettingsModal } from './components/modals/SettingsModal';
import { NewProjectModal } from './components/modals/NewProjectModal';
import { wsClient } from './services/websocket';
import { 
  CodeFile, 
  AgentMessage, 
  ArbiterVerdict, 
  ExecutionMode, 
  ReviewIssue, 
  MemoryEntry, 
  SkillManifest 
} from './types';
import { Wifi, WifiOff, GitBranch, Scale, Terminal as TermIcon } from 'lucide-react';

export const App: React.FC = () => {
  // Project & Layout State
  const [projectName, setProjectName] = useState<string | null>(null);
  const [activeSidebarView, setActiveSidebarView] = useState<SidebarView>('explorer');
  const [showLeftSidebar, setShowLeftSidebar] = useState<boolean>(true);
  const [showBottomPanel, setShowBottomPanel] = useState<boolean>(false);
  const [showComposer, setShowComposer] = useState<boolean>(true);
  const [showDiff, setShowDiff] = useState<boolean>(false);

  // Modals State
  const [isSettingsOpen, setIsSettingsOpen] = useState<boolean>(false);
  const [isNewProjectOpen, setIsNewProjectOpen] = useState<boolean>(false);

  // Core Swarm State
  const [files, setFiles] = useState<CodeFile[]>([]);
  const [openFiles, setOpenFiles] = useState<string[]>([]);
  const [activeFile, setActiveFile] = useState<string>('');
  const [messages, setMessages] = useState<AgentMessage[]>([]);
  const [verdict, setVerdict] = useState<ArbiterVerdict | null>(null);
  const [criticIssues, setCriticIssues] = useState<ReviewIssue[]>([]);
  const [securityVulns, setSecurityVulns] = useState<ReviewIssue[]>([]);
  const [terminalLogs, setTerminalLogs] = useState<string[]>([
    "NexusAI IDE Initialized.",
    "Connected to Go daemon orchestrator on ws://localhost:8080/ws"
  ]);

  // Settings & Configuration State
  const [executionMode, setExecutionMode] = useState<ExecutionMode>('rolebased');
  const [selectedModel, setSelectedModel] = useState<string>('gpt-4o');
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const [isRunning, setIsRunning] = useState<boolean>(false);
  const [currentRound, setCurrentRound] = useState<number>(1);

  const [apiKeys, setApiKeys] = useState(() => {
    const saved = localStorage.getItem('nexusai_api_keys');
    if (saved) {
      try { return JSON.parse(saved); } catch (e) {}
    }
    return {
      openai: '',
      openrouter: '',
      anthropic: '',
      gemini: '',
      ollamaUrl: 'http://localhost:11434/v1',
      customBaseUrl: '',
    };
  });

  const [memories, setMemories] = useState<MemoryEntry[]>([]);
  const [skills, setSkills] = useState<SkillManifest[]>([
    {
      name: "docker-deploy",
      description: "Generates multi-stage Dockerfiles and container configurations.",
      version: "1.0.0",
      agents: ["coder", "architect"],
      triggers: ["docker", "deploy", "container"],
      enabled: true,
    },
    {
      name: "api-designer",
      description: "Designs RESTful APIs and gRPC contracts following standards.",
      version: "1.0.0",
      agents: ["architect", "coder"],
      triggers: ["api", "rest", "grpc"],
      enabled: true,
    },
    {
      name: "code-review",
      description: "Deep AST inspection for edge cases, race conditions, and leaks.",
      version: "1.0.0",
      agents: ["critic"],
      triggers: ["review", "audit"],
      enabled: true,
    }
  ]);

  // Connect WebSocket & Listen for Swarm Events
  useEffect(() => {
    wsClient.connect();
    const unsubStatus = wsClient.onStatus(setIsConnected);

    const unsubEvent = wsClient.onEvent((event) => {
      switch (event.type) {
        case 'round_started':
          setCurrentRound(event.round);
          setIsRunning(true);
          setTerminalLogs((prev) => [...prev, `[Swarm] Round ${event.round} started...`]);
          break;

        case 'agent_output':
          if (event.message) {
            setMessages((prev) => [...prev, event.message!]);
            if (event.message.summary) {
              setTerminalLogs((prev) => [...prev, `[${event.message!.from.toUpperCase()}] ${event.message!.summary}`]);
            }
          }
          if (event.files && event.files.length > 0) {
            setFiles(event.files);
            // Auto open first file if none open
            if (openFiles.length === 0) {
              setOpenFiles([event.files[0].path]);
              setActiveFile(event.files[0].path);
            }
          }
          break;

        case 'code_updated':
          if (event.files && event.files.length > 0) {
            setFiles(event.files);
          }
          break;

        case 'round_complete':
          if (event.verdict) {
            setVerdict(event.verdict);
            setTerminalLogs((prev) => [
              ...prev,
              `[Arbiter] Round Score: ${event.verdict!.final_score.toFixed(1)}% (Passed: ${event.verdict!.passed})`
            ]);
          }
          break;

        case 'consensus_reached':
          if (event.verdict) {
            setVerdict(event.verdict);
          }
          if (event.files) {
            setFiles(event.files);
          }
          setIsRunning(false);
          setTerminalLogs((prev) => [
            ...prev,
            `[Swarm Consensus Reached] Final Quality Score: ${event.verdict?.final_score.toFixed(1)}% - Ready to merge!`
          ]);
          break;

        case 'error':
          setIsRunning(false);
          setTerminalLogs((prev) => [...prev, `[Error] ${event.payload || 'Swarm error'}`]);
          break;
      }
    });

    return () => {
      unsubStatus();
      unsubEvent();
    };
  }, [openFiles]);

  const handleSaveApiKeys = (keys: any) => {
    setApiKeys(keys);
    localStorage.setItem('nexusai_api_keys', JSON.stringify(keys));
  };

  const handleCreateProject = (name: string, template: string) => {
    setProjectName(name);
    // If template has base files:
    if (template === 'go-microservice') {
      const initialFile: CodeFile = {
        path: 'main.go',
        language: 'go',
        content: `package main\n\nimport (\n\t"fmt"\n\t"log"\n\t"net/http"\n)\n\nfunc main() {\n\thttp.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {\n\t\tfmt.Fprintf(w, "OK")\n\t})\n\tlog.Println("Server running on :8080")\n\tlog.Fatal(http.ListenAndServe(":8080", nil))\n}\n`,
      };
      setFiles([initialFile]);
      setOpenFiles(['main.go']);
      setActiveFile('main.go');
    } else {
      setFiles([]);
      setOpenFiles([]);
      setActiveFile('');
    }
    setTerminalLogs((prev) => [...prev, `[Project] Opened project: ${name} (${template})`]);
  };

  const handleSelectFile = (path: string) => {
    if (!openFiles.includes(path)) {
      setOpenFiles((prev) => [...prev, path]);
    }
    setActiveFile(path);
  };

  const handleCloseTab = (path: string) => {
    const updated = openFiles.filter((p) => p !== path);
    setOpenFiles(updated);
    if (activeFile === path) {
      setActiveFile(updated[updated.length - 1] || '');
    }
  };

  const handleSendPrompt = (promptText: string) => {
    const proj = projectName || 'my-nexus-project';
    if (!projectName) {
      setProjectName(proj);
    }
    setIsRunning(true);

    // Append user prompt to messages
    setMessages((prev) => [
      ...prev,
      {
        id: `user-${Date.now()}`,
        from: 'architect',
        round: currentRound,
        type: 'user_prompt',
        content: promptText,
        summary: `User Prompt: ${promptText}`,
        timestamp: new Date().toISOString(),
      },
    ]);

    // Send to Go backend daemon via WebSocket
    wsClient.send('start_swarm', proj, { mode: executionMode, model: selectedModel }, promptText);
  };

  const currentFileObj = files.find((f) => f.path === activeFile) || null;

  return (
    <div className="flex flex-col h-screen w-screen bg-[#141414] text-zinc-100 font-sans select-none overflow-hidden">
      {/* 1. Cursor Style Top Window Title Bar */}
      <TitleBar
        projectName={projectName || ''}
        onOpenSettings={() => setIsSettingsOpen(true)}
        showLeftSidebar={showLeftSidebar}
        onToggleLeftSidebar={() => setShowLeftSidebar(!showLeftSidebar)}
        showBottomPanel={showBottomPanel}
        onToggleBottomPanel={() => setShowBottomPanel(!showBottomPanel)}
        showComposer={showComposer}
        onToggleComposer={() => setShowComposer(!showComposer)}
        onOpenCommandPalette={() => setIsNewProjectOpen(true)}
      />

      {/* 2. Main Workbench Workspace */}
      <div className="flex-1 flex overflow-hidden">
        {/* Far-left Activity Bar */}
        <ActivityBar
          activeView={showLeftSidebar ? activeSidebarView : null}
          onSelectView={(v) => {
            if (v === null) {
              setShowLeftSidebar(false);
            } else {
              setActiveSidebarView(v);
              setShowLeftSidebar(true);
            }
          }}
          onOpenSettings={() => setIsSettingsOpen(true)}
        />

        {/* Primary Collapsible Sidebar */}
        {showLeftSidebar && activeSidebarView && (
          <Sidebar
            view={activeSidebarView}
            projectName={projectName}
            files={files}
            activeFile={activeFile}
            onSelectFile={handleSelectFile}
            onNewFile={() => {
              const name = prompt('Enter new file name:');
              if (name) {
                const newF: CodeFile = { path: name, language: name.endsWith('.go') ? 'go' : 'typescript', content: '' };
                setFiles((prev) => [...prev, newF]);
                handleSelectFile(name);
              }
            }}
            onNewProject={() => setIsNewProjectOpen(true)}
            onDeleteFile={(p) => {
              setFiles((prev) => prev.filter((f) => f.path !== p));
              handleCloseTab(p);
            }}
            memories={memories}
            skills={skills}
            verdict={verdict}
          />
        )}

        {/* Center Area: Editor or Welcome Screen */}
        <div className="flex-1 flex flex-col h-full overflow-hidden bg-[#181818]">
          {openFiles.length > 0 && currentFileObj ? (
            <div className="flex-1 flex flex-col h-full overflow-hidden">
              {/* Tab Bar */}
              <EditorTabs
                openFiles={openFiles}
                activeFile={activeFile}
                files={files}
                onSelectTab={(p) => setActiveFile(p)}
                onCloseTab={handleCloseTab}
                showDiff={showDiff}
                onToggleDiff={() => setShowDiff(!showDiff)}
              />

              {/* Monaco Editor Canvas */}
              <div className="flex-1 overflow-hidden relative">
                <CodeEditor
                  file={currentFileObj}
                  criticIssues={criticIssues}
                  securityVulns={securityVulns}
                  onChange={(newVal) => {
                    setFiles((prev) =>
                      prev.map((f) => (f.path === activeFile ? { ...f, content: newVal } : f))
                    );
                  }}
                />
              </div>

              {/* Collapsible Bottom Terminal Panel */}
              {showBottomPanel && (
                <TerminalView
                  logs={terminalLogs}
                  onClear={() => setTerminalLogs([])}
                  onRunCommand={(cmd) => {
                    setTerminalLogs((prev) => [...prev, cmd, `[exec] Command executed in workspace.`]);
                  }}
                />
              )}
            </div>
          ) : (
            // Cursor Style Clean Welcome Screen
            <WelcomeScreen
              onNewProject={() => setIsNewProjectOpen(true)}
              onOpenSettings={() => setIsSettingsOpen(true)}
              onQuickPrompt={(p) => {
                if (!projectName) {
                  handleCreateProject('nexus-quickstart', 'go-microservice');
                }
                handleSendPrompt(p);
              }}
            />
          )}
        </div>

        {/* Right Area: Cursor Composer / AI Chat Panel */}
        {showComposer && (
          <ComposerPanel
            messages={messages}
            isRunning={isRunning}
            verdict={verdict}
            executionMode={executionMode}
            onSetExecutionMode={setExecutionMode}
            selectedModel={selectedModel}
            onSelectModel={setSelectedModel}
            onSendPrompt={handleSendPrompt}
            onAcceptChanges={() => {
              setTerminalLogs((prev) => [...prev, "[Composer] Changes accepted and merged into workspace."]);
            }}
          />
        )}
      </div>

      {/* 3. Bottom Status Bar */}
      <footer className="h-6 bg-[#121212] border-t border-[#202020] px-3 flex items-center justify-between text-[11px] text-zinc-400 select-none z-30 font-mono">
        <div className="flex items-center gap-4">
          <span className="flex items-center gap-1.5 text-zinc-300">
            <GitBranch size={12} className="text-zinc-500" />
            <span>main</span>
          </span>

          <span className="flex items-center gap-1.5 text-zinc-400">
            <span>Project:</span>
            <strong className="text-zinc-200">{projectName || 'None'}</strong>
          </span>

          <span className="flex items-center gap-1.5 text-zinc-400">
            <span>Swarm Mode:</span>
            <strong className="text-indigo-400 capitalize">{executionMode}</strong>
          </span>

          <button
            onClick={() => setShowBottomPanel(!showBottomPanel)}
            className="flex items-center gap-1 text-zinc-400 hover:text-zinc-200"
          >
            <TermIcon size={11} />
            <span>Terminal</span>
          </button>
        </div>

        <div className="flex items-center gap-4">
          {verdict && (
            <span className="flex items-center gap-1">
              <Scale size={11} className="text-amber-400" />
              <span>Consensus:</span>
              <strong className={verdict.passed ? 'text-emerald-400' : 'text-amber-400'}>
                {verdict.final_score.toFixed(1)}%
              </strong>
            </span>
          )}

          {isConnected ? (
            <span className="flex items-center gap-1 text-emerald-400">
              <Wifi size={12} />
              <span>Daemon Live (:8080)</span>
            </span>
          ) : (
            <span className="flex items-center gap-1 text-zinc-500">
              <WifiOff size={12} />
              <span>Daemon Offline</span>
            </span>
          )}

          <span className="text-zinc-500">UTF-8</span>
        </div>
      </footer>

      {/* Settings Modal */}
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        apiKeys={apiKeys}
        onSaveApiKeys={handleSaveApiKeys}
      />

      {/* New Project Modal */}
      <NewProjectModal
        isOpen={isNewProjectOpen}
        onClose={() => setIsNewProjectOpen(false)}
        onCreateProject={handleCreateProject}
      />
    </div>
  );
};
