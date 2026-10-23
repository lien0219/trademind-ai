package shop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func TestOAuthBridgeWritesRequireMatchingTenantContext(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Shop{}, &ShopAuthToken{}))

	enc, err := encrypt.NewService("oauth-bridge-test-key")
	require.NoError(t, err)
	service := &Service{DB: db, Encrypter: enc}
	shopRow := Shop{
		Base:       model.Base{ID: uuid.New()},
		TenantID:   41,
		Platform:   "tiktok",
		ShopName:   "tenant shop",
		Status:     StatusActive,
		AuthStatus: AuthUnauthorized,
	}
	require.NoError(t, db.Create(&shopRow).Error)
	tok := ShopAuthToken{
		Base:         model.Base{ID: uuid.New()},
		ShopID:       shopRow.ID,
		Platform:     shopRow.Platform,
		AuthType:     "oauth2",
		TokenVersion: 3,
	}
	require.NoError(t, db.Create(&tok).Error)

	t.Run("missing tenant context is rejected", func(t *testing.T) {
		err := service.setAuthStatusCtx(context.Background(), shopRow.ID, AuthAuthorized)
		require.ErrorIs(t, err, security.ErrTenantContextMissing)
		err = service.persistOAuthTokenRefresh(context.Background(), shopRow.ID, "new-access", "new-refresh", nil, nil)
		require.ErrorIs(t, err, security.ErrTenantContextMissing)
	})

	t.Run("cross tenant status and token refresh writes are rejected", func(t *testing.T) {
		ctx := oauthBridgeTenantContext(42)
		require.ErrorIs(t, service.setAuthStatusCtx(ctx, shopRow.ID, AuthAuthorized), gorm.ErrRecordNotFound)
		require.ErrorIs(t, service.persistOAuthTokenRefresh(ctx, shopRow.ID, "new-access", "new-refresh", nil, nil), gorm.ErrRecordNotFound)

		var storedShop Shop
		require.NoError(t, db.First(&storedShop, "id = ?", shopRow.ID).Error)
		require.Equal(t, AuthUnauthorized, storedShop.AuthStatus)
		var storedToken ShopAuthToken
		require.NoError(t, db.First(&storedToken, "shop_id = ?", shopRow.ID).Error)
		require.EqualValues(t, 3, storedToken.TokenVersion)
		require.Empty(t, storedToken.AccessTokenEnc)
		require.Empty(t, storedToken.RefreshTokenEnc)
	})

	t.Run("matching tenant updates status and persists encrypted token refresh", func(t *testing.T) {
		ctx := oauthBridgeTenantContext(41)
		require.NoError(t, service.setAuthStatusCtx(ctx, shopRow.ID, AuthAuthorized))
		expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
		refreshExpiresAt := expiresAt.Add(24 * time.Hour)
		require.NoError(t, service.persistOAuthTokenRefresh(ctx, shopRow.ID, "new-access", "new-refresh", &expiresAt, &refreshExpiresAt))

		var storedShop Shop
		require.NoError(t, db.First(&storedShop, "id = ?", shopRow.ID).Error)
		require.Equal(t, AuthAuthorized, storedShop.AuthStatus)
		var storedToken ShopAuthToken
		require.NoError(t, db.First(&storedToken, "shop_id = ?", shopRow.ID).Error)
		require.EqualValues(t, 4, storedToken.TokenVersion)
		access, err := enc.Decrypt(storedToken.AccessTokenEnc)
		require.NoError(t, err)
		require.Equal(t, "new-access", string(access))
		refresh, err := enc.Decrypt(storedToken.RefreshTokenEnc)
		require.NoError(t, err)
		require.Equal(t, "new-refresh", string(refresh))
		require.WithinDuration(t, expiresAt, *storedToken.ExpiresAt, time.Second)
		require.WithinDuration(t, refreshExpiresAt, *storedToken.RefreshExpiresAt, time.Second)
	})

	t.Run("unknown shops fail without reporting a successful write", func(t *testing.T) {
		missingID := uuid.New()
		err := service.setAuthStatusCtx(oauthBridgeTenantContext(41), missingID, AuthAuthorized)
		require.Error(t, err)
		require.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	})
}

func oauthBridgeTenantContext(tenantID int64) context.Context {
	authSource := security.AuthSourceWorker
	if tenantID == 0 {
		authSource = security.AuthSourceLegacyDevZero
	}
	return security.WithTenantContext(context.Background(), &security.TenantContext{
		TenantID:   tenantID,
		AuthSource: authSource,
	})
}
