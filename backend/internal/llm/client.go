package llm

import (
	"context"
)

// Config holds credentials and parameters for an LLM provider.
type Config struct {
	Name        string  `json:"name"`
	BaseURL     string  `json:"base_url"`
	APIKey      string  `json:"api_key"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

// TokenCallback is called whenever an incremental token chunk is received.
type TokenCallback func(token string)

// Request holds prompt inputs for an LLM call.
type Request struct {
	Model        string        `json:"model,omitempty"`
	SystemPrompt string        `json:"system_prompt"`
	UserPrompt   string        `json:"user_prompt"`
	JSONSchema   interface{}   `json:"json_schema,omitempty"`
	Temperature  float64       `json:"temperature,omitempty"`
	APIKey       string        `json:"api_key,omitempty"`
	BaseURL      string        `json:"base_url,omitempty"`
	OnToken      TokenCallback `json:"-"`
}

// Response holds the text and token metadata from an LLM call.
type Response struct {
	Content      string `json:"content"`
	Reasoning    string `json:"reasoning,omitempty"`
	PromptTokens int    `json:"prompt_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
}

// Client defines the interface for calling LLM models.
type Client interface {
	Generate(ctx context.Context, req Request) (*Response, error)
}
