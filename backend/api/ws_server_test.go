package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nexusai/nexusai/backend/internal/agent"
	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/llm"
	"github.com/nexusai/nexusai/backend/internal/orchestrator"
)

type dummyLLM struct{}

func (d *dummyLLM) Generate(ctx context.Context, req llm.Request) (*llm.Response, error) {
	return &llm.Response{
		Content: `{"files":[{"path":"main.go","language":"go","content":"package main"}],"file_tree":["main.go"]}`,
	}, nil
}

func TestWSServer_Connection(t *testing.T) {
	dummy := &dummyLLM{}
	arch := agent.NewArchitectAgent(dummy)
	coder := agent.NewCoderAgent(dummy)
	critic := agent.NewCriticAgent(dummy)
	sec := agent.NewSecurityAgent(dummy)
	arb := agent.NewArbiterAgent(dummy)
	matrix := agent.NewMatrixEngine(dummy)
	orch := orchestrator.NewOrchestrator(
		orchestrator.Config{MaxRounds: 2, Mode: domain.ModeSequential},
		arch, coder, critic, sec, arb, nil, matrix, t.TempDir(),
	)

	wsHub := NewWSServer(orch, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go wsHub.Start(ctx)

	router := NewRouter(wsHub, nil, nil)
	server := httptest.NewServer(router)
	defer server.Close()

	// 1. Test /health
	resp, err := http.Get(server.URL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("healthcheck failed: status %v, err %v", resp.StatusCode, err)
	}

	// 2. Test WebSocket connection
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer wsConn.Close()

	// Send ping
	if err := wsConn.WriteMessage(websocket.TextMessage, []byte(`{"action":"ping"}`)); err != nil {
		t.Fatalf("failed to send ping: %v", err)
	}

	// Read pong
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read pong: %v", err)
	}
	if !strings.Contains(string(msg), "pong") {
		t.Errorf("expected pong response, got: %s", string(msg))
	}
}
