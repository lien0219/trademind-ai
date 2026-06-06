package douyinshop

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestBuildAuthorizeURL(t *testing.T) {
	cfg := RuntimeConfig{AuthBaseURL: "https://fuwu.jinritemai.com", ServiceID: "svc_1"}
	u, err := BuildAuthorizeURL(cfg, "state123")
	if err != nil {
		t.Fatalf("BuildAuthorizeURL() error = %v", err)
	}
	if !strings.HasPrefix(u, "https://fuwu.jinritemai.com/authorize?") {
		t.Fatalf("unexpected authorize url: %s", u)
	}
	if !strings.Contains(u, "service_id=svc_1") || !strings.Contains(u, "state=state123") {
		t.Fatalf("missing official authorize params: %s", u)
	}
}

func TestBuildAuthorizeURLMissingServiceID(t *testing.T) {
	if _, err := BuildAuthorizeURL(RuntimeConfig{}, "s"); err == nil {
		t.Fatalf("expected missing service_id error")
	}
}

func TestSignStableAndExcludesSign(t *testing.T) {
	params := map[string]string{
		"method":      "token.create",
		"app_key":     "ak",
		"timestamp":   "1700000000",
		"v":           "2",
		"param_json":  `{"code":"c"}`,
		"sign_method": "hmac-sha256",
		"sign":        "old",
	}
	got := Sign(params, "secret")
	params["sign"] = "changed"
	if got == "" || got != Sign(params, "secret") {
		t.Fatalf("sign should be stable and ignore existing sign")
	}
	if got != strings.ToUpper(got) {
		t.Fatalf("sign should be uppercase hex")
	}
}

func TestSignCanonicalOrderAndParamJSON(t *testing.T) {
	params := map[string]string{
		"timestamp":    "1700000000",
		"param_json":   `{"b":2,"a":1}`,
		"app_key":      "ak",
		"method":       "shop.info",
		"v":            "2",
		"access_token": "access-secret",
	}
	got := CanonicalSigningString(params, "secret")
	want := `secretapp_keyakmethodshop.infoparam_json{"b":2,"a":1}timestamp1700000000v2secret`
	if got != want {
		t.Fatalf("canonical string mismatch:\n got %s\nwant %s", got, want)
	}
	if strings.Contains(got, "access-secret") {
		t.Fatalf("access token must not participate in signing")
	}
}

func TestSignEmptyParamsStable(t *testing.T) {
	a := Sign(map[string]string{}, "secret")
	b := Sign(map[string]string{"sign": "ignored", "access_token": "hidden"}, "secret")
	if a == "" || a != b {
		t.Fatalf("empty signing set should be stable")
	}
}

func TestExchangeCodeParsesTokenResponse(t *testing.T) {
	now := time.Date(2026, 6, 6, 1, 2, 3, 0, time.UTC)
	c := Client{
		Config: RuntimeConfig{AppKey: "ak", AppSecret: "secret", APIBaseURL: "https://openapi-fxg.jinritemai.com"},
		Now:    func() time.Time { return now },
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/token/create" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			if req.URL.Query().Get("method") != MethodTokenCreate || req.URL.Query().Get("sign") == "" {
				t.Fatalf("missing common signed params: %s", req.URL.RawQuery)
			}
			if req.URL.Query().Get("param_json") != "" {
				t.Fatalf("param_json should be posted in body, not query")
			}
			body, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(body), `"code":"code"`) {
				t.Fatalf("unexpected body: %s", string(body))
			}
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(`{
					"code": 10000,
					"data": {
						"access_token": "access",
						"refresh_token": "refresh",
						"expires_in": 3600,
						"refresh_expires_in": 7200,
						"shop_id": "shop-1",
						"shop_name": "Demo Shop",
						"scope": "a,b"
					}
				}`)),
			}, nil
		}),
	}
	tok, err := c.ExchangeCode(context.Background(), "code")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}
	if tok.AccessToken != "access" || tok.RefreshToken != "refresh" || tok.PlatformShopID != "shop-1" || tok.ShopName != "Demo Shop" {
		t.Fatalf("unexpected token bundle: %+v", tok)
	}
	if tok.AccessExpiresAt == nil || !tok.AccessExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected access expiry: %v", tok.AccessExpiresAt)
	}
	if len(tok.Scopes) != 2 {
		t.Fatalf("unexpected scopes: %+v", tok.Scopes)
	}
}

func TestRefreshTokenFailure(t *testing.T) {
	c := Client{
		Config: RuntimeConfig{AppKey: "ak", AppSecret: "secret", APIBaseURL: "https://openapi-fxg.jinritemai.com"},
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"code":20001,"msg":"invalid refresh token","log_id":"r1"}`)),
			}, nil
		}),
	}
	_, err := c.RefreshToken(context.Background(), "bad")
	if err == nil {
		t.Fatalf("expected refresh failure")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinAuthExpired {
		t.Fatalf("expected auth expired mapping, got %#v err=%v", de, err)
	}
}

func TestDoSuccessParsesResponseAndLogsSafely(t *testing.T) {
	var log SafeRequestLog
	c := &Client{
		Config:      RuntimeConfig{AppKey: "ak", AppSecret: "secret", APIBaseURL: "https://openapi-fxg.jinritemai.com"},
		AccessToken: "access-token",
		Logger: SafeLoggerFunc(func(ctx context.Context, entry SafeRequestLog) {
			log = entry
		}),
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Query().Get("access_token") != "access-token" {
				t.Fatalf("missing access_token in query")
			}
			if req.URL.Query().Get("sign") == "" {
				t.Fatalf("missing signature")
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"code":10000,"log_id":"rid-1","data":{"ok":true,"name":"demo"}}`)),
			}, nil
		}),
	}
	var out struct {
		OK   bool   `json:"ok"`
		Name string `json:"name"`
	}
	if err := c.Do(context.Background(), "shop.info", map[string]any{"a": 1}, &out); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if !out.OK || out.Name != "demo" {
		t.Fatalf("unexpected decoded output: %+v", out)
	}
	if !log.Success || log.RequestID != "rid-1" || log.Method != "shop.info" {
		t.Fatalf("unexpected safe log: %+v", log)
	}
	if strings.Contains(strings.ToLower(log.Method+log.RequestID+log.ErrorCode+log.PlatformCode), "secret") {
		t.Fatalf("safe log leaked secret: %+v", log)
	}
}

func TestDoPlatformErrorConverted(t *testing.T) {
	c := &Client{
		Config:      RuntimeConfig{AppKey: "ak", AppSecret: "secret", APIBaseURL: "https://openapi-fxg.jinritemai.com"},
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"code":429,"msg":"rate limit","log_id":"rid-2"}`)),
			}, nil
		}),
	}
	err := c.Do(context.Background(), "shop.info", nil, &struct{}{})
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinRateLimited || !de.RateLimited || de.RequestID != "rid-2" {
		t.Fatalf("unexpected converted error: %#v err=%v", de, err)
	}
}

func TestEnsureFreshAccessRefreshesExpiredToken(t *testing.T) {
	now := time.Date(2026, 6, 6, 1, 2, 3, 0, time.UTC)
	var persisted *TokenBundle
	c := &Client{
		Config:                RuntimeConfig{AppKey: "ak", AppSecret: "secret", APIBaseURL: "https://openapi-fxg.jinritemai.com"},
		Now:                   func() time.Time { return now },
		AccessToken:           "old-access",
		RefreshTokenValue:     "old-refresh",
		AccessTokenExpiresAt:  ptrTime(now.Add(-time.Minute)),
		RefreshTokenExpiresAt: ptrTime(now.Add(time.Hour)),
		PersistRefreshedToken: func(ctx context.Context, tok *TokenBundle) error {
			persisted = tok
			return nil
		},
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/token/refresh" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"code":10000,"data":{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"refresh_expires_in":7200,"shop_id":"shop-1","shop_name":"Demo"}}`)),
			}, nil
		}),
	}
	got, err := c.EnsureFreshAccess(context.Background())
	if err != nil {
		t.Fatalf("EnsureFreshAccess() error = %v", err)
	}
	if got != "new-access" || persisted == nil || persisted.RefreshToken != "new-refresh" {
		t.Fatalf("refresh was not persisted: got=%s persisted=%+v", got, persisted)
	}
}

func TestEnsureFreshAccessMarksExpiredWhenRefreshExpired(t *testing.T) {
	now := time.Date(2026, 6, 6, 1, 2, 3, 0, time.UTC)
	var marked string
	c := &Client{
		Now:                   func() time.Time { return now },
		RefreshTokenValue:     "old-refresh",
		RefreshTokenExpiresAt: ptrTime(now.Add(-time.Minute)),
		MarkAuthStatus: func(ctx context.Context, status string) error {
			marked = status
			return nil
		},
	}
	if _, err := c.EnsureFreshAccess(context.Background()); err == nil {
		t.Fatalf("expected expired refresh token error")
	}
	if marked != "expired" {
		t.Fatalf("expected expired mark, got %q", marked)
	}
}

func TestGetShopInfoUsesTokenRefreshResponse(t *testing.T) {
	c := &Client{
		Config:            RuntimeConfig{AppKey: "ak", AppSecret: "secret", APIBaseURL: "https://openapi-fxg.jinritemai.com"},
		RefreshTokenValue: "refresh",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"code":10000,"data":{"access_token":"access","refresh_token":"refresh2","expires_in":3600,"shop_id":"shop-1","shop_name":"Demo Shop","shop_status":"normal","scope":"product,shop"}}`)),
			}, nil
		}),
	}
	info, err := c.GetShopInfo(context.Background(), "shop-1")
	if err != nil {
		t.Fatalf("GetShopInfo() error = %v", err)
	}
	if info.PlatformShopID != "shop-1" || info.ShopName != "Demo Shop" || len(info.AuthorizedScopes) != 2 {
		t.Fatalf("unexpected shop info: %+v", info)
	}
}

func TestSanitizeErrorTextMasksSecrets(t *testing.T) {
	for _, raw := range []string{"app_secret=secret", "access_token=abc", "refresh_token=xyz"} {
		if got := SanitizeErrorText(raw); strings.Contains(strings.ToLower(got), "secret") || strings.Contains(strings.ToLower(got), "token") {
			t.Fatalf("sensitive text not sanitized: %q -> %q", raw, got)
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
