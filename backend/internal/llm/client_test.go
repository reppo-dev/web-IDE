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

func TestOpenAICompatClient_NineRouterHybridPayload(t *testing.T) {
	// NineRouter returns a full JSON body followed by "data: [DONE]"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		payload := `{"id":"gen-123","choices":[{"message":{"role":"assistant","content":"hello world","reasoning":"step 1"}}]}` + "\ndata: [DONE]\n\n"
		w.Write([]byte(payload))
	}))
	defer server.Close()

	client := NewOpenAICompatClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "nemotron",
	})

	resp, err := client.Generate(context.Background(), Request{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", resp.Content)
	}
	if resp.Reasoning != "step 1" {
		t.Errorf("expected 'step 1', got %q", resp.Reasoning)
	}
}

func TestOpenAICompatClient_SSEStream(t *testing.T) {
	// Standard SSE chunk stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		stream := "data: {\"choices\":[{\"delta\":{\"content\":\"chunk1 \",\"reasoning\":\"r1\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"chunk2\"}}]}\n\n" +
			"data: [DONE]\n\n"
		w.Write([]byte(stream))
	}))
	defer server.Close()

	client := NewOpenAICompatClient(Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "flash_5",
	})

	resp, err := client.Generate(context.Background(), Request{UserPrompt: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "chunk1 chunk2" {
		t.Errorf("expected 'chunk1 chunk2', got %q", resp.Content)
	}
	if resp.Reasoning != "r1" {
		t.Errorf("expected 'r1', got %q", resp.Reasoning)
	}
}

