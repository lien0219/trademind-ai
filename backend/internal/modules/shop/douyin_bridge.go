package shop

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// DouyinShopsBridge satisfies platform/douyinshop persistence hooks.
func (s *Service) DouyinShopsBridge() platformdouyin.ShopsBridge {
	return douyinBridge{svc: s}
}

type douyinBridge struct {
	svc *Service
}

func (b douyinBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	return b.svc.persistOAuthTokenRefreshForPlatform(ctx, shopID, access, refresh, accessExp, refreshExp, "shop.oauth.douyin.refresh")
}

func (b douyinBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	if _, ok := platformp.OAuthExpectedTokenVersion(ctx); ok {
		return b.svc.setAuthStatusVersionedForPlatform(ctx, shopID, status, "shop.oauth.douyin.status")
	}
	return b.svc.setAuthStatusCtx(ctx, shopID, status)
}

func (b douyinBridge) DouyinGlobalSettings(ctx context.Context) (map[string]string, error) {
	cfg, err := b.svc.douyinGlobalConfig(ctx)
	if err != nil {
		return nil, err
	}
	m := map[string]string{
		"app_key":                        cfg.AppKey,
		"app_secret":                     cfg.AppSecret,
		"service_id":                     cfg.ServiceID,
		"redirect_uri":                   cfg.RedirectURI,
		"environment":                    cfg.Environment,
		"auth_base_url":                  cfg.AuthBaseURL,
		"api_base_url":                   cfg.APIBaseURL,
		"real_api_enabled":               boolStr(cfg.RealAPIEnabled),
		"order_sync_enabled":             boolStr(cfg.OrderSyncEnabled),
		"inventory_sync_enabled":         boolStr(cfg.InventoryEnabled),
		"product_publish_enabled":        boolStr(cfg.ProductDraftEnabled),
		"platform_runtime_status":        cfg.RuntimeStatus,
		"platform_runtime_status_reason": cfg.RuntimeStatusReason,
		"timeout_sec":                    strconv.Itoa(int(cfg.HTTPTimeout.Seconds())),
	}
	out := map[string]string{}
	for k, v := range m {
		out[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	return out, nil
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
