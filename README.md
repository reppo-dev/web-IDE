# 🌌 NexusAI — Multi-Agent Adversarial Collaborative IDE

> **"A relentless, self-critiquing AI engineering team that misses nothing and delivers production-grade software instead of superficial placeholders."**

NexusAI is an autonomous software development environment powered by a coordinated ensemble of **5 specialized AI agents** operating in an **adversarial round-robin synthesis cycle**. Built with a high-concurrency **Go backend** (`nexusd`) and a dark, minimalist **Cursor-style desktop IDE** (React + Monaco Editor + TypeScript).

---

## 💡 The Core Vision & Philosophy (چشم‌انداز و فلسفه پروژه)

### 🚨 The Problem with Single-Agent AI Tools
Today's AI coding assistants (ChatGPT, Copilot, vanilla Cursor) behave like an **unsupervised junior developer**:
1. **Laziness & Shortcuts:** They generate snippets with `// TODO: implement later`, mock functions, and half-finished logic.
2. **Blind Spots:** An AI model cannot see its own hallucinations or flawed assumptions.
3. **Ignored Edge Cases:** Concurrency race conditions, memory leaks, missing error checks, and security vulnerabilities are consistently missed.
4. **Manual User Burden:** The human developer is forced to spend hours reviewing, debugging, and re-prompting the AI.

### 🛡️ The NexusAI Solution
NexusAI replaces the single AI with a **full autonomous software engineering department**:
- Rather than trusting one model, generative and critical models are pitted against each other.
- **Every agent cross-examines the outputs of the other agents.**
- Code is **actually compiled and verified in a real sandbox** (no hallucinated syntax validation).
- An **Arbiter Judge** enforces a strict **85% consensus quality threshold** across 5 weighted dimensions before code is ever merged into the workspace.
- A **Mem0-style persistent SQLite memory** retains architectural decisions, past bugs, and user preferences across projects.

---

## 👥 The 5-Agent Swarm

```
                         [User Prompt: Persian or English]
                                        │
                                        ▼
                  ┌───────────────────────────────────────────┐
                  │        1. System Architect 🏗️             │
                  │  Decomposes requirements into blueprints, │
                  │  file trees & API contracts (zero missed) │
                  └─────────────────────┬─────────────────────┘
                                        ▼
                  ┌───────────────────────────────────────────┐
                  │        2. Lead Coder (Engineer) 💻        │
                  │  Writes complete, robust code for each   │
                  │  file with zero placeholders or TODOs     │
                  └─────────────────────┬─────────────────────┘
                                        ▼
             ┌──────────────────────────┴──────────────────────────┐
             ▼                                                     ▼
┌─────────────────────────┐                               ┌─────────────────────────┐
│ 3. Adversarial Critic 🔴│                               │  4. Security Guard 🛡️   │
│  The Red Team: hunts    │◄─────────────────────────────►│  Audits against OWASP,  │
│  race conditions, logic │   (Concurrent Cross-Review)   │  injection, secret leaks│
│  flaws & edge cases     │                               │  and sanitization gaps  │
└────────────┬────────────┘                               └────────────┬────────────┘
             └──────────────────────────┬──────────────────────────────┘
                                        ▼
                  ┌───────────────────────────────────────────┐
                  │ 5. Arbiter & Consensus Judge ⚖️           │
                  │  Calculates 5-dimension score (0-100%).   │
                  │  Score >= 85% ?                           │
                  └──────┬─────────────────────────────┬──────┘
                         │ NO (< 85%)                  │ YES (>= 85%)
                         ▼                             ▼
              [Refinement Round]           [Consensus Approved]
            Feedback returned to Coder      Synced to Disk & Git Auto-Commit
```

| Agent | Persona & Purpose | Key Responsibilities |
| :--- | :--- | :--- |
| **Architect** | The Strategist & Decomposer | Analyzes requests (in Persian or English). Extracts all edge cases and designs clean file trees and API contracts. |
| **Lead Coder** | The Senior Implementer | Writes clean, complete production source code. Explains rationale and fixes bugs flagged in subsequent rounds. |
| **Adversarial Critic** | The Red Team Code Reviewer | Relentlessly scrutinizes code for logic errors, deadlocks, unhandled exceptions, and performance bottlenecks. Demands concrete code corrections. |
| **Security Auditor** | The Shield & Threat Modeler | Scans for OWASP Top 10, CWE vulnerabilities, SQL/command injection, authentication bypass, and sensitive data leaks. |
| **The Arbiter** | Consensus Judge & Scorekeeper | Tracks penalties, computes 5-dimension weighted scores, breaks stalemates, and issues the final consensus verdict. |

---

## ⚡ Concurrency & Architecture (Go Backend)

The backend (`backend/`) is written in **Go 1.22+** to take advantage of native goroutines, typed channels, and sub-millisecond coordination:

### 1. Three Swarm Execution Modes
- **Role-Based (Phased - Recommended):** Architect plans -> Coder drafts -> Critic & Security inspect in parallel -> Arbiter judges.
- **Concurrent (Channel-Driven):** All agents run concurrently as goroutines, exchanging messages via typed channels (`chan domain.AgentMessage`).
- **Sequential (Linear):** Classical turn-based round-robin loop.

### 2. All-to-All Peer Cross-Examination Matrix
In every round, every agent reviews the other 4 agents' work (**$5 \times 4 = 20$ pairwise evaluations**), producing an agreement/disagreement matrix and convergence metrics.

### 3. Real Compiler Sandbox & Ground Truth
`internal/sandbox/runner.go` runs real syntax and compiler checks (`go vet`, `python -m compileall`, etc.) on disk. If the compiler fails, real terminal stack traces are fed directly to the Critic as critical bugs.

### 4. Virtual Workspace & Git Auto-Versioning
`internal/workspace/workspace.go` manages physical project files on disk, computes line-by-line unified diffs, and automatically creates a **Git Commit** for each swarm round:
```
[NexusAI Swarm] Round 1 | Score: 91.5% | Summary of consensus
```

### 5. Mem0-Style Persistent Memory (Pure SQLite)
`internal/memory/sqlite.go` uses embedded pure Go SQLite (`modernc.org/sqlite` — zero CGO dependencies):
- **Project Memory (`data/projects/{id}/memories.db`):** Architecture decisions, bugs caught, patterns, and insights scoped to the active project.
- **Global Memory (`data/global.db`):** User coding preferences, favorite conventions, and cross-project knowledge.

### 6. Model Context Protocol (MCP) Integration
`internal/mcp/protocol.go` implements standard **JSON-RPC 2.0 over Stdio**, allowing the swarm to execute tools from external MCP servers (e.g. GitHub, PostgreSQL, Docker, Snyk).

---

## 🖥️ Cursor-Style Desktop IDE Frontend (`frontend/`)

The frontend is modeled directly after **Cursor IDE** with a clean, dark `#141414` palette:

1. **Title Bar:** Window controls, quick project switcher, and central Command Palette search (`Ctrl + P`).
2. **Activity Bar (48px left):**
   - 📁 **Explorer:** File tree with clean "No folder opened" empty state.
   - 🔍 **Search:** Full-text project search.
   - 🌿 **Source Control:** Swarm round-by-round commit history.
   - 📦 **Skills & MCP:** Installed developer skills and connected MCP servers.
   - 🧠 **Memory (Mem0):** Search and filter persistent memories by tags and types.
   - ⚖️ **Arbiter Audit:** Quality score breakdown and consensus status.
   - ⚙️ **Settings:** Opens the Cursor-style API configuration modal.
3. **Editor Area:**
   - **Welcome Screen:** When no files are open (clean branding, quick starters, shortcuts).
   - **Editor Tabs:** Tab navigation with dirty indicators and **Diff Mode Toggle**.
   - **Monaco Code Editor:** VS Code dark theme, syntax highlighting, and inline agent review popovers.
   - **Collapsible Terminal Panel:** Interactive command execution and live build output.
4. **Cursor Composer (Right Panel):**
   - Model Selector: GPT-4o, Claude 3.7 Sonnet, Gemini 2.5 Flash, DeepSeek R1, Ollama Local.
   - Swarm Mode Selector (`Role-Based`, `Concurrent`, `Sequential`).
   - Live stream of agent thoughts, critique cards, consensus progress pill, and **Accept Changes** button.
5. **Settings Modal ⚙️:**
   - Enter API Keys for **OpenRouter**, **OpenAI**, **Google Gemini**, **Anthropic**, or custom endpoints.
   - Configure local **Ollama** URL (`http://localhost:11434/v1`).
   - Customize Consensus Threshold (default 85%) and Max Rounds.
   - Assign different AI models to individual agent personas.

---

## 📁 Repository Structure

```
nexus-ai/
├── backend/                        # Go Concurrent Daemon & Swarm Core
│   ├── cmd/nexusd/main.go          # Daemon entrypoint (starts :8080 & WS)
│   ├── api/                        # WebSocket hub and HTTP router
│   │   ├── router.go               # HTTP routes (/health, /api/skills, /api/memories)
│   │   └── ws_server.go            # Real-time WebSocket event streaming
│   ├── internal/
│   │   ├── agent/                  # The 5 Swarm Personas & Cross-Examination
│   │   │   ├── agent.go            # Base agent interface and SwarmContext
│   │   │   ├── architect.go        # Architect persona (blueprints, Persian/English)
│   │   │   ├── coder.go            # Lead Coder persona (multi-file generator)
│   │   │   ├── critic.go           # Adversarial Critic persona (bug hunter)
│   │   │   ├── security.go         # Security Auditor persona (OWASP/CWE scanner)
│   │   │   ├── arbiter.go          # Arbiter persona (judge and scorer)
│   │   │   └── matrix.go           # N-to-N cross-examination debate matrix (20 pairs)
│   │   ├── domain/types.go         # Core data types, messages, verdicts
│   │   ├── llm/                    # Provider-agnostic client (OpenAI, OpenRouter, Gemini)
│   │   │   ├── client.go           # Client interface and request models
│   │   │   └── openai_compat.go    # HTTP client with JSON Schema formatting
│   │   ├── mcp/                    # Model Context Protocol host & JSON-RPC 2.0 stdio
│   │   │   ├── host.go             # Registry of servers and tools
│   │   │   └── protocol.go         # Stdio transport and JSON-RPC 2.0 client
│   │   ├── memory/                 # Mem0-style SQLite persistent memory
│   │   │   ├── store.go            # MemoryStore interface
│   │   │   └── sqlite.go           # Pure Go SQLite driver (projects & global DB)
│   │   ├── orchestrator/           # Swarm execution engines (3 modes)
│   │   │   ├── orchestrator.go     # Event hub and coordinator
│   │   │   ├── rolebased.go        # Phased execution with sandbox and git commit
│   │   │   ├── concurrent.go       # Goroutine & channel fan-out
│   │   │   └── sequential.go       # Linear loop
│   │   ├── sandbox/runner.go       # Real compiler & syntax execution
│   │   ├── scoring/                # 5-dimension scoring and stalemate breaker
│   │   │   ├── scorer.go           # Weighted dimension calculation
│   │   │   └── convergence.go      # Stalemate detector & tie-breaker
│   │   ├── skills/manager.go       # Skills loading and trigger matching
│   │   └── workspace/workspace.go  # Disk filesystem sync, unified diffs & git
│   └── go.mod
│
├── frontend/                       # Cursor-Style Desktop IDE (React + Monaco + Vite)
│   ├── src/
│   │   ├── components/
│   │   │   ├── layout/             # TitleBar, ActivityBar, Sidebar, StatusBar
│   │   │   ├── editor/             # EditorTabs, WelcomeScreen
│   │   │   ├── ide/                # CodeEditor (Monaco), TerminalView
│   │   │   ├── composer/           # ComposerPanel (Cursor AI Chat side-panel)
│   │   │   └── modals/             # SettingsModal, NewProjectModal
│   │   ├── services/websocket.ts   # Auto-reconnecting WebSocket client
│   │   ├── types.ts                # TypeScript interfaces matching backend
│   │   ├── App.tsx                 # Root workbench application
│   │   └── main.tsx                # Entrypoint
│   └── package.json
│
├── docs/superpowers/
│   ├── specs/2026-09-29-nexusai-design.md        # Full architectural specification
│   └── plans/2026-09-29-nexusai-implementation.md  # 18-task implementation plan
│
├── Makefile                        # Build automation (build, test, run)
├── AI_GUIDE.md                     # Dedicated instructions for future AI collaborators
└── README.md                       # This master documentation
```

---

## 🚀 Quick Start Guide

### 1. Prerequisites
- **Go 1.22+** (`go version`)
- **Node.js 18+** & **npm** (`node -v`, `npm -v`)
- **Git** (`git --version`)

### 2. Configure Your API Key
NexusAI works out-of-the-box with **OpenRouter** (which includes free models like `deepseek-r1:free`, `llama-3.3-70b:free`) or **Google Gemini** (via Google AI Studio free tier):

**Option A: Via Environment Variables**
```powershell
# OpenRouter (Recommended: access all models with one key)
$env:OPENROUTER_API_KEY = "sk-or-v1-..."

# Or Google Gemini / OpenAI
$env:OPENAI_API_KEY = "AIzaSy..."
$env:OPENAI_BASE_URL = "https://generativelanguage.googleapis.com/v1beta/openai"
```

**Option B: Via the IDE Settings Modal (⚙️)**
You can also launch the IDE and paste your key directly into the **Settings** dialog. It will be saved securely to local storage.

### 3. Build & Run the Backend
```powershell
cd g:\v\IDE\backend
go run ./cmd/nexusd
```
The daemon starts on `http://localhost:8080` with WebSocket streaming at `ws://localhost:8080/ws`.

### 4. Start the Cursor-Style Frontend
In a separate terminal:
```powershell
cd g:\v\IDE\frontend
npm run dev
```
Open **[http://localhost:3000](http://localhost:3000)** in your browser.

### 5. Running Automated Tests
All packages have 100% automated unit tests:
```powershell
cd g:\v\IDE\backend
go test -v ./...
```

---

## 🔮 Roadmap & Future Enhancements

1. **Native Desktop Executable (.exe):** Wrap the Vite frontend using Electron to produce a standalone Windows application window with no browser borders.
2. **Streaming Tokens via SSE:** Stream token-by-token reasoning inside the Composer chat for instantaneous feedback.
3. **Multi-Model Swarm Profiles:** In Settings, assign a reasoning model (e.g. DeepSeek R1) to the Critic and a coding model (e.g. Claude 3.7 Sonnet / Qwen 2.5 Coder) to the Coder.
4. **Deep VS Code Extension Mode:** Allow running the NexusAI Swarm directly inside standard VS Code via an extension host.

---

## 🤝 For AI Agents Collaborating on This Repo
If you are an AI assistant (Claude, GPT, Gemini, Cursor) reading this repository, **you must read [AI_GUIDE.md](file:///g:/v/IDE/AI_GUIDE.md)** before proposing or making changes. It details the project's strict design principles, testing protocols, and core philosophies.
