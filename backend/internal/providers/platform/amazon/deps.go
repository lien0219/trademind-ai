package amazon

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ShopsBridge abstracts settings + token persistence (wired from api router).
type ShopsBridge interface {
	PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error
	SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error
	AmazonGlobalSettings(ctx context.Context) (map[string]string, error)
}

var bridges ShopsBridge

// BindShops sets callbacks (call before RegisterProvider).
func BindShops(b ShopsBridge) {
	bridges = b
}
