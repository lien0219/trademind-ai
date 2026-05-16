package shop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
)

// TikTokShopsBridge satisfies platform/tiktok persistence hooks.
func (s *Service) TikTokShopsBridge() platformtiktok.ShopsBridge {
	return tikTokBridge{svc: s}
}

type tikTokBridge struct {
	svc *Service
}

func (b tikTokBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	return b.svc.persistOAuthTokenRefresh(ctx, shopID, access, refresh, accessExp, refreshExp)
}

func (b tikTokBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	return b.svc.setAuthStatusCtx(ctx, shopID, status)
}

func (b tikTokBridge) TikTokGlobalSettings(ctx context.Context) (map[string]string, error) {
	m, err := b.svc.tiktokGlobalSettingsPlain(ctx)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func (s *Service) tiktokGlobalSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_tiktok")
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

func (s *Service) setAuthStatusCtx(ctx context.Context, shopID uuid.UUID, status string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	st := strings.TrimSpace(status)
	if st == "" {
		return nil
	}
	return s.DB.WithContext(ctx).Model(&Shop{}).Where("id = ?", shopID).Update("auth_status", st).Error
}

func (s *Service) persistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	if s.Encrypter == nil {
		return fmt.Errorf("shop: encryption not configured")
	}
	var tok ShopAuthToken
	if err := s.DB.WithContext(ctx).Where("shop_id = ?", shopID).First(&tok).Error; err != nil {
		return err
	}
	if strings.TrimSpace(access) != "" {
		ct, err := s.Encrypter.Encrypt([]byte(strings.TrimSpace(access)))
		if err != nil {
			return err
		}
		tok.AccessTokenEnc = ct
	}
	if strings.TrimSpace(refresh) != "" {
		ct, err := s.Encrypter.Encrypt([]byte(strings.TrimSpace(refresh)))
		if err != nil {
			return err
		}
		tok.RefreshTokenEnc = ct
	}
	tok.ExpiresAt = accessExp
	tok.RefreshExpiresAt = refreshExp
	return s.DB.WithContext(ctx).Save(&tok).Error
}
