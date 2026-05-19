package openaicompat

import (
	"context"
	"net/http"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
)

const providerName = "openai_compatible"

// Message is one chat message (legacy alias).
type Message = compatclient.Message

// Request is a chat/completions payload (legacy alias).
type Request = compatclient.Request

// Result is a normalized completion outcome (legacy alias).
type Result = compatclient.Result

// Client calls an OpenAI-compatible POST /chat/completions endpoint.
type Client struct {
	inner *compatclient.Client
}

// Name identifies this adapter.
func (c *Client) Name() string { return providerName }

// NewClient creates a legacy Client wrapper.
func NewClient(baseURL, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		inner: &compatclient.Client{
			BaseURL:    baseURL,
			APIKey:     apiKey,
			HTTPClient: httpClient,
		},
	}
}

// Chat calls the remote API.
func (c *Client) Chat(ctx context.Context, req Request) (*Result, error) {
	if c == nil || c.inner == nil {
		return nil, compatclient.ErrNilClient()
	}
	return c.inner.Chat(ctx, req)
}

// Inner exposes the shared compat client for adapters.
func (c *Client) Inner() *compatclient.Client {
	if c == nil {
		return nil
	}
	return c.inner
}
