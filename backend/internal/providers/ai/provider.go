package ai

import "context"

// Provider is a pluggable AI backend (OpenAI-compatible, DeepSeek, etc.).
type Provider interface {
	Name() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
