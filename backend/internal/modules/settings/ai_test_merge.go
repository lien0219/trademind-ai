package settings

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

// TestAIOverrides carries optional form values for POST /settings/test-ai (test before save).
type TestAIOverrides struct {
	Provider   string
	BaseURL    string
	Model      string
	APIKey     string
	TimeoutSec string
}

// MergeAIPlain overlays non-empty test overrides onto stored plaintext ai settings.
// Masked api_key placeholders are ignored so the stored secret is used.
func MergeAIPlain(stored map[string]string, ov *TestAIOverrides) map[string]string {
	out := make(map[string]string, len(stored)+6)
	for k, v := range stored {
		out[k] = v
	}
	if ov == nil {
		return out
	}
	pname := aigate.NormalizeProviderName(ov.Provider)
	if pname == "" {
		pname = aigate.NormalizeProviderName(stored["provider"])
	}
	if pname == "" {
		pname = "openai_compatible"
	}
	if s := strings.TrimSpace(ov.Provider); s != "" {
		out["provider"] = s
	}
	if s := strings.TrimSpace(ov.BaseURL); s != "" {
		out[aigate.ProviderBaseURLKey(pname)] = s
	}
	if s := strings.TrimSpace(ov.Model); s != "" {
		out[aigate.ProviderModelKey(pname)] = s
	}
	if s := strings.TrimSpace(ov.APIKey); s != "" && !encrypt.LooksMasked(s) {
		out[aigate.ProviderAPIKeyKey(pname)] = s
	}
	if s := strings.TrimSpace(ov.TimeoutSec); s != "" {
		out["timeout_sec"] = s
	}
	return out
}
