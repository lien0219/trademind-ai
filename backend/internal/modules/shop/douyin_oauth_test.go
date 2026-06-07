package shop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/gorm"
)

func TestDouyinStatePayloadRoundTrip(t *testing.T) {
	raw, err := encodeDouyinStatePayload(douyinOAuthStatePayload{
		Platform: "douyin_shop",
		AdminID:  "admin-1",
		ShopID:   "shop-1",
		Created:  1,
	})
	if err != nil {
		t.Fatalf("encodeDouyinStatePayload() error = %v", err)
	}
	got, err := decodeDouyinStatePayload(raw)
	if err != nil {
		t.Fatalf("decodeDouyinStatePayload() error = %v", err)
	}
	if got.Platform != "douyin_shop" || got.AdminID != "admin-1" || got.ShopID != "shop-1" {
		t.Fatalf("unexpected state payload: %+v", got)
	}
}

func TestDouyinStatePayloadRejectsWrongPlatform(t *testing.T) {
	if _, err := decodeDouyinStatePayload(`{"platform":"tiktok"}`); err == nil {
		t.Fatalf("expected platform mismatch error")
	}
}

func TestDouyinFriendlyMessages(t *testing.T) {
	for _, code := range []string{
		DouyinAppConfigIncomplete,
		DouyinOAuthStateInvalid,
		DouyinTokenExchangeFailed,
		DouyinShopInfoFailed,
		DouyinAuthExpired,
	} {
		if douyinFriendlyMessage(code) == "" || douyinFriendlyMessage(code) == code {
			t.Fatalf("missing friendly message for %s", code)
		}
	}
}

func TestPersistDouyinShopInfoUpdatesShopAndToken(t *testing.T) {
	db := newDouyinShopTestDB(t)
	svc := &Service{DB: db}
	shopID := uuid.New()
	if err := db.Create(&Shop{
		Platform:   "douyin_shop",
		ShopName:   "Old Shop",
		Status:     StatusActive,
		AuthStatus: AuthNeedCheck,
		Currency:   "CNY",
	}).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	var shopRow Shop
	if err := db.First(&shopRow).Error; err != nil {
		t.Fatalf("load shop: %v", err)
	}
	shopID = shopRow.ID
	if err := db.Create(&ShopAuthToken{
		ShopID:   shopID,
		Platform: "douyin_shop",
		AuthType: "oauth2",
	}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	exp := time.Date(2026, 6, 6, 1, 2, 3, 0, time.UTC)
	if err := svc.persistDouyinShopInfo(context.Background(), shopID, &platformdouyin.ShopInfo{
		PlatformShopID:   "shop-1",
		ShopName:         "Demo Shop",
		ShopStatus:       "normal",
		AuthorityID:      "auth-1",
		ShopBizType:      "local",
		AuthorizedScopes: []any{"shop", "product"},
		ExpiresAt:        &exp,
		Raw:              map[string]any{"shop_id": "shop-1"},
	}); err != nil {
		t.Fatalf("persistDouyinShopInfo() error = %v", err)
	}
	if err := db.First(&shopRow, "id = ?", shopID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if shopRow.ShopName != "Demo Shop" || shopRow.ExternalShopID != "shop-1" || shopRow.AuthStatus != AuthAuthorized {
		t.Fatalf("unexpected shop row: %+v", shopRow)
	}
	var tok ShopAuthToken
	if err := db.First(&tok, "shop_id = ?", shopID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if tok.ExpiresAt == nil || !tok.ExpiresAt.Equal(exp) {
		t.Fatalf("unexpected expiry: %v", tok.ExpiresAt)
	}
	if strings.Contains(strings.ToLower(string(tok.RawData)), "token") || strings.Contains(strings.ToLower(string(tok.RawData)), "secret") {
		t.Fatalf("raw data leaked sensitive text: %s", string(tok.RawData))
	}
	var scopes []any
	if err := json.Unmarshal(tok.Scopes, &scopes); err != nil || len(scopes) != 2 {
		t.Fatalf("unexpected scopes: %s err=%v", string(tok.Scopes), err)
	}
}

func TestMarkDouyinShopInfoFailedIsSafe(t *testing.T) {
	db := newDouyinShopTestDB(t)
	svc := &Service{DB: db}
	shop := Shop{Platform: "douyin_shop", ShopName: "Demo", Status: StatusActive, AuthStatus: AuthAuthorized}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := db.Create(&ShopAuthToken{ShopID: shop.ID, Platform: "douyin_shop", AuthType: "oauth2"}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	svc.markDouyinShopInfoFailed(context.Background(), shop.ID, DouyinShopInfoFailed, "access_token=secret", AuthNeedCheck)
	var row Shop
	if err := db.First(&row, "id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if row.AuthStatus != AuthNeedCheck {
		t.Fatalf("expected need_check, got %s", row.AuthStatus)
	}
	var tok ShopAuthToken
	if err := db.First(&tok, "shop_id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	raw := strings.ToLower(string(tok.RawData))
	if strings.Contains(raw, "access_token") || strings.Contains(raw, "secret") {
		t.Fatalf("raw data leaked sensitive text: %s", string(tok.RawData))
	}
}

func newDouyinShopTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&Shop{}, &ShopAuthToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}
