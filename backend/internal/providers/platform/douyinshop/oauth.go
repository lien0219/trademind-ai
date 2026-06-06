package douyinshop

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	MethodTokenCreate  = "token.create"
	MethodTokenRefresh = "token.refresh"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	Config RuntimeConfig
	HTTP   HTTPDoer
	Now    func() time.Time
}

type TokenBundle struct {
	AccessToken           string
	RefreshToken          string
	AccessExpiresAt       *time.Time
	RefreshExpiresAt      *time.Time
	Scopes                []any
	PlatformShopID        string
	ShopName              string
	ShopLogo              string
	ShopStatus            string
	RawNonSensitiveFields map[string]any
}

type APIError struct {
	Code    string
	Message string
	LogID   string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

func BuildAuthorizeURL(cfg RuntimeConfig, state string) (string, error) {
	serviceID := strings.TrimSpace(cfg.ServiceID)
	if serviceID == "" {
		return "", fmt.Errorf("douyin_shop service_id is required for OAuth authorize URL")
	}
	base := strings.TrimSpace(cfg.AuthBaseURL)
	if base == "" {
		base = "https://fuwu.jinritemai.com"
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if err == nil {
			err = fmt.Errorf("missing scheme or host")
		}
		return "", fmt.Errorf("douyin_shop: invalid auth_base_url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/authorize"
	q := u.Query()
	q.Set("service_id", serviceID)
	q.Set("state", strings.TrimSpace(state))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func Sign(params map[string]string, appSecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if strings.TrimSpace(k) == "" || k == "sign" || k == "access_token" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	secret := strings.TrimSpace(appSecret)
	b.WriteString(secret)
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(secret)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(b.String()))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

func (c Client) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}

func (c Client) httpClient() HTTPDoer {
	if c.HTTP != nil {
		return c.HTTP
	}
	timeout := c.Config.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

func (c Client) call(ctx context.Context, method string, param map[string]any, accessToken string) (map[string]any, error) {
	paramJSON, err := json.Marshal(param)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSpace(c.Config.APIBaseURL)
	if base == "" {
		base = "https://openapi-fxg.jinritemai.com"
	}
	u, err := url.Parse(strings.TrimRight(base, "/") + "/" + strings.ReplaceAll(method, ".", "/"))
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"app_key":     strings.TrimSpace(c.Config.AppKey),
		"method":      method,
		"param_json":  string(paramJSON),
		"sign_method": "hmac-sha256",
		"timestamp":   strconv.FormatInt(c.now().Unix(), 10),
		"v":           "2",
	}
	if at := strings.TrimSpace(accessToken); at != "" {
		params["access_token"] = at
	}
	params["sign"] = Sign(params, c.Config.AppSecret)
	q := u.Query()
	for k, v := range params {
		if k == "param_json" {
			continue
		}
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(paramJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Code: strconv.Itoa(resp.StatusCode), Message: "douyin openapi http error"}
	}
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if !apiSuccess(env) {
		return nil, &APIError{Code: apiCode(env), Message: apiMessage(env), LogID: stringFromAny(env["log_id"])}
	}
	if d, ok := env["data"].(map[string]any); ok {
		return d, nil
	}
	return env, nil
}

func apiSuccess(env map[string]any) bool {
	if env == nil {
		return false
	}
	if v, ok := env["code"]; ok {
		s := stringFromAny(v)
		return s == "" || s == "0" || s == "10000" || strings.EqualFold(s, "success")
	}
	if v, ok := env["err_no"]; ok {
		return stringFromAny(v) == "0"
	}
	if v, ok := env["errno"]; ok {
		return stringFromAny(v) == "0"
	}
	return true
}

func apiCode(env map[string]any) string {
	for _, k := range []string{"code", "err_no", "errno"} {
		if s := stringFromAny(env[k]); s != "" {
			return s
		}
	}
	return "UNKNOWN_DOUYIN_AUTH_ERROR"
}

func apiMessage(env map[string]any) string {
	for _, k := range []string{"msg", "message", "sub_msg"} {
		if s := stringFromAny(env[k]); s != "" {
			return s
		}
	}
	return "douyin openapi returned an error"
}

func stringFromAny(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		if float64(int64(x)) == x {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func int64FromAny(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		return 0
	}
}

func pickString(data map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := stringFromAny(data[k]); s != "" {
			return s
		}
	}
	return ""
}

func pickExpiresIn(data map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if n := int64FromAny(data[k]); n > 0 {
			return n
		}
	}
	return 0
}

func parseTokenData(now time.Time, data map[string]any) (*TokenBundle, error) {
	access := pickString(data, "access_token", "accessToken")
	if access == "" {
		return nil, fmt.Errorf("douyin token response missing access_token")
	}
	refresh := pickString(data, "refresh_token", "refreshToken")
	accessIn := pickExpiresIn(data, "expires_in", "expire_in", "access_token_expire_in", "access_token_expires_in")
	refreshIn := pickExpiresIn(data, "refresh_expires_in", "refresh_expire_in", "refresh_token_expire_in", "refresh_token_expires_in")
	var accessExp *time.Time
	if accessIn > 0 {
		t := now.Add(time.Duration(accessIn) * time.Second)
		accessExp = &t
	}
	var refreshExp *time.Time
	if refreshIn > 0 {
		t := now.Add(time.Duration(refreshIn) * time.Second)
		refreshExp = &t
	}
	var scopes []any
	if raw, ok := data["scope"]; ok {
		if s := stringFromAny(raw); s != "" {
			for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
				if p := strings.TrimSpace(part); p != "" {
					scopes = append(scopes, p)
				}
			}
		}
	}
	if raw, ok := data["scopes"].([]any); ok {
		scopes = raw
	}
	return &TokenBundle{
		AccessToken:      access,
		RefreshToken:     refresh,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		Scopes:           scopes,
		PlatformShopID:   pickString(data, "shop_id", "shopId", "shop_cipher", "shopCipher", "seller_id"),
		ShopName:         pickString(data, "shop_name", "shopName", "name"),
		ShopLogo:         pickString(data, "shop_logo", "logo", "shopLogo"),
		ShopStatus:       pickString(data, "shop_status", "status", "shopStatus"),
		RawNonSensitiveFields: map[string]any{
			"shop_id":     pickString(data, "shop_id", "shopId", "shop_cipher", "shopCipher", "seller_id"),
			"shop_name":   pickString(data, "shop_name", "shopName", "name"),
			"shop_status": pickString(data, "shop_status", "status", "shopStatus"),
			"saved_at":    now.Format(time.RFC3339),
		},
	}, nil
}

func (c Client) ExchangeCode(ctx context.Context, code string) (*TokenBundle, error) {
	data, err := c.call(ctx, MethodTokenCreate, map[string]any{"code": strings.TrimSpace(code), "grant_type": "authorization_code"}, "")
	if err != nil {
		return nil, err
	}
	return parseTokenData(c.now(), data)
}

func (c Client) RefreshToken(ctx context.Context, refreshToken string) (*TokenBundle, error) {
	data, err := c.call(ctx, MethodTokenRefresh, map[string]any{"refresh_token": strings.TrimSpace(refreshToken), "grant_type": "refresh_token"}, "")
	if err != nil {
		return nil, err
	}
	return parseTokenData(c.now(), data)
}
