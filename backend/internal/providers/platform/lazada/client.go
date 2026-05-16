package lazada

import (
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

func nowMillisString() string {
	return strconv.FormatInt(time.Now().UnixMilli(), 10)
}

// buildSignedQuery adds sign to extra system params (excludes empty keys; sign computed last).
func buildSignedQuery(cfg RuntimeConfig, apiPath string, accessToken string, extra map[string]string) url.Values {
	p := map[string]string{
		"app_key":     cfg.AppKey,
		"timestamp":   nowMillisString(),
		"sign_method": SignMethodSHA256,
	}
	if strings.TrimSpace(accessToken) != "" {
		p["access_token"] = accessToken
	}
	for k, v := range extra {
		kk := strings.TrimSpace(k)
		if kk == "" || strings.EqualFold(kk, "sign") {
			continue
		}
		if strings.TrimSpace(v) == "" {
			continue
		}
		p[kk] = v
	}
	p["sign"] = Sign(apiPath, p, "", cfg.AppSecret)
	q := url.Values{}
	for k, v := range p {
		q.Set(k, v)
	}
	return q
}

func getSigned(ctx context.Context, cfg RuntimeConfig, restBase, apiPath, accessToken string, extra map[string]string) (map[string]any, error) {
	restBase = strings.TrimSuffix(strings.TrimSpace(restBase), "/")
	apiPath = "/" + strings.TrimPrefix(apiPath, "/")
	q := buildSignedQuery(cfg, apiPath, accessToken, extra)
	u := restBase + apiPath + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("lazada http %d: %s", resp.StatusCode, trimPreview(string(b), 400))
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		return nil, fmt.Errorf("lazada: invalid json: %w", err)
	}
	if err := lazadaErr(root); err != nil {
		return nil, err
	}
	return root, nil
}

func lazadaErr(m map[string]any) error {
	if m == nil {
		return nil
	}
	code := m["code"]
	ok := false
	switch c := code.(type) {
	case string:
		ok = c == "0"
	case float64:
		ok = c == 0
	}
	if ok {
		return nil
	}
	msg := pickStr(m, "message", "detail", "type")
	return fmt.Errorf("lazada api: code=%v message=%s", code, strings.TrimSpace(msg))
}

func trimPreview(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// --- seller / orders (authenticated) ---

func apiGetSeller(ctx context.Context, cfg RuntimeConfig, access string) (map[string]any, error) {
	root, err := getSigned(ctx, cfg, cfg.APIRESTBase, PathSellerGet, access, nil)
	if err != nil {
		return nil, err
	}
	if d, ok := root["data"].(map[string]any); ok {
		return d, nil
	}
	return map[string]any{}, nil
}

// GetSellerInfo returns seller payload for TestConnection.
func GetSellerInfo(ctx context.Context, cfg RuntimeConfig, access string) (name, sellerID, region, shortCode string, err error) {
	d, err := apiGetSeller(ctx, cfg, access)
	if err != nil {
		return "", "", "", "", err
	}
	name = firstNonEmpty(
		pickStr(d, "name", "shop_name", "short_code"),
	)
	sellerID = firstNonEmpty(
		pickStr(d, "seller_id", "seller_short_code"),
		fmt.Sprint(d["seller_id"]),
	)
	shortCode = pickStr(d, "short_code", "seller_short_code")
	region = pickStr(d, "region", "country", "venture")
	return strings.TrimSpace(name), strings.TrimSpace(sellerID), strings.TrimSpace(region), strings.TrimSpace(shortCode), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func pickStr(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if strings.TrimSpace(t) != "" {
					return strings.TrimSpace(t)
				}
			case float64:
				return strconv.FormatInt(int64(t), 10)
			default:
				s := strings.TrimSpace(fmt.Sprint(t))
				if s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// --- OAuth token endpoints (auth REST base, no access_token) ---

// ExchangeAuthCode trades OAuth code for tokens.
func ExchangeAuthCode(ctx context.Context, auth platformp.TestConnectionRequest, code string) (TokenEnvelope, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return TokenEnvelope{}, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return TokenEnvelope{}, fmt.Errorf("code required")
	}
	root, err := getSigned(ctx, cfg, cfg.AuthRESTBase, PathAuthTokenCreate, "", map[string]string{
		"code": code,
	})
	if err != nil {
		return TokenEnvelope{}, err
	}
	return parseTokenEnvelope(root), nil
}

// RefreshAccessToken refreshes tokens.
func RefreshAccessToken(ctx context.Context, auth platformp.TestConnectionRequest, refresh string) (TokenEnvelope, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return TokenEnvelope{}, err
	}
	refresh = strings.TrimSpace(refresh)
	if refresh == "" {
		return TokenEnvelope{}, fmt.Errorf("refresh_token required")
	}
	root, err := getSigned(ctx, cfg, cfg.AuthRESTBase, PathAuthTokenRefresh, "", map[string]string{
		"refresh_token": refresh,
	})
	if err != nil {
		return TokenEnvelope{}, err
	}
	return parseTokenEnvelope(root), nil
}

// TokenEnvelope is normalized OAuth token payload (no logging of secrets).
type TokenEnvelope struct {
	AccessToken           string
	RefreshToken          string
	AccessExpiresAt       *time.Time
	RefreshExpiresAt      *time.Time
	SellerID              string
	AccountID             string
	Country               string
	RefreshTokenExpiresIn int64
}

func parseTokenEnvelope(root map[string]any) TokenEnvelope {
	var out TokenEnvelope
	pick := root
	if d, ok := root["data"].(map[string]any); ok && len(d) > 0 {
		pick = d
	}
	out.AccessToken = firstNonEmpty(pickStr(pick, "access_token"), pickStr(root, "access_token"))
	out.RefreshToken = firstNonEmpty(pickStr(pick, "refresh_token"), pickStr(root, "refresh_token"))
	out.AccountID = firstNonEmpty(pickStr(pick, "account", "account_id"), pickStr(root, "account"))
	out.SellerID = pickStr(pick, "seller_id")
	if out.SellerID == "" && pick["seller_id"] != nil {
		out.SellerID = strings.TrimSpace(fmt.Sprint(pick["seller_id"]))
	}
	if out.AccountID != "" && out.SellerID == "" {
		out.SellerID = out.AccountID
	}
	out.Country = firstNonEmpty(pickStr(pick, "country"), pickStr(root, "country"))

	if exp, ok := toFloat64(pick["expires_in"]); ok && exp > 0 {
		t := time.Now().UTC().Add(time.Duration(exp) * time.Second)
		out.AccessExpiresAt = &t
	}
	if exp, ok := toFloat64(pick["refresh_expires_in"]); ok && exp > 0 {
		t := time.Now().UTC().Add(time.Duration(exp) * time.Second)
		out.RefreshExpiresAt = &t
		out.RefreshTokenExpiresIn = int64(exp)
	}
	return out
}

func toFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}
