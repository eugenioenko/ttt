package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewOpenAIProviderDefaultTimeout(t *testing.T) {
	provider := NewOpenAIProvider(OpenAIConfig{}, nil)
	if provider.config.Timeout != 15*time.Second {
		t.Fatalf("default timeout = %s, want 15s", provider.config.Timeout)
	}
	if provider.client.Timeout != 15*time.Second {
		t.Fatalf("HTTP client timeout = %s, want 15s", provider.client.Timeout)
	}
}

func TestOpenAIProviderComplete(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		want       string
		wantErr    string
		checkInput func(t *testing.T, request chatRequest, auth string)
	}{
		{
			name:   "completion",
			status: http.StatusOK,
			body:   "{\"choices\":[{\"message\":{\"content\":\"```go\\nPrintln()\\n```\"}}]}",
			want:   "Println()",
			checkInput: func(t *testing.T, request chatRequest, auth string) {
				t.Helper()
				if auth != "Bearer test-key" || request.Model != "test-model" {
					t.Fatalf("request auth/model = %q/%q", auth, request.Model)
				}
				if !strings.Contains(request.Messages[1].Content, "fmt.P") {
					t.Fatalf("prompt did not include source context: %q", request.Messages[1].Content)
				}
			},
		},
		{name: "provider error", status: http.StatusBadGateway, body: `{"error":{"message":"upstream unavailable"}}`, wantErr: "upstream unavailable"},
	}
	if err := os.Setenv("TTT_TEST_AI_KEY", "test-key"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TTT_TEST_AI_KEY") })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatalf("unexpected request: %s %s auth=%q", request.Method, request.URL.Path, request.Header.Get("Authorization"))
				}
				var decoded chatRequest
				if err := json.NewDecoder(request.Body).Decode(&decoded); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if tt.checkInput != nil {
					tt.checkInput(t, decoded, request.Header.Get("Authorization"))
				}
				return &http.Response{
					StatusCode: tt.status,
					Status:     http.StatusText(tt.status),
					Body:       io.NopCloser(strings.NewReader(tt.body)),
					Header:     make(http.Header),
				}, nil
			})}
			provider := NewOpenAIProvider(OpenAIConfig{Endpoint: "http://test.invalid", Model: "test-model", APIKeyEnv: "TTT_TEST_AI_KEY"}, client)
			got, err := provider.Complete(context.Background(), CompletionRequest{Language: "go", FilePath: "a.go", Line: 1, Column: 5, Prefix: "fmt.P", Text: "fmt.P"})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Complete() error = %v", err)
			}
			if len(got) != 1 || got[0].Text != tt.want {
				t.Fatalf("completion = %+v, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenAIProviderCompleteCancellation(t *testing.T) {
	if err := os.Setenv("TTT_TEST_AI_KEY", "test-key"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("TTT_TEST_AI_KEY") })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	})}
	provider := NewOpenAIProvider(OpenAIConfig{Endpoint: "http://test.invalid", APIKeyEnv: "TTT_TEST_AI_KEY"}, client)
	_, err := provider.Complete(ctx, CompletionRequest{})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
