package shop

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformamazon "github.com/trademind-ai/trademind/backend/internal/providers/platform/amazon"
)

// AmazonShopsBridge satisfies platform/amazon persistence hooks.
func (s *Service) AmazonShopsBridge() platformamazon.ShopsBridge {
	return amazonBridge{svc: s}
}

type amazonBridge struct {
	svc *Service
}

func (b amazonBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	return b.svc.persistOAuthTokenRefreshForPlatform(ctx, shopID, access, refresh, accessExp, refreshExp, "shop.oauth.amazon.refresh")
}

func (b amazonBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	if _, ok := platformp.OAuthExpectedTokenVersion(ctx); ok {
		return b.svc.setAuthStatusVersionedForPlatform(ctx, shopID, status, "shop.oauth.amazon.status")
	}
	return b.svc.setAuthStatusCtx(ctx, shopID, status)
}

func (b amazonBridge) AmazonGlobalSettings(ctx context.Context) (map[string]string, error) {
	return b.svc.amazonGlobalSettingsPlain(ctx)
}

func (b amazonBridge) AmazonPublishSettings(ctx context.Context) (map[string]string, error) {
	return b.svc.amazonPublishSettingsPlain(ctx)
}

func (s *Service) amazonGlobalSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_amazon")
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

func (s *Service) amazonPublishSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_publish_amazon")
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
