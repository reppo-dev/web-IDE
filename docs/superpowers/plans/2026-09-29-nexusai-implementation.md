# NexusAI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build NexusAI, an autonomous desktop IDE powered by a 5-agent adversarial collaborative swarm, a Go-based concurrent orchestrator with 3 execution modes, a Mem0-style persistent SQLite memory system, skills/MCP support, and an Eclipse Theia/Monaco dark-themed desktop frontend.

**Architecture:** The desktop app communicates via WebSocket and gRPC with a local Go daemon (`nexusd`). The daemon manages 5 agent personas (Architect, Coder, Critic, Security, Arbiter) using goroutines and channels across 3 modes (Sequential, Concurrent, Role-based), connects to provider-agnostic OpenAI-compatible LLM endpoints, manages SQLite memories (per-project and global), and streams real-time updates to Monaco Editor, Agent Arena, Arbiter Scoreboard, and Memory panels.

**Tech Stack:** 
- Backend: Go 1.22+ (`go1.26.0`), SQLite (`modernc.org/sqlite` or `github.com/mattn/go-sqlite3`), Gorilla WebSocket, gRPC/Protobuf
- Frontend: TypeScript, React, Monaco Editor, Tailwind CSS / Theia extensions, Electron / Vite
- Protocol: JSON-over-WebSocket for UI streaming, OpenAI-compatible JSON Schema for LLM calls

**Spec:** [docs/superpowers/specs/2026-09-29-nexusai-design.md](file:///g:/v/IDE/docs/superpowers/specs/2026-09-29-nexusai-design.md)

## Global Constraints

- Backend must be strictly Go with zero external database dependencies (embedded SQLite only).
- All 5 agents must produce structured JSON validated against JSON Schemas.
- Arbiter consensus requires a score >= 85% with 5 weighted dimensions.
- Concurrency must use native Go goroutines, channels, and context cancellation.
- Memory must separate per-project data (`memories.db`) and cross-project knowledge (`global.db`).
- All code must include comprehensive automated tests (unit tests for Go packages, components for UI).

---

### Task 1: Repository Structure & Backend Initialization

**Files:**
- Create: `backend/go.mod`
- Create: `backend/cmd/nexusd/main.go`
- Create: `Makefile`
- Create: `.gitignore`

**Interfaces:**
- Consumes: Go toolchain
- Produces: Base Go module and build automation

- [ ] **Step 1: Initialize Git and .gitignore**
  Initialize git repo if needed, create `.gitignore` ignoring Go binaries, node_modules, `.db`, data directory.

- [ ] **Step 2: Initialize Go module**
  Run `go mod init github.com/nexusai/nexusai/backend` in `backend/` and add dependencies (`gorilla/websocket`, `google/uuid`, `modernc.org/sqlite`).

- [ ] **Step 3: Create base Makefile**
  Add commands: `make build-backend`, `make test-backend`, `make run-backend`.

- [ ] **Step 4: Verify build**
  Run `go test ./...` in `backend/` to verify clean setup.

- [ ] **Step 5: Commit**
  `git add . && git commit -m "chore: initialize repository and backend go module"`

---

### Task 2: Core Domain Types & Data Models

**Files:**
- Create: `backend/internal/domain/types.go`
- Test: `backend/internal/domain/types_test.go`

**Interfaces:**
- Consumes: None
- Produces: `AgentRole`, `ExecutionMode`, `AgentMessage`, `MemoryEntry`, `SwarmStatus`, `ScoringDimension`, `ArbiterVerdict`

- [ ] **Step 1: Write failing domain types test**
  Test JSON serialization and deserialization of `AgentMessage`, `MemoryEntry`, and `ArbiterVerdict`.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/domain/...`

- [ ] **Step 3: Implement domain types**
  Implement the enum constants and structs in `backend/internal/domain/types.go`.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test ./backend/internal/domain/...`

- [ ] **Step 5: Commit**
  `git add backend/internal/domain/ && git commit -m "feat(domain): add core domain types and models"`

---

### Task 3: Mem0-Style Local Memory System (SQLite)

**Files:**
- Create: `backend/internal/memory/store.go`
- Create: `backend/internal/memory/sqlite.go`
- Test: `backend/internal/memory/sqlite_test.go`

**Interfaces:**
- Consumes: `domain.MemoryEntry`
- Produces: `MemoryStore` interface (`WriteProject`, `QueryProject`, `WriteGlobal`, `QueryGlobal`, `DeleteMemory`)

- [ ] **Step 1: Write failing SQLite memory tests**
  Write tests covering:
  - Database table initialization for project and global DBs
  - Writing memories with tags and confidence
  - Querying by agent, type, and tag filtering
  - Global cross-project memory retrieval

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/memory/...`

- [ ] **Step 3: Implement SQLite memory store**
  Implement `backend/internal/memory/sqlite.go` with pure Go SQLite driver (`modernc.org/sqlite`).
  Include indices for fast retrieval and thread-safe mutex operations.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/memory/...`

- [ ] **Step 5: Commit**
  `git add backend/internal/memory/ && git commit -m "feat(memory): implement mem0-style sqlite persistent memory"`

---

### Task 4: Provider-Agnostic LLM Client & JSON Schema Output

**Files:**
- Create: `backend/internal/llm/client.go`
- Create: `backend/internal/llm/openai_compat.go`
- Create: `backend/internal/llm/schema.go`
- Test: `backend/internal/llm/client_test.go`

**Interfaces:**
- Consumes: LLM Provider config (`BaseURL`, `APIKey`, `Model`)
- Produces: `LLMClient` interface (`CompleteStructured(ctx, prompt, schema) (string, error)`)

- [ ] **Step 1: Write failing tests with mock HTTP server**
  Test OpenAI-compatible endpoint client with JSON schema structured output and mock HTTP server returning simulated JSON.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/llm/...`

- [ ] **Step 3: Implement LLM client**
  Implement HTTP request formatting, retry with backoff, authentication header injection, and response parsing.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/llm/...`

- [ ] **Step 5: Commit**
  `git add backend/internal/llm/ && git commit -m "feat(llm): implement provider-agnostic structured llm client"`

---

### Task 5: The 5 Specialized Agent Personas & Prompts

**Files:**
- Create: `backend/internal/agent/agent.go`
- Create: `backend/internal/agent/architect.go`
- Create: `backend/internal/agent/coder.go`
- Create: `backend/internal/agent/critic.go`
- Create: `backend/internal/agent/security.go`
- Create: `backend/internal/agent/arbiter.go`
- Test: `backend/internal/agent/agent_test.go`

**Interfaces:**
- Consumes: `domain.AgentRole`, `llm.LLMClient`, `memory.MemoryStore`
- Produces: `Agent` interface (`Execute(ctx, input) (AgentOutput, error)`)

- [ ] **Step 1: Write failing agent persona tests**
  Verify each persona loads its system prompt, injects relevant memories, and invokes LLM client with expected JSON schema.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/agent/...`

- [ ] **Step 3: Implement 5 agent personas**
  - Architect: Decomposes requirements into blueprints and file trees (Persian & English support).
  - Lead Coder: Generates complete multi-file code with rationale.
  - Critic (Red Team): Detects logic flaws, edge cases, and demands specific refactors.
  - Security Auditor: Scans for OWASP/CWE security vulnerabilities.
  - Arbiter: Evaluates rounds, calculates performance and issues verdicts.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/agent/...`

- [ ] **Step 5: Commit**
  `git add backend/internal/agent/ && git commit -m "feat(agent): implement 5 specialized swarm agents"`

---

### Task 6: Arbiter Scoring & Consensus Engine

**Files:**
- Create: `backend/internal/scoring/scorer.go`
- Create: `backend/internal/scoring/consensus.go`
- Test: `backend/internal/scoring/scorer_test.go`

**Interfaces:**
- Consumes: Critic feedback, Security audit, Coder changes
- Produces: Weighted score (0-100%), Verdict (`APPROVED`, `REFINE`, `REJECT`), Consensus report

- [ ] **Step 1: Write failing scoring tests**
  Test weight calculation:
  - Correctness (30%)
  - Bug Resolution (25%)
  - Security Compliance (20%)
  - Code Quality (15%)
  - Spec Adherence (10%)
  Verify threshold behavior (<85% triggers REFINE, >=85% triggers APPROVED).

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/scoring/...`

- [ ] **Step 3: Implement scoring engine and consensus decider**
  Calculate metrics, track penalty points, and produce consensus decision.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/scoring/...`

- [ ] **Step 5: Commit**
  `git add backend/internal/scoring/ && git commit -m "feat(scoring): implement arbiter weighted consensus engine"`

---

### Task 7: Multi-Agent Orchestrator — Sequential Mode

**Files:**
- Create: `backend/internal/orchestrator/orchestrator.go`
- Create: `backend/internal/orchestrator/sequential.go`
- Test: `backend/internal/orchestrator/sequential_test.go`

**Interfaces:**
- Consumes: 5 Agents, MemoryStore, ScoringEngine
- Produces: `Orchestrator` execution pipeline for Sequential mode with channel event streaming

- [ ] **Step 1: Write failing sequential mode test**
  Simulate full multi-round execution from prompt to consensus using mock agents.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/orchestrator/...`

- [ ] **Step 3: Implement sequential orchestrator**
  Execute: Architect -> Coder -> Critic -> Security -> Arbiter in order. If score < 85%, loop back to Coder with feedback.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/orchestrator/sequential_test.go`

- [ ] **Step 5: Commit**
  `git add backend/internal/orchestrator/ && git commit -m "feat(orchestrator): implement sequential swarm execution loop"`

---

### Task 8: Multi-Agent Orchestrator — Concurrent Mode

**Files:**
- Create: `backend/internal/orchestrator/concurrent.go`
- Test: `backend/internal/orchestrator/concurrent_test.go`

**Interfaces:**
- Consumes: 5 Agents, Go channels, WaitGroups
- Produces: Concurrent pipeline where agents execute in parallel and exchange messages via channels

- [ ] **Step 1: Write failing concurrent mode test**
  Verify goroutine fan-out, cross-agent communication channel, and synchronization via Arbiter.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/orchestrator/...`

- [ ] **Step 3: Implement concurrent orchestrator**
  Spawn goroutines per agent with message broadcast channels, select blocks, and sync coordination.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/orchestrator/concurrent_test.go`

- [ ] **Step 5: Commit**
  `git add backend/internal/orchestrator/concurrent* && git commit -m "feat(orchestrator): implement concurrent swarm execution mode"`

---

### Task 9: Multi-Agent Orchestrator — Role-Based Mode

**Files:**
- Create: `backend/internal/orchestrator/rolebased.go`
- Test: `backend/internal/orchestrator/rolebased_test.go`

**Interfaces:**
- Consumes: 5 Agents
- Produces: Phased execution (Phase 1: Architect -> Phase 2: Coder+Critic+Security in parallel -> Phase 3: Arbiter)

- [ ] **Step 1: Write failing role-based mode test**
  Test phased execution flow and intermediate channel event broadcasts.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/orchestrator/...`

- [ ] **Step 3: Implement role-based orchestrator**
  Structure phase transitions, collect feedback concurrently in Phase 2, feed into Arbiter in Phase 3.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/orchestrator/rolebased_test.go`

- [ ] **Step 5: Commit**
  `git add backend/internal/orchestrator/rolebased* && git commit -m "feat(orchestrator): implement role-based swarm execution mode"`

---

### Task 10: Skills Manager & MCP Host

**Files:**
- Create: `backend/internal/skills/manager.go`
- Create: `backend/internal/mcp/host.go`
- Test: `backend/internal/skills/manager_test.go`
- Test: `backend/internal/mcp/host_test.go`

**Interfaces:**
- Consumes: Skill folders (`skill.yaml`, `prompt.md`), external MCP server configurations
- Produces: `SkillsManager` and `MCPHost` integrated into agent execution

- [ ] **Step 1: Write failing tests for skills and MCP host**
  Test skill manifest parsing, trigger matching, and tool registry.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/internal/skills/... ./backend/internal/mcp/...`

- [ ] **Step 3: Implement skills manager and MCP host**
  Scan `skills/` directory, load metadata, bind MCP tools to agent prompts.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/internal/skills/... ./backend/internal/mcp/...`

- [ ] **Step 5: Commit**
  `git add backend/internal/skills/ backend/internal/mcp/ && git commit -m "feat(skills,mcp): implement skills manager and mcp host"`

---

### Task 11: Real-time WebSocket Server & API Streaming

**Files:**
- Create: `backend/api/ws_server.go`
- Create: `backend/api/router.go`
- Test: `backend/api/ws_server_test.go`

**Interfaces:**
- Consumes: Orchestrator event channel
- Produces: WebSocket server broadcasting events (`agent_thinking`, `agent_output`, `round_complete`, `consensus_reached`, `code_update`)

- [ ] **Step 1: Write failing WebSocket server test**
  Connect a WebSocket test client and verify stream of swarm events.

- [ ] **Step 2: Run test to verify it fails**
  Run: `go test ./backend/api/...`

- [ ] **Step 3: Implement WebSocket server and broadcast hub**
  Implement client connection management, subscription to orchestrator events, and JSON serialization.

- [ ] **Step 4: Run test to verify it passes**
  Run: `go test -v ./backend/api/...`

- [ ] **Step 5: Commit**
  `git add backend/api/ && git commit -m "feat(api): implement websocket streaming server for ide"`

---

### Task 12: Backend Daemon Assembly & Configuration

**Files:**
- Create: `backend/internal/config/config.go`
- Create: `backend/cmd/nexusd/main.go`
- Test: `backend/cmd/nexusd/main_test.go`

**Interfaces:**
- Consumes: `config.yaml`, environment variables
- Produces: Executable binary `nexusd` running all subsystems

- [ ] **Step 1: Implement configuration loader**
  Load providers, agent model assignments, execution mode, database paths.

- [ ] **Step 2: Implement daemon main.go**
  Initialize SQLite memory, LLM client, agents, orchestrator, and WebSocket server on `:8080`.

- [ ] **Step 3: Test daemon startup and healthcheck**
  Add HTTP health endpoint and verify graceful shutdown with SIGINT/SIGTERM.

- [ ] **Step 4: Commit**
  `git add backend/ && git commit -m "feat(daemon): assemble nexusd daemon with config and graceful shutdown"`

---

### Task 13: Desktop Frontend Scaffolding & Setup

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/styles/globals.css`

**Interfaces:**
- Consumes: Vite, React, Tailwind CSS, Lucide icons, Monaco Editor
- Produces: Running desktop web/Electron UI

- [ ] **Step 1: Initialize frontend project**
  Install React 18/19, TypeScript, Tailwind CSS, Monaco Editor, Lucide React icons.

- [ ] **Step 2: Setup dark IDE layout framework**
  Create header bar, main tab navigation (`IDE`, `Agent Arena`, `Arbiter`, `Skills`, `Memory`), and status bar.

- [ ] **Step 3: Verify frontend dev server runs**
  Run build command to verify bundle succeeds.

- [ ] **Step 4: Commit**
  `git add frontend/ && git commit -m "feat(frontend): scaffold desktop ide application layout"`

---

### Task 14: Integrated IDE View (Monaco Editor + File Explorer + Chat)

**Files:**
- Create: `frontend/src/components/ide/FileExplorer.tsx`
- Create: `frontend/src/components/ide/CodeEditor.tsx`
- Create: `frontend/src/components/ide/TerminalView.tsx`
- Create: `frontend/src/components/ide/ChatPanel.tsx`
- Create: `frontend/src/components/ide/IdeView.tsx`

**Interfaces:**
- Consumes: Virtual file system state, WebSocket client
- Produces: 3-pane IDE layout with Monaco Editor, inline agent badges, file tree, terminal, and live chat

- [ ] **Step 1: Implement FileExplorer component**
  Tree view supporting open, create, delete, and selecting active file.

- [ ] **Step 2: Implement CodeEditor with Monaco**
  Syntax highlighting, line numbers, agent review comments as inline decorations/popovers.

- [ ] **Step 3: Implement ChatPanel component**
  Accept Persian/English inputs, prompt suggestions, streaming message bubbles, round indicators.

- [ ] **Step 4: Implement TerminalView component**
  Simulated command execution and log output.

- [ ] **Step 5: Assemble IdeView and verify rendering**
  `git add frontend/src/components/ide/ && git commit -m "feat(ui): implement integrated ide view with monaco and chat"`

---

### Task 15: Agent Arena & Debate Timeline View

**Files:**
- Create: `frontend/src/components/arena/AgentCard.tsx`
- Create: `frontend/src/components/arena/DebateTimeline.tsx`
- Create: `frontend/src/components/arena/RoundSwitcher.tsx`
- Create: `frontend/src/components/arena/AgentArenaView.tsx`

**Interfaces:**
- Consumes: Swarm event stream, round history
- Produces: Visual graph and debate stream of 5 agents arguing, refactoring, and justifying choices

- [ ] **Step 1: Implement AgentCard component**
  Shows persona avatar, model info, active thinking spinner, current thesis/objection.

- [ ] **Step 2: Implement DebateTimeline component**
  Message thread tagged by agent with code diff snippets and objection tags.

- [ ] **Step 3: Implement RoundSwitcher component**
  Interactive buttons (Round 1, Round 2, Round 3, Consensus) displaying state changes across iterations.

- [ ] **Step 4: Assemble AgentArenaView**
  `git add frontend/src/components/arena/ && git commit -m "feat(ui): implement agent arena and debate timeline"`

---

### Task 16: Arbiter Scoreboard, Diff Viewer & Audit Matrix

**Files:**
- Create: `frontend/src/components/arbiter/ScoreCard.tsx`
- Create: `frontend/src/components/arbiter/DiffViewer.tsx`
- Create: `frontend/src/components/arbiter/Leaderboard.tsx`
- Create: `frontend/src/components/arbiter/ArbiterView.tsx`

**Interfaces:**
- Consumes: Arbiter scores, before/after file snapshots
- Produces: Split side-by-side diff viewer, accuracy metrics (0-100%), consensus threshold indicator

- [ ] **Step 1: Implement DiffViewer component**
  Side-by-side before vs. after comparison with highlighted additions and deletions.

- [ ] **Step 2: Implement ScoreCard & Leaderboard**
  Radar or bar visualizer for 5 scoring metrics, agent bug catch rate, false positive penalties.

- [ ] **Step 3: Assemble ArbiterView**
  `git add frontend/src/components/arbiter/ && git commit -m "feat(ui): implement arbiter scoreboard and diff viewer"`

---

### Task 17: Skills Manager & Mem0 Memory UI

**Files:**
- Create: `frontend/src/components/skills/SkillsView.tsx`
- Create: `frontend/src/components/memory/MemoryView.tsx`

**Interfaces:**
- Consumes: Memory and Skills API
- Produces: Interactive settings and management panels

- [ ] **Step 1: Implement SkillsView component**
  List installed skills, active MCP servers, add/remove buttons, toggle status.

- [ ] **Step 2: Implement MemoryView component**
  Filter memories by project or global, tag search, view confidence, delete/edit memory records.

- [ ] **Step 3: Commit**
  `git add frontend/src/components/skills/ frontend/src/components/memory/ && git commit -m "feat(ui): implement skills and memory management views"`

---

### Task 18: End-to-End Swarm Integration & Polish

**Files:**
- Create: `frontend/src/services/apiClient.ts`
- Create: `frontend/src/services/websocket.ts`
- Modify: `frontend/src/App.tsx`

**Interfaces:**
- Consumes: Go backend on `:8080`
- Produces: Live connected full-stack application

- [ ] **Step 1: Implement WebSocket client service with auto-reconnect**
  Connect frontend to Go backend WebSocket.

- [ ] **Step 2: Connect IDE actions to Swarm orchestrator**
  Send prompt from chat -> Go orchestrator triggers 5 agents -> stream updates to Arena and Editor.

- [ ] **Step 3: Verify full round-robin adversarial flow**
  Run simulated or real round through all 5 tabs and verify smooth UI transitions.

- [ ] **Step 4: Final commit**
  `git add . && git commit -m "feat: complete end-to-end integration of NexusAI multi-agent IDE"`
