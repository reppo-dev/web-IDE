package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/memory"
	"github.com/nexusai/nexusai/backend/internal/orchestrator"
	"github.com/nexusai/nexusai/backend/internal/skills"
	"github.com/nexusai/nexusai/backend/internal/team"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all local IDE origins
	},
}

// ClientMessage represents an action sent from the frontend IDE to the backend.
type ClientMessage struct {
	Action     string          `json:"action"` // "start_swarm", "get_status", "query_memories", "ping"
	ProjectID  string          `json:"project_id"`
	UserPrompt string          `json:"user_prompt,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// WSServer manages active WebSocket connections and relays swarm events.
type WSServer struct {
	orch       *orchestrator.Orchestrator
	memStore   memory.MemoryStore
	skillsMgr  *skills.Manager
	teamStore  team.Store
	clients    map[*websocket.Conn]bool
	clientsMu  sync.RWMutex
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan []byte
}

// NewWSServer creates a new WebSocket hub.
func NewWSServer(orch *orchestrator.Orchestrator, mem memory.MemoryStore, sm *skills.Manager, ts team.Store) *WSServer {
	return &WSServer{
		orch:       orch,
		memStore:   mem,
		skillsMgr:  sm,
		teamStore:  ts,
		clients:    make(map[*websocket.Conn]bool),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan []byte, 100),
	}
}

// reply sends a JSON response to one client, redacting any secrets (PROJECT_VISION §25).
func reply(conn *websocket.Conn, payload interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	redacted := llm.RedactSecrets(string(b))
	_ = conn.WriteMessage(websocket.TextMessage, []byte(redacted))
}

// resolveTeam finds the team a project should use: the explicit request wins,
// then the project binding, then the global default.
func (s *WSServer) resolveTeam(ctx context.Context, projectID, requestedID string) (*domain.Team, error) {
	if s.teamStore == nil {
		return nil, errors.New("team store is not available")
	}

	if requestedID != "" {
		return s.teamStore.Get(ctx, requestedID)
	}

	// Fall back to the default so a project that was never configured still runs.
	return s.teamStore.GetDefault(ctx)
}

// Start begins the event loop forwarding events to connected clients.
func (s *WSServer) Start(ctx context.Context) {
	// Subscribe to orchestrator events
	eventCh := s.orch.Subscribe()

	go func() {
		for {
			select {
			case <-ctx.Done():
				s.orch.Unsubscribe(eventCh)
				return
			case evt, ok := <-eventCh:
				if !ok {
					return
				}
				bytes, err := json.Marshal(evt)
				if err == nil {
					redacted := llm.RedactSecrets(string(bytes))
					s.broadcast <- []byte(redacted)
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case client := <-s.register:
			s.clientsMu.Lock()
			s.clients[client] = true
			s.clientsMu.Unlock()
		case client := <-s.unregister:
			s.clientsMu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				client.Close()
			}
			s.clientsMu.Unlock()
		case msg := <-s.broadcast:
			s.clientsMu.RLock()
			for client := range s.clients {
				err := client.WriteMessage(websocket.TextMessage, msg)
				if err != nil {
					client.Close()
					delete(s.clients, client)
				}
			}
			s.clientsMu.RUnlock()
		}
	}
}

// HandleWS upgrades HTTP connection to WebSocket and handles incoming client messages.
func (s *WSServer) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	s.register <- conn
	defer func() {
		s.unregister <- conn
	}()

	// Push available teams to newly connected client immediately
	if s.teamStore != nil {
		if teams, err := s.teamStore.List(context.Background()); err == nil {
			reply(conn, map[string]interface{}{
				"type":            "teams_data",
				"teams":           teams,
				"default_team_id": "default",
			})
		}
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(msgBytes, &clientMsg); err != nil {
			continue
		}

		switch clientMsg.Action {
		case "list_teams":
			if s.teamStore == nil {
				reply(conn, map[string]interface{}{"type": "teams_data", "teams": []interface{}{}})
				break
			}
			teams, err := s.teamStore.List(context.Background())
			if err != nil {
				reply(conn, map[string]interface{}{"type": "error", "message": err.Error()})
				break
			}
			def, _ := s.teamStore.GetDefault(context.Background())
			defID := ""
			if def != nil {
				defID = def.ID
			}
			reply(conn, map[string]interface{}{
				"type":           "teams_data",
				"teams":          teams,
				"default_team_id": defID,
			})

		case "save_team":
			var t domain.Team
			if err := json.Unmarshal(clientMsg.Payload, &t); err != nil {
				reply(conn, map[string]interface{}{"type": "error", "message": "invalid team payload: " + err.Error()})
				break
			}
			if s.teamStore == nil {
				reply(conn, map[string]interface{}{"type": "error", "message": "team store unavailable"})
				break
			}
			if err := s.teamStore.Save(context.Background(), &t); err != nil {
				reply(conn, map[string]interface{}{"type": "error", "message": err.Error()})
				break
			}
			reply(conn, map[string]interface{}{"type": "team_saved", "team": t})

		case "delete_team":
			var payload struct {
				TeamID string `json:"team_id"`
			}
			_ = json.Unmarshal(clientMsg.Payload, &payload)
			if s.teamStore == nil || payload.TeamID == "" {
				reply(conn, map[string]interface{}{"type": "error", "message": "team id required"})
				break
			}
			if err := s.teamStore.Delete(context.Background(), payload.TeamID); err != nil {
				reply(conn, map[string]interface{}{"type": "error", "message": err.Error()})
				break
			}
			reply(conn, map[string]interface{}{"type": "team_deleted", "team_id": payload.TeamID})

		case "start_swarm":
			log.Printf("WS RECEIVED start_swarm: project=%s prompt=%s", clientMsg.ProjectID, clientMsg.UserPrompt)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
				defer cancel()

				var payload struct {
					Mode    string            `json:"mode"`
					Models  map[string]string `json:"models"`
					APIKeys map[string]string `json:"apiKeys"`
					TeamID  string            `json:"team_id"`
				}
				if len(clientMsg.Payload) > 0 {
					_ = json.Unmarshal(clientMsg.Payload, &payload)
				}

				mode := domain.ExecutionMode(payload.Mode)

				// Tell the client which pipeline is about to run. Without this
				// the UI looks identical whether the selected team ran or the
				// legacy five-agent roster did, which made a silently ignored
				// team look like a broken model.
				runPath := "swarm"
				if payload.TeamID != "" {
					runPath = "team:" + payload.TeamID
				}
				log.Printf("RUN START project=%s path=%s team_id=%q models_sent=%v",
					clientMsg.ProjectID, runPath, payload.TeamID, payload.Models != nil)
				b, _ := json.Marshal(orchestrator.SwarmEvent{
					Type:      orchestrator.EventAgentThinking,
					ProjectID: clientMsg.ProjectID,
					Agent:     "manager",
					Payload:   runPath,
				})
				_ = conn.WriteMessage(websocket.TextMessage, b)

				// A user-built team takes over the whole pipeline: its agents,
				// its model per agent, and its consensus threshold.
				if payload.TeamID != "" || (s.teamStore != nil && payload.Models == nil) {
					selected, err := s.resolveTeam(ctx, clientMsg.ProjectID, payload.TeamID)
					if err != nil {
						log.Printf("team resolve failed: %v", err)
						b, _ := json.Marshal(orchestrator.SwarmEvent{
							Type:      orchestrator.EventError,
							ProjectID: clientMsg.ProjectID,
							Payload:   "team not found: " + err.Error(),
						})
						conn.WriteMessage(websocket.TextMessage, b)
						return
					}
					if _, err := s.orch.RunTeam(ctx, clientMsg.ProjectID, clientMsg.UserPrompt,
						orchestrator.TeamRun{
							Team:    selected,
							Mode:    mode,
							APIKeys: payload.APIKeys,
						}); err != nil {
						log.Printf("TEAM RUN FAILED for %s: %v", clientMsg.ProjectID, err)
						b, _ := json.Marshal(orchestrator.SwarmEvent{
							Type:      orchestrator.EventError,
							ProjectID: clientMsg.ProjectID,
							Payload:   err.Error(),
						})
						conn.WriteMessage(websocket.TextMessage, b)
					} else {
						log.Printf("TEAM RUN FINISHED for %s with %d agents", clientMsg.ProjectID, len(selected.Agents))
					}
					return
				}

				models := make(map[domain.AgentRole]string)
				for k, v := range payload.Models {
					models[domain.AgentRole(k)] = v
				}

				_, err := s.orch.RunWithOptions(ctx, clientMsg.ProjectID, clientMsg.UserPrompt, mode, models, payload.APIKeys)
				log.Printf("SWARM (legacy five-agent roster) FINISHED for %s, err=%v", clientMsg.ProjectID, err)
				if err != nil {
					errEvt := orchestrator.SwarmEvent{
						Type:      orchestrator.EventError,
						ProjectID: clientMsg.ProjectID,
						Payload:   err.Error(),
					}
					b, _ := json.Marshal(errEvt)
					conn.WriteMessage(websocket.TextMessage, b)
				}
			}()

		case "query_memories":
			if s.memStore != nil {
				mems, _ := s.memStore.QueryProject(context.Background(), clientMsg.ProjectID, memory.QueryFilter{Limit: 20})
				gmems, _ := s.memStore.QueryGlobal(context.Background(), memory.QueryFilter{Limit: 20})
				resp := map[string]interface{}{
					"type":     "memories_data",
					"project":  mems,
					"global":   gmems,
				}
				b, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, b)
			}

		case "list_skills":
			if s.skillsMgr != nil {
				skillsList := s.skillsMgr.ListSkills()
				resp := map[string]interface{}{
					"type":   "skills_data",
					"skills": skillsList,
				}
				b, _ := json.Marshal(resp)
				conn.WriteMessage(websocket.TextMessage, b)
			}

		case "ping":
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pong"}`))
		}
	}
}
