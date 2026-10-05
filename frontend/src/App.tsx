import React, { useState, useEffect, useCallback, useMemo } from 'react';
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
  SkillManifest,
  ProjectPlan,
  AgentUsage,
  TaskStreamStep,
  FileLockInfo
} from './types';
import { SwarmPreset, SWARM_PRESETS } from './config/models';
import { TaskStreamCard } from './components/TaskStreamCard';
import { groupStepsByAgent } from './components/AgentConversationPanel';
import { TeamBuilderModal } from './components/teams/TeamBuilderModal';
import { Team } from './types/team';
import { OpenProjectModal } from './components/projects/OpenProjectModal';
import { ProjectRef, ProjectTree, ProjectFileContent } from './types/project';
import { Wifi, WifiOff, GitBranch, Scale, Terminal as TermIcon } from 'lucide-react';

/** The five swarm personas. Every one must be assigned a model by the user. */
const AGENT_ROLES = ['architect', 'coder', 'critic', 'security', 'arbiter'] as const;

export const App: React.FC = () => {
  // Project & Layout State
  const [projectId, setProjectId] = useState<string | null>(null);
  const [projectName, setProjectName] = useState<string | null>(null);
  const [projectPlan, setProjectPlan] = useState<ProjectPlan | null>(null);
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
  const [selectedModel, setSelectedModel] = useState<string>('deepseek/deepseek-chat:free');
  const [agentModels, setAgentModels] = useState<Record<string, string>>(() => {
    const saved = localStorage.getItem('nexusai_agent_models');
    if (saved) {
      try {
        const parsed = JSON.parse(saved);
        // Only honour previously saved choices if every agent still has one.
        // Never silently substitute a default model for an unassigned agent.
        if (AGENT_ROLES.every((r) => typeof parsed[r] === 'string' && parsed[r].length > 0)) {
          return parsed;
        }
      } catch (e) {}
    }
    // No default assignment: the user must choose a model per agent.
    return {} as Record<string, string>;
  });

  const unassignedAgents = AGENT_ROLES.filter((role) => !agentModels[role]);
  const [isConnected, setIsConnected] = useState<boolean>(false);
  // Mirrors openFiles so the long-lived WebSocket handler always reads the
  // current value without needing to re-subscribe.
  const openFilesRef = React.useRef<string[]>([]);
  const [isRunning, setIsRunning] = useState<boolean>(false);
  const [stalled, setStalled] = useState<boolean>(false);
  const [currentRound, setCurrentRound] = useState<number>(1);
  const [thinkingAgent, setThinkingAgent] = useState<{ role: string, status: string } | null>(null);
  const [agentUsage, setAgentUsage] = useState<Record<string, AgentUsage>>({});
  /**
   * Live per-task conversations, keyed by task id.
   *
   * This is what replaced "one bubble per utterance". A task's steps accumulate
   * here and render as a single card, so the transcript stays readable at 25
   * tasks instead of collapsing into hundreds of disconnected messages.
   */
  const [taskStreams, setTaskStreams] = useState<Record<string, TaskStreamStep[]>>({});
  const [taskStreamTitles, setTaskStreamTitles] = useState<Record<string, string>>({});
  const [activeTaskId, setActiveTaskId] = useState<string | null>(null);
  const [fileLocks, setFileLocks] = useState<FileLockInfo[]>([]);

  /** Append one step to its task's stream, replacing its contents on a retry. */
  const appendTaskStep = useCallback((taskId: string, step: TaskStreamStep, reset?: boolean) => {
    setTaskStreams((prev) => {
      const existing = reset ? [] : (prev[taskId] ?? []);
      return { ...prev, [taskId]: [...existing, step] };
    });
  }, []);

  /**
   * The per-agent threads shown in the panel.
   *
   * Recomputed with useMemo rather than stored: it is a pure rearrangement of
   * state that already exists, and keeping a second copy in sync was how the two
   * views of "what happened" would drift apart.
   */
  const agentConversations = useMemo(
    () =>
      groupStepsByAgent(
        Object.values(taskStreams).flat(),
        taskStreamTitles,
        activeTaskId,
        fileLocks.map((l) => ({ agent: l.agent, path: l.path })),
      ),
    [taskStreams, taskStreamTitles, activeTaskId, fileLocks],
  );

  const handleUpdateAgentModel = (role: string, model: string) => {
    setAgentModels((prev) => {
      const next = { ...prev, [role]: model };
      localStorage.setItem('nexusai_agent_models', JSON.stringify(next));
      return next;
    });
  };

  const handleApplyPreset = (preset: SwarmPreset) => {
    setAgentModels(preset.models);
    localStorage.setItem('nexusai_agent_models', JSON.stringify(preset.models));
    setTerminalLogs((prev) => [...prev, `[Team Config] Swarm preset active: ${preset.nameFa}`]);
  };

  const [apiKeys, setApiKeys] = useState(() => {
    const defaults = {
      openai: '',
      openrouter: '',
      anthropic: '',
      gemini: '',
      ninerouter: '',
      ollamaUrl: 'http://localhost:11434/v1',
      customBaseUrl: '',
    };
    const saved = localStorage.getItem('nexusai_api_keys');
    if (saved) {
      try {
        // Merge over the defaults: an older saved payload may be missing keys
        // that were added later, and undefined would render as an uncontrolled
        // input.
        return { ...defaults, ...JSON.parse(saved) };
      } catch (e) {}
    }
    return defaults;
  });

  const [memories, setMemories] = useState<MemoryEntry[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [selectedTeamId, setSelectedTeamId] = useState<string>('default');
  const [showTeamBuilder, setShowTeamBuilder] = useState<boolean>(false);
  const [projects, setProjects] = useState<ProjectRef[]>([]);
  const [activeProject, setActiveProject] = useState<ProjectRef | null>(null);
  const [showOpenProject, setShowOpenProject] = useState<boolean>(false);
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
    openFilesRef.current = openFiles;
  }, [openFiles]);

  // If the swarm goes quiet for a while the UI must not stay locked on
  // "running" forever. The backend has a 15 minute context timeout, so warn
  // before that and let the user send a new request.
  useEffect(() => {
    if (!isRunning) {
      setStalled(false);
      return;
    }
    const timer = setTimeout(() => {
      setStalled(true);
      setTerminalLogs((prev) => [
        ...prev,
        '[Warning] No swarm activity for 3 minutes. The daemon may be busy or unreachable.',
      ]);
    }, 3 * 60 * 1000);
    return () => clearTimeout(timer);
  }, [isRunning, currentRound]);

  useEffect(() => {
    wsClient.connect();
    const unsubStatus = wsClient.onStatus(setIsConnected);

    const unsubEvent = wsClient.onEvent((event) => {
      if (event.usage) {
        setAgentUsage((prev) => ({ ...prev, ...event.usage }));
      }
      switch (event.type) {
        case 'task_step': {
          // A step joins its agent's existing thread rather than becoming a new
          // message. Threads are per agent, not per task: under a parallel batch
          // one agent's reasoning is split across several tasks, and splitting it
          // further by task would make it unreadable.
          if (!event.task_id || !event.stream) break;
          const step = event.stream;
          // A parallel worker reports the coder role but names itself, so the
          // threads stay separate. Without this, three concurrent coders would
          // render as one agent.
          const threaded = { ...step, agent: (event.stream_agent || step.agent) as TaskStreamStep['agent'] };
          appendTaskStep(event.task_id, threaded, event.stream_reset);
          setActiveTaskId(event.task_id);
          // The title travels with the event. It used to be recovered by parsing
          // the opening step's prose, which broke the moment that sentence was
          // reworded or translated.
          if (event.task_title) {
            setTaskStreamTitles((prev) => ({ ...prev, [event.task_id!]: event.task_title! }));
          }
          break;
        }

        case 'locks_changed':
          setFileLocks(event.locks ?? []);
          break;

        case 'round_started':
          setCurrentRound(event.round);
          setIsRunning(true);
          // A new round means the swarm is responsive again.
          setStalled(false);
          if (event.plan) {
            setProjectPlan(event.plan);
          }
          setTerminalLogs((prev) => [...prev, `[Swarm] Round ${event.round} started...`]);
          break;

        case 'agent_thinking':
          // The manager reports which pipeline is running. Surfacing it makes
          // it obvious when a selected team was silently ignored.
          if (event.agent === 'manager' && typeof event.payload === 'string') {
            const path = event.payload;
            const isTeam = path.startsWith('team:');
            setTerminalLogs((prev) => [
              ...prev,
              isTeam
                ? `[Manager] تیم دلخواه در حال اجرا: ${path.slice(5)}`
                : '[Manager] اجرای تیم پیش‌فرض ۵ نفره (تیم دلخواه اعمال نشد)',
            ]);
            break;
          }
          if (event.payload) {
            const role = event.agent || 'ai';
            const status = typeof event.payload === 'string' ? event.payload : (event.payload.status || 'در حال تحلیل...');
            setThinkingAgent({ role, status });
          }
          break;

        case 'agent_output':
          setThinkingAgent(null);
          if (event.message) {
            setMessages((prev) => [...prev, event.message!]);
            if (event.message.summary) {
              setTerminalLogs((prev) => [...prev, `[${event.message!.from.toUpperCase()}] ${event.message!.summary}`]);
            }
          }
          if (event.files && event.files.length > 0) {
            setFiles(event.files);
            // Auto open first file if none open
            if (openFilesRef.current.length === 0) {
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

        case 'consensus_reached': {
          if (event.verdict) {
            setVerdict(event.verdict);
          }
          if (event.files) {
            setFiles(event.files);
          }
          if (event.plan) {
            setProjectPlan(event.plan);
          }
          setIsRunning(false);

          // The run ending is not the same as the project being finished. A run
          // can end with tasks still open because the team could not close them,
          // and announcing "ready to merge" in that case is how unfinished work
          // gets mistaken for delivered work. The backend sends the real
          // outcome in payload; it is used whenever present.
          const unfinished = (event.plan?.tasks ?? []).filter(
            (t) => t.state !== 'DONE' && t.state !== 'CANCELLED'
          );
          const summary =
            (typeof event.payload === 'string' && event.payload.trim()) ||
            (unfinished.length > 0
              ? `Run finished. ${unfinished.length} task(s) still open: ` +
                unfinished.map((t) => `${t.id} ${t.title}`).join(', ')
              : `[Swarm Consensus Reached] Final Quality Score: ${event.verdict?.final_score.toFixed(1)}% - Ready to merge!`);

          setTerminalLogs((prev) => [...prev, summary]);
          break;
        }

        case 'plan_updated':
          if (event.plan) {
            setProjectPlan(event.plan);
            const done = event.plan.tasks.filter((t: any) => t.state === 'DONE').length;
            const total = event.plan.tasks.length;
            setTerminalLogs((prev) => [
              ...prev,
              `[Plan] پیشرفت: ${done}/${total} تسک تکمیل شد.`,
            ]);
          }
          break;

        case 'error':
          setIsRunning(false);
          setThinkingAgent(null);
          setStalled(false);
          setTerminalLogs((prev) => [...prev, `[Error] ${event.payload || 'Swarm error'}`]);
          break;

        case 'teams_data':
          setTeams(event.teams || []);
          if (event.default_team_id) {
            // Adopt the server's default once, but never override a team the
            // user has already picked.
            const adopted = (current: string) =>
              current && current !== 'default' ? current : event.default_team_id!;
            setSelectedTeamId(adopted);
          }
          break;

        case 'team_saved':
          if (event.team) {
            const saved = event.team as Team;
            setTeams((prev) => {
              const idx = prev.findIndex((t) => t.id === saved.id);
              if (idx >= 0) {
                const next = [...prev];
                next[idx] = saved;
                return next;
              }
              return [...prev, saved];
            });
            if (!saved.id) {
              setSelectedTeamId(saved.id);
            }
            setTerminalLogs((prev) => [...prev, `[Team] ذخیره شد: ${saved.name}`]);
          }
          break;

        case 'team_deleted':
          setTeams((prev) => prev.filter((t) => t.id !== event.team_id));
          setTerminalLogs((prev) => [...prev, `[Team] حذف شد.`]);
          break;
      }
    });

    return () => {
      unsubStatus();
      unsubEvent();
    };
    // The swarm subscription must survive state changes. Re-subscribing on
    // every openFiles/projectName change tore down the WebSocket handler
    // mid-round and silently dropped swarm events.
  }, []);

  const handleSaveApiKeys = (keys: any) => {
    setApiKeys(keys);
    localStorage.setItem('nexusai_api_keys', JSON.stringify(keys));
  };

  // ---- Custom teams ----
  const handleSaveTeam = (team: Team): Promise<void> => {
    return new Promise((resolve) => {
      // Save over HTTP so validation errors come back with a real message
      // instead of a silently ignored WebSocket frame.
      fetch('/api/teams', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(team),
      })
        .then(async (res) => {
          const text = await res.text();
          if (!res.ok) {
            let msg = text || `HTTP ${res.status}`;
            try {
              const parsed = JSON.parse(text);
              msg = parsed.error || parsed.message || msg;
            } catch (e) {}
            throw new Error(msg);
          }
          const saved: Team = JSON.parse(text);
          setTeams((prev) => {
            const idx = prev.findIndex((t) => t.id === saved.id && saved.id);
            if (idx >= 0) {
              const next = [...prev];
              next[idx] = saved;
              return next;
            }
            return [...prev, saved];
          });
          setSelectedTeamId(saved.id);
          resolve();
        })
        .catch((e) => {
          // Surface the failure through the modal's own error state.
          throw e;
        });
    });
  };

  // ---- Projects on disk ----
  const loadProjectList = useCallback(async () => {
    try {
      const res = await fetch('/api/projects');
      if (!res.ok) return;
      const data: ProjectRef[] = await res.json();
      setProjects(Array.isArray(data) ? data : []);
    } catch (e) {
      // The daemon may not be running yet; the modal will report the real error.
    }
  }, []);

  useEffect(() => {
    loadProjectList();
  }, [loadProjectList]);

  const handleOpenProject = async (path: string): Promise<ProjectRef> => {
    const ref = await handleOpenFolder(path);
    await loadProjectList();
    return ref;
  };

  const handleSelectProject = async (ref: ProjectRef) => {
    await handleOpenFolder(ref.root);
  };

  const handleDeleteTeam = async (teamId: string) => {
    try {
      const res = await fetch(`/api/teams/${teamId}`, { method: 'DELETE' });
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP ${res.status}`);
      }
      setTeams((prev) => prev.filter((t) => t.id !== teamId));
      if (selectedTeamId === teamId) {
        setSelectedTeamId('default');
      }
    } catch (e: any) {
      setTerminalLogs((prev) => [...prev, `[Team] حذف ناموفق: ${e?.message || e}`]);
    }
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

  const handleSelectFile = async (path: string, pid?: string) => {
    const currentPid = pid || projectId;
    if (!openFiles.includes(path)) {
      setOpenFiles((prev) => [...prev, path]);
    }
    setActiveFile(path);

    const existing = files.find((f) => f.path === path);
    if (currentPid && (!existing || !existing.content)) {
      try {
        const res = await fetch(`/api/projects/${currentPid}/file?path=${encodeURIComponent(path)}`);
        if (res.ok) {
          const data = await res.json();
          setFiles((prev) => {
            const idx = prev.findIndex((f) => f.path === path);
            const updated: CodeFile = {
              path: data.path || path,
              language: data.language || 'plaintext',
              content: data.content || '',
            };
            if (idx >= 0) {
              const copy = [...prev];
              copy[idx] = updated;
              return copy;
            }
            return [...prev, updated];
          });
        }
      } catch (err) {
        console.error('Failed to load file content:', err);
      }
    }
  };

  const handleOpenFolder = async (path: string): Promise<ProjectRef> => {
    try {
      setTerminalLogs((prev) => [...prev, `[Project] Opening folder: ${path}...`]);
      const res = await fetch('/api/projects', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path }),
      });
      if (!res.ok) {
        const text = await res.text();
        let msg = text || `HTTP ${res.status}`;
        try {
          const parsed = JSON.parse(text);
          msg = parsed.error || msg;
        } catch (e) {}
        throw new Error(msg);
      }
      const ref = await res.json();
      setProjectId(ref.id);
      setProjectName(ref.name);

      // Fetch file tree
      const treeRes = await fetch(`/api/projects/${ref.id}`);
      if (treeRes.ok) {
        const tree = await treeRes.json();
        const treeFiles: CodeFile[] = (tree.files || []).map((f: { path: string }) => {
          const ext = f.path.split('.').pop()?.toLowerCase() || '';
          let lang = 'plaintext';
          if (ext === 'go') lang = 'go';
          else if (ext === 'ts' || ext === 'tsx') lang = 'typescript';
          else if (ext === 'js' || ext === 'jsx') lang = 'javascript';
          else if (ext === 'py') lang = 'python';
          else if (ext === 'json') lang = 'json';
          else if (ext === 'md') lang = 'markdown';
          return {
            path: f.path,
            language: lang,
            content: '',
          };
        });
        setFiles(treeFiles);
        if (treeFiles.length > 0) {
          handleSelectFile(treeFiles[0].path, ref.id);
        } else {
          setOpenFiles([]);
          setActiveFile('');
        }
        setTerminalLogs((prev) => [
          ...prev,
          `[Project] Folder opened: ${ref.name} (${treeFiles.length} files discovered)`,
        ]);

        // Fetch plan if available
        try {
          const planRes = await fetch(`/api/projects/${ref.id}/plan`);
          if (planRes.ok) {
            const planData = await planRes.json();
            if (planData && Array.isArray(planData.tasks) && planData.tasks.length > 0) {
              setProjectPlan(planData);
            } else {
              setProjectPlan(null);
            }
          }
        } catch {
          setProjectPlan(null);
        }
      }
      return ref;
    } catch (e: any) {
      setTerminalLogs((prev) => [...prev, `[Project Error] Failed to open folder: ${e?.message || e}`]);
      // Propagate so the modal can show why instead of closing silently.
      throw e;
    }
  };

  const handleCloseTab = (path: string) => {
    const updated = openFiles.filter((p) => p !== path);
    setOpenFiles(updated);
    if (activeFile === path) {
      setActiveFile(updated[updated.length - 1] || '');
    }
  };

  const handleSendPrompt = (promptText: string) => {
    const trimmed = promptText.trim();
    if (!trimmed) return;

    // A custom team carries its own per-agent models, so the fixed-role model
    // picker does not apply. Only the built-in roster needs that check.
    const activeTeam = teams.find((t) => t.id === selectedTeamId);
    // A saved team is only "custom" once it is actually loaded. Before the
    // list arrives every id is unknown, and treating that as the built-in
    // roster would silently send the old five-agent request instead.
    const usingCustomTeam = !!activeTeam && selectedTeamId !== 'default';

    if (usingCustomTeam && !activeTeam!.agents.some((a) => a.model)) {
      setMessages((prev) => [
        ...prev,
        {
          id: `system-${Date.now()}`,
          from: 'architect',
          round: currentRound,
          type: 'error',
          content: '',
          summary: `تیم «${activeTeam!.name}» مدل ندارد.`,
          timestamp: new Date().toISOString(),
        },
      ]);
      return;
    }

    if (!usingCustomTeam && unassignedAgents.length > 0) {
      setMessages((prev) => [
        ...prev,
        {
          id: `system-${Date.now()}`,
          from: 'architect',
          round: currentRound,
          type: 'error',
          content: '',
          summary:
            `Swarm not started: choose a model for ${unassignedAgents.join(', ')} first.`,
          timestamp: new Date().toISOString(),
        },
      ]);
      return;
    }

    const proj = projectId || projectName || 'my-nexus-project';
    if (!projectName) {
      setProjectName(proj);
    }
    setIsRunning(true);
    setAgentUsage({});

    // Append user prompt to messages
    setMessages((prev) => [
      ...prev,
      {
        id: `user-${Date.now()}`,
        from: 'architect',
        round: currentRound,
        type: 'user_prompt',
        content: trimmed,
        summary: `User Prompt: ${trimmed}`,
        timestamp: new Date().toISOString(),
      },
    ]);

    // Send to Go backend daemon via WebSocket with individual agent models and API keys
    wsClient.send(
      'start_swarm',
      proj,
      {
        mode: executionMode,
        // When a custom team is selected the backend runs that team instead of
        // the fixed five, so the roster map must not be sent.
        ...(usingCustomTeam
          ? { team_id: selectedTeamId }
          : { models: agentModels }),
        apiKeys,
      },
      trimmed,
    );
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
        onOpenProjectModal={() => setShowOpenProject(true)}
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
            projectId={projectId}
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

            </div>
          ) : (
            // Cursor Style Clean Welcome Screen
            <div className="flex-1 overflow-hidden relative">
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
            </div>
          )}

          {/* Collapsible Bottom Terminal Panel */}
          {showBottomPanel && (
            <TerminalView
              logs={terminalLogs}
              onClear={() => setTerminalLogs([])}
              onRunCommand={(cmd) => {
                // The daemon exposes no shell endpoint, so be explicit rather
                // than echoing a fake success line the user would trust.
                setTerminalLogs((prev) => [
                  ...prev,
                  `> ${cmd}`,
                  '[error] Shell execution is not wired up yet. Run the command in your own terminal.',
                ]);
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
            agentModels={agentModels}
            onUpdateAgentModel={handleUpdateAgentModel}
            onApplyPreset={handleApplyPreset}
            onSendPrompt={handleSendPrompt}
            onAcceptChanges={() => {
              setTerminalLogs((prev) => [...prev, "[Composer] Changes accepted and merged into workspace."]);
            }}
            thinkingAgent={thinkingAgent}
            stalled={stalled}
            connected={isConnected}
            teams={teams}
            selectedTeamId={selectedTeamId}
            onSelectTeam={setSelectedTeamId}
            onOpenTeamBuilder={() => setShowTeamBuilder(true)}
            plan={projectPlan}
            usage={agentUsage}
            taskStreams={taskStreams}
            taskStreamTitles={taskStreamTitles}
            activeTaskId={activeTaskId}
            fileLocks={fileLocks}
            agentConversations={agentConversations}
          />
        )}

      {showTeamBuilder && (
        <TeamBuilderModal
          isOpen={showTeamBuilder}
          onClose={() => setShowTeamBuilder(false)}
          teams={teams}
          selectedTeamId={selectedTeamId}
          onSelectTeam={setSelectedTeamId}
          onSaveTeam={handleSaveTeam}
          onDeleteTeam={handleDeleteTeam}
        />
      )}

      {showOpenProject && (
        <OpenProjectModal
          isOpen={showOpenProject}
          onClose={() => setShowOpenProject(false)}
          projects={projects}
          activeProjectId={activeProject?.id || ''}
          onOpenProject={handleOpenProject}
          onSelectProject={handleSelectProject}
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
        agentModels={agentModels}
        onUpdateAgentModel={handleUpdateAgentModel}
        onApplyPreset={handleApplyPreset}
      />

      {/* New Project Modal */}
      <NewProjectModal
        isOpen={isNewProjectOpen}
        onClose={() => setIsNewProjectOpen(false)}
        onCreateProject={handleCreateProject}
        onOpenFolder={handleOpenFolder}
      />
    </div>
  );
};
