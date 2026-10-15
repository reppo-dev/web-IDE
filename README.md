# 🌌 NexusAI — Autonomous Multi-Agent Software Engineering IDE & Daemon

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?style=flat&logo=react)](https://reactjs.org)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0-3178C6?style=flat&logo=typescript)](https://www.typescriptlang.org)
[![Monaco Editor](https://img.shields.io/badge/Monaco_Editor-VS_Code_Core-blue)](https://microsoft.github.io/monaco-editor/)
[![Tests](https://img.shields.io/badge/Tests-100%25_Passing-brightgreen)](https://github.com)
[![License](https://img.shields.io/badge/License-MIT-purple.svg)](LICENSE)

> **"A deterministic, self-critiquing multi-agent engineering engine that compiles, tests, and verifies code on disk — rejecting AI hallucinations through compiler feedback and strict gatekeepers."**

---

## 📖 Executive Summary

**NexusAI** is an experimental next-generation IDE and autonomous software development engine. Unlike conventional "wrapper" tools that merely stream ChatGPT responses into an editor, NexusAI orchestrates a **swarm of specialized AI agents** (Architect, Lead Coder, Adversarial Critic, Security Auditor, Arbiter) working over a shared local workspace.

It replaces the superficial "chat-bubble" paradigm with a **Plan-Driven State Machine**, **Atomic File-Lease Locks**, **Multi-Worker Parallelism**, and a **Ground Truth Verification Engine** that runs native compilers, linters, and unit test suites on your machine before code is ever accepted.

> 🤝 **Meta-Engineering Experiment:** This entire codebase was co-engineered simultaneously by **two independent autonomous AI agents** (Claude & Antigravity) running on the same local directory, coordinating in real-time through atomic file locks and communication boards (`coordination/`).

---

## 🏛️ System Architecture

```
                                  [ User Request / Feature Goal ]
                                                 │
                                                 ▼
                             ┌───────────────────────────────────────┐
                             │    Intent Classifier & Context Engine │
                             │   (Triage: Chat vs Question vs Build) │
                             └───────────────────┬───────────────────┘
                                                 │
                                                 ▼
                             ┌───────────────────────────────────────┐
                             │       Plan-Driven Task Engine         │
                             │  (Auto-Generates 10-35 Native Tasks)  │
                             │   .project/plan.json & tasks.md       │
                             └───────────────────┬───────────────────┘
                                                 │
                                                 ▼
                      ┌─────────────────────────────────────────────────────┐
                      │    True Task Parallelism & File Lease Coordinator   │
                      │   (Dispatches isolated context forks to N workers)  │
                      └──────────────┬───────────────────┬──────────────────┘
                                     │                   │
                  ┌──────────────────┴──┐             ┌──┴──────────────────┐
                  ▼                     ▼             ▼                     ▼
          ┌───────────────┐     ┌───────────────┐ ┌───────────────┐ ┌───────────────┐
          │ Coder Worker 1│     │ Coder Worker 2│ │ Critic Review │ │ Security Guard│
          │ (e.g. Qwen)   │     │ (e.g. Claude) │ │ (Adversarial) │ │ (OWASP/CWE)   │
          └───────┬───────┘     └───────┬───────┘ └───────┬───────┘ └───────┬───────┘
                  │                     │                 │                 │
                  └─────────────────────┼─────────────────┴─────────────────┘
                                        ▼
                             ┌───────────────────────────────────────┐
                             │   Ground Truth Verification Engine    │
                             │   (Real 'go test', 'flutter test',    │
                             │    'dart analyze', syntax sandboxes)  │
                             └───────────────────┬───────────────────┘
                                                 │
                                                 ▼
                             ┌───────────────────────────────────────┐
                             │           Task State Gate             │
                             │   Pass: DONE -> Git Checkpoint Ref    │
                             │   Fail: REWORK (Max 3) -> Human Alert │
                             └───────────────────┬───────────────────┘
                                                 │
                                                 ▼
                             ┌───────────────────────────────────────┐
                             │   Change Ledger (.project/ledger)     │
                             │   + UI Live Stream (Token Chunk WS)   │
                             └───────────────────────────────────────┘
```

---

## ⚡ Core Engineering Features

### 1. Ground Truth Verification Engine
Language models hallucinate syntax and import paths. NexusAI completely eliminates ungrounded code by executing real compiler tooling (`go test`, `flutter test`, `dart analyze`, `python compileall`, `tsc`) in isolated sub-processes with 30-second execution timeouts. If a test fails, the raw compiler stack trace is injected directly into the next rework cycle.

### 2. Plan-Driven State Machine & Task Decomposition
Instead of writing an entire application in one giant prompt, the Architect decomposes features into a structured backlog stored on disk (`.project/plan.json` and human-readable `.project/tasks.md`):
$$\text{Task States: } \text{TODO} \longrightarrow \text{IN\_PROGRESS} \longrightarrow \text{DONE} \mid \text{REWORK (Max 3)} \mid \text{WAITING\_FOR\_HUMAN}$$

### 3. True Task Parallelism & File Lease Locks
Multiple coder agents can work concurrently on independent tasks. A central **File Lease Coordinator** prevents race conditions:
- Agents obtain timed leases on specific files (`auth.dart`, `models.dart`).
- Shared files cannot be modified simultaneously.
- When an agent completes or fails, locks are atomically released.

### 4. Git Checkpoint Isolation (`refs/nexusai/checkpoints`)
NexusAI never clobbers or pollutes the user's `main` branch with intermediate experimental commits. Every round creates a tree object committed to a hidden reference (`refs/nexusai/checkpoints`). Only when the user clicks **"Accept Changes"** (or calls `POST /api/projects/:id/accept`) are verified changes cleanly merged into the active working tree.

### 5. Multi-Model Heterogeneous Swarms
NexusAI avoids vendor lock-in. Workers can be distributed across different model providers simultaneously:
- **Planner:** Claude 3.7 Sonnet / DeepSeek R1
- **Parallel Workers:** Qwen 2.5 Coder, Nemotron, Llama 3.3, OpenAI GPT-4o
- **Local Fallback:** Local Ollama / vLLM / NineRouter models

### 6. Real-Time Token Streaming over WebSocket (`token_chunk`)
Supports SSE streaming directly through an internal concurrency-limiting guard (`llm.Guard`). As tokens are generated by the upstream LLM, they stream live over WebSockets directly into the active Task Card in the Cursor-style UI.

---

## 🔍 The Hard Truth: Shortcomings & What's Missing

An honest post-mortem is worth more than a dozen marketing claims. While NexusAI solved difficult concurrency and verification problems, building an autonomous AI IDE revealed significant engineering boundaries and limitations:

### ⚠️ 1. Token Economics & Cost Explosion
- **The Issue:** A multi-agent consensus run with 5 agents, cross-reviews, and 3 rounds of verification on a 30-task project can easily make 150+ LLM API calls and consume **300,000 to 1,500,000+ tokens**.
- **Real-world Impact:** Running this on expensive proprietary models (like GPT-4o or Claude 3.7) costs several dollars per feature. Without aggressive token budgeting and cheap local models (Qwen/DeepSeek), it is economically unviable for continuous hobbyist use.

### ⚠️ 2. Semantic vs Syntactic Conflict Resolution
- **The Issue:** While file-lease locking prevents two agents from editing the same file at the same time (syntactic collision), it **cannot prevent semantic regressions across files**.
- **Example:** Coder 1 modifies a function signature in `user_service.dart`, while concurrent Coder 2 calls the old signature in `profile_screen.dart`. Both write syntactically valid code in their own files, but the project breaks when compiled together.
- **Current State:** Caught by the Verification Gate after merge, triggering a Rework cycle; but this burns extra rounds.

### ⚠️ 3. Host Process Execution vs Full Containerization
- **The Issue:** The test runner executes commands directly on the host operating system (`exec.Command` on Windows/Linux).
- **Security Limitation:** Unlike production sandbox environments (Docker, Firecracker MicroVMs, gVisor), there is no kernel-level filesystem or network isolation. If an untrusted package or hallucinated malicious shell command is introduced, it executes with the user's permissions.

### ⚠️ 4. Standalone IDE Shell vs VS Code Extension Reality
- **The Problem:** Building an entire desktop IDE from scratch (Monaco editor tabs, file tree, terminal emulator, git status widget) requires hundreds of hours of frontend maintenance.
- **The Market Reality:** Developers do not want to abandon their custom VS Code / Cursor extensions, themes, and keybindings for a new standalone web-based IDE. The core moat of NexusAI is the **daemon and orchestration engine (`nexusd`)**, which would be far more effective packaged as a **VS Code Extension** or **CLI Agent (similar to Claude Code or Aider)**.

### ⚠️ 5. The "Chatbot Swarm" Illusion in Commodity Tools
- Tools like Grok, Hermes, and generic multi-agent wrappers let users create "teams" of agents. However, these are almost entirely **chat-based simulations**: they talk to each other in plain text without local compiler feedback, disk-level file locking, or git integrity.
- **NexusAI's Lesson:** Building real software autonomously cannot be achieved through chatting alone. It requires deterministic systems engineering, hard timeouts, state machines, and human escalation gates.

---

## 🛠️ Technology Stack

| Layer | Technology | Rationale |
| :--- | :--- | :--- |
| **Backend Core** | Go 1.22+ (`nexusd`) | Low memory footprint, sub-millisecond goroutines, typed concurrency (`sync.RWMutex`), zero-CGO SQLite. |
| **State & Memory** | SQLite (`modernc.org/sqlite`) | Embedded, pure Go, zero external database setup. Stores Project Brain, Facts, Assumptions, and Decisions. |
| **Frontend Shell** | React 18, TypeScript, Vite | Sub-3-second production builds, strict typing, responsive state management. |
| **Code Editor** | Monaco Editor (`@monaco-editor/react`) | Industry-standard VS Code engine with diff viewers and syntax highlighting. |
| **Communication** | WebSocket (`ws://:8080/ws`) | Full-duplex JSON event streaming for live task cards, token chunks, and file locks. |
| **Verification** | Native CLI Subprocesses | Real execution of `flutter`, `dart`, `go`, `node`, `python` test suites. |

---

## 🚦 Verification & Test Suite

The backend contains exhaustive unit, integration, and race-detector tests across all 12 Go packages:

```powershell
# Run full test suite across all 12 backend packages:
cd backend
go test -count=1 ./internal/... ./api/...

# Verify zero race conditions:
go test -race ./internal/agent/... ./internal/orchestrator/...

# Static analysis:
go vet ./...
```

**Result:** 100% Pass Rate across all packages:
- `internal/agent`: Context engine, isolation forks, token streaming, memory grounding.
- `internal/orchestrator`: Plan loop, parallel batching, stall detection, lease coordination.
- `internal/llm`: Guard retries, backoff, session budgets, SSE streaming chunks.
- `internal/workspace`: Disk syncing, git checkpointing, path validation, change ledger.

---

## 📦 How to Run Locally

### 1. Prerequisites
- **Go 1.22+**
- **Node.js 18+ & npm**
- **Git**

### 2. Configure Environment
Create a `.env` file in the project root:
```env
OPENROUTER_API_KEY=sk-or-v1-...
# Or custom / local endpoint:
OPENAI_BASE_URL=http://localhost:11434/v1
OPENAI_API_KEY=ollama
```

### 3. Launch Backend Daemon
```powershell
cd backend
go run ./cmd/nexusd
# Listens on http://localhost:8080
```

### 4. Launch Frontend IDE
```powershell
cd frontend
npm install
npm run dev
# Opens at http://localhost:3000
```

---

## 💡 Lessons Learned for the AI Engineering Community

1. **Don't Let Agents Chat Without Tools:** Freeform multi-agent debates quickly spiral into sycophancy or infinite repetition unless bound to concrete deliverables (tasks, files, tests).
2. **The Compiler Is the Ultimate Arbiter:** Human review and LLM self-critique are fallible; a failing compiler test is undeniable ground truth.
3. **Decouple Brain from UI:** If you build an autonomous engineering tool, build it as a daemon or CLI first. The IDE UI is secondary to the reliability of the core execution loop.

---

## 📄 License
MIT License. Created as an advanced research experiment in Multi-Agent Autonomous Software Engineering.
