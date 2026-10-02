package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAICompatClient implements Client for any OpenAI-compatible API endpoint.
type OpenAICompatClient struct {
	config     Config
	httpClient *http.Client
}

// NewOpenAICompatClient creates a new OpenAI-compatible client.
func NewOpenAICompatClient(cfg Config) *OpenAICompatClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.openai.com/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o"
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.2
	}
	return &OpenAICompatClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
		},
	}
}

type chatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Stream         *bool           `json:"stream,omitempty"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema interface{} `json:"json_schema,omitempty"`
}

type chatCompletionChoice struct {
	Message chatMessage `json:"message"`
}

type chatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
	Usage   chatCompletionUsage    `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Generate sends the prompt to the API endpoint and returns the generated content.
func (c *OpenAICompatClient) Generate(ctx context.Context, req Request) (*Response, error) {
	baseURL := c.config.BaseURL
	if req.BaseURL != "" {
		baseURL = req.BaseURL
	}
	url := fmt.Sprintf("%s/chat/completions", baseURL)

	messages := []chatMessage{}
	if req.SystemPrompt != "" {
		messages = append(messages, chatMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	messages = append(messages, chatMessage{
		Role:    "user",
		Content: req.UserPrompt,
	})

	temp := c.config.Temperature
	if req.Temperature > 0 {
		temp = req.Temperature
	}

	model := c.config.Model
	if req.Model != "" {
		model = req.Model
	}

	streamFalse := false
	payload := chatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temp,
		MaxTokens:   c.config.MaxTokens,
		Stream:      &streamFalse,
	}

	if req.JSONSchema != nil {
		payload.ResponseFormat = &responseFormat{
			Type: "json_object",
		}
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	apiKey := c.config.APIKey
	if req.APIKey != "" {
		apiKey = req.APIKey
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatCompletionResponse
	trimmedBody := bytes.TrimSpace(respBody)

	if bytes.HasPrefix(trimmedBody, []byte("data:")) || strings.Contains(string(trimmedBody), "\ndata:") {
		// Server-Sent Events stream: accumulate tokens from chunks
		var sb strings.Builder
		var reasonSb strings.Builder
		lines := strings.Split(string(respBody), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(payload), &chunk); err == nil && len(chunk.Choices) > 0 {
				sb.WriteString(chunk.Choices[0].Delta.Content)
				reasonSb.WriteString(chunk.Choices[0].Delta.ReasoningContent)
			}
		}
		chatResp.Choices = []chatCompletionChoice{
			{
				Message: chatMessage{
					Role:             "assistant",
					Content:          sb.String(),
					ReasoningContent: reasonSb.String(),
				},
			},
		}
	} else {
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("llm returned error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	rawContent := chatResp.Choices[0].Message.Content
	reasoning := chatResp.Choices[0].Message.ReasoningContent

	if strings.Contains(rawContent, "<think>") && strings.Contains(rawContent, "</think>") {
		sIdx := strings.Index(rawContent, "<think>") + len("<think>")
		eIdx := strings.Index(rawContent, "</think>")
		if eIdx > sIdx {
			if reasoning == "" {
				reasoning = strings.TrimSpace(rawContent[sIdx:eIdx])
			}
			rawContent = strings.TrimSpace(rawContent[:sIdx-len("<think>")] + rawContent[eIdx+len("</think>"):])
		}
	}

	return &Response{
		Content:      rawContent,
		Reasoning:    reasoning,
		PromptTokens: chatResp.Usage.PromptTokens,
		OutputTokens: chatResp.Usage.CompletionTokens,
	}, nil
}
