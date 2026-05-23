package imagetask

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
)

// MergeImagePlain overlays non-empty test overrides onto stored plaintext image settings.
// Masked secret placeholders are ignored so stored secrets are used.
func MergeImagePlain(stored map[string]string, overrides map[string]string) map[string]string {
	out := make(map[string]string, len(stored)+len(overrides))
	for k, v := range stored {
		out[k] = v
	}
	if len(overrides) == 0 {
		return out
	}
	for k, v := range overrides {
		s := strings.TrimSpace(v)
		if s == "" || encrypt.LooksMasked(s) {
			continue
		}
		out[k] = v
	}
	return out
}
