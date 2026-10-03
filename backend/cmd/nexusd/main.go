package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nexusai/nexusai/backend/api"
	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/config"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/memory"
	"github.com/nexusai/nexusai/backend/internal/orchestrator"
	"github.com/nexusai/nexusai/backend/internal/skills"
	"github.com/nexusai/nexusai/backend/internal/team"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

func main() {
	cfg := config.LoadConfig()
	log.Printf("Starting NexusAI Daemon on :%s (Mode: %s)...", cfg.Port, cfg.Mode)

	// 1. Initialize SQLite Memory Store
	memStore, err := memory.NewSQLiteMemoryStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to init SQLite memory: %v", err)
	}
	defer memStore.Close()
	log.Printf("Persistent SQLite memory initialized at %s", cfg.DataDir)

	// 2. Initialize Skills Manager
	skillsMgr := skills.NewManager(cfg.SkillsDir)
	if err := skillsMgr.LoadAll(); err != nil {
		log.Printf("warning loading skills: %v", err)
	} else {
		log.Printf("Loaded %d skills from %s", len(skillsMgr.ListSkills()), cfg.SkillsDir)
	}

	// 3. Initialize LLM Client, wrapped in the shared resilience Guard:
	// concurrency limit toward the provider, retry with backoff on transient
	// errors (429/5xx/network), and session token budget (PROJECT_VISION §13, §26).
	llmClient := llm.NewGuard(llm.NewOpenAICompatClient(cfg.LLM), llm.GuardOptions{
		MaxConcurrent: cfg.LLMMaxConcurrent,
		MaxAttempts:   cfg.LLMMaxAttempts,
		Budget:        llm.NewBudget(cfg.MaxTotalTokens),
	})

	// 4. Initialize 5 Specialized Agents
	arch := agent.NewArchitectAgent(llmClient)
	coder := agent.NewCoderAgent(llmClient)
	critic := agent.NewCriticAgent(llmClient)
	sec := agent.NewSecurityAgent(llmClient)
	arb := agent.NewArbiterAgent(llmClient)
	matrix := agent.NewMatrixEngine(llmClient)

	// 5. Initialize Swarm Orchestrator
	orch := orchestrator.NewOrchestrator(
		orchestrator.Config{
			MaxRounds: cfg.MaxRounds,
			Mode:      cfg.Mode,
		},
		arch, coder, critic, sec, arb, memStore, matrix, cfg.DataDir,
	)
	orch.SetLLMClient(llmClient)

	// 6. Initialize Custom Team Store
	teamStore, err := team.NewSQLiteStore(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to init team store: %v", err)
	}
	defer teamStore.Close()
	_ = teamStore.SeedDevelopersTeam()
	log.Printf("Custom Team Store initialized at %s", cfg.DataDir)

	// 7. Load the registry of folders the user has opened, so a project id
	// always maps back to the real directory on disk.
	projectReg, err := workspace.NewRegistry(cfg.DataDir)
	if err != nil {
		log.Fatalf("failed to init project registry: %v", err)
	}
	workspace.SetRegistry(projectReg)
	log.Printf("Project registry loaded (%d folders)", len(projectReg.List()))

	// 8. Initialize WebSocket Streaming Hub
	wsHub := api.NewWSServer(orch, memStore, skillsMgr, teamStore)
	hubCtx, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go wsHub.Start(hubCtx)

	// 9. Initialize HTTP Server
	router := api.NewRouter(wsHub, memStore, skillsMgr, teamStore, projectReg)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: router,
	}

	go func() {
		log.Printf("NexusAI Daemon listening on http://localhost:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down NexusAI Daemon...")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("NexusAI Daemon stopped cleanly.")
}
