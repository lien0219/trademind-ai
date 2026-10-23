package shop

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
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

func TestDouyinOAuthStateBindsTenantAndIsSingleUse(t *testing.T) {
	db := newDouyinShopTestDB(t)
	if err := db.AutoMigrate(&admin.AdminUser{}, &Shop{}, &DouyinOAuthState{}); err != nil {
		t.Fatalf("migrate oauth state: %v", err)
	}
	svc := &Service{DB: db}
	userID := uuid.New()
	shopID := uuid.New()
	actor := admin.AdminUser{TenantID: 41, Username: admin.NewInternalUsername(), Email: "oauth-state@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 3}
	actor.ID = userID
	if err := db.Create(&actor).Error; err != nil {
		t.Fatalf("create OAuth actor: %v", err)
	}
	shop := Shop{TenantID: 41, Platform: douyinPlatform, ShopName: "OAuth fixture", Status: StatusActive, AuthStatus: AuthUnauthorized}
	shop.ID = shopID
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create OAuth shop: %v", err)
	}
	if err := svc.persistDouyinOAuthState(context.Background(), "state-tenant", 41, &userID, &shopID, "https://example.test/callback"); err != nil {
		t.Fatalf("persistDouyinOAuthState() error = %v", err)
	}
	var row DouyinOAuthState
	if err := db.Where("state_hash = ?", hashDouyinOAuthState("state-tenant")).First(&row).Error; err != nil {
		t.Fatalf("load state: %v", err)
	}
	if row.TenantID != 41 || row.TokenVersion != 3 || row.UserID == nil || *row.UserID != userID || row.ShopBindingContext != shopID.String() {
		t.Fatalf("state binding mismatch: %+v", row)
	}
	consumed, err := svc.consumeDouyinOAuthState(context.Background(), "state-tenant")
	if err != nil {
		t.Fatalf("consumeDouyinOAuthState() error = %v", err)
	}
	if consumed == nil || consumed.TenantID != 41 || consumed.ConsumedAt == nil {
		t.Fatalf("unexpected consumed state: %+v", consumed)
	}
	if _, err := svc.consumeDouyinOAuthState(context.Background(), "state-tenant"); err == nil {
		t.Fatal("expected replayed oauth state to be rejected")
	}
	if err := svc.persistDouyinOAuthState(context.Background(), "state-revoked", 41, &userID, nil, "https://example.test/callback"); err != nil {
		t.Fatalf("persist second state: %v", err)
	}
	if err := db.Model(&admin.AdminUser{}).Where("id = ?", userID).UpdateColumn("token_version", 4).Error; err != nil {
		t.Fatalf("revoke actor: %v", err)
	}
	if _, err := svc.consumeDouyinOAuthState(context.Background(), "state-revoked"); err == nil {
		t.Fatal("expected state created before token revocation to be rejected")
	}
}

func TestDouyinOAuthStateCacheMissDoesNotInvalidateDurableState(t *testing.T) {
	db := newDouyinShopTestDB(t)
	if err := db.AutoMigrate(&admin.AdminUser{}, &Shop{}, &DouyinOAuthState{}); err != nil {
		t.Fatalf("migrate oauth state: %v", err)
	}
	svc := &Service{DB: db}
	userID, shopID := uuid.New(), uuid.New()
	actor := admin.AdminUser{TenantID: 41, Username: admin.NewInternalUsername(), Email: "oauth-cache-miss@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 3}
	actor.ID = userID
	if err := db.Create(&actor).Error; err != nil {
		t.Fatalf("create actor: %v", err)
	}
	shop := Shop{TenantID: 41, Platform: douyinPlatform, ShopName: "OAuth fixture", Status: StatusActive, AuthStatus: AuthUnauthorized}
	shop.ID = shopID
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := svc.persistDouyinOAuthState(context.Background(), "state-cache-miss", 41, &userID, &shopID, "https://example.test/callback"); err != nil {
		t.Fatalf("persist state: %v", err)
	}
	state, err := svc.consumeDouyinOAuthState(context.Background(), "state-cache-miss")
	if err != nil {
		t.Fatalf("consume durable state after Redis cache loss: %v", err)
	}
	if err := validateDouyinOAuthStateCache("", state); err != nil {
		t.Fatalf("a missing cache entry must use the valid DB state: %v", err)
	}
	wrongCache, err := encodeDouyinStatePayload(douyinOAuthStatePayload{Platform: "douyin_shop", TenantID: 99, AdminID: uuid.NewString(), ShopID: uuid.NewString()})
	if err != nil {
		t.Fatalf("encode mismatched state: %v", err)
	}
	if err := validateDouyinOAuthStateCache(wrongCache, state); err == nil {
		t.Fatal("an existing mismatched cache entry must be rejected")
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
	if err := svc.persistDouyinShopInfo(oauthBridgeTenantContext(0), shopID, &platformdouyin.ShopInfo{
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
	if err := svc.markDouyinShopInfoFailed(oauthBridgeTenantContext(0), shop.ID, DouyinShopInfoFailed, "access_token=secret", AuthNeedCheck); err != nil {
		t.Fatalf("markDouyinShopInfoFailed() error = %v", err)
	}
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

func TestDouyinClientForShopReturnsAuthStatusPersistenceFailure(t *testing.T) {
	db := newDouyinShopTestDB(t)
	shop := Shop{
		Platform:   "douyin_shop",
		ShopName:   "Demo",
		Status:     StatusActive,
		AuthStatus: AuthAuthorized,
	}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	if err := db.Create(&ShopAuthToken{ShopID: shop.ID, Platform: "douyin_shop", AuthType: "oauth2"}).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	if err := db.Exec("CREATE TRIGGER reject_douyin_auth_status BEFORE UPDATE OF auth_status ON shops BEGIN SELECT RAISE(FAIL, 'injected write failure'); END").Error; err != nil {
		t.Fatalf("create status failure trigger: %v", err)
	}

	svc := &Service{DB: db}
	client, _, _, err := svc.douyinClientForShop(nil, oauthBridgeTenantContext(0), shop.ID, nil)
	if err == nil || !strings.Contains(err.Error(), "status could not be persisted") {
		t.Fatalf("expected status persistence failure, got client=%v err=%v", client, err)
	}
	if client != nil {
		t.Fatal("expected no client when expired auth status cannot be persisted")
	}
	var stored Shop
	if err := db.First(&stored, "id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if stored.AuthStatus != AuthAuthorized {
		t.Fatalf("status changed despite injected write failure: %s", stored.AuthStatus)
	}
}

func TestDouyinOAuthRevokeRollsBackWhenAuditIsUnavailable(t *testing.T) {
	db := newDouyinShopTestDB(t)
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}); err != nil {
		t.Fatalf("migrate admin models: %v", err)
	}
	actor := admin.AdminUser{
		TenantID: 41, Username: admin.NewInternalUsername(), Email: "revoke-audit@example.test",
		PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1,
	}
	if err := db.Create(&actor).Error; err != nil {
		t.Fatalf("create actor: %v", err)
	}
	shop := Shop{
		TenantID:   actor.TenantID,
		Platform:   "douyin_shop",
		ShopName:   "Demo",
		Status:     StatusActive,
		AuthStatus: AuthAuthorized,
	}
	if err := db.Create(&shop).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	token := ShopAuthToken{
		ShopID: shop.ID, Platform: "douyin_shop", AuthType: "oauth2",
		AccessTokenEnc: "encrypted-access", RefreshTokenEnc: "encrypted-refresh",
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	svc := &Service{DB: db}
	if _, err := svc.DouyinOAuthRevoke(oauthStateContext(actor), shop.ID, &actor.ID); err == nil {
		t.Fatal("expected revoke to fail when transactional audit is unavailable")
	}
	var storedShop Shop
	if err := db.First(&storedShop, "id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload shop: %v", err)
	}
	if storedShop.AuthStatus != AuthAuthorized {
		t.Fatalf("shop status changed despite audit failure: %s", storedShop.AuthStatus)
	}
	var storedToken ShopAuthToken
	if err := db.First(&storedToken, "shop_id = ?", shop.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}
	if storedToken.AccessTokenEnc != token.AccessTokenEnc || storedToken.RefreshTokenEnc != token.RefreshTokenEnc {
		t.Fatal("credentials changed despite audit failure")
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
