package douyinshop

import (
	"context"
	"fmt"
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

func orderListResponse(page, size, total int, ids ...string) string {
	orders := make([]string, 0, len(ids))
	for _, id := range ids {
		orders = append(orders, fmt.Sprintf(`{"order_id":"%s","order_status":"105","pay_amount":"100"}`, id))
	}
	list := strings.Join(orders, ",")
	return fmt.Sprintf(`{"code":10000,"msg":"success","data":{"page":%d,"size":%d,"total":%d,"shop_order_list":[%s]}}`, page, size, total, list)
}

func TestSyncOrdersPaginatedMultiPageSuccess(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	pageCalls := 0
	client := &Client{
		Config:      testRuntimeConfig(),
		Now:         func() time.Time { return now },
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			pageCalls++
			var body string
			switch pageCalls {
			case 1:
				body = orderListResponse(0, 20, 40, "O1", "O2")
			case 2:
				body = orderListResponse(1, 20, 40, "O3")
			default:
				body = orderListResponse(2, 20, 40)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	res, err := SyncOrdersPaginated(context.Background(), client, "", 20, 5, nil, nil, nil)
	if err != nil {
		t.Fatalf("SyncOrdersPaginated failed: %v", err)
	}
	if len(res.Orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(res.Orders))
	}
	if res.TotalPages != 2 || res.SuccessPages != 2 || res.FailedPages != 0 {
		t.Fatalf("unexpected page stats: %+v", res)
	}
	if res.TotalFetched != 3 {
		t.Fatalf("expected totalFetched=3, got %d", res.TotalFetched)
	}
	if pageCalls != 2 {
		t.Fatalf("expected 2 page calls, got %d", pageCalls)
	}
}

func TestSyncOrdersPaginatedPartialSuccess(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	pageCalls := 0
	client := &Client{
		Config:      testRuntimeConfig(),
		Now:         func() time.Time { return now },
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			pageCalls++
			var body string
			if pageCalls == 1 {
				body = orderListResponse(0, 20, 60, "O1")
			} else {
				body = `{"code":10001,"msg":"rate limited","data":{}}`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	res, err := SyncOrdersPaginated(context.Background(), client, "", 20, 2, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected partial result without top-level error, got %v", err)
	}
	if len(res.Orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(res.Orders))
	}
	if res.SuccessPages != 1 || res.FailedPages != 1 {
		t.Fatalf("expected 1 success + 1 failed page, got success=%d failed=%d", res.SuccessPages, res.FailedPages)
	}
	if len(res.PageErrors) != 1 || res.PageErrors[0].Page != 1 {
		t.Fatalf("unexpected page errors: %+v", res.PageErrors)
	}
}

func TestSyncOrdersPaginatedMaxPagesCap(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	pageCalls := 0
	client := &Client{
		Config:      testRuntimeConfig(),
		Now:         func() time.Time { return now },
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			pageCalls++
			page := pageCalls - 1
			body := orderListResponse(page, 10, 100, fmt.Sprintf("O%d", pageCalls))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	res, err := SyncOrdersPaginated(context.Background(), client, "", 10, 2, nil, nil, nil)
	if err != nil {
		t.Fatalf("SyncOrdersPaginated failed: %v", err)
	}
	if res.TotalPages != 2 || len(res.Orders) != 2 {
		t.Fatalf("expected 2 pages / 2 orders, got pages=%d orders=%d", res.TotalPages, len(res.Orders))
	}
	if pageCalls != 2 {
		t.Fatalf("expected maxPages=2 to cap calls, got %d", pageCalls)
	}
	if !res.HasMore {
		t.Fatalf("expected hasMore when capped before total")
	}
}

func TestResolveOrderSyncMaxPages(t *testing.T) {
	cfg := testRuntimeConfig()
	cfg.OrderSyncMaxPages = 7
	if got := ResolveOrderSyncMaxPages(0, cfg); got != 7 {
		t.Fatalf("expected config default 7, got %d", got)
	}
	if got := ResolveOrderSyncMaxPages(3, cfg); got != 3 {
		t.Fatalf("expected task override 3, got %d", got)
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

func TestSyncOrdersRetryPagesOnly(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	pagesHit := map[int]bool{}
	client := &Client{
		Config:      testRuntimeConfig(),
		Now:         func() time.Time { return now },
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := orderListResponse(1, 20, 60, "O-RETRY-1")
			pagesHit[1] = true
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	res, err := SyncOrdersPaginated(context.Background(), client, "", 20, 5, nil, nil, []int{1})
	if err != nil {
		t.Fatalf("retry pages sync failed: %v", err)
	}
	if !pagesHit[1] {
		t.Fatal("expected page 1 to be fetched")
	}
	if pagesHit[0] {
		t.Fatal("page 0 should not be fetched on targeted retry")
	}
	if len(res.Orders) != 1 || res.Orders[0].ExternalOrderID != "O-RETRY-1" {
		t.Fatalf("unexpected orders: %+v", res.Orders)
	}
	if res.FailedPages != 0 || len(res.PageErrors) != 0 {
		t.Fatalf("unexpected failures: %+v", res.PageErrors)
	}
}
