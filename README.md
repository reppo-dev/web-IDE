# NexusAI — Multi-Agent Adversarial Collaborative IDE

NexusAI is an autonomous software development environment powered by an ensemble of 5 specialized AI agents working in an adversarial round-robin synthesis cycle.

Instead of relying on a single AI that can hallucinate or miss critical edge cases, NexusAI pits generative and critical models against each other:
1. **Architect (Strategist):** Analyzes user requests (in Persian or English) and decomposes them into clean file trees, architecture designs, and API contracts.
2. **Lead Coder (Code Generator):** Generates complete, production-ready code for every file with rationale and explanations.
3. **Adversarial Critic (The Red Team):** Inspects code for logic flaws, race conditions, edge-case failures, unhandled exceptions, and demands concrete refactors.
4. **Security Auditor (The Shield):** Audits code against OWASP/CWE for injection vulnerabilities, unauthorized access, sensitive data exposure, and sanitization gaps.
5. **The Arbiter (Consensus Judge & Scorekeeper):** Scores mistakes across 5 weighted dimensions and decides consensus. Approves code when quality crosses the 85% threshold, or triggers a refinement round.

---

## 3 Swarm Execution Modes (Go Concurrency)

The Go backend orchestrator (`nexusd`) supports 3 execution modes:
- **Role-Based (Phased):** Architect drafts -> Coder implements -> Critic and Security run in parallel -> Arbiter scores and decides.
- **Concurrent (Channel-driven):** All agents run concurrently as goroutines, communicating via typed Go channels.
- **Sequential (Linear):** Agents execute in linear order (Architect -> Coder -> Critic -> Security -> Arbiter).

---

## Mem0-Style Persistent Memory System (SQLite)

- **Project Memory (`data/projects/{project-id}/memories.db`):** Stores decisions, bugs found, patterns, and round history scoped to the active project.
- **Global Memory (`data/global.db`):** Stores developer preferences and cross-project knowledge shared across all sessions.
- **Pure Go SQLite:** Powered by `modernc.org/sqlite` with zero CGO dependencies.

---

## 5 IDE Workspace Views

1. **Full IDE View:** File tree explorer, Monaco Editor with syntax highlighting, inline agent review badges, terminal runner, and bilingual chat panel.
2. **Agent Arena:** Visual debate timeline, persona cards, live arguments, and step-by-step round switcher (Round 1 -> 2 -> 3 -> Consensus).
3. **Arbiter Matrix:** Side-by-side split code diff viewer (Before Critic vs. After Critic), agent accuracy leaderboard, and 5-dimension radar scores.
4. **Skills & MCP Ecosystem:** Installable skill-packs with keyword triggers and Model Context Protocol server connectors.
5. **Mem0 Memory Explorer:** Search and filter memories by project, tag, agent, or confidence.

---

## Quick Start

### 1. Requirements
- Go 1.22+
- Node.js 18+ and npm

### 2. Configure API Key (Optional / Flexible)
Set your preferred OpenAI-compatible provider (OpenAI, OpenRouter, Groq, Ollama, Gemini):
```bash
export OPENAI_API_KEY="your-api-key"
# or export OPENROUTER_API_KEY="your-api-key"
```

### 3. Run Backend Daemon
```bash
cd backend
go run ./cmd/nexusd
```
The daemon runs on `http://localhost:8080` and starts the WebSocket server on `ws://localhost:8080/ws`.

### 4. Run Desktop IDE Frontend
In a new terminal:
```bash
cd frontend
npm run dev
```
Open `http://localhost:3000` in your browser.
