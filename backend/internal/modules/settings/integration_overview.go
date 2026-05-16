package settings

import (
	"context"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	platformtiktok "github.com/trademind-ai/trademind/backend/internal/providers/platform/tiktok"
)

// IntegrationOverviewAI is AI text readiness snapshot.
type IntegrationOverviewAI struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
}

// IntegrationOverviewImage removes partial readiness.
type IntegrationOverviewImage struct {
	ProviderCurrent string `json:"providerCurrent,omitempty"`
	RemoveBG        bool   `json:"removebg"`
	OpenAIImage     bool   `json:"openaiImage"`
	ComfyUI         bool   `json:"comfyui"`
}

// IntegrationOverviewStorage summarizes storage group.
type IntegrationOverviewStorage struct {
	Kind       string `json:"kind,omitempty"`
	Configured bool   `json:"configured"`
}

// IntegrationOverviewMail is SMTP readiness (merged mail + legacy email).
type IntegrationOverviewMail struct {
	Configured bool `json:"configured"`
}

// IntegrationOverviewPlatformItem is one Open Platform app settings row.
type IntegrationOverviewPlatformItem struct {
	Platform      string `json:"platform"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	GroupKey      string `json:"groupKey,omitempty"`
	AppConfigured bool   `json:"appConfigured"`
}

// IntegrationsOverview is GET /api/v1/settings/integrations/overview.
type IntegrationsOverview struct {
	AI              IntegrationOverviewAI             `json:"ai"`
	Image           IntegrationOverviewImage          `json:"image"`
	Storage         IntegrationOverviewStorage        `json:"storage"`
	Mail            IntegrationOverviewMail           `json:"mail"`
	Platforms       []IntegrationOverviewPlatformItem `json:"platforms"`
	CollectRules    int64                             `json:"collectRulesCount"`
	DisclaimerShort string                            `json:"disclaimerShort"`
}

func loweredPlainMap(in map[string]string) map[string]string {
	mm := make(map[string]string)
	for k, v := range in {
		kk := strings.TrimSpace(strings.ToLower(k))
		if kk == "" {
			continue
		}
		mm[kk] = strings.TrimSpace(v)
	}
	return mm
}

func deployAppSettingsComplete(platformSlug string, schema platformp.PlatformAppConfigSchema, merged map[string]string) bool {
	plat := strings.TrimSpace(strings.ToLower(platformSlug))
	mm := loweredPlainMap(merged)
	switch plat {
	case "tiktok":
		_, err := platformtiktok.RuntimeFromMergedMap(mm)
		return err == nil
	default:
		for _, f := range schema.Fields {
			if !f.Required {
				continue
			}
			key := strings.TrimSpace(strings.ToLower(f.Name))
			if strings.TrimSpace(mm[key]) == "" {
				return false
			}
		}
		return true
	}
}

func plainForPlatformSchema(schema platformp.PlatformAppConfigSchema, db map[string]string) map[string]string {
	out := make(map[string]string)
	for _, f := range schema.Fields {
		v := ""
		want := strings.TrimSpace(strings.ToLower(f.Name))
		for k, val := range db {
			if strings.TrimSpace(strings.ToLower(k)) == want {
				v = strings.TrimSpace(val)
				break
			}
		}
		out[f.Name] = v
	}
	return out
}

func storageOverviewConfigured(kind string, m map[string]string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "local"
	}
	switch kind {
	case "local":
		return true
	case "s3", "r2", "minio":
		return strings.TrimSpace(m["s3_bucket"]) != "" &&
			strings.TrimSpace(m["s3_access_key_id"]) != "" &&
			strings.TrimSpace(m["s3_secret_access_key"]) != ""
	case "cos":
		return strings.TrimSpace(m["cos_bucket"]) != "" &&
			strings.TrimSpace(m["cos_secret_id"]) != "" &&
			strings.TrimSpace(m["cos_secret_key"]) != ""
	case "oss":
		return strings.TrimSpace(m["oss_bucket"]) != "" &&
			strings.TrimSpace(m["oss_access_key_id"]) != "" &&
			strings.TrimSpace(m["oss_access_key_secret"]) != ""
	default:
		return false
	}
}

// BuildIntegrationOverview aggregates non-secret readiness for the integrations hub page.
func (s *Service) BuildIntegrationOverview(ctx context.Context) (*IntegrationsOverview, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("settings: no db")
	}

	out := &IntegrationsOverview{
		DisclaimerShort: "贸灵开源发行版不包含任何第三方密钥；请在各开放平台与云厂商自助申请，仅在后台填写并由后端加密存储与调用。",
	}

	ai, err := s.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return nil, err
	}
	out.AI.Provider = strings.TrimSpace(ai["provider"])
	out.AI.Model = strings.TrimSpace(ai["model"])
	out.AI.Configured = strings.TrimSpace(ai["api_key"]) != "" && strings.TrimSpace(ai["base_url"]) != ""

	img, err := s.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return nil, err
	}
	out.Image.ProviderCurrent = strings.TrimSpace(img["provider"])
	out.Image.RemoveBG = strings.TrimSpace(img["removebg_api_key"]) != ""
	out.Image.OpenAIImage = strings.TrimSpace(img["openai_image_api_key"]) != ""
	out.Image.ComfyUI = strings.TrimSpace(img["comfyui_base_url"]) != ""

	st, err := s.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return nil, err
	}
	kind := strings.TrimSpace(st["kind"])
	out.Storage.Kind = kind
	out.Storage.Configured = storageOverviewConfigured(kind, st)

	mail, err := s.PlainMailSettings(ctx)
	if err != nil {
		return nil, err
	}
	out.Mail.Configured = strings.TrimSpace(mail["smtp_host"]) != "" && strings.TrimSpace(mail["smtp_from"]) != ""

	for _, p := range platformp.All() {
		sch := p.AppConfigSchema()
		gk := strings.TrimSpace(sch.GroupKey)
		if gk == "" {
			continue
		}
		plain, err := s.PlainByGroup(ctx, 0, gk)
		if err != nil {
			return nil, err
		}
		merged := plainForPlatformSchema(sch, plain)
		appOK := deployAppSettingsComplete(p.Platform(), sch, merged)
		out.Platforms = append(out.Platforms, IntegrationOverviewPlatformItem{
			Platform:      p.Platform(),
			Name:          p.Name(),
			Status:        p.Status(),
			GroupKey:      gk,
			AppConfigured: appOK,
		})
	}

	var n int64
	if err := s.DB.WithContext(ctx).Model(&collectrule.CollectRule{}).Count(&n).Error; err != nil {
		return nil, err
	}
	out.CollectRules = n

	return out, nil
}
