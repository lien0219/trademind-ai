package douyinshop

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func setAuthStatusMaybe(ctx context.Context, shopID uuid.UUID, status string) error {
	if bridges == nil || shopID == uuid.Nil {
		return nil
	}
	return bridges.SetShopAuthStatus(ctx, shopID, status)
}

func newClientFromAuth(ctx context.Context, shopID uuid.UUID, auth platformp.TestConnectionRequest) (*Client, RuntimeConfig, error) {
	cfg, err := ResolveRuntime(ctx, auth)
	if err != nil {
		return nil, RuntimeConfig{}, err
	}
	client := &Client{
		Config:                cfg,
		AccessToken:           strings.TrimSpace(auth.AccessToken),
		RefreshTokenValue:     strings.TrimSpace(auth.RefreshToken),
		AccessTokenExpiresAt:  auth.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: auth.RefreshTokenExpiresAt,
		PersistRefreshedToken: func(ctx context.Context, tok *TokenBundle) error {
			if tok == nil || bridges == nil || shopID == uuid.Nil {
				return nil
			}
			refresh := strings.TrimSpace(tok.RefreshToken)
			if refresh == "" {
				refresh = strings.TrimSpace(auth.RefreshToken)
			}
			return bridges.PersistOAuthTokenRefresh(ctx, shopID, tok.AccessToken, refresh, tok.AccessExpiresAt, tok.RefreshExpiresAt)
		},
		MarkAuthStatus: func(ctx context.Context, status string) error {
			return setAuthStatusMaybe(ctx, shopID, status)
		},
	}
	return client, cfg, nil
}

func ensureFreshClient(ctx context.Context, shopID uuid.UUID, auth platformp.TestConnectionRequest) (*Client, RuntimeConfig, error) {
	if strings.TrimSpace(auth.RefreshToken) == "" && strings.TrimSpace(auth.AccessToken) == "" {
		return nil, RuntimeConfig{}, NewError(CodeDouyinStoreNotAuthorized, "douyin store not authorized", "", "", "")
	}
	client, cfg, err := newClientFromAuth(ctx, shopID, auth)
	if err != nil {
		return nil, RuntimeConfig{}, err
	}
	client.ShopID = shopID.String()
	now := client.now()
	if client.accessFresh(now) {
		return client, cfg, nil
	}
	if !client.refreshUsable(now) {
		_ = setAuthStatusMaybe(ctx, shopID, "expired")
		return nil, RuntimeConfig{}, NewError(CodeDouyinAuthExpired, "douyin authorization expired", "", "", "")
	}
	if _, err := client.RefreshAccessToken(ctx); err != nil {
		return nil, RuntimeConfig{}, err
	}
	return client, cfg, nil
}

func orderSyncEnabled(cfg RuntimeConfig) bool {
	return cfg.OrderSyncEnabled
}

func defaultOrderWindow(start, end *time.Time) (*time.Time, *time.Time) {
	now := time.Now().UTC()
	st := start
	et := end
	if st == nil && et == nil {
		et = &now
		x := now.Add(-168 * time.Hour)
		st = &x
	}
	if st == nil {
		x := et.Add(-168 * time.Hour)
		st = &x
	}
	if et == nil {
		x := st.Add(168 * time.Hour)
		if x.After(now) {
			x = now
		}
		et = &x
	}
	return st, et
}
