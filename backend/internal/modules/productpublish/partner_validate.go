package productpublish

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func partnerValuePresent(val string) bool {
	v := strings.TrimSpace(val)
	if v == "" {
		return false
	}
	if strings.Contains(v, "****") {
		return true
	}
	return true
}

func ensurePartnerOpenConfig(ctx context.Context, settingsSvc *settings.Service, prov platformp.Provider) error {
	if settingsSvc == nil || prov == nil {
		return nil
	}
	p := strings.TrimSpace(strings.ToLower(prov.Platform()))
	if p == "mock" || p == "manual" {
		return nil
	}
	sch := prov.AppConfigSchema()
	gk := strings.TrimSpace(sch.GroupKey)
	if gk == "" {
		return nil
	}
	m, err := settingsSvc.PlainByGroup(ctx, 0, gk)
	if err != nil {
		return fmt.Errorf("platform config incomplete: please configure settings.%s first", gk)
	}
	lower := map[string]string{}
	for k, v := range m {
		lower[strings.ToLower(strings.TrimSpace(k))] = v
	}
	for _, f := range sch.Fields {
		if !f.Required {
			continue
		}
		nk := strings.ToLower(strings.TrimSpace(f.Name))
		if partnerValuePresent(lower[nk]) {
			continue
		}
		return fmt.Errorf("platform config incomplete: please configure settings.%s first", gk)
	}
	return nil
}

func ensureShopAuthorizedForPublish(row *shop.Shop, auth platformp.TestConnectionRequest) error {
	if row == nil {
		return fmt.Errorf("shop not found")
	}
	if strings.TrimSpace(row.Status) != shop.StatusActive {
		return fmt.Errorf("shop is not active")
	}
	if strings.TrimSpace(row.AuthStatus) != shop.AuthAuthorized {
		return fmt.Errorf("shop is not authorized")
	}
	if strings.TrimSpace(strings.ToLower(row.Platform)) == "mock" {
		return nil
	}
	if strings.TrimSpace(auth.AccessToken) == "" && strings.TrimSpace(auth.RefreshToken) == "" {
		return fmt.Errorf("shop is not authorized")
	}
	return nil
}
