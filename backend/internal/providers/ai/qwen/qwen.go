package qwen

import (
	"context"
	"net/http"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
	"github.com/trademind-ai/trademind/backend/internal/providers/ai/errmap"
)

const (
	ProviderName   = "qwen"
	DefaultBaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	DefaultModel   = "qwen-plus"
	providerLabel  = "通义千问"
)

// Client calls Qwen Chat Completions via DashScope OpenAI-compatible API.
type Client struct {
	inner *compatclient.Client
}

// New creates a Qwen client. Empty baseURL uses DefaultBaseURL.
func New(baseURL, apiKey string, httpClient *http.Client) *Client {
	base := baseURL
	if base == "" {
		base = DefaultBaseURL
	}
	return &Client{
		inner: &compatclient.Client{
			BaseURL:    base,
			APIKey:     apiKey,
			HTTPClient: httpClient,
		},
	}
}

// Name returns the provider identifier.
func (c *Client) Name() string { return ProviderName }

// Chat performs a chat completion.
func (c *Client) Chat(ctx context.Context, req compatclient.Request) (*compatclient.Result, error) {
	if c == nil || c.inner == nil {
		return nil, MapError(compatclient.ErrNilClient())
	}
	res, err := c.inner.Chat(ctx, req)
	if err != nil {
		return nil, MapError(err)
	}
	return res, nil
}

// MapError converts errors to Chinese user-facing messages for Qwen.
func MapError(err error) error {
	return errmap.MapChatError(providerLabel, err)
}
