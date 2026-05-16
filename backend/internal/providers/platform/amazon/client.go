package amazon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func doSPAPI(ctx context.Context, cfg RuntimeConfig, method, relPath string, query url.Values, body []byte, lwaAccess string) (int, []byte, error) {
	base := strings.TrimSuffix(cfg.SPAPIBaseURL, "/")
	rel := "/" + strings.TrimPrefix(relPath, "/")
	u, err := url.Parse(base + rel)
	if err != nil {
		return 0, nil, err
	}
	if query != nil && len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", "TradeMind-SPAPI/1.0")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	tok := strings.TrimSpace(lwaAccess)
	if tok == "" {
		return 0, nil, fmt.Errorf("amazon: missing LWA access token for SP-API")
	}
	req.Header.Set("x-amz-access-token", tok)

	if err := signSPAPIRequest(ctx, req, cfg, body); err != nil {
		return 0, nil, err
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("amazon: sp-api request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}

func apiErrorSnippet(body []byte) string {
	var w struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &w); err != nil || len(w.Errors) == 0 {
		if len(body) > 480 {
			return string(body[:480]) + "..."
		}
		return string(body)
	}
	var parts []string
	for _, e := range w.Errors {
		parts = append(parts, strings.TrimSpace(e.Code)+": "+strings.TrimSpace(e.Message))
	}
	return strings.Join(parts, "; ")
}
