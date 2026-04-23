package llm

import (
	"context"
	"fmt"
	"strings"
)

const defaultOllamaHost = "http://localhost:11434"

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
	// API key required for hosted providers; ollama runs locally and is exempt.
	if cfg.Provider != "ollama" && cfg.APIKey == "" {
		return nil, fmt.Errorf("LLM API key is not set for provider %q", cfg.Provider)
	}

	switch cfg.Provider {
	case "claude":
		return newClaudeClient(cfg), nil
	case "openai":
		return newOpenAIClient(cfg), nil
	case "ollama":
		// Accept either the host (http://localhost:11434) or the full endpoint;
		// the OpenAI-compat client needs the full /v1/chat/completions path.
		host := strings.TrimRight(cfg.BaseURL, "/")
		if host == "" {
			host = defaultOllamaHost
		}
		if !strings.Contains(host, "/chat/completions") {
			host = host + "/v1/chat/completions"
		}
		cfg.BaseURL = host
		return newOpenAIClient(cfg), nil
	case "gemini":
		return newGeminiClient(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
