package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/memory"
	"github.com/nexusai/nexusai/backend/internal/orchestrator"
	"github.com/nexusai/nexusai/backend/internal/skills"
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
	clients    map[*websocket.Conn]bool
	clientsMu  sync.RWMutex
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan []byte
}

// NewWSServer creates a new WebSocket hub.
func NewWSServer(orch *orchestrator.Orchestrator, mem memory.MemoryStore, sm *skills.Manager) *WSServer {
	return &WSServer{
		orch:       orch,
		memStore:   mem,
		skillsMgr:  sm,
		clients:    make(map[*websocket.Conn]bool),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		broadcast:  make(chan []byte, 100),
	}
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
					s.broadcast <- bytes
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
		case "start_swarm":
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
				defer cancel()

				var payload struct {
					Mode string `json:"mode"`
				}
				if len(clientMsg.Payload) > 0 {
					_ = json.Unmarshal(clientMsg.Payload, &payload)
				}

				mode := domain.ExecutionMode(payload.Mode)
				_, err := s.orch.RunWithMode(ctx, clientMsg.ProjectID, clientMsg.UserPrompt, mode)
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
