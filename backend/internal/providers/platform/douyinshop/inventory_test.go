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

func testInventoryRuntimeConfig() RuntimeConfig {
	cfg := testRuntimeConfig()
	cfg.InventoryEnabled = true
	return cfg
}

func TestInventorySyncDisabledInSettings(t *testing.T) {
	cfg := testInventoryRuntimeConfig()
	cfg.InventoryEnabled = false
	err := validateInventorySyncConfig(cfg)
	if err == nil {
		t.Fatal("expected disabled error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinInventorySyncNotReady {
		t.Fatalf("expected DOUYIN_INVENTORY_SYNC_NOT_READY, got %v", err)
	}
}

func TestInventorySyncUnauthorizedStore(t *testing.T) {
	p := provider{}
	_, err := p.SyncInventory(context.Background(), platformp.SyncInventoryRequest{
		ShopID:            uuid.New(),
		ExternalProductID: "123",
		ExternalSKUID:     "456",
		Stock:             10,
	})
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinStoreNotAuthorized {
		t.Fatalf("expected DOUYIN_STORE_NOT_AUTHORIZED, got %v", err)
	}
}

func TestInventorySyncMissingProductBinding(t *testing.T) {
	p := provider{}
	_, err := p.SyncInventory(context.Background(), platformp.SyncInventoryRequest{
		ShopID: uuid.New(),
		Auth:   testAuth(),
		Stock:  5,
	})
	if err == nil {
		t.Fatal("expected product not bound error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinProductNotBound {
		t.Fatalf("expected DOUYIN_PRODUCT_NOT_BOUND, got %v", err)
	}
}

func TestInventorySyncMissingSkuBinding(t *testing.T) {
	p := provider{}
	_, err := p.SyncInventory(context.Background(), platformp.SyncInventoryRequest{
		ShopID:            uuid.New(),
		Auth:              testAuth(),
		ExternalProductID: "3539925204033339668",
		Stock:             5,
	})
	if err == nil {
		t.Fatal("expected sku not bound error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinSKUNotBound {
		t.Fatalf("expected DOUYIN_SKU_NOT_BOUND, got %v", err)
	}
}

func TestInventorySyncInvalidStock(t *testing.T) {
	p := provider{}
	_, err := p.SyncInventory(context.Background(), platformp.SyncInventoryRequest{
		ShopID:            uuid.New(),
		Auth:              testAuth(),
		ExternalProductID: "3539925204033339668",
		ExternalSKUID:     "1737398770243598",
		Stock:             -1,
	})
	if err == nil {
		t.Fatal("expected invalid stock error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinStockInvalid {
		t.Fatalf("expected DOUYIN_STOCK_INVALID, got %v", err)
	}
}

func TestClientSyncStockSuccess(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	client := &Client{
		Config:      testInventoryRuntimeConfig(),
		Now:         func() time.Time { return now },
		AccessToken: "fresh-access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.Contains(req.URL.Path, "sku/syncStock") {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			body := `{"code":10000,"msg":"success","data":{"product_id":"3539925204033339668","sku_id":"1737398770243598"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	var data map[string]any
	if err := client.Do(context.Background(), MethodSkuSyncStock, map[string]any{
		"product_id":  "3539925204033339668",
		"sku_id":      "1737398770243598",
		"stock_num":   42,
		"incremental": false,
	}, &data); err != nil {
		t.Fatalf("sync stock failed: %v", err)
	}
	if data["product_id"] == nil || data["sku_id"] == nil {
		t.Fatalf("unexpected response data: %+v", data)
	}
}

func TestInventorySyncPermissionDeniedNotRetryableClass(t *testing.T) {
	err := mapInventorySyncError(NewError(CodeDouyinPermissionDenied, "denied", "403", "forbidden", "rid"))
	var de *Error
	if !AsError(err, &de) {
		t.Fatal("expected douyin error")
	}
	if de.Code != CodeDouyinInventoryPermissionDenied {
		t.Fatalf("expected permission denied inventory code, got %s", de.Code)
	}
	if de.Retryable {
		t.Fatal("permission denied should not be retryable")
	}
}

func TestInventorySyncRateLimitedRetryable(t *testing.T) {
	err := mapInventorySyncError(NewError(CodeDouyinRateLimited, "limited", "429", "rate", "rid"))
	var de *Error
	if !AsError(err, &de) {
		t.Fatal("expected douyin error")
	}
	if de.Code != CodeDouyinInventoryRateLimited {
		t.Fatalf("expected rate limit inventory code, got %s", de.Code)
	}
	if !de.Retryable || !de.RateLimited {
		t.Fatal("rate limit should be retryable")
	}
}

func TestInventorySyncTokenRefreshPath(t *testing.T) {
	shopID := uuid.New()
	refreshCalled := false
	BindShops(stubDouyinBridge{
		settings: map[string]string{
			"app_key":                "ak",
			"app_secret":             "secret",
			"redirect_uri":           "https://example.com/callback",
			"service_id":             "svc",
			"timeout_sec":            "30",
			"inventory_sync_enabled": "true",
		},
		refresh: func(ctx context.Context, sid uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
			refreshCalled = true
			return nil
		},
	})

	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	call := 0
	client := &Client{
		Config:            testInventoryRuntimeConfig(),
		Now:               func() time.Time { return now },
		AccessToken:       "old-access",
		RefreshTokenValue: "refresh-token",
		AccessTokenExpiresAt: func() *time.Time {
			x := now.Add(-2 * time.Hour)
			return &x
		}(),
		PersistRefreshedToken: func(ctx context.Context, tok *TokenBundle) error {
			refreshCalled = true
			return nil
		},
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			call++
			if strings.Contains(req.URL.Path, "token/refresh") || strings.Contains(req.URL.RawQuery, "token.refresh") {
				body := `{"code":10000,"msg":"success","data":{"access_token":"new-access","refresh_token":"refresh-token","expires_in":86400,"refresh_token_expires_in":2592000}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}
			body := `{"code":10000,"msg":"success","data":{"product_id":"1","sku_id":"2"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}

	var data map[string]any
	if err := client.Do(context.Background(), MethodSkuSyncStock, map[string]any{
		"product_id": "1", "sku_id": "2", "stock_num": 3, "incremental": false,
	}, &data); err != nil {
		t.Fatalf("expected refresh then sync success, got %v", err)
	}
	if !refreshCalled {
		t.Fatal("expected token refresh persistence")
	}
	if call < 2 {
		t.Fatalf("expected refresh + sync calls, got %d", call)
	}
	_ = shopID
}

func TestInventorySyncLogsDoNotLeakSecrets(t *testing.T) {
	var logged SafeRequestLog
	client := &Client{
		Config:      testInventoryRuntimeConfig(),
		AccessToken: "secret-access-token-value",
		Logger: SafeLoggerFunc(func(ctx context.Context, entry SafeRequestLog) {
			logged = entry
		}),
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":10000,"msg":"success","data":{}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	_ = client.Do(context.Background(), MethodSkuSyncStock, map[string]any{
		"product_id": "1", "sku_id": "2", "stock_num": 1, "incremental": false,
	}, &map[string]any{})
	raw := logged.Method + logged.ErrorCode
	if strings.Contains(strings.ToLower(raw), "secret-access-token-value") {
		t.Fatal("token leaked into safe log")
	}
}
