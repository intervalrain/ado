package llm

import (
	"context"
	"fmt"
)

// Message represents a chat message.
type Message struct {
	Role    string // "user" or "assistant"
	Content string
}

// Response holds the LLM output.
type Response struct {
	Content string
	Model   string
	Usage   Usage
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Client is the abstraction over LLM providers.
type Client interface {
	Complete(ctx context.Context, system string, messages []Message) (*Response, error)
}

// Config holds provider-specific settings.
type Config struct {
	Provider  string
	Model     string
	APIKey    string
	BaseURL   string
	MaxTokens int
}

// New creates the appropriate client based on config.
func New(cfg Config) (Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("LLM API key is not set (check %s env var)", cfg.Provider)
	}

	switch cfg.Provider {
	case "claude":
		return newClaudeClient(cfg), nil
	case "openai", "ollama":
		return newOpenAIClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
