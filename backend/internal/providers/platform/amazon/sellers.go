package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// MarketplaceParticipationsProbe calls Sellers API for connectivity.
func MarketplaceParticipationsProbe(ctx context.Context, cfg RuntimeConfig, lwaAccess string) (*platformp.TestConnectionResult, error) {
	code, raw, err := doSPAPI(ctx, cfg, http.MethodGet, "/sellers/v1/marketplaceParticipations", nil, nil, lwaAccess)
	if err != nil {
		return nil, err
	}
	if code == http.StatusTooManyRequests {
		return nil, fmt.Errorf("amazon: SP-API rate limited (429)")
	}
	if code < 200 || code >= 300 {
		return &platformp.TestConnectionResult{OK: false, Message: fmt.Sprintf("amazon: marketplaceParticipations http %d: %s", code, apiErrorSnippet(raw))}, nil
	}
	var wrap struct {
		Payload []map[string]any `json:"payload"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: "amazon: invalid marketplaceParticipations response"}, nil
	}
	var region, cur string
	for _, row := range wrap.Payload {
		if mp, ok := row["marketplace"].(map[string]any); ok {
			if cc := pickStrMap(mp, "countryCode"); cc != "" && region == "" {
				region = cc
			}
			if dv := pickStrMap(mp, "defaultCurrencyCode"); dv != "" && cur == "" {
				cur = dv
			}
		}
	}
	return &platformp.TestConnectionResult{
		OK:       true,
		Message:  "amazon SP-API connection ok",
		Region:   region,
		Currency: cur,
	}, nil
}

func pickStrMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}
