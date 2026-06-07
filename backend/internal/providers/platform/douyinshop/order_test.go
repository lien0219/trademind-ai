package douyinshop

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type stubDouyinBridge struct {
	settings map[string]string
	refresh  func(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error
}

func (s stubDouyinBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	if s.refresh != nil {
		return s.refresh(ctx, shopID, access, refresh, accessExp, refreshExp)
	}
	return nil
}

func (s stubDouyinBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	return nil
}

func (s stubDouyinBridge) DouyinGlobalSettings(ctx context.Context) (map[string]string, error) {
	return s.settings, nil
}

func testRuntimeConfig() RuntimeConfig {
	return RuntimeConfig{
		AppKey:           "ak",
		AppSecret:        "secret",
		RedirectURI:      "https://example.com/callback",
		ServiceID:        "svc",
		APIBaseURL:       "https://openapi-fxg.jinritemai.com",
		AuthBaseURL:      "https://fuwu.jinritemai.com",
		Environment:      "production",
		HTTPTimeout:      30 * time.Second,
		OrderSyncEnabled: true,
	}
}

func testAuth() platformp.TestConnectionRequest {
	exp := time.Now().UTC().Add(-time.Hour)
	return platformp.TestConnectionRequest{
		AccessToken:          "old-access",
		RefreshToken:         "refresh-token",
		AccessTokenExpiresAt: &exp,
	}
}

func TestAssertShopUnauthorizedWithoutTokens(t *testing.T) {
	err := assertShopAuthorized(platformp.TestConnectionRequest{})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinStoreNotAuthorized {
		t.Fatalf("expected DOUYIN_STORE_NOT_AUTHORIZED, got %v", err)
	}
}

func TestOrderSyncDisabledInSettings(t *testing.T) {
	cfg := testRuntimeConfig()
	cfg.OrderSyncEnabled = false
	if err := validateOrderSyncConfig(cfg); err == nil {
		t.Fatal("expected disabled error")
	}
}

func TestMapOrderStatusAndMoney(t *testing.T) {
	if MapOrderStatus("105") != "paid" {
		t.Fatalf("unexpected paid map: %s", MapOrderStatus("105"))
	}
	if MapOrderStatus("3") != "shipped" {
		t.Fatalf("unexpected shipped map")
	}
	if MapOrderStatus("99999") != "unknown" {
		t.Fatalf("expected unknown status")
	}
	if parseMoneyCent("12345") != 123.45 {
		t.Fatalf("money parse failed")
	}
}

func TestMapDouyinOrderSanitizesBuyerAndItems(t *testing.T) {
	raw := map[string]any{
		"order_id":              "O123",
		"order_status":          "105",
		"pay_amount":            "10000",
		"create_time":           "1700000000",
		"user_nick_name":        "13800138000",
		"encrypt_post_receiver": "secret-name",
		"encrypt_post_tel":      "secret-tel",
		"sku_order_list": []any{
			map[string]any{
				"order_id":      "SKU1",
				"product_id":    "P1",
				"sku_id":        "SK1",
				"product_name":  "测试商品",
				"code":          "LOCAL-SKU",
				"item_num":      "2",
				"origin_amount": "5000",
			},
		},
	}
	po := mapDouyinOrder(raw)
	if po.ExternalOrderID != "O123" {
		t.Fatalf("order id mismatch")
	}
	if po.TotalAmount != 100 {
		t.Fatalf("expected 100 yuan, got %v", po.TotalAmount)
	}
	if strings.Contains(po.CustomerName, "13800138000") {
		t.Fatalf("customer name should be masked")
	}
	if len(po.Items) != 1 || po.Items[0].ExternalSKUID != "SK1" {
		t.Fatalf("sku mapping failed: %+v", po.Items)
	}
	if po.Items[0].UnitPrice != 50 {
		t.Fatalf("unit price expected 50, got %v", po.Items[0].UnitPrice)
	}
	rawJSON := po.RawData["platformStatus"]
	if rawJSON == nil {
		t.Fatalf("missing raw summary")
	}
}

func TestSyncOrdersPageListFailure(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	client := &Client{
		Config:      testRuntimeConfig(),
		Now:         func() time.Time { return now },
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":10001,"msg":"permission denied","data":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	_, _, _, _, err := SyncOrdersPage(context.Background(), client, "", 20, nil, nil)
	if err == nil {
		t.Fatal("expected list error")
	}
	var de *Error
	if !AsError(err, &de) {
		t.Fatalf("expected douyin error, got %v", err)
	}
	if de.Code != CodeDouyinOrderPermissionDenied && de.Code != CodeDouyinOrderListFailed {
		t.Fatalf("unexpected code %s", de.Code)
	}
}

func TestSyncOrdersPageParseFailure(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	client := &Client{
		Config:      testRuntimeConfig(),
		Now:         func() time.Time { return now },
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":10000,"msg":"success","data":{"page":0,"size":20,"total":1,"shop_order_list":[{"order_status":"105"}]}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	_, _, _, _, err := SyncOrdersPage(context.Background(), client, "", 20, nil, nil)
	if err == nil {
		t.Fatal("expected parse failure for missing order_id")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinOrderParseFailed {
		t.Fatalf("expected DOUYIN_ORDER_PARSE_FAILED, got %v", err)
	}
}

func TestClientRefreshThenSyncOrders(t *testing.T) {
	bridges = stubDouyinBridge{
		settings: map[string]string{
			"app_key":            "ak",
			"app_secret":         "secret",
			"redirect_uri":       "https://example.com/cb",
			"service_id":         "svc",
			"timeout_sec":        "30",
			"order_sync_enabled": "true",
		},
	}
	defer func() { bridges = nil }()

	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	refreshed := false
	shopID := uuid.New()
	expired := now.Add(-2 * time.Hour)
	auth := platformp.TestConnectionRequest{
		AppKey:               "ak",
		AppSecret:            "secret",
		AccessToken:          "old-access",
		RefreshToken:         "refresh-token",
		AccessTokenExpiresAt: &expired,
		RefreshTokenExpiresAt: func() *time.Time {
			t := now.Add(24 * time.Hour)
			return &t
		}(),
		Extra: map[string]string{
			"redirect_uri":       "https://example.com/cb",
			"service_id":         "svc",
			"timeout_sec":        "30",
			"order_sync_enabled": "true",
		},
	}

	client, _, err := newClientFromAuth(context.Background(), shopID, auth)
	if err != nil {
		t.Fatalf("newClientFromAuth failed: %v", err)
	}
	client.Now = func() time.Time { return now }
	client.HTTP = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/token/refresh") {
			refreshed = true
			body := `{"code":10000,"msg":"success","data":{"access_token":"new-access","refresh_token":"refresh-token","expires_in":86400}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}
		body := `{"code":10000,"msg":"success","data":{"page":0,"size":20,"total":0,"shop_order_list":[]}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	if _, err := client.EnsureFreshAccess(context.Background()); err != nil {
		t.Fatalf("EnsureFreshAccess failed: %v", err)
	}
	if !refreshed {
		t.Fatalf("expected token refresh call")
	}
	_, _, _, _, err = SyncOrdersPage(context.Background(), client, "", 20, nil, nil)
	if err != nil {
		t.Fatalf("sync after refresh failed: %v", err)
	}
}

func TestProviderSyncOrdersUnauthorized(t *testing.T) {
	p := NewProvider()
	_, err := p.SyncOrders(context.Background(), platformp.SyncOrdersRequest{
		ShopID: uuid.New(),
		Auth:   platformp.TestConnectionRequest{},
	})
	if err == nil {
		t.Fatal("expected unauthorized")
	}
}

func TestSanitizeErrorTextHidesTokens(t *testing.T) {
	msg := SanitizeErrorText("access_token=abc123 refresh_token=xyz")
	if strings.Contains(strings.ToLower(msg), "abc123") {
		t.Fatalf("token leaked in sanitized message: %s", msg)
	}
}

func TestMaskCustomerName(t *testing.T) {
	if maskCustomerName("13800138000") == "13800138000" {
		t.Fatal("phone-like nickname should be masked")
	}
}

func TestMapDouyinOrderDuplicateExternalIDStable(t *testing.T) {
	raw := map[string]any{
		"order_id":     "DUP1",
		"order_status": "105",
		"pay_amount":   "100",
	}
	a := mapDouyinOrder(raw)
	b := mapDouyinOrder(raw)
	if a.ExternalOrderID != b.ExternalOrderID {
		t.Fatal("external id should be stable")
	}
}
