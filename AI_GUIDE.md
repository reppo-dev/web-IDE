# 🤖 AI Agent Collaboration Guide for NexusAI

> **CRITICAL NOTICE FOR AI CODING ASSISTANTS (Claude, GPT, Gemini, Cursor, Antigravity, etc.):**
> Read this document thoroughly before proposing, modifying, or refactoring code in this repository.

---

## 🎯 1. The Creator's Intent & Quality Standard

The creator of this project built NexusAI out of deep frustration with today's single-agent AI tools.
When working on this repository, **you must adhere to the following non-negotiable principles**:

1. **NO LAZINESS OR PLACEHOLDERS:** Never write `// TODO: implement later`, truncated functions, or superficial mockups. The creator explicitly demands a team that *misses nothing* and delivers *production-ready, complete software*.
2. **NO FAKE / TOY DEMOS:** Do not hardcode mockup data where real architecture should exist. When features are added, wire them into the real Go backend, real WebSocket channels, real SQLite tables, and real UI states.
3. **ADVERSARIAL COGNITION IS SACRED:** The core thesis is that a single AI has blind spots. NexusAI works by putting models in an adversarial arena: **Architect plans -> Coder implements -> Critic hunts bugs -> Security audits -> Arbiter judges**. Do not simplify this into a single prompt call.
4. **RESPECT BILINGUAL NATURE:** The user interacts in Persian (فارسی). Architect and system prompts must understand Persian user intent, decompose requirements accurately in Persian, while producing clean, idiomatic English code and API contracts.

---

## 🏗️ 2. Architectural Map & Ground Truth

### Backend (`backend/` — Go 1.22+ `go1.26.0`)
- **`cmd/nexusd/main.go`:** Main daemon process. Initializes SQLite memory, LLM client, 5 personas, Matrix engine, Sandbox, Orchestrator, and starts HTTP & WebSocket on `:8080`.
- **`internal/domain/types.go`:** Authoritative types: `AgentRole`, `ExecutionMode`, `AgentMessage`, `MemoryEntry`, `CodeFile`, `ScoringDimension`, `ArbiterVerdict`.
- **`internal/agent/`:**
  - `agent.go`: Base interface & `SwarmContext` (shares memory, files, critic reviews).
  - `architect.go`: Analyzes user prompts (Persian/English), produces blueprints and file trees.
  - `coder.go`: Generates complete multi-file code; fixes critic and security issues in rounds $\ge 2$.
  - `critic.go`: Red Team. Flags logic flaws, race conditions, edge cases with exact file/line suggestions.
  - `security.go`: Scans against OWASP/CWE for injection, secret leaks, broken auth.
  - `arbiter.go`: Uses `scoring.Scorer` to evaluate rounds, tracks consensus, decides merge approval.
  - `matrix.go`: **All-to-All Peer Cross-Examination Matrix**. Every agent reviews the other 4 ($5 \times 4 = 20$ pairwise evaluations concurrently via goroutines).
- **`internal/orchestrator/`:**
  - `rolebased.go`: Phased mode (Architect $\rightarrow$ Coder $\rightarrow$ Critic + Security in parallel $\rightarrow$ Matrix $\rightarrow$ Arbiter). Syncs files to disk, runs compiler sandbox, auto-commits to Git.
  - `concurrent.go`: Fan-out mode using typed channels (`chan *domain.AgentMessage`).
  - `sequential.go`: Linear round-robin execution.
- **`internal/memory/`:**
  - `store.go`: `MemoryStore` interface.
  - `sqlite.go`: Pure Go SQLite (`modernc.org/sqlite`, zero CGO). Manages per-project `data/projects/{id}/memories.db` and cross-project `data/global.db`.
- **`internal/sandbox/runner.go`:**
  - Executes real compiler/syntax commands on disk (`go vet ./...`, `python -m compileall`). If errors occur, extracts actual stack traces and hands them to the Critic as critical bugs.
- **`internal/workspace/workspace.go`:**
  - Manages `data/workspaces/{id}/` on disk, calculates line-by-line unified diffs, and creates Git commits for each swarm round.
- **`internal/scoring/`:**
  - `scorer.go`: 5-dimension weighted score (Correctness 30%, Bug Resolution 25%, Security 20%, Quality 15%, Spec Adherence 10%). Consensus threshold = 85.0%.
  - `convergence.go`: Detects score stagnancy across rounds ($< 1.5\%$ delta) and invokes a tie-breaker rule to prevent infinite debate loops.
- **`internal/mcp/`:**
  - `protocol.go`: Standard JSON-RPC 2.0 over Stdio transport for external MCP processes.
  - `host.go`: Server and tool registry.
- **`internal/llm/openai_compat.go`:**
  - Provider-agnostic client supporting OpenAI, OpenRouter, Google Gemini, Ollama, and custom endpoints. Formats JSON Schema structured outputs.
- **`api/ws_server.go` & `api/router.go`:**
  - Real-time WebSocket hub broadcasting `SwarmEvent`s (`round_started`, `agent_output`, `code_updated`, `consensus_reached`, `error`) to frontend clients.

### Frontend (`frontend/` — React 18, Vite, TypeScript, Tailwind CSS, Monaco Editor)
The UI is strictly styled after **Cursor IDE** (dark `#141414` palette):
- **`src/components/layout/TitleBar.tsx`:** Window controls, project name, central Command Palette pill (`Ctrl + P`), layout toggles, Settings gear.
- **`src/components/layout/ActivityBar.tsx`:** 48px left rail with icons (Explorer, Search, Git, Skills, Memory, Arbiter, Settings ⚙️).
- **`src/components/layout/Sidebar.tsx`:** 240px collapsible primary sidebar with clean empty state ("No Folder Opened"), real file tree, memory browser, and skills manager.
- **`src/components/editor/WelcomeScreen.tsx`:** Cursor-style home screen when no files are open (project templates, quick swarm starters, API key link).
- **`src/components/editor/EditorTabs.tsx`:** File tabs with active states, close buttons, and Diff toggle.
- **`src/components/ide/CodeEditor.tsx`:** Monaco Editor integration with inline agent review drawers.
- **`src/components/ide/TerminalView.tsx`:** Collapsible bottom terminal panel.
- **`src/components/composer/ComposerPanel.tsx`:** Right side Cursor Composer with model selector (GPT-4o, Claude 3.7, Gemini 2.5, DeepSeek R1, Ollama), swarm mode dropdown, live agent steps, consensus banner, and chat input.
- **`src/components/modals/SettingsModal.tsx`:** Full modal for configuring API keys (OpenRouter, OpenAI, Gemini, Anthropic, Ollama URL, custom BaseURL), swarm thresholds, and per-agent model assignments.
- **`src/components/modals/NewProjectModal.tsx`:** Project creation with starter templates.
- **`src/services/websocket.ts`:** Client with auto-reconnect and typed event dispatching.

---

## 🚫 3. Invariants: What You Must NEVER Break

1. **Pure Go SQLite Only:** Never import `github.com/mattn/go-sqlite3` because it requires CGO (GCC on Windows), which breaks zero-dependency compilation. Always use `modernc.org/sqlite`.
2. **Preserve 100% Test Pass Rate:** Before completing any work, you MUST run:
   ```powershell
   cd g:\v\IDE\backend
   go test -v ./...
   ```
   Every test must pass without failures.
3. **Keep Frontend Build Clean:** Always run:
   ```powershell
   cd g:\v\IDE\frontend
   npm run build
   ```
   Must compile cleanly with zero TypeScript errors.
4. **Do Not Revert to Generic Dashboard UI:** The user explicitly rejected generic dashboards with top horizontal tabs. Maintain the **Cursor IDE layout** (ActivityBar on the left, Explorer in sidebar, Monaco in center, Composer on the right, Status bar on bottom).
5. **No Hardcoded Project Files by Default:** If no project is selected, show the `WelcomeScreen`. Do not hardcode sample files into the open files array.

---

## 🧭 4. Next Development Milestones (For Future AI Contributors)

If the user asks you to continue developing, prioritize these high-value tasks:

### Milestone 1: Electron Standalone Desktop Packaging
- Package the Vite frontend into an Electron shell so it launches as a native `.exe` window without needing a browser.
- Automatically start `nexusd.exe` as a child process when the Electron desktop app launches, and shut it down cleanly on app exit.

### Milestone 2: Token-Level Streaming Over WebSocket
- Currently, agents send completed JSON blocks. Add Server-Sent Events (SSE) or streaming WebSocket chunks so the Composer shows tokens generating in real time.

### Milestone 3: Per-Agent Model Routing in Backend
- Allow `nexusd` to read the model assignments from Settings and dispatch the Critic to a reasoning model (e.g. `deepseek/deepseek-r1`) and the Lead Coder to a coding specialist (e.g. `claude-3-7-sonnet` or `qwen/qwen-2.5-coder-32b`).

### Milestone 4: In-Editor Inline Diff (Cmd+K / Ctrl+K)
- Implement Cursor's green/red inline diff decoration directly inside the Monaco Editor lines, allowing the user to press `Ctrl+Enter` to Accept or `Escape` to Reject agent refactors on specific line blocks.

---

*This guide was generated to preserve complete context and engineering integrity across AI sessions. Respect the user's vision and code with extreme craftsmanship.*
