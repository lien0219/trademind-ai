package shopee

import (
	"context"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// PublishImageFetcher loads listing image bytes for Shopee Media Space upload (storage + public HTTP).
type PublishImageFetcher interface {
	FetchProductImageBytes(ctx context.Context, img platformp.PlatformProductImage) ([]byte, string, error)
}

var publishImages PublishImageFetcher

// BindPublishImages wires optional image loading via Storage Provider (nil disables uploads).
func BindPublishImages(p PublishImageFetcher) {
	publishImages = p
}

// ShopsBridge abstracts settings + token persistence (wired from api router).
type ShopsBridge interface {
	PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error
	SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error
	ShopeeGlobalSettings(ctx context.Context) (map[string]string, error)
	// ShopeePublishSettings reads decrypted plaintext for group platform_publish_shopee (warehouse_id, etc.).
	ShopeePublishSettings(ctx context.Context) (map[string]string, error)
}

var bridges ShopsBridge

// BindShops sets callbacks (call before RegisterProvider).
func BindShops(b ShopsBridge) {
	bridges = b
}
