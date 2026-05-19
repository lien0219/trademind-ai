package openaicompat

import (
	"context"
	"net/http"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/compatclient"
	"github.com/trademind-ai/trademind/backend/internal/providers/ai/errmap"
)

const providerLabel = "OpenAI 兼容"

// Adapter wraps compatclient for the openai_compatible provider name.
type Adapter struct {
	inner *compatclient.Client
}

// NewAdapter creates an OpenAI-compatible provider adapter.
func NewAdapter(baseURL, apiKey string, httpClient *http.Client) *Adapter {
	return &Adapter{
		inner: &compatclient.Client{
			BaseURL:    baseURL,
			APIKey:     apiKey,
			HTTPClient: httpClient,
		},
	}
}

// Name returns the provider identifier.
func (a *Adapter) Name() string { return providerName }

// Chat performs a chat completion.
func (a *Adapter) Chat(ctx context.Context, req compatclient.Request) (*compatclient.Result, error) {
	if a == nil || a.inner == nil {
		return nil, MapError(compatclient.ErrNilClient())
	}
	res, err := a.inner.Chat(ctx, req)
	if err != nil {
		return nil, MapError(err)
	}
	return res, nil
}

// MapError converts errors to Chinese user-facing messages.
func MapError(err error) error {
	return errmap.MapChatError(providerLabel, err)
}
