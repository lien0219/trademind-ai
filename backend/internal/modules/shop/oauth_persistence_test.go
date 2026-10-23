package shop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
	"gorm.io/datatypes"
)

func TestTikTokOAuthCallbackPersistenceIsAtomic(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	redisConfig, ok, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	if !ok {
		t.Skip("TEST_REDIS_URL is not set; skipping isolated OAuth callback integration test")
	}
	options, err := goredis.ParseURL(redisConfig.URL)
	require.NoError(t, err)
	client := goredis.NewClient(options)
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	for _, failure := range []string{"metadata", "audit", "none"} {
		t.Run(failure, func(t *testing.T) {
			db, actor, c := newShopAuditFixture(t, true)
			platformtiktok.RegisterProvider()
			enc, err := encrypt.NewService("oauth-callback-test-only")
			require.NoError(t, err)
			service := &Service{DB: db, Encrypter: enc, OpLog: &operationlog.Service{DB: db}, Redis: &rdb.Client{Client: client}}
			platformtiktok.BindShops(service.TikTokShopsBridge())
			t.Cleanup(func() { platformtiktok.BindShops(nil) })
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v2/token/get" {
					_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"e2e-new-access","refresh_token":"e2e-new-refresh"}}`))
					return
				}
				_, _ = w.Write([]byte(`{"code":0,"data":{"shops":[{"cipher":"e2e-cipher","name":"after","id":"e2e-platform-shop"}]}}`))
			}))
			t.Cleanup(upstream.Close)
			config, err := json.Marshal(map[string]string{
				"auth_base_url": upstream.URL, "api_base_url": upstream.URL,
				"app_key": "e2e-client", "app_secret": "e2e-test-secret", "redirect_uri": upstream.URL + "/callback",
				"api_version": "202309", "timeout_sec": "5",
			})
			require.NoError(t, err)
			row := Shop{TenantID: actor.TenantID, Platform: "tiktok", ShopName: "before", Status: StatusActive, AuthStatus: AuthUnauthorized}
			require.NoError(t, db.Create(&row).Error)
			beforeAccess, err := enc.Encrypt([]byte("e2e-old-access"))
			require.NoError(t, err)
			token := ShopAuthToken{ShopID: row.ID, Platform: row.Platform, AuthType: "oauth2", AccessTokenEnc: beforeAccess, AuthConfig: datatypes.JSON(config)}
			require.NoError(t, db.Create(&token).Error)
			c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), &security.TenantContext{TenantID: actor.TenantID, UserID: actor.ID}))
			state := "test-" + uuid.NewString()
			require.NoError(t, service.saveOAuthState(c.Request.Context(), tiktokOAuthRedisPrefix, state, oauthStateBinding{TenantID: actor.TenantID, UserID: actor.ID, TokenVersion: actor.TokenVersion, ShopID: row.ID}))
			if failure == "metadata" {
				require.NoError(t, db.Exec("CREATE TRIGGER reject_oauth_metadata BEFORE UPDATE OF merchant_id ON shop_auth_tokens WHEN NEW.merchant_id = 'e2e-cipher' BEGIN SELECT RAISE(ABORT, 'metadata unavailable'); END").Error)
			} else if failure == "audit" {
				require.NoError(t, db.Exec("CREATE TRIGGER reject_oauth_audit BEFORE INSERT ON operation_logs WHEN NEW.action = 'shop.oauth.tiktok.success' BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END").Error)
			}
			detail, err := service.TikTokOAuthCallback(c, row.ID, TikTokOAuthCallbackBody{Code: "e2e-code", State: state}, &actor.ID)
			var stored ShopAuthToken
			require.NoError(t, db.First(&stored, "shop_id = ?", row.ID).Error)
			var savedShop Shop
			require.NoError(t, db.First(&savedShop, "id = ?", row.ID).Error)
			if failure != "none" {
				require.Error(t, err)
				require.Nil(t, detail)
				require.Equal(t, beforeAccess, stored.AccessTokenEnc, "callback failure must preserve previous credentials")
				require.Empty(t, stored.MerchantID)
				require.Equal(t, "before", savedShop.ShopName)
				require.Equal(t, AuthUnauthorized, savedShop.AuthStatus)
				var successes int64
				require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("status = ?", "success").Count(&successes).Error)
				require.Zero(t, successes, "rolled-back authorization must not have a success audit")
			} else {
				require.NoError(t, err)
				require.NotNil(t, detail)
				require.Equal(t, "e2e-cipher", stored.MerchantID)
				require.Equal(t, "after", savedShop.ShopName)
				var rows []operationlog.OperationLog
				require.NoError(t, db.Find(&rows).Error)
				require.Len(t, rows, 2)
				for _, audit := range rows {
					require.NotContains(t, audit.Message, "e2e-new-access")
					require.NotContains(t, audit.Message, "e2e-new-refresh")
				}
			}
			_, replayErr := service.consumeOAuthState(context.Background(), tiktokOAuthRedisPrefix, state)
			require.Error(t, replayErr, "a consumed callback state must not be reusable")
		})
	}
}

func TestDouyinOAuthBundleRollsBackAllWritesWhenAuditFails(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "new shop"
		if existing {
			name = "existing shop"
		}
		t.Run(name, func(t *testing.T) {
			db, actor, c := newShopAuditFixture(t, false)
			enc, err := encrypt.NewService("oauth-persistence-test-only")
			require.NoError(t, err)
			service := &Service{DB: db, Encrypter: enc, OpLog: &operationlog.Service{DB: db}}
			setOAuthExpectedTokenVersion(c, int64(actor.TokenVersion))
			var shopID *uuid.UUID
			var before Shop
			var beforeToken ShopAuthToken
			if existing {
				before = Shop{
					TenantID: actor.TenantID, Platform: "douyin_shop", ShopName: "before", ShopCode: "before-code",
					ExternalShopID: "before-id", Currency: "USD", Status: StatusActive, AuthStatus: AuthUnauthorized,
				}
				require.NoError(t, db.Create(&before).Error)
				beforeToken = ShopAuthToken{
					ShopID: before.ID, Platform: before.Platform, AuthType: "oauth2", AppKey: "before-key",
					AccessTokenEnc: "before-access", RefreshTokenEnc: "before-refresh", RawData: datatypes.JSON(`{"provider":"before"}`),
				}
				require.NoError(t, db.Create(&beforeToken).Error)
				shopID = &before.ID
			}
			ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: actor.TenantID, UserID: actor.ID})
			detail, authErr := service.persistDouyinOAuthBundle(c, ctx, actor.TenantID, shopID, &actor.ID,
				platformdouyin.RuntimeConfig{AppKey: "test-client", AppSecret: "test-secret"},
				&platformdouyin.TokenBundle{AccessToken: "test-access", RefreshToken: "test-refresh", PlatformShopID: "new-id", ShopName: "after"})
			require.NotNil(t, authErr)
			require.Nil(t, detail)
			if existing {
				var stored Shop
				require.NoError(t, db.First(&stored, "id = ?", before.ID).Error)
				require.Equal(t, before.ShopName, stored.ShopName)
				require.Equal(t, before.ShopCode, stored.ShopCode)
				require.Equal(t, before.ExternalShopID, stored.ExternalShopID)
				require.Equal(t, before.Currency, stored.Currency)
				require.Equal(t, before.AuthStatus, stored.AuthStatus)
				var storedToken ShopAuthToken
				require.NoError(t, db.First(&storedToken, "shop_id = ?", before.ID).Error)
				require.Equal(t, beforeToken.AppKey, storedToken.AppKey)
				require.Equal(t, beforeToken.AccessTokenEnc, storedToken.AccessTokenEnc)
				require.Equal(t, beforeToken.RefreshTokenEnc, storedToken.RefreshTokenEnc)
				require.JSONEq(t, string(beforeToken.RawData), string(storedToken.RawData))
			} else {
				var count int64
				require.NoError(t, db.Model(&Shop{}).Count(&count).Error)
				require.Zero(t, count, "audit failure must also roll back OAuth shop creation")
				require.NoError(t, db.Model(&ShopAuthToken{}).Count(&count).Error)
				require.Zero(t, count)
			}
		})
	}
}

func TestOAuthCredentialPersistenceRevalidatesActorAfterStateValidation(t *testing.T) {
	for _, mutation := range []string{"token version", "disabled actor", "tenant changed", "readonly role", "disabled custom role", "custom permission revoked", "store grant revoked"} {
		t.Run(mutation, func(t *testing.T) {
			db, actor, c := newShopAuditFixture(t, true)
			row := Shop{TenantID: actor.TenantID, Platform: "mock", ShopName: "test shop", Status: StatusActive, AuthStatus: AuthUnauthorized}
			require.NoError(t, db.Create(&row).Error)
			token := ShopAuthToken{ShopID: row.ID, Platform: row.Platform, AuthType: "oauth2", AppKey: "before"}
			require.NoError(t, db.Create(&token).Error)
			customRole := mutation == "disabled custom role" || mutation == "custom permission revoked" || mutation == "store grant revoked"
			if customRole {
				require.NoError(t, db.AutoMigrate(&admin.RoleDefinition{}))
				actor.Role = "r_1234567890abcdef12345678"
				require.NoError(t, db.Model(&actor).Update("role", actor.Role).Error)
				require.NoError(t, db.Create(&admin.RoleDefinition{TenantID: actor.TenantID, Code: actor.Role, Name: "test authorizer", PermissionsJSON: `["store.view","store.operate"]`, Enabled: true}).Error)
				require.NoError(t, db.Create(&admin.UserStorePermission{UserID: actor.ID, StoreID: row.ID, PermissionScope: admin.StorePermScopeOperate}).Error)
			}
			service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
			binding := oauthStateBinding{TenantID: actor.TenantID, UserID: actor.ID, TokenVersion: actor.TokenVersion, ShopID: row.ID}
			require.NoError(t, service.validateOAuthStateBinding(c, binding, row.ID))
			switch mutation {
			case "token version":
				require.NoError(t, db.Model(&actor).Update("token_version", actor.TokenVersion+1).Error)
			case "disabled actor":
				require.NoError(t, db.Model(&actor).Update("status", "disabled").Error)
			case "tenant changed":
				require.NoError(t, db.Model(&actor).Update("tenant_id", actor.TenantID+1).Error)
			case "readonly role":
				require.NoError(t, db.Model(&actor).Update("role", adminperm.RoleReadonly).Error)
			case "disabled custom role":
				require.NoError(t, db.Model(&admin.RoleDefinition{}).Where("tenant_id = ? AND code = ?", actor.TenantID, actor.Role).Update("enabled", false).Error)
			case "custom permission revoked":
				require.NoError(t, db.Model(&admin.RoleDefinition{}).Where("tenant_id = ? AND code = ?", actor.TenantID, actor.Role).Update("permissions_json", `["store.view"]`).Error)
			case "store grant revoked":
				require.NoError(t, db.Where("user_id = ? AND store_id = ?", actor.ID, row.ID).Delete(&admin.UserStorePermission{}).Error)
			}
			_, err := service.UpdateAuth(c, row.ID, UpdateAuthBody{AppKey: "replacement"}, &actor.ID)
			require.Error(t, err, "revocation during token exchange must prevent credential writes")
			var stored ShopAuthToken
			require.NoError(t, db.First(&stored, "shop_id = ?", row.ID).Error)
			require.Equal(t, "before", stored.AppKey)
			var logs int64
			require.NoError(t, db.Model(&operationlog.OperationLog{}).Count(&logs).Error)
			require.Zero(t, logs)
		})
	}
}

func TestDouyinOAuthBundleRejectsRevokedActorBeforeCreatingShop(t *testing.T) {
	db, actor, c := newShopAuditFixture(t, true)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	_, err := adminperm.LoadPrincipal(c, db)
	require.NoError(t, err)
	setOAuthExpectedTokenVersion(c, int64(actor.TokenVersion))
	require.NoError(t, db.Model(&actor).Update("token_version", actor.TokenVersion+1).Error)
	detail, authErr := service.persistDouyinOAuthBundle(c, context.Background(), actor.TenantID, nil, &actor.ID,
		platformdouyin.RuntimeConfig{AppKey: "test-client"}, &platformdouyin.TokenBundle{PlatformShopID: "new-id", ShopName: "must not persist"})
	require.NotNil(t, authErr)
	require.Nil(t, detail)
	var count int64
	require.NoError(t, db.Model(&Shop{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestDouyinOAuthBundleRollsBackAuditWhenFinalRawDataWriteFails(t *testing.T) {
	db, actor, c := newShopAuditFixture(t, true)
	service := &Service{DB: db, OpLog: &operationlog.Service{DB: db}}
	setOAuthExpectedTokenVersion(c, int64(actor.TokenVersion))
	require.NoError(t, db.Exec("CREATE TRIGGER reject_oauth_raw_data BEFORE UPDATE OF raw_data ON shop_auth_tokens WHEN NEW.raw_data IS NOT NULL BEGIN SELECT RAISE(FAIL, 'injected raw data failure'); END").Error)
	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: actor.TenantID, UserID: actor.ID})
	detail, authErr := service.persistDouyinOAuthBundle(c, ctx, actor.TenantID, nil, &actor.ID,
		platformdouyin.RuntimeConfig{AppKey: "test-client"}, &platformdouyin.TokenBundle{PlatformShopID: "new-id", ShopName: "test shop"})
	require.NotNil(t, authErr)
	require.Nil(t, detail)
	for _, model := range []any{&Shop{}, &ShopAuthToken{}, &operationlog.OperationLog{}} {
		var count int64
		require.NoError(t, db.Model(model).Count(&count).Error)
		require.Zero(t, count)
	}
}
