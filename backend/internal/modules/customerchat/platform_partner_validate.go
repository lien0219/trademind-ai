package customerchat

import (
	"context"
	"fmt"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func valuePresentForPartner(val string) bool {
	v := strings.TrimSpace(val)
	if v == "" {
		return false
	}
	if strings.Contains(v, "****") {
		return true
	}
	return true
}

// ensurePlatformPartnerConfig checks required Open Platform fields from settings (never logs secrets).
func (s *Service) ensurePlatformPartnerConfig(ctx context.Context, prov platformp.Provider) error {
	if s == nil || prov == nil || s.Settings == nil {
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
	m, err := s.Settings.PlainByGroup(ctx, 0, gk)
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
		if valuePresentForPartner(lower[nk]) {
			continue
		}
		return fmt.Errorf("platform config incomplete: please configure settings.%s first", gk)
	}
	return nil
}
