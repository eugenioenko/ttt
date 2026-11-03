package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAIConfig configures an OpenAI-compatible chat-completions endpoint.
type OpenAIConfig struct {
	Endpoint  string
	Model     string
	APIKeyEnv string
	Timeout   time.Duration
}

// OpenAIProvider implements Provider using an OpenAI-compatible HTTP API.
type OpenAIProvider struct {
	config OpenAIConfig
	client *http.Client
}

// NewOpenAIProvider returns a provider using the supplied HTTP client. A nil
// client uses a client with the configured timeout.
func NewOpenAIProvider(config OpenAIConfig, client *http.Client) *OpenAIProvider {
	if config.Endpoint == "" {
		config.Endpoint = "https://api.openai.com/v1/chat/completions"
	}
	if config.Model == "" {
		config.Model = "gpt-4.1-mini"
	}
	if config.APIKeyEnv == "" {
		config.APIKeyEnv = "OPENAI_API_KEY"
	}
	if config.Timeout <= 0 {
		config.Timeout = 15 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	return &OpenAIProvider{config: config, client: client}
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete requests a short code continuation. The prompt asks the model for
// only the text after the cursor so the existing editor insertion path can
// preserve undo behavior and LSP-style word filtering.
func (p *OpenAIProvider) Complete(ctx context.Context, request CompletionRequest) ([]Completion, error) {
	apiKey := os.Getenv(p.config.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("AI API key environment variable %q is not set", p.config.APIKeyEnv)
	}

	body, err := json.Marshal(chatRequest{
		Model: p.config.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a code completion engine. Return only the code that should be inserted after the cursor. Do not use markdown fences, explanations, or quotes. Keep the continuation short."},
			{Role: "user", Content: completionPrompt(request)},
		},
		Temperature: 0.2,
		MaxTokens:   128,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal AI completion request: %w", err)
	}
	slog.DebugContext(ctx, "AI completion prompt", "prompt", completionPrompt(request))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create AI completion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send AI completion request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read AI completion response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		var failure errorResponse
		if json.Unmarshal(responseBody, &failure) == nil && failure.Error.Message != "" {
			return nil, fmt.Errorf("AI completion request failed with status %s: %s", resp.Status, failure.Error.Message)
		}
		return nil, fmt.Errorf("AI completion request failed with status %s", resp.Status)
	}

	var decoded chatResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, fmt.Errorf("decode AI completion response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return nil, nil
	}
	slog.DebugContext(ctx, "AI completion result", "choices", decoded.Choices)

	text := cleanCompletion(decoded.Choices[0].Message.Content)
	if text == "" {
		return nil, nil
	}
	return []Completion{{Text: text, Detail: "AI"}}, nil
}

func completionPrompt(request CompletionRequest) string {
	return fmt.Sprintf("Language: %s\nFile: %s\nCursor: line %d, column %d\nPrefix: %q\n\nCode:\n%s", request.Language, request.FilePath, request.Line, request.Column, request.Prefix, request.Text)
}

func cleanCompletion(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") && strings.HasSuffix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 2 {
			text = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	return strings.TrimSpace(text)
}

var _ Provider = (*OpenAIProvider)(nil)
