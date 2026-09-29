import React, { useState, useEffect } from 'react';
import { 
  Code, 
  Swords, 
  Scale, 
  Package, 
  BrainCircuit, 
  Settings, 
  Layers, 
  Cpu, 
  Wifi, 
  WifiOff 
} from 'lucide-react';
import { IdeView } from './components/ide/IdeView';
import { AgentArenaView } from './components/arena/AgentArenaView';
import { ArbiterView } from './components/arbiter/ArbiterView';
import { SkillsView } from './components/skills/SkillsView';
import { MemoryView } from './components/memory/MemoryView';
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

type ActiveTab = 'ide' | 'arena' | 'arbiter' | 'skills' | 'memory';

export const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState<ActiveTab>('ide');
  const [executionMode, setExecutionMode] = useState<ExecutionMode>('rolebased');
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const [isRunning, setIsRunning] = useState<boolean>(false);
  const [currentRound, setCurrentRound] = useState<number>(1);
  const [projectID, setProjectID] = useState<string>('nexus-project-alpha');

  // Core State
  const [files, setFiles] = useState<CodeFile[]>([
    {
      path: 'main.go',
      language: 'go',
      content: `package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Server coordinates HTTP requests with timeout protection.
type Server struct {
	addr string
}

func NewServer(addr string) *Server {
	return &Server{addr: addr}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\\"status\\":\\"healthy\\",\\"time\\":\\"%s\\"}", time.Now().UTC().Format(time.RFC3339))
	})

	srv := &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("NexusAI service listening on %s", s.addr)
	return srv.ListenAndServe()
}

func main() {
	srv := NewServer(":8080")
	if err := srv.Start(context.Background()); err != nil {
		log.Fatalf("service stopped: %v", err)
	}
}
`,
    },
    {
      path: 'auth/jwt.go',
      language: 'go',
      content: `package auth

import (
	"errors"
	"time"
)

var ErrExpiredToken = errors.New("token has expired")

type TokenManager struct {
	secretKey []byte
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secretKey: []byte(secret)}
}

func (m *TokenManager) Validate(token string) (bool, error) {
	if token == "" {
		return false, errors.New("empty token")
	}
	// Constant-time token verification
	return true, nil
}
`,
    }
  ]);

  const [messages, setMessages] = useState<AgentMessage[]>([
    {
      id: 'init-1',
      from: 'architect',
      round: 1,
      type: 'blueprint',
      content: 'Decomposed project into main server and auth subpackages.',
      summary: 'Architecture Blueprint established: Server with JWT authentication and Redis rate-limiting contracts.',
      timestamp: new Date().toISOString(),
    },
    {
      id: 'init-2',
      from: 'coder',
      round: 1,
      type: 'code_draft',
      content: 'Generated initial main.go and auth/jwt.go.',
      summary: 'Lead Coder generated complete code with graceful timeouts and clean interfaces.',
      timestamp: new Date().toISOString(),
    },
    {
      id: 'init-3',
      from: 'critic',
      round: 1,
      type: 'critic_review',
      content: 'Detected missing timeout in HTTP server configuration.',
      summary: 'Critic flagged missing server read/write timeouts (Refactored in main.go).',
      timestamp: new Date().toISOString(),
    },
    {
      id: 'init-4',
      from: 'security',
      round: 1,
      type: 'security_report',
      content: 'Verified token validation sanitization and lack of exposed secrets.',
      summary: 'Security Auditor passed JWT verification check with 0 vulnerabilities.',
      timestamp: new Date().toISOString(),
    },
    {
      id: 'init-5',
      from: 'arbiter',
      round: 1,
      type: 'arbiter_verdict',
      content: 'Consensus score: 91.5% - Approved.',
      summary: 'Consensus reached in Round 1: Final Quality Score 91.5% (Threshold >= 85%).',
      timestamp: new Date().toISOString(),
    }
  ]);

  const [criticIssues, setCriticIssues] = useState<ReviewIssue[]>([
    {
      file: 'main.go',
      severity: 'minor',
      description: 'Ensure graceful shutdown signal trap is added.',
      suggested_fix: 'signal.Notify(stop, os.Interrupt)',
    }
  ]);

  const [securityVulns, setSecurityVulns] = useState<ReviewIssue[]>([]);
  const [verdict, setVerdict] = useState<ArbiterVerdict | null>({
    round: 1,
    final_score: 91.5,
    threshold: 85.0,
    passed: true,
    needs_refinement: false,
    dimensions: [
      { name: 'Code Correctness', weight: 0.30, score: 92.0, explanation: 'Idiomatic Go with zero race hazards' },
      { name: 'Bug Resolution', weight: 0.25, score: 90.0, explanation: 'Critic suggestions fully addressed' },
      { name: 'Security Compliance', weight: 0.20, score: 96.0, explanation: 'No injection or credential leakage' },
      { name: 'Code Quality', weight: 0.15, score: 88.0, explanation: 'Clean Architecture with comments' },
      { name: 'Spec Adherence', weight: 0.10, score: 92.0, explanation: 'Matches Architect blueprint contracts' },
    ],
    feedback_to_coder: 'Approved for workspace merge.',
    summary: 'Swarm reached consensus with 91.5% rating.',
  });

  const [projectMemories, setProjectMemories] = useState<MemoryEntry[]>([]);
  const [globalMemories, setGlobalMemories] = useState<MemoryEntry[]>([]);
  const [skillsList, setSkillsList] = useState<SkillManifest[]>([]);

  // Connect WebSocket on Mount
  useEffect(() => {
    wsClient.connect();
    const unsubStatus = wsClient.onStatus(setIsConnected);

    const unsubEvent = wsClient.onEvent((event) => {
      switch (event.type) {
        case 'round_started':
          setCurrentRound(event.round);
          setIsRunning(true);
          break;

        case 'agent_output':
          if (event.message) {
            setMessages((prev) => [...prev, event.message!]);
          }
          if (event.files && event.files.length > 0) {
            setFiles(event.files);
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
          break;

        case 'error':
          setIsRunning(false);
          break;
      }
    });

    return () => {
      unsubStatus();
      unsubEvent();
    };
  }, []);

  const handleSendPrompt = (prompt: string) => {
    setIsRunning(true);
    // Add user note
    setMessages((prev) => [
      ...prev,
      {
        id: `user-${Date.now()}`,
        from: 'architect',
        round: currentRound,
        type: 'user_prompt',
        content: prompt,
        summary: `User Prompt: ${prompt}`,
        timestamp: new Date().toISOString(),
      },
    ]);

    // Dispatch to Go daemon via WebSocket
    wsClient.send('start_swarm', projectID, { mode: executionMode }, prompt);
  };

  const handleUpdateFile = (path: string, newContent: string) => {
    setFiles((prev) =>
      prev.map((f) => (f.path === path ? { ...f, content: newContent } : f))
    );
  };

  return (
    <div className="flex flex-col h-screen w-screen bg-[#141416] text-zinc-100 select-none overflow-hidden">
      {/* Top Application Header */}
      <header className="h-11 bg-[#18181b] border-b border-[#27272a] px-4 flex items-center justify-between z-10">
        <div className="flex items-center gap-4">
          {/* Logo & Brand */}
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded-md bg-gradient-to-tr from-indigo-600 to-purple-500 flex items-center justify-center font-black text-xs text-white shadow-sm shadow-indigo-500/30">
              N
            </div>
            <span className="font-bold text-sm tracking-tight text-zinc-100">
              Nexus<span className="text-indigo-400">AI</span>
            </span>
            <span className="text-[10px] font-mono text-zinc-500 ml-1">IDE v1.0</span>
          </div>

          {/* Navigation Tabs */}
          <nav className="flex items-center gap-1 ml-4 bg-zinc-900/80 p-0.5 rounded-lg border border-[#27272a]">
            <button
              onClick={() => setActiveTab('ide')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-all ${
                activeTab === 'ide'
                  ? 'bg-indigo-600 text-white shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              <Code size={13} />
              <span>IDE</span>
            </button>

            <button
              onClick={() => setActiveTab('arena')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-all ${
                activeTab === 'arena'
                  ? 'bg-indigo-600 text-white shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              <Swords size={13} />
              <span>Agent Arena</span>
            </button>

            <button
              onClick={() => setActiveTab('arbiter')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-all ${
                activeTab === 'arbiter'
                  ? 'bg-indigo-600 text-white shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              <Scale size={13} />
              <span>Arbiter Matrix</span>
              {verdict && (
                <span className="text-[10px] font-mono px-1 rounded bg-amber-950/60 text-amber-400 ml-0.5">
                  {verdict.final_score.toFixed(0)}%
                </span>
              )}
            </button>

            <button
              onClick={() => setActiveTab('skills')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-all ${
                activeTab === 'skills'
                  ? 'bg-indigo-600 text-white shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              <Package size={13} />
              <span>Skills & MCP</span>
            </button>

            <button
              onClick={() => setActiveTab('memory')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-md text-xs font-medium transition-all ${
                activeTab === 'memory'
                  ? 'bg-indigo-600 text-white shadow-sm'
                  : 'text-zinc-400 hover:text-zinc-200'
              }`}
            >
              <BrainCircuit size={13} />
              <span>Memory (Mem0)</span>
            </button>
          </nav>
        </div>

        {/* Right Header Controls */}
        <div className="flex items-center gap-3">
          {/* Execution Mode Selector */}
          <div className="flex items-center gap-1.5 text-xs bg-zinc-900/80 px-2 py-1 rounded border border-[#27272a]">
            <Cpu size={12} className="text-zinc-400" />
            <span className="text-zinc-400 text-[11px]">Mode:</span>
            <select
              value={executionMode}
              onChange={(e) => setExecutionMode(e.target.value as ExecutionMode)}
              className="bg-transparent text-indigo-300 font-medium text-xs outline-none cursor-pointer"
            >
              <option value="rolebased" className="bg-zinc-900 text-zinc-100">Role-Based (Phased)</option>
              <option value="concurrent" className="bg-zinc-900 text-zinc-100">Concurrent (Channels)</option>
              <option value="sequential" className="bg-zinc-900 text-zinc-100">Sequential (Linear)</option>
            </select>
          </div>

          {/* Connection Status Indicator */}
          <div className="flex items-center gap-1.5 text-[11px]">
            {isConnected ? (
              <span className="flex items-center gap-1 text-emerald-400 font-medium">
                <Wifi size={13} />
                <span>Daemon Live (:8080)</span>
              </span>
            ) : (
              <span className="flex items-center gap-1 text-zinc-500 font-medium">
                <WifiOff size={13} />
                <span>Daemon Offline</span>
              </span>
            )}
          </div>
        </div>
      </header>

      {/* Main View Port */}
      <main className="flex-1 flex overflow-hidden">
        {activeTab === 'ide' && (
          <IdeView
            files={files}
            messages={messages}
            criticIssues={criticIssues}
            securityVulns={securityVulns}
            isRunning={isRunning}
            onSendPrompt={handleSendPrompt}
            onUpdateFileContent={handleUpdateFile}
          />
        )}

        {activeTab === 'arena' && (
          <AgentArenaView
            messages={messages}
            verdict={verdict}
            currentRound={currentRound}
          />
        )}

        {activeTab === 'arbiter' && (
          <ArbiterView
            verdict={verdict}
            files={files}
          />
        )}

        {activeTab === 'skills' && (
          <SkillsView skills={skillsList} />
        )}

        {activeTab === 'memory' && (
          <MemoryView
            projectMemories={projectMemories}
            globalMemories={globalMemories}
          />
        )}
      </main>

      {/* Bottom Status Bar */}
      <footer className="h-6 bg-[#18181b] border-t border-[#27272a] px-3 flex items-center justify-between text-[11px] text-zinc-400 z-10 font-mono">
        <div className="flex items-center gap-4">
          <span className="flex items-center gap-1 text-indigo-400">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-500"></span>
            Nexus Swarm Active
          </span>
          <span>Project: <strong className="text-zinc-200">{projectID}</strong></span>
          <span>Round: <strong className="text-zinc-200">{currentRound}/5</strong></span>
          <span>Execution: <strong className="text-zinc-200 capitalize">{executionMode}</strong></span>
          <span>Swarm Team: <strong className="text-zinc-200">5 Agents (Architect, Coder, Critic, Security, Arbiter)</strong></span>
        </div>

        <div className="flex items-center gap-3">
          {verdict && (
            <span className="flex items-center gap-1">
              <span>Consensus:</span>
              <strong className={verdict.passed ? 'text-emerald-400' : 'text-amber-400'}>
                {verdict.final_score.toFixed(1)}% ({verdict.passed ? 'Approved' : 'Iterating'})
              </strong>
            </span>
          )}
          <span>UTF-8</span>
          <span>Go 1.26</span>
        </div>
      </footer>
    </div>
  );
};
