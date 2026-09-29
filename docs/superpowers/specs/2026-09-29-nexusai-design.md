# NexusAI — Multi-Agent Adversarial Collaborative IDE

## Overview

NexusAI is an autonomous software development environment powered by an ensemble of 5 specialized AI agents working in an adversarial round-robin synthesis cycle. Built as a desktop application on Eclipse Theia (Electron), it combines a full IDE, integrated chat, and multi-agent orchestration into one unified app.

**Core Idea:** Instead of relying on a single AI, NexusAI pits generative and critical models against each other until consensus is reached (≥85% quality score).

## Decisions Log

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Desktop Framework | Eclipse Theia (Electron) | Modular IDE framework, VS Code extension API compatible, designed for custom IDEs |
| Backend Language | Go | Native concurrency (goroutines/channels) for multi-agent orchestration |
| LLM Strategy | Provider-agnostic (API-based) | User adds any API endpoint (OpenAI-compatible). Like OpenRouter |
| Memory System | Mem0-style, SQLite-based | Per-project + global memory. Local, no extra install needed |
| Communication | gRPC (orchestration) + WebSocket (UI streaming) | Best of both: type-safe internals + real-time frontend |
| Storage | SQLite embedded | No external DB dependency. Safe, portable |
| Language | English-first, Persian supported | UI in English, chat handles both languages |
| Build Strategy | Full feature set from start | All 5 agents + all views + skills + MCP |

---

## 1. System Architecture

```
+-----------------------------------------------------------+
|               NexusAI Desktop App                         |
|             (Eclipse Theia + Electron)                    |
|                                                           |
|  +----------+ +--------------+ +---------------+         |
|  | IDE View | | Agent Arena  | | Arbiter       |         |
|  | (Monaco) | | (Debate UI)  | | Scoreboard    |         |
|  | FileTree | | Chat Stream  | | Diff Viewer   |         |
|  | Terminal | | Round Switch | | Leaderboard   |         |
|  +----+-----+ +------+-------+ +------+--------+         |
|       +---------------+-+--------------+                  |
|                   WebSocket                               |
+------------------------+----------------------------------+
                         |
               +---------+---------+
               |   Go Backend      |
               |   (Orchestrator)  |
               |                   |
               |  +-------------+  |
               |  | Agent Swarm |  |  <-- gRPC to LLM APIs
               |  | Scheduler   |  |
               |  | Memory Mgr  |  |  <-- SQLite (local)
               |  | Skills Mgr  |  |
               |  | MCP Host    |  |
               |  +-------------+  |
               +-------------------+
```

Three layers:
1. **Theia Frontend** — IDE + Chat + Agent Arena + Scoreboard UI
2. **Go Orchestrator** — Brain: agent management, memory, skills, MCP hosting
3. **LLM APIs** — Any provider the user configures with an API key

---

## 2. Agent Swarm System

### 2.1 The Five Agents

| Agent | Role | Output |
|-------|------|--------|
| **Architect** | Analyzes requests, decomposes into file trees, architecture designs, API contracts | Structured blueprint JSON |
| **Lead Coder** | Writes complete source code, explains rationale and design decisions | Code files + explanations |
| **Critic (Red Team)** | Inspects for logic flaws, race conditions, edge-case failures, performance issues | Bug reports + refactor demands |
| **Security Auditor** | Evaluates for injection, unauthorized access, data exposure, sanitization gaps | Vulnerability reports |
| **Arbiter** | Tracks each agent's performance, scores errors, decides consensus | Score card + verdict |

Each agent has:
- A fixed system prompt (persona definition)
- A configurable LLM model (provider-agnostic)
- Read/write access to shared memory
- Structured JSON output via JSON Schema

### 2.2 Three Execution Modes

**Sequential:**
```
Architect -> Coder -> Critic -> Security -> Arbiter
(linear, each waits for previous)
```
Best for: simple projects, debugging

**Concurrent:**
```
All 5 start simultaneously
Each sees others' output in real-time via Go channels
```
Best for: speed, large projects

**Role-based (Recommended):**
```
Phase 1: Architect alone (plan)
Phase 2: Coder + Critic + Security in parallel (build + review)
Phase 3: Arbiter alone (judge)
```
Best for: balance of quality and speed

### 2.3 Adversarial Round-Robin Loop

```
Round 1: Draft
  Architect -> plan
  Coder -> code
  Critic -> bugs found: [...]
  Security -> vulns found: [...]
  Arbiter -> score: 62% (< 85%) FAIL
      |
      v feedback returned to Coder
Round 2: Refinement
  Coder -> fixed code
  Critic -> re-review: fewer bugs
  Security -> re-scan: 1 remaining
  Arbiter -> score: 78% FAIL
      |
      v
Round 3: Polish
  Coder -> final fixes
  Critic -> clean
  Security -> clean
  Arbiter -> score: 91% PASS CONSENSUS
      |
      v
  Final merged code -> IDE workspace
```

### 2.4 Go Concurrency Model

- Each agent runs as a **goroutine**
- Communication via **typed channels** (`chan AgentMessage`)
- Concurrent mode uses `sync.WaitGroup` + `select` for fan-out/fan-in
- Arbiter is a **supervisor goroutine** deciding if another round is needed
- Max rounds configurable (default: 5) to prevent infinite loops
- Context with timeout for each round

```go
type AgentMessage struct {
    From      AgentRole
    To        AgentRole // broadcast if empty
    Round     int
    Type      MessageType // code, review, score, feedback
    Content   json.RawMessage
    Timestamp time.Time
}
```

---

## 3. Memory System (Mem0-style)

### 3.1 Architecture

```
+-------------+   +-------------+   +-------------+
| Project A   |   | Project B   |   | Project C   |
| Memory      |   | Memory      |   | Memory      |
| (SQLite)    |   | (SQLite)    |   | (SQLite)    |
+------+------+   +------+------+   +------+------+
       +----------------+-+--------------+
                        v
               +-----------------+
               | Global Memory   |
               | (Cross-Project) |
               | (SQLite)        |
               +-----------------+
```

All 5 agents have read/write access to all memory layers.

### 3.2 Four Memory Layers

| Layer | What it stores | Example |
|-------|---------------|---------|
| **Project Memory** | Architecture decisions, bugs found, code patterns | "This project uses Repository Pattern" |
| **Agent Memory** | Per-agent experiences, mistakes, accuracy | "Critic gave false positive in round 2" |
| **User Preferences** | Coding style, language, preferred frameworks | "User prefers 4-space tabs, works mostly in Go" |
| **Global Memory** | Shared knowledge across all projects | "User always wants error handling first" |

### 3.3 SQLite Schema

```sql
-- Per-project: data/projects/{project-id}/memories.db
CREATE TABLE memories (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    agent       TEXT NOT NULL,
    type        TEXT NOT NULL,  -- decision/mistake/pattern/preference/insight
    content     TEXT NOT NULL,
    tags        TEXT,           -- JSON array
    round       INTEGER,
    confidence  REAL DEFAULT 1.0,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_agent ON memories(agent);

-- Global: data/global.db
CREATE TABLE global_memories (
    id             TEXT PRIMARY KEY,
    type           TEXT NOT NULL,
    content        TEXT NOT NULL,
    source_project TEXT,
    tags           TEXT,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 3.4 Memory Operations

```go
// Write
memory.Write(projectID, MemoryEntry{
    Type:    "decision",
    Content: "Switched from REST to gRPC for internal services",
    Agent:   "architect",
    Round:   2,
    Tags:    []string{"architecture", "grpc"},
})

// Read/Query
entries := memory.Query(projectID, Query{
    Type:  "mistake",
    Agent: "coder",
    Limit: 10,
})

// Cross-project query
global := memory.QueryGlobal(Query{Type: "preference"})
```

---

## 4. Skills System

### 4.1 Skill Structure

```
skills/
  docker-deploy/
    skill.yaml          <- name, description, dependencies
    prompt.md           <- additional system prompt for agents
    tools/
      deploy.sh         <- executable script (optional)
    examples/
      Dockerfile        <- reference examples
```

### 4.2 Skill Manifest

```yaml
# skill.yaml
name: docker-deploy
description: "Deploy projects using Docker containers"
version: "1.0.0"
agents: [coder, architect]
triggers:
  - keyword: "docker"
  - keyword: "deploy"
  - keyword: "container"
requires:
  - docker CLI installed
```

### 4.3 Built-in Skills

- `code-review` — Enhanced code review patterns
- `test-generator` — Auto-generate test suites
- `api-designer` — REST/gRPC API design patterns
- `docker-deploy` — Docker containerization

### 4.4 Skill Discovery

Agents automatically activate relevant skills based on:
1. Keyword triggers in user prompt
2. File types detected in project
3. Explicit user selection
4. Arbiter approval of skill relevance

---

## 5. MCP Server Integration

### 5.1 Configuration

```yaml
# settings.yaml
mcp_servers:
  - name: github
    command: "npx @github/mcp-server"
    env:
      GITHUB_TOKEN: "${GITHUB_TOKEN}"

  - name: custom-db
    command: "python my_mcp_server.py"
    args: ["--port", "3001"]
```

### 5.2 How Agents Use MCP Tools

- Go backend hosts MCP connections (stdio/SSE transport)
- Tools are registered in a shared tool registry
- Any agent can invoke MCP tools as function calls
- Example: Coder uses `github.create_file`, Security uses `snyk.scan`

---

## 6. User Interface

### 6.1 Five Main Tabs

| Tab | Content |
|-----|---------|
| **IDE** | File tree + Monaco Editor + Terminal + Chat panel (right side). Inline agent annotations on code |
| **Agent Arena** | Agent debate timeline — who said what, why they disagreed, code snippets. Round switcher |
| **Arbiter** | Split diff viewer (before/after), Agent performance leaderboard, line-by-line justification table |
| **Skills** | Skills and MCP server management — install, remove, configure |
| **Memory** | Memory browser — filter by project/agent/type, manual edit and delete |

### 6.2 IDE View Layout

```
+----------+------------------------------+-------------------+
| File     | Code Editor (Monaco)         | Chat / Agent      |
| Explorer |                              | Panel             |
|          | Line annotations from agents |                   |
| src/     | (hover = popover with full   | User messages     |
|   main   |  agent reasoning)            | Agent responses   |
|   srv    |                              | Round status      |
|   db     |------------------------------| Score indicator   |
| test/    | Terminal                     |                   |
|          | $ go run main.go             | [Round 2 >>]      |
+----------+------------------------------+-------------------+
| Status: Round 2/3 | Mode: Role-based | Agents: 5/5 active |
+------------------------------------------------------------+
```

### 6.3 Key UI Features

- **Inline Agent Annotations**: Agent icons + comments on code lines, hover for full popover
- **Chat Panel**: Always visible, accepts Persian/English, shows agent outputs
- **Round Switcher**: `Round 1 -> 2 -> 3` buttons to see code evolution
- **Live Status Bar**: Execution mode, active agents count, consensus %, round progress
- **Dark Theme**: Default dark IDE theme

---

## 7. Communication Protocol

### 7.1 gRPC Services (backend internal)

```protobuf
service OrchestratorService {
  rpc StartSwarm(SwarmRequest) returns (stream SwarmEvent);
  rpc StepThrough(StepRequest) returns (StepResponse);
  rpc PauseSwarm(PauseRequest) returns (PauseResponse);
  rpc GetStatus(StatusRequest) returns (SwarmStatus);
}

service MemoryService {
  rpc WriteMemory(WriteRequest) returns (WriteResponse);
  rpc QueryMemory(QueryRequest) returns (QueryResponse);
  rpc QueryGlobal(QueryRequest) returns (QueryResponse);
}

service SkillsService {
  rpc ListSkills(Empty) returns (SkillList);
  rpc InstallSkill(InstallRequest) returns (InstallResponse);
  rpc RemoveSkill(RemoveRequest) returns (RemoveResponse);
}
```

### 7.2 WebSocket Events (frontend streaming)

```typescript
// Frontend receives these events via WebSocket
type WSEvent =
  | { type: "agent_thinking"; agent: AgentRole; round: number }
  | { type: "agent_output"; agent: AgentRole; round: number; content: AgentOutput }
  | { type: "round_complete"; round: number; score: number; passed: boolean }
  | { type: "consensus_reached"; finalRound: number; finalScore: number }
  | { type: "code_update"; files: FileChange[] }
  | { type: "memory_write"; entry: MemoryEntry }
  | { type: "error"; message: string };
```

---

## 8. Additional Features

| Feature | Description |
|---------|-------------|
| **Project Templates** | Quick start templates (Go API, React App, Python ML, etc.). Agents begin from chosen template |
| **Agent Config Panel** | Customize each agent: model, temperature, system prompt, severity level |
| **Export/Import** | Export project + memories as zip. Transfer between machines |
| **Conversation History** | Full searchable history of all chat conversations |
| **Hot Reload Agent** | Change agent persona or model mid-run without restart |
| **Rate Limiter** | Control API calls per minute. Prevent excessive costs |
| **Cost Tracker** | Show estimated cost per round based on token usage |
| **Git Integration** | Auto-commit each round. Agent-driven change history |

---

## 9. Project Structure

```
nexus-ai/
├── proto/                          <- gRPC/protobuf definitions
│   ├── agent.proto
│   ├── memory.proto
│   ├── orchestrator.proto
│   └── skills.proto
│
├── backend/                        <- Go backend
│   ├── cmd/
│   │   └── nexusd/                 <- main daemon entrypoint
│   │       └── main.go
│   ├── internal/
│   │   ├── orchestrator/           <- Execution loop (3 modes)
│   │   │   ├── orchestrator.go
│   │   │   ├── sequential.go
│   │   │   ├── concurrent.go
│   │   │   └── rolebased.go
│   │   ├── agent/                  <- Agent definitions and LLM calls
│   │   │   ├── agent.go
│   │   │   ├── architect.go
│   │   │   ├── coder.go
│   │   │   ├── critic.go
│   │   │   ├── security.go
│   │   │   └── arbiter.go
│   │   ├── llm/                    <- Provider-agnostic LLM client
│   │   │   ├── client.go
│   │   │   ├── provider.go
│   │   │   ├── openai_compat.go
│   │   │   └── schema.go
│   │   ├── memory/                 <- Mem0-style memory system
│   │   │   ├── store.go
│   │   │   ├── sqlite.go
│   │   │   └── query.go
│   │   ├── skills/                 <- Skills manager
│   │   │   ├── manager.go
│   │   │   ├── loader.go
│   │   │   └── registry.go
│   │   ├── mcp/                    <- MCP host
│   │   │   ├── host.go
│   │   │   └── transport.go
│   │   ├── workspace/              <- Virtual filesystem
│   │   │   ├── workspace.go
│   │   │   └── diff.go
│   │   └── scoring/                <- Arbiter scoring engine
│   │       ├── scorer.go
│   │       └── consensus.go
│   ├── api/
│   │   ├── grpc_server.go
│   │   └── ws_server.go
│   └── go.mod
│
├── frontend/                       <- Theia-based desktop app
│   ├── app/                        <- Electron app entry
│   │   ├── package.json
│   │   └── electron-main.ts
│   ├── extensions/                 <- Custom Theia extensions
│   │   ├── nexus-chat/
│   │   ├── nexus-arena/
│   │   ├── nexus-arbiter/
│   │   ├── nexus-skills/
│   │   ├── nexus-memory/
│   │   └── nexus-annotations/
│   ├── packages/
│   │   ├── nexus-common/
│   │   └── nexus-theme/
│   └── package.json
│
├── skills/                         <- Built-in skills
│   ├── code-review/
│   ├── test-generator/
│   ├── api-designer/
│   └── docker-deploy/
│
├── data/
│   ├── global.db
│   └── projects/
│       └── {project-id}/
│           └── memories.db
│
├── turbo.json
├── README.md
└── Makefile
```

---

## 10. LLM Provider Configuration

Provider-agnostic design. User configures API endpoints:

```yaml
# config.yaml
providers:
  - name: "openai"
    base_url: "https://api.openai.com/v1"
    api_key: "${OPENAI_API_KEY}"
    models:
      - id: "gpt-4o"
        max_tokens: 128000

  - name: "google"
    base_url: "https://generativelanguage.googleapis.com/v1beta"
    api_key: "${GEMINI_API_KEY}"
    models:
      - id: "gemini-2.5-flash"
        max_tokens: 1000000

  - name: "openrouter"
    base_url: "https://openrouter.ai/api/v1"
    api_key: "${OPENROUTER_API_KEY}"
    models:
      - id: "anthropic/claude-sonnet-4"
      - id: "google/gemini-2.5-pro"

  - name: "local-ollama"
    base_url: "http://localhost:11434/v1"
    api_key: ""
    models:
      - id: "llama3"

# Agent-to-model assignment
agents:
  architect:
    provider: "google"
    model: "gemini-2.5-flash"
    temperature: 0.3
  coder:
    provider: "openrouter"
    model: "anthropic/claude-sonnet-4"
    temperature: 0.4
  critic:
    provider: "openai"
    model: "gpt-4o"
    temperature: 0.2
  security:
    provider: "google"
    model: "gemini-2.5-pro"
    temperature: 0.1
  arbiter:
    provider: "openai"
    model: "gpt-4o"
    temperature: 0.1
```

---

## 11. Scoring & Consensus (Arbiter)

### 11.1 Scoring Dimensions

| Metric | Weight | Description |
|--------|--------|-------------|
| Code Correctness | 30% | Does the code compile and run? Logic errors? |
| Bug Resolution | 25% | Were Critic's bugs actually fixed? |
| Security Compliance | 20% | Were Security's vulnerabilities addressed? |
| Code Quality | 15% | Readability, patterns, maintainability |
| Spec Adherence | 10% | Does code match Architect's blueprint? |

### 11.2 Agent Performance Tracking

```sql
CREATE TABLE agent_scores (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL,
    agent       TEXT NOT NULL,
    round       INTEGER NOT NULL,
    metric      TEXT NOT NULL,
    score       REAL NOT NULL,
    details     TEXT,  -- JSON explanation
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 11.3 Consensus Rules

- Score >= 85% → **APPROVED** — code goes to workspace
- Score 70-84% → **REFINE** — feedback to Coder, new round
- Score < 70% → **REJECT** — feedback to Architect + Coder, major rework
- Max rounds reached → **FORCE MERGE** with warnings displayed to user

---

## 12. Security Considerations

- API keys stored encrypted in OS keychain (via `keytar` in Electron)
- SQLite databases are local-only, never uploaded
- MCP servers run in sandboxed subprocesses
- No telemetry or data collection
- All LLM communication over HTTPS
- User can review all memory entries and delete any

---

## 13. Non-Goals (v1)

- Cloud/SaaS deployment (desktop-only for now)
- Multi-user collaboration (single user)
- Custom agent creation by user (fixed 5 agents, customizable config only)
- Plugin marketplace server (local skills only)
- Mobile support
