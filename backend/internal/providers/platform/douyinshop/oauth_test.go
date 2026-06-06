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
	if _, err := c.RefreshToken(context.Background(), "bad"); err == nil {
		t.Fatalf("expected refresh failure")
	}
}
