package platform

// Preset app-level Open Platform configuration schemas (settings group_key = platform_<id>).

func GenericPlannedAppSchema(platformID, title string) PlatformAppConfigSchema {
	gk := "platform_" + platformID
	return PlatformAppConfigSchema{
		GroupKey:    gk,
		Title:       title,
		Description: "该平台能力仍在对接中，可先保存 Partner 应用参数；TestConnection / OAuth / 订单同步将返回未实现。",
		Fields: []AppConfigField{
			{Name: "app_key", Label: "App Key / Client ID", Type: "text", Required: false, Sensitive: false, Placeholder: "按开放平台申请填写"},
			{Name: "app_secret", Label: "App Secret", Type: "password", Required: false, Sensitive: true},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: false, Sensitive: false},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: false, Sensitive: false},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: false, Sensitive: false, DefaultValue: 30, Help: "外部 HTTP 超时，5–600"},
		},
	}
}

func TikTokShopAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_tiktok",
		Title:       "TikTok Shop",
		Description: "请在 TikTok Shop Partner Center 创建 Open API 应用后填写；敏感项加密存储。需同时配置 API 路径版本 api_version。",
		Fields: []AppConfigField{
			{Name: "app_key", Label: "App Key", Type: "text", Required: true, Sensitive: false},
			{Name: "app_secret", Label: "App Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "auth_base_url", Label: "Auth Base URL", Type: "text", Required: true, Sensitive: false, Placeholder: "https://auth.tiktok-shops.com"},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: true, Sensitive: false, Placeholder: "https://open-api.tiktokglobalshop.com"},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "api_version", Label: "API Version（路径段）", Type: "text", Required: true, Sensitive: false, DefaultValue: "202309", Help: "用于 /authorization/{version}/… 与 /order/{version}/…"},
			{Name: "region", Label: "Region（可选）", Type: "text", Required: false, Sensitive: false},
			{Name: "oauth_scopes", Label: "OAuth Scopes（可选）", Type: "textarea", Required: false, Sensitive: false, Help: "空格分隔；留空则用 Partner 应用默认"},
			{Name: "sandbox_enabled", Label: "Sandbox 标记", Type: "switch", Required: false, Sensitive: false, Help: "仅存 true/false；是否沙箱以你在 Partner 与实际 URL 为准"},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30, Help: "5–600"},
		},
	}
}

func ShopeeAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_shopee",
		Title:       "Shopee",
		Description: "请在 Shopee Open Platform 创建应用后填写 Partner 参数；OAuth 与订单同步为 beta。",
		Fields: []AppConfigField{
			{Name: "partner_id", Label: "Partner ID", Type: "text", Required: true, Sensitive: false},
			{Name: "partner_key", Label: "Partner Key", Type: "password", Required: true, Sensitive: true},
			{Name: "auth_base_url", Label: "Auth Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "region", Label: "Region", Type: "text", Required: false, Sensitive: false, Help: "站点/市场区域标识"},
			{Name: "sandbox_enabled", Label: "Sandbox", Type: "switch", Required: false, Sensitive: false},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30},
		},
	}
}

func LazadaAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_lazada",
		Title:       "Lazada",
		Description: "请在 Lazada Open Platform 创建应用后填写。OAuth、TestConnection、订单同步为 beta。",
		Fields: []AppConfigField{
			{Name: "app_key", Label: "App Key", Type: "text", Required: true, Sensitive: false},
			{Name: "app_secret", Label: "App Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "auth_base_url", Label: "Auth Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "region", Label: "Region", Type: "text", Required: false, Sensitive: false},
			{Name: "sandbox_enabled", Label: "Sandbox", Type: "switch", Required: false, Sensitive: false},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30},
		},
	}
}

func AmazonSPAPIAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_amazon",
		Title:       "Amazon SP-API",
		Description: "LWA 与 Selling Partner API：SigV4 签名需运行时 AWS 凭证（环境变量、实例配置或 ECS/Task 角色）；可选 role_arn 走 STS AssumeRole。店铺级 refresh_token 保存在「店铺授权」。",
		Fields: []AppConfigField{
			{Name: "client_id", Label: "LWA Client ID", Type: "text", Required: true, Sensitive: false},
			{Name: "client_secret", Label: "LWA Client Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "refresh_token", Label: "Refresh Token（可选，应用级占位）", Type: "password", Required: false, Sensitive: true, Help: "通常按店铺保存在 shop_auth_tokens；此处仅占位扩展"},
			{Name: "lwa_auth_base_url", Label: "LWA / Seller Central Auth Base URL", Type: "text", Required: true, Sensitive: false, Placeholder: "https://sellercentral.amazon.com"},
			{Name: "lwa_token_url", Label: "LWA Token URL", Type: "text", Required: true, Sensitive: false, Placeholder: "https://api.amazon.com/auth/o2/token"},
			{Name: "sp_api_base_url", Label: "SP-API Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "redirect_uri", Label: "OAuth Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "marketplace_id", Label: "Marketplace ID（默认）", Type: "text", Required: true, Sensitive: false, Help: "orders 查询默认值；店铺授权可覆盖"},
			{Name: "region", Label: "AWS SigV4 Region", Type: "text", Required: false, Sensitive: false, Help: "留空则按 SP-API 主机推断（na/eu/fe）"},
			{Name: "role_arn", Label: "Role ARN（可选）", Type: "text", Required: false, Sensitive: false, Help: "设置后用默认链路的凭证 AssumeRole 再签名"},
			{Name: "sandbox_enabled", Label: "Sandbox", Type: "switch", Required: false, Sensitive: false},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30},
		},
	}
}

func AliExpressAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_aliexpress",
		Title:       "AliExpress",
		Description: "AliExpress Open Platform 应用参数（对接仍 planned）。",
		Fields: []AppConfigField{
			{Name: "app_key", Label: "App Key", Type: "text", Required: true, Sensitive: false},
			{Name: "app_secret", Label: "App Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "auth_base_url", Label: "Auth Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "region", Label: "Region", Type: "text", Required: false, Sensitive: false},
			{Name: "sandbox_enabled", Label: "Sandbox", Type: "switch", Required: false, Sensitive: false},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30},
		},
	}
}

func ShopifyAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_shopify",
		Title:       "Shopify",
		Description: "Shopify Partners / Custom App OAuth 参数（对接仍 planned）。",
		Fields: []AppConfigField{
			{Name: "client_id", Label: "Client ID", Type: "text", Required: true, Sensitive: false},
			{Name: "client_secret", Label: "Client Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "shop_domain", Label: "Shop domain（可选）", Type: "text", Required: false, Sensitive: false, Placeholder: "your-store.myshopify.com"},
			{Name: "auth_base_url", Label: "Auth Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "scopes", Label: "Scopes", Type: "textarea", Required: false, Sensitive: false},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30},
		},
	}
}

func WooCommerceAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_woocommerce",
		Title:       "WooCommerce",
		Description: "REST API 凭证（对接仍 planned）；生产环境请使用 HTTPS。",
		Fields: []AppConfigField{
			{Name: "consumer_key", Label: "Consumer Key", Type: "password", Required: true, Sensitive: true},
			{Name: "consumer_secret", Label: "Consumer Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "store_url", Label: "Store URL", Type: "text", Required: true, Sensitive: false, Placeholder: "https://your-store.com"},
			{Name: "api_version", Label: "API Version", Type: "text", Required: true, Sensitive: false, DefaultValue: "wc/v3"},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30},
		},
	}
}

func EbayAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_ebay",
		Title:       "eBay",
		Description: "eBay Developers Program 应用（对接仍 planned）。",
		Fields: []AppConfigField{
			{Name: "client_id", Label: "Client ID (App ID)", Type: "text", Required: true, Sensitive: false},
			{Name: "client_secret", Label: "Client Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "dev_id", Label: "Dev ID（可选）", Type: "text", Required: false, Sensitive: false},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "auth_base_url", Label: "Auth Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: true, Sensitive: false},
			{Name: "marketplace_id", Label: "Marketplace ID", Type: "text", Required: false, Sensitive: false},
			{Name: "sandbox_enabled", Label: "Sandbox", Type: "switch", Required: false, Sensitive: false},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30},
		},
	}
}
