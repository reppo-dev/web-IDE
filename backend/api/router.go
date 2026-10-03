package api

import (
	"encoding/json"
	"net/http"

	"github.com/nexusai/nexusai/backend/internal/memory"
	"github.com/nexusai/nexusai/backend/internal/skills"
	"github.com/nexusai/nexusai/backend/internal/team"
	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// Router sets up HTTP endpoints for the IDE backend.
func NewRouter(ws *WSServer, mem memory.MemoryStore, sm *skills.Manager, teamStore team.Store, projectReg *workspace.Registry) http.Handler {
	mux := http.NewServeMux()

	teamHandler := NewTeamHandler(teamStore)
	projectHandler := NewProjectHandler(projectReg)

	// Custom teams endpoints
	mux.HandleFunc("/api/teams", teamHandler.HandleTeams)
	mux.HandleFunc("/api/teams/", teamHandler.HandleTeams)
	mux.HandleFunc("/api/teams/default", teamHandler.HandleDefaultTeam)

	// Opening real folders from disk
	mux.HandleFunc("/api/projects", projectHandler.HandleProjects)
	mux.HandleFunc("/api/projects/", projectHandler.HandleProjects)

	// WebSocket streaming endpoint
	mux.HandleFunc("/ws", ws.HandleWS)

	// Healthcheck
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "healthy",
			"version": "1.0.0",
			"app":     "NexusAI Daemon",
		})
	})

	// Skills list endpoint
	mux.HandleFunc("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if sm == nil {
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		json.NewEncoder(w).Encode(sm.ListSkills())
	})

	// Memories endpoint
	mux.HandleFunc("/api/memories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		projectID := r.URL.Query().Get("project_id")
		if mem == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{"project": []interface{}{}, "global": []interface{}{}})
			return
		}
		pmems, _ := mem.QueryProject(r.Context(), projectID, memory.QueryFilter{Limit: 50})
		gmems, _ := mem.QueryGlobal(r.Context(), memory.QueryFilter{Limit: 50})
		json.NewEncoder(w).Encode(map[string]interface{}{
			"project": pmems,
			"global":  gmems,
		})
	})

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
