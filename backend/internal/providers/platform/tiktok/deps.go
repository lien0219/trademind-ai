package tiktok

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ShopsBridge abstracts shop persistence callbacks (wired from router; avoids import cycles).
type ShopsBridge interface {
	PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error
	SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error
	TikTokGlobalSettings(ctx context.Context) (map[string]string, error)
}

var bridges ShopsBridge

// BindShops sets token/auth persistence callbacks (called from api.Register after shops.Service exists).
func BindShops(b ShopsBridge) {
	bridges = b
}
