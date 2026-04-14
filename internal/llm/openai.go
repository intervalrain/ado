package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultOpenAIURL = "https://api.openai.com/v1/chat/completions"

type openaiClient struct {
	apiKey    string
	model     string
	baseURL   string
	maxTokens int
	http      *http.Client
}

func newOpenAIClient(cfg Config) *openaiClient {
	base := cfg.BaseURL
	if base == "" {
		base = defaultOpenAIURL
	}
	return &openaiClient{
		apiKey:    cfg.APIKey,
		model:     cfg.Model,
		baseURL:   base,
		maxTokens: cfg.MaxTokens,
		http:      &http.Client{},
	}
}

type openaiRequest struct {
	Model     string      `json:"model"`
	Messages  []openaiMsg `json:"messages"`
	MaxTokens int         `json:"max_tokens,omitempty"`
}

type openaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (c *openaiClient) Complete(ctx context.Context, system string, messages []Message) (*Response, error) {
	msgs := make([]openaiMsg, 0, len(messages)+1)
	if system != "" {
		msgs = append(msgs, openaiMsg{Role: "system", Content: system})
	}
	for _, m := range messages {
		msgs = append(msgs, openaiMsg{Role: m.Role, Content: m.Content})
	}

	body := openaiRequest{
		Model:     c.model,
		Messages:  msgs,
		MaxTokens: c.maxTokens,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai API %d: %s", resp.StatusCode, string(respBody))
	}

	var result openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}

	text := ""
	if len(result.Choices) > 0 {
		text = result.Choices[0].Message.Content
	}

	return &Response{
		Content: text,
		Model:   result.Model,
		Usage: Usage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
		},
	}, nil
}
