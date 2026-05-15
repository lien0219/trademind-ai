package ai

import "context"

// Provider abstracts OpenAI-compatible and other LLM backends.
type Provider interface {
	Ping(ctx context.Context) error
}
