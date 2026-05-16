package shop

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	platformlazada "github.com/trademind-ai/trademind/backend/internal/providers/platform/lazada"
)

// LazadaShopsBridge satisfies platform/lazada persistence hooks.
func (s *Service) LazadaShopsBridge() platformlazada.ShopsBridge {
	return lazadaBridge{svc: s}
}

type lazadaBridge struct {
	svc *Service
}

func (b lazadaBridge) PersistOAuthTokenRefresh(ctx context.Context, shopID uuid.UUID, access, refresh string, accessExp, refreshExp *time.Time) error {
	return b.svc.persistOAuthTokenRefresh(ctx, shopID, access, refresh, accessExp, refreshExp)
}

func (b lazadaBridge) SetShopAuthStatus(ctx context.Context, shopID uuid.UUID, status string) error {
	return b.svc.setAuthStatusCtx(ctx, shopID, status)
}

func (b lazadaBridge) LazadaGlobalSettings(ctx context.Context) (map[string]string, error) {
	return b.svc.lazadaGlobalSettingsPlain(ctx)
}

func (s *Service) lazadaGlobalSettingsPlain(ctx context.Context) (map[string]string, error) {
	if s == nil || s.Settings == nil || s.Settings.DB == nil {
		return map[string]string{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "platform_lazada")
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
