package shop

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TikTokShopsBridge satisfies platform/tiktok persistence hooks.
func (s *Service) TikTokShopsBridge() platformtiktok.ShopsBridge {
	return tikTokBridge{svc: s}
}

type tikTokBridge struct {
	svc *Service
}

func (b tikTokBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	return b.svc.persistOAuthTokenRefreshForPlatform(ctx, shopID, access, refresh, accessExp, refreshExp, "shop.oauth.tiktok.refresh")
}

func (b tikTokBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	if _, ok := platformp.OAuthExpectedTokenVersion(ctx); ok {
		return b.svc.setAuthStatusVersionedForPlatform(ctx, shopID, status, "shop.oauth.tiktok.status")
	}
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

func (b tikTokBridge) TikTokPublishSettings(ctx context.Context) (map[string]string, error) {
	m, err := b.svc.tiktokPublishSettingsPlain(ctx)
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

func (s *Service) tiktokPublishSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_publish_tiktok")
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
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	st := strings.TrimSpace(status)
	if st == "" {
		return nil
	}
	result := oauthPersistenceDB(ctx, s.DB).Model(&Shop{}).
		Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).
		Update("auth_status", st)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) updateShopOAuthFields(ctx context.Context, shopID uuid.UUID, updates map[string]any) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	result := oauthPersistenceDB(ctx, s.DB).Model(&Shop{}).
		Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) updateShopAuthTokenOAuthFields(ctx context.Context, shopID uuid.UUID, updates map[string]any) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	persist := func(tx *gorm.DB) error {
		var shopRow Shop
		if err := tx.Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
			return err
		}
		result := tx.Model(&ShopAuthToken{}).Where("shop_id = ?", shopRow.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}
	if tx, ok := oauthPersistenceTx(ctx); ok {
		return persist(tx.WithContext(ctx))
	}
	return s.DB.WithContext(ctx).Transaction(persist)
}

func (s *Service) persistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("shop: no db")
	}
	if s.Encrypter == nil {
		return fmt.Errorf("shop: encryption not configured")
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var shopRow Shop
		if err := tx.Where("id = ? AND tenant_id = ?", shopID, tenant.TenantID).First(&shopRow).Error; err != nil {
			return err
		}
		var tok ShopAuthToken
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("shop_id = ?", shopRow.ID).First(&tok).Error; err != nil {
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
		tok.TokenVersion++
		return tx.Save(&tok).Error
	})
}
