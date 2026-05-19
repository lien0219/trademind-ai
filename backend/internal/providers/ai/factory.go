package ai

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/providers/ai/deepseek"
	"github.com/trademind-ai/trademind/backend/internal/providers/ai/openai"
	openaicompat "github.com/trademind-ai/trademind/backend/internal/providers/ai/openai_compatible"
	"github.com/trademind-ai/trademind/backend/internal/providers/ai/qwen"
)

// NewProvider builds a concrete Provider from resolved settings.
func NewProvider(providerName, baseURL, apiKey string, httpClient *http.Client) (Provider, error) {
	pname := normalizeProviderName(providerName)
	if pname == "" {
		pname = "openai_compatible"
	}
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	key := strings.TrimSpace(apiKey)

	var inner compatCaller
	switch pname {
	case "openai":
		if base == "" {
			base = strings.TrimRight(openai.DefaultBaseURL, "/")
		}
		inner = openai.New(base, key, httpClient)
	case "openai_compatible":
		if base == "" {
			return nil, fmt.Errorf("请配置 base_url")
		}
		inner = openaicompat.NewAdapter(base, key, httpClient)
	case "deepseek":
		inner = deepseek.New(base, key, httpClient)
	case "qwen":
		inner = qwen.New(base, key, httpClient)
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", strings.TrimSpace(providerName))
	}
	return &compatProvider{inner: inner}, nil
}
