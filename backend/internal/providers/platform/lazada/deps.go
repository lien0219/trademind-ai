package lazada

import (
	"context"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// ShopsBridge abstracts settings + token persistence (wired from api router).
type ShopsBridge interface {
	PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error
	SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error
	LazadaGlobalSettings(ctx context.Context) (map[string]string, error)
	// LazadaPublishSettings reads decrypted plaintext for group platform_publish_lazada (warehouse_id optional, etc.).
	LazadaPublishSettings(ctx context.Context) (map[string]string, error)
}

// PublishImageFetcher loads listing image bytes (storage + public HTTP), same contract as Shopee.
type PublishImageFetcher interface {
	FetchProductImageBytes(ctx context.Context, img platformp.PlatformProductImage) ([]byte, string, error)
}

var bridges ShopsBridge
var publishImages PublishImageFetcher

// BindShops sets callbacks (call before RegisterProvider).
func BindShops(b ShopsBridge) {
	bridges = b
}

// BindPublishImages wires listing image fetcher for CreateProduct image URLs.
func BindPublishImages(p PublishImageFetcher) {
	publishImages = p
}
