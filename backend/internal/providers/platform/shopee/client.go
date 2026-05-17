package shopee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func nowUnix() int64 { return time.Now().Unix() }

func postPublic(ctx context.Context, cfg RuntimeConfig, apiPath string, body map[string]any) (map[string]any, error) {
	ts := nowUnix()
	baseStr := BaseStringPublic(cfg.PartnerID, apiPath, ts)
	sign := SignHMAC(cfg.PartnerKey, baseStr)
	u := fmt.Sprintf("%s%s?partner_id=%d&timestamp=%d&sign=%s",
		cfg.APIBaseURL, apiPath, cfg.PartnerID, ts, sign)
	return postJSON(ctx, cfg, u, body)
}

func postShop(ctx context.Context, cfg RuntimeConfig, apiPath string, shopID int64, accessToken string, body map[string]any) (map[string]any, error) {
	r, _, err := postShopWithStatus(ctx, cfg, apiPath, shopID, accessToken, body)
	return r, err
}

// postShopWithStatus calls a shop-level POST endpoint and returns HTTP status for upper-layer error classification.
func postShopWithStatus(ctx context.Context, cfg RuntimeConfig, apiPath string, shopID int64, accessToken string, body map[string]any) (map[string]any, int, error) {
	ts := nowUnix()
	baseStr := BaseStringShop(cfg.PartnerID, apiPath, ts, accessToken, shopID)
	sign := SignHMAC(cfg.PartnerKey, baseStr)
	q := url.Values{}
	q.Set("partner_id", strconv.FormatInt(cfg.PartnerID, 10))
	q.Set("shop_id", strconv.FormatInt(shopID, 10))
	q.Set("timestamp", strconv.FormatInt(ts, 10))
	q.Set("access_token", accessToken)
	q.Set("sign", sign)
	u := cfg.APIBaseURL + apiPath + "?" + q.Encode()
	return postJSONWithStatus(ctx, cfg, u, body)
}

// getShopWithStatus calls a shop-level GET endpoint (query in URL, signature over base shop params only).
func getShopWithStatus(ctx context.Context, cfg RuntimeConfig, apiPath string, shopID int64, accessToken string, extra url.Values) (map[string]any, int, error) {
	ts := nowUnix()
	baseStr := BaseStringShop(cfg.PartnerID, apiPath, ts, accessToken, shopID)
	sign := SignHMAC(cfg.PartnerKey, baseStr)
	q := url.Values{}
	q.Set("partner_id", strconv.FormatInt(cfg.PartnerID, 10))
	q.Set("shop_id", strconv.FormatInt(shopID, 10))
	q.Set("timestamp", strconv.FormatInt(ts, 10))
	q.Set("access_token", accessToken)
	q.Set("sign", sign)
	if extra != nil {
		for k, vs := range extra {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
	}
	u := cfg.APIBaseURL + apiPath + "?" + q.Encode()
	return getJSONWithStatus(ctx, cfg, u)
}

func postJSON(ctx context.Context, cfg RuntimeConfig, urlStr string, body map[string]any) (map[string]any, error) {
	r, _, err := postJSONWithStatus(ctx, cfg, urlStr, body)
	return r, err
}

func postJSONWithStatus(ctx context.Context, cfg RuntimeConfig, urlStr string, body map[string]any) (map[string]any, int, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	if len(payload) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	return doShopeeHTTPRead(ctx, cfg, req)
}

func getJSONWithStatus(ctx context.Context, cfg RuntimeConfig, urlStr string) (map[string]any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, 0, err
	}
	return doShopeeHTTPRead(ctx, cfg, req)
}

func doShopeeHTTPRead(ctx context.Context, cfg RuntimeConfig, req *http.Request) (map[string]any, int, error) {
	client := http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	st := resp.StatusCode
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		if st < 200 || st >= 300 {
			return nil, st, fmt.Errorf("shopee http %d: %s", st, trimPreview(string(b), 400))
		}
		return nil, st, fmt.Errorf("shopee: invalid json: %w", err)
	}
	if st < 200 || st >= 300 {
		if err := shopeeErr(root); err != nil {
			return root, st, err
		}
		return root, st, fmt.Errorf("shopee http %d: %s", st, trimPreview(string(b), 400))
	}
	if err := shopeeErr(root); err != nil {
		return nil, st, err
	}
	out, err := unwrapResponse(root)
	return out, st, err
}

func shopeeErr(root map[string]any) error {
	if root == nil {
		return nil
	}
	// Top-level "error" is often empty string on success
	if v, ok := root["error"]; ok && v != nil {
		switch t := v.(type) {
		case string:
			if strings.TrimSpace(t) != "" {
				if msg, _ := root["message"].(string); strings.TrimSpace(msg) != "" {
					return fmt.Errorf("shopee: %s (%s)", t, msg)
				}
				return fmt.Errorf("shopee: %s", t)
			}
		case float64:
			if t != 0 {
				msg, _ := root["message"].(string)
				return fmt.Errorf("shopee: error %v %s", t, strings.TrimSpace(msg))
			}
		}
	}
	return nil
}

func unwrapResponse(root map[string]any) (map[string]any, error) {
	if r, ok := root["response"].(map[string]any); ok {
		return r, nil
	}
	return root, nil
}

func trimPreview(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// TokenEnvelope is normalized OAuth token payload.
type TokenEnvelope struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  *time.Time
	RefreshExpiresAt *time.Time
}

// ExchangeAuthCode exchanges OAuth code for shop tokens.
func ExchangeAuthCode(ctx context.Context, auth platformp.TestConnectionRequest, code string, shopID int64) (TokenEnvelope, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return TokenEnvelope{}, err
	}
	code = strings.TrimSpace(code)
	if code == "" || shopID <= 0 {
		return TokenEnvelope{}, fmt.Errorf("code and shop_id required")
	}
	body := map[string]any{
		"code":       code,
		"shop_id":    shopID,
		"partner_id": cfg.PartnerID,
	}
	r, err := postPublic(ctx, cfg, PathAuthTokenGet, body)
	if err != nil {
		return TokenEnvelope{}, err
	}
	return parseTokenEnvelope(r), nil
}

// RefreshAccessToken refreshes access_token using refresh_token.
func RefreshAccessToken(ctx context.Context, auth platformp.TestConnectionRequest, refresh string, shopID int64) (TokenEnvelope, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return TokenEnvelope{}, err
	}
	refresh = strings.TrimSpace(refresh)
	if refresh == "" || shopID <= 0 {
		return TokenEnvelope{}, fmt.Errorf("refresh_token and shop_id required")
	}
	body := map[string]any{
		"refresh_token": refresh,
		"shop_id":       shopID,
		"partner_id":    cfg.PartnerID,
	}
	r, err := postPublic(ctx, cfg, PathAuthAccessToken, body)
	if err != nil {
		return TokenEnvelope{}, err
	}
	return parseTokenEnvelope(r), nil
}

func parseTokenEnvelope(r map[string]any) TokenEnvelope {
	var out TokenEnvelope
	out.AccessToken = strField(r, "access_token")
	out.RefreshToken = strField(r, "refresh_token")
	// expire_in seconds
	if exp, ok := r["expire_in"].(float64); ok && exp > 0 {
		t := time.Now().UTC().Add(time.Duration(exp) * time.Second)
		out.AccessExpiresAt = &t
	} else if exp, ok := r["expires_in"].(float64); ok && exp > 0 {
		t := time.Now().UTC().Add(time.Duration(exp) * time.Second)
		out.AccessExpiresAt = &t
	}
	return out
}

func strField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}
