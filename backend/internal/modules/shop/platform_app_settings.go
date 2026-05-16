package shop

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/trademind-ai/trademind/backend/internal/encrypt"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
)

// PlatformAppSettingsDTO is GET /api/v1/platform/settings/:platform and PUT response body.
type PlatformAppSettingsDTO struct {
	Platform string            `json:"platform"`
	GroupKey string            `json:"groupKey"`
	Values   map[string]string `json:"values"`
}

func getCurField(cur map[string]string, name string) string {
	n := strings.TrimSpace(strings.ToLower(name))
	if cur == nil {
		return ""
	}
	for k, v := range cur {
		if strings.TrimSpace(strings.ToLower(k)) == n {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func mergedFromCurrent(schema platformp.PlatformAppConfigSchema, cur map[string]string) map[string]string {
	out := map[string]string{}
	for _, f := range schema.Fields {
		out[f.Name] = getCurField(cur, f.Name)
	}
	return out
}

func coerceJSONScalar(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		if float64(int64(x)) == x && x <= 1e12 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func normalizeFieldValue(f platformp.AppConfigField, raw string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(f.Type))
	raw = strings.TrimSpace(raw)

	switch t {
	case "switch":
		if raw == "" {
			return "", nil
		}
		l := strings.ToLower(raw)
		switch l {
		case "true", "1", "on", "yes":
			return "true", nil
		case "false", "0", "off", "no":
			return "false", nil
		default:
			return "", fmt.Errorf("字段 %s 需要布尔开关值", f.Name)
		}
	case "number":
		if raw == "" {
			return "", nil
		}
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return "", fmt.Errorf("字段 %s 需要数字", f.Name)
		}
		if float64(int64(n)) == n {
			return strconv.FormatInt(int64(n), 10), nil
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case "select":
		if raw == "" {
			return "", nil
		}
		if len(f.Options) == 0 {
			return raw, nil
		}
		for _, opt := range f.Options {
			if strings.TrimSpace(opt.Value) == raw {
				return raw, nil
			}
		}
		return "", fmt.Errorf("字段 %s 的值不在允许的选项中", f.Name)
	default:
		return raw, nil
	}
}

func schemaFieldsByLowerName(schema platformp.PlatformAppConfigSchema) map[string]platformp.AppConfigField {
	m := map[string]platformp.AppConfigField{}
	for _, f := range schema.Fields {
		k := strings.TrimSpace(strings.ToLower(f.Name))
		if k != "" {
			m[k] = f
		}
	}
	return m
}

func loweredMap(in map[string]string) map[string]string {
	mm := map[string]string{}
	for k, v := range in {
		kk := strings.TrimSpace(strings.ToLower(k))
		if kk == "" {
			continue
		}
		mm[kk] = strings.TrimSpace(v)
	}
	return mm
}

func validateMergedAppSettings(platformSlug string, schema platformp.PlatformAppConfigSchema, merged map[string]string) error {
	plat := strings.TrimSpace(platformSlug)

	mm := loweredMap(merged)

	switch plat {
	case "tiktok":
		if _, err := platformtiktok.RuntimeFromMergedMap(mm); err != nil {
			return err
		}
		return nil
	default:
		for _, f := range schema.Fields {
			if !f.Required {
				continue
			}
			key := strings.TrimSpace(strings.ToLower(f.Name))
			if strings.TrimSpace(mm[key]) == "" {
				return fmt.Errorf("required setting missing: %s", f.Name)
			}
		}
		return nil
	}
}

func (s *Service) snapshotMaskedApp(schema platformp.PlatformAppConfigSchema, plain map[string]string) map[string]string {
	out := schema.SnapshotForAPI(plain)
	for _, f := range schema.Fields {
		if f.Sensitive && strings.TrimSpace(out[f.Name]) != "" {
			out[f.Name] = "****"
		}
	}
	return out
}

// GetPlatformAppSettings returns decrypted-then-masked snapshot for UI.
func (s *Service) GetPlatformAppSettings(ctx context.Context, platformSlug string) (*PlatformAppSettingsDTO, error) {
	if s == nil || s.Settings == nil {
		return nil, errors.New("shop: settings unavailable")
	}
	plat := strings.TrimSpace(strings.ToLower(platformSlug))
	p := platformp.Get(plat)
	if p == nil {
		return nil, fmt.Errorf("unknown platform %q", plat)
	}
	sch := p.AppConfigSchema()
	gk := strings.TrimSpace(sch.GroupKey)
	if gk == "" {
		return nil, fmt.Errorf("platform %q has no deploy-level app settings schema", plat)
	}
	cur, err := s.Settings.PlainByGroup(ctx, 0, gk)
	if err != nil {
		return nil, err
	}
	return &PlatformAppSettingsDTO{
		Platform: plat,
		GroupKey: gk,
		Values:   s.snapshotMaskedApp(sch, cur),
	}, nil
}

func (s *Service) PutPlatformAppSettings(c *gin.Context, platformSlug string, values map[string]interface{}) (*PlatformAppSettingsDTO, error) {
	if s == nil || s.Settings == nil || c == nil {
		return nil, errors.New("shop: settings unavailable")
	}
	ctx := c.Request.Context()
	plat := strings.TrimSpace(strings.ToLower(platformSlug))
	p := platformp.Get(plat)
	if p == nil {
		return nil, fmt.Errorf("unknown platform %q", plat)
	}
	sch := p.AppConfigSchema()
	gk := strings.TrimSpace(sch.GroupKey)
	if gk == "" {
		return nil, fmt.Errorf("platform %q has no deploy-level app settings schema", plat)
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
			continue // keep encrypted current in DB via masked placeholder
		}
		merged[f.Name] = nv
	}

	if err := validateMergedAppSettings(plat, sch, merged); err != nil {
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
				Action:     "platform.settings.update",
				Resource:   "platform_app_settings",
				ResourceID: plat,
				Status:     "failed",
				Message:    err.Error(),
			})
		}
		return nil, err
	}

	out, err := s.GetPlatformAppSettings(ctx, plat)
	if err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "platform.settings.update",
			Resource:   "platform_app_settings",
			ResourceID: plat,
			Status:     "success",
			Message:    fmt.Sprintf("group %s saved", gk),
		})
	}
	return out, nil
}
