package deepseek

import (
	"context"
	"net/http"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
	"github.com/trademind-ai/trademind/backend/internal/providers/ai/errmap"
)

const (
	ProviderName   = "deepseek"
	DefaultBaseURL = "https://api.deepseek.com/v1"
	DefaultModel   = "deepseek-chat"
	providerLabel  = "DeepSeek"
)

// Client calls DeepSeek Chat Completions (OpenAI-compatible).
type Client struct {
	inner *compatclient.Client
}

// New creates a DeepSeek client. Empty baseURL uses DefaultBaseURL.
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

// MapError converts errors to Chinese user-facing messages for DeepSeek.
func MapError(err error) error {
	return errmap.MapChatError(providerLabel, err)
}
