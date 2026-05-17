package shop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type PlatformPublishSettingsDTO struct {
	Platform string                            `json:"platform"`
	GroupKey string                            `json:"groupKey"`
	Schema   platformp.PlatformAppConfigSchema `json:"schema"`
	Values   map[string]string                 `json:"values"`
}

func (s *Service) snapshotMaskedPublish(schema platformp.PlatformAppConfigSchema, plain map[string]string) map[string]string {
	out := map[string]string{}
	for _, f := range schema.Fields {
		out[f.Name] = getCurField(plain, f.Name)
		if f.Sensitive && strings.TrimSpace(out[f.Name]) != "" {
			out[f.Name] = "****"
		}
	}
	return out
}

func validateMergedPublishPut(p platformp.Provider, schema platformp.PlatformAppConfigSchema, merged map[string]string) error {
	if p.Status() != platformp.StatusPlanned {
		for _, f := range schema.Fields {
			if !f.Required {
				continue
			}
			if strings.TrimSpace(merged[f.Name]) == "" {
				return fmt.Errorf("required publish setting missing: %s", f.Label)
			}
		}
	}
	return nil
}

func (s *Service) GetPlatformPublishSettings(ctx context.Context, platformSlug string) (*PlatformPublishSettingsDTO, error) {
	if s == nil || s.Settings == nil {
		return nil, errors.New("shop: settings unavailable")
	}
	plat := strings.TrimSpace(strings.ToLower(platformSlug))
	p := platformp.Get(plat)
	if p == nil {
		return nil, fmt.Errorf("unknown platform %q", plat)
	}
	sch := p.PublishConfigSchema()
	gk := strings.TrimSpace(sch.GroupKey)
	if gk == "" {
		return nil, fmt.Errorf("platform %q has no publish settings schema", plat)
	}
	cur, err := s.Settings.PlainByGroup(ctx, 0, gk)
	if err != nil {
		return nil, err
	}
	return &PlatformPublishSettingsDTO{
		Platform: plat,
		GroupKey: gk,
		Schema:   sch,
		Values:   s.snapshotMaskedPublish(sch, cur),
	}, nil
}

func (s *Service) PutPlatformPublishSettings(c *gin.Context, platformSlug string, values map[string]interface{}) (*PlatformPublishSettingsDTO, error) {
	if s == nil || s.Settings == nil || c == nil {
		return nil, errors.New("shop: settings unavailable")
	}
	ctx := c.Request.Context()
	plat := strings.TrimSpace(strings.ToLower(platformSlug))
	p := platformp.Get(plat)
	if p == nil {
		return nil, fmt.Errorf("unknown platform %q", plat)
	}
	sch := p.PublishConfigSchema()
	gk := strings.TrimSpace(sch.GroupKey)
	if gk == "" {
		return nil, fmt.Errorf("platform %q has no publish settings schema", plat)
	}
	if values == nil {
		return nil, fmt.Errorf("values required")
	}
	byName := schemaFieldsByLowerName(sch)
	for k := range values {
		lk := strings.TrimSpace(strings.ToLower(k))
		if _, ok := byName[lk]; !ok {
			return nil, fmt.Errorf("unknown field %q for platform %s", k, plat)
		}
	}
	cur, err := s.Settings.PlainByGroup(ctx, 0, gk)
	if err != nil {
		return nil, err
	}
	merged := mergedFromCurrent(sch, cur)
	for k, vRaw := range values {
		coerced := coerceJSONScalar(vRaw)
		lk := strings.TrimSpace(strings.ToLower(k))
		f := byName[lk]
		nv, ferr := normalizeFieldValue(f, coerced)
		if ferr != nil {
			return nil, ferr
		}
		if f.Sensitive && encrypt.LooksMasked(nv) {
			continue
		}
		merged[f.Name] = nv
	}
	if err := validateMergedPublishPut(p, sch, merged); err != nil {
		return nil, err
	}

	var items []settings.PutItem
	for _, f := range sch.Fields {
		val := merged[f.Name]
		items = append(items, settings.PutItem{
			TenantID:    0,
			GroupKey:    gk,
			ItemKey:     f.Name,
			ItemValue:   val,
			ValueType:   "string",
			IsEncrypted: f.Sensitive,
		})
	}
	if err := s.Settings.PutBulk(ctx, items); err != nil {
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				Action:     "platform.publish_settings.update",
				Resource:   "platform_publish_settings",
				ResourceID: plat,
				Status:     "failed",
				Message:    err.Error(),
			})
		}
		return nil, err
	}
	out, err := s.GetPlatformPublishSettings(ctx, plat)
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "platform.publish_settings.update",
			Resource:   "platform_publish_settings",
			ResourceID: plat,
			Status:     "success",
			Message:    fmt.Sprintf("group %s saved", gk),
		})
	}
	return out, nil
}
