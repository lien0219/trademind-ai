package tiktok

import (
	"context"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// PublishImageFetcher loads listing image bytes for TikTok upload (storage + public HTTP).
type PublishImageFetcher interface {
	FetchProductImageBytes(ctx context.Context, img platformp.PlatformProductImage) ([]byte, string, error)
}

var publishImages PublishImageFetcher

// BindPublishImages wires optional image loading via Storage Provider (nil disables uploads).
func BindPublishImages(p PublishImageFetcher) {
	publishImages = p
}

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
