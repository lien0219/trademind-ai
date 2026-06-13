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

func DouyinShopAppConfigSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_douyin_shop",
		Title:       "抖店 / Douyin Shop",
		Description: "请在抖店开放平台创建应用后填写；App Secret 加密存储。完成配置后在「店铺管理」授权；订单/库存/商品草稿能力需在下方开关中启用。",
		Fields: []AppConfigField{
			{Name: "app_key", Label: "App Key / Client Key", Type: "text", Required: true, Sensitive: false, Placeholder: "在抖店开放平台应用中获取"},
			{Name: "app_secret", Label: "App Secret / Client Secret", Type: "password", Required: true, Sensitive: true},
			{Name: "service_id", Label: "Service ID", Type: "text", Required: false, Sensitive: false, Help: "抖店服务市场自定义授权 URL 使用 service_id；发起授权前必须填写。"},
			{Name: "auth_base_url", Label: "Auth Base URL", Type: "text", Required: false, Sensitive: false, Placeholder: "按抖店开放平台官方文档填写"},
			{Name: "api_base_url", Label: "API Base URL", Type: "text", Required: false, Sensitive: false, Placeholder: "按抖店开放平台官方文档填写"},
			{Name: "redirect_uri", Label: "Redirect URI", Type: "text", Required: true, Sensitive: false},
			{Name: "environment", Label: "环境", Type: "select", Required: true, Sensitive: false, DefaultValue: "production", Options: []AppConfigOption{
				{Label: "生产 production", Value: "production"},
				{Label: "沙箱 sandbox", Value: "sandbox"},
			}},
			{Name: "real_api_enabled", Label: "启用真实接口", Type: "switch", Required: false, Sensitive: false, Help: "Phase 1 仅保存开关；真实接口调用将在后续阶段逐项接入。"},
			{Name: "order_sync_enabled", Label: "启用订单同步", Type: "switch", Required: false, Sensitive: false},
			{Name: "order_sync_max_pages", Label: "订单同步最大页数", Type: "number", Required: false, Sensitive: false, DefaultValue: 5, Help: "单次任务最多拉取页数（默认 5）；每页 size 由任务 limit 控制，总条数上限 500"},
			{Name: "inventory_sync_enabled", Label: "启用库存同步", Type: "switch", Required: false, Sensitive: false},
			{Name: "product_publish_enabled", Label: "启用商品草稿创建", Type: "switch", Required: false, Sensitive: false},
			{Name: "gray_release_enabled", Label: "启用灰度发布", Type: "switch", Required: false, Sensitive: false, Help: "开启后仅 gray_shop_ids 白名单店铺可执行写操作"},
			{Name: "gray_shop_ids", Label: "灰度店铺 ID 列表", Type: "textarea", Required: false, Sensitive: false, DefaultValue: "[]", Help: "JSON 数组或逗号分隔 UUID"},
			{Name: "write_operations_enabled", Label: "允许写操作", Type: "switch", Required: false, Sensitive: false, Help: "关闭后阻止抖店写任务（读操作不受影响）"},
			{Name: "scheduled_order_sync_enabled", Label: "允许定时订单同步", Type: "switch", Required: false, Sensitive: false},
			{Name: "scheduled_inventory_sync_enabled", Label: "允许定时库存同步", Type: "switch", Required: false, Sensitive: false},
			{Name: "alert_scan_enabled", Label: "启用抖店告警扫描", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
			{Name: "alert_scan_interval_seconds", Label: "告警扫描间隔(秒)", Type: "number", Required: false, Sensitive: false, DefaultValue: 120},
			{Name: "alert_token_refresh_fail_threshold", Label: "Token 刷新失败告警阈值", Type: "number", Required: false, Sensitive: false, DefaultValue: 3},
			{Name: "alert_stale_tasks_threshold", Label: "stale 任务告警阈值", Type: "number", Required: false, Sensitive: false, DefaultValue: 5},
			{Name: "alert_failure_backlog_threshold", Label: "失败任务积压告警阈值", Type: "number", Required: false, Sensitive: false, DefaultValue: 20},
			{Name: "alert_rate_limit_threshold", Label: "限流次数告警阈值(24h)", Type: "number", Required: false, Sensitive: false, DefaultValue: 10},
			{Name: "alert_product_draft_fail_threshold", Label: "商品草稿失败告警阈值(24h)", Type: "number", Required: false, Sensitive: false, DefaultValue: 3},
			{Name: "alert_inventory_sync_fail_threshold", Label: "库存同步失败告警阈值(24h)", Type: "number", Required: false, Sensitive: false, DefaultValue: 5},
			{Name: "alert_image_upload_fail_rate_pct", Label: "图片上传失败率告警(%)", Type: "number", Required: false, Sensitive: false, DefaultValue: 30},
			{Name: "stale_timeout_product_draft_min", Label: "商品草稿超时(分钟)", Type: "number", Required: false, Sensitive: false, DefaultValue: 10},
			{Name: "stale_timeout_image_upload_min", Label: "图片上传超时(分钟)", Type: "number", Required: false, Sensitive: false, DefaultValue: 15},
			{Name: "stale_timeout_order_sync_min", Label: "订单同步超时(分钟)", Type: "number", Required: false, Sensitive: false, DefaultValue: 30},
			{Name: "stale_timeout_inventory_sync_min", Label: "库存同步超时(分钟)", Type: "number", Required: false, Sensitive: false, DefaultValue: 15},
			{Name: "timeout_sec", Label: "Timeout (seconds)", Type: "number", Required: true, Sensitive: false, DefaultValue: 30, Help: "外部 HTTP 超时，5–600"},
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
