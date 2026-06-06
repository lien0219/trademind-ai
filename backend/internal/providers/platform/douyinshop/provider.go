package douyinshop

import (
	"context"
	"fmt"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type provider struct{}

// NewProvider constructs the Douyin Shop provider. Phase 1 registers app-level
// configuration only; live OAuth/API adapters are added in later phases.
func NewProvider() platformp.Provider { return provider{} }

func (provider) Platform() string { return "douyin_shop" }

func (provider) Name() string { return "抖店 / Douyin Shop" }

func (provider) Status() string { return platformp.StatusBeta }

func (provider) Capabilities() []platformp.Capability {
	return []platformp.Capability{
		platformp.CapShopInfo,
	}
}

func (provider) AuthSchema() platformp.AuthSchema {
	return platformp.AuthSchema{
		AuthType: "oauth2",
		Fields: []platformp.AuthField{
			{Name: "appKey", Label: "覆盖 App Key（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取「设置 → 平台开放配置」中的 platform_douyin_shop.app_key；仅多应用调试时填写"},
			{Name: "appSecret", Label: "覆盖 App Secret（可选）", Type: "password", Required: false, Sensitive: true, Hint: "默认读取平台配置中的 app_secret（加密存储）；留空不覆盖"},
			{Name: "redirectUri", Label: "覆盖 Redirect URI（可选）", Type: "text", Required: false, Sensitive: false, Hint: "默认读取平台配置的 redirect_uri；须与抖店开放平台登记一致"},
		},
	}
}

func (provider) AppConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.DouyinShopAppConfigSchema()
}

func (provider) PublishConfigSchema() platformp.PlatformAppConfigSchema {
	return platformp.PublishConfigPresetForPlatform("douyin_shop")
}

func (provider) TestConnection(ctx context.Context, req platformp.TestConnectionRequest) (*platformp.TestConnectionResult, error) {
	_ = ctx
	cfg, err := RuntimeFromMergedMap(map[string]string{
		"app_key":                 req.AppKey,
		"app_secret":              req.AppSecret,
		"service_id":              req.Extra["service_id"],
		"redirect_uri":            req.Extra["redirect_uri"],
		"auth_base_url":           req.Extra["auth_base_url"],
		"api_base_url":            req.Extra["api_base_url"],
		"environment":             req.Extra["environment"],
		"real_api_enabled":        req.Extra["real_api_enabled"],
		"order_sync_enabled":      req.Extra["order_sync_enabled"],
		"inventory_sync_enabled":  req.Extra["inventory_sync_enabled"],
		"product_publish_enabled": req.Extra["product_publish_enabled"],
		"timeout_sec":             req.Extra["timeout_sec"],
	})
	if err != nil {
		return &platformp.TestConnectionResult{OK: false, Message: err.Error()}, nil
	}
	return &platformp.TestConnectionResult{
		OK:       true,
		Message:  fmt.Sprintf("douyin_shop config ok (%s); OAuth is available in Phase 2", cfg.Environment),
		Currency: "CNY",
	}, nil
}
