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

	// 3. Initialize LLM Client
	llmClient := llm.NewOpenAICompatClient(cfg.LLM)

	// 4. Initialize 5 Specialized Agents
	arch := agent.NewArchitectAgent(llmClient)
	coder := agent.NewCoderAgent(llmClient)
	critic := agent.NewCriticAgent(llmClient)
	sec := agent.NewSecurityAgent(llmClient)
	arb := agent.NewArbiterAgent(llmClient)

	// 5. Initialize Swarm Orchestrator
	orch := orchestrator.NewOrchestrator(
		orchestrator.Config{
			MaxRounds: cfg.MaxRounds,
			Mode:      cfg.Mode,
		},
		arch, coder, critic, sec, arb, memStore,
	)

	// 6. Initialize WebSocket Streaming Hub
	wsHub := api.NewWSServer(orch, memStore, skillsMgr)
	hubCtx, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go wsHub.Start(hubCtx)

	// 7. Initialize HTTP Server
	router := api.NewRouter(wsHub, memStore, skillsMgr)
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
