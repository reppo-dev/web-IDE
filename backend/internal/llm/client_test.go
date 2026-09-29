package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatClient_Generate(t *testing.T) {
	expectedReply := `{"files": ["main.go"], "plan": "Initialize HTTP server"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-secret-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var req chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if len(req.Messages) != 2 {
			t.Fatalf("expected 2 messages (system + user), got %d", len(req.Messages))
		}

		resp := chatCompletionResponse{
			Choices: []chatCompletionChoice{
				{
					Message: chatMessage{
						Role:    "assistant",
						Content: expectedReply,
					},
				},
			},
			Usage: chatCompletionUsage{
				PromptTokens:     42,
				CompletionTokens: 18,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAICompatClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-secret-key",
		Model:   "gpt-4o",
	})

	resp, err := client.Generate(context.Background(), Request{
		SystemPrompt: "You are the Architect.",
		UserPrompt:   "Build an API in Go",
		JSONSchema:   map[string]interface{}{"type": "object"},
	})

	if err != nil {
		t.Fatalf("client generate failed: %v", err)
	}

	if resp.Content != expectedReply {
		t.Errorf("expected reply %s, got %s", expectedReply, resp.Content)
	}
	if resp.PromptTokens != 42 || resp.OutputTokens != 18 {
		t.Errorf("unexpected token count: %+v", resp)
	}
}
