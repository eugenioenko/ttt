// Package ai provides adapters for AI-powered editor services.
package ai

import "context"

// CompletionRequest describes the editor state sent to an AI completion
// provider. Text is intentionally supplied by the caller so providers do not
// need access to editor or filesystem state.
type CompletionRequest struct {
	Language string
	FilePath string
	Text     string
	Line     int
	Column   int
	Prefix   string
}

// Completion is a provider-neutral completion suggestion.
type Completion struct {
	Text   string
	Detail string
}

// Provider returns AI-generated completion suggestions.
type Provider interface {
	Complete(ctx context.Context, request CompletionRequest) ([]Completion, error)
}
