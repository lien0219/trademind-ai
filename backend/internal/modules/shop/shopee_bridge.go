package shop

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	platformshopee "github.com/trademind-ai/trademind/backend/internal/providers/platform/shopee"
)

// ShopeeShopsBridge satisfies platform/shopee persistence hooks.
func (s *Service) ShopeeShopsBridge() platformshopee.ShopsBridge {
	return shopeeBridge{svc: s}
}

type shopeeBridge struct {
	svc *Service
}

func (b shopeeBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	return b.svc.persistOAuthTokenRefresh(ctx, shopID, access, refresh, accessExp, refreshExp)
}

func (b shopeeBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	return b.svc.setAuthStatusCtx(ctx, shopID, status)
}

func (b shopeeBridge) ShopeeGlobalSettings(ctx context.Context) (map[string]string, error) {
	return b.svc.shopeeGlobalSettingsPlain(ctx)
}

func (b shopeeBridge) ShopeePublishSettings(ctx context.Context) (map[string]string, error) {
	return b.svc.shopeePublishSettingsPlain(ctx)
}

func (s *Service) shopeeGlobalSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_shopee")
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for k, v := range m {
		out[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	return out, nil
}

func (s *Service) shopeePublishSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_publish_shopee")
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for k, v := range m {
		out[strings.TrimSpace(strings.ToLower(k))] = strings.TrimSpace(v)
	}
	return out, nil
}
