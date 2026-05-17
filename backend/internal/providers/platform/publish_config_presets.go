package platform

import "strings"

func emptyPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{}
}

func mockPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_mock",
		Title:       "Mock 店铺商品刊登默认配置",
		Description: "开发测试用途；类目/模板等占位字段可留空。",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID（可选）", Type: "text", Required: false, Sensitive: false},
			{Name: "shipping_template_id", Label: "物流模板 ID（可选）", Type: "text", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认发布为草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

// PublishConfigPresetForPlatform returns persisted publish-settings schema group for the platform slug.
func PublishConfigPresetForPlatform(platformID string) PlatformAppConfigSchema {
	switch strings.ToLower(strings.TrimSpace(platformID)) {
	case "tiktok":
		return tiktokPublishSchema()
	case "shopee":
		return shopeePublishSchema()
	case "lazada":
		return lazadaPublishSchema()
	case "amazon":
		return amazonPublishSchema()
	case "aliexpress":
		return aliexpressPublishSchema()
	case "shopify":
		return shopifyPublishSchema()
	case "woocommerce":
		return wooPublishSchema()
	case "ebay":
		return ebayPublishSchema()
	case "temu":
		return temuPublishSchema()
	case "shein":
		return sheinPublishSchema()
	case "mock":
		return mockPublishSchema()
	case "manual":
		return emptyPublishSchema()
	case "custom":
		return genericCustomPublishSchema()
	default:
		return genericPlannedPublishSchema(platformID)
	}
}

func tiktokPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_tiktok",
		Title:       "TikTok Shop 商品刊登配置",
		Description: "用于 TikTok 商品刊登默认参数；请在 TikTok Seller / Partner Center 获取类目与模板 ID。",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID", Type: "text", Required: true, Sensitive: false},
			{Name: "default_brand_id", Label: "默认品牌 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "shipping_template_id", Label: "物流模板 ID", Type: "text", Required: true, Sensitive: false},
			{Name: "warehouse_id", Label: "默认仓库 ID", Type: "text", Required: true, Sensitive: false},
			{Name: "default_weight", Label: "默认包裹重量", Type: "text", Required: false, Sensitive: false, Help: "与平台度量单位保持一致；仅作草稿默认"},
			{Name: "default_length", Label: "默认长度", Type: "text", Required: false, Sensitive: false},
			{Name: "default_width", Label: "默认宽度", Type: "text", Required: false, Sensitive: false},
			{Name: "default_height", Label: "默认高度", Type: "text", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认发布为草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
			{Name: "product_status", Label: "商品状态（平台取值字符串）", Type: "textarea", Required: false, Sensitive: false, Help: "由平台定义的取值，自行填写文本"},
			{Name: "size_chart_id", Label: "尺码表 ID（可选）", Type: "text", Required: false, Sensitive: false},
			{Name: "return_policy_id", Label: "售后/退货模板 ID（可选）", Type: "text", Required: false, Sensitive: false},
		},
	}
}

func shopeePublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_shopee",
		Title:       "Shopee 商品刊登配置",
		Description: "Shopee 类目、物流与企业参数请在 Open Platform / Seller Centre 查阅后填入。",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID", Type: "text", Required: true, Sensitive: false},
			{Name: "default_brand_id", Label: "默认品牌 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "logistic_channel_id", Label: "物流渠道 ID", Type: "text", Required: true, Sensitive: false},
			{Name: "warehouse_id", Label: "默认仓库 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "default_weight", Label: "默认包裹重量 (kg)", Type: "number", Required: false, Sensitive: false},
			{Name: "default_length", Label: "默认长度 (cm)", Type: "number", Required: false, Sensitive: false},
			{Name: "default_width", Label: "默认宽度 (cm)", Type: "number", Required: false, Sensitive: false},
			{Name: "default_height", Label: "默认高度 (cm)", Type: "number", Required: false, Sensitive: false},
			{Name: "condition", Label: "商品成色", Type: "select", Required: false, Sensitive: false, Options: []AppConfigOption{
				{Label: "NEW", Value: "NEW"},
				{Label: "USED", Value: "USED"},
				{Label: "自行填写文本（选择其它后在扩展字段补充）", Value: "_custom"},
			}},
			{Name: "days_to_ship", Label: "发货天数", Type: "number", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认发布为草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func lazadaPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_lazada",
		Title:       "Lazada 商品刊登配置",
		Description: "Lazada 类目、度量与履约参数请在开放平台或卖家后台获取。",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID", Type: "text", Required: true, Sensitive: false},
			{Name: "default_brand_id", Label: "默认品牌 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "package_weight", Label: "包裹重量 (kg)", Type: "number", Required: false, Sensitive: false},
			{Name: "package_length", Label: "包裹长度 (cm)", Type: "number", Required: false, Sensitive: false},
			{Name: "package_width", Label: "包裹宽度 (cm)", Type: "number", Required: false, Sensitive: false},
			{Name: "package_height", Label: "包裹高度 (cm)", Type: "number", Required: false, Sensitive: false},
			{Name: "warranty_type", Label: "保修类型说明", Type: "text", Required: false, Sensitive: false},
			{Name: "warranty_period", Label: "保修期描述", Type: "text", Required: false, Sensitive: false},
			{Name: "delivery_option", Label: "配送选项（平台取值）", Type: "text", Required: true, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认发布为草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func amazonPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_amazon",
		Title:       "Amazon SP-API 商品刊登配置",
		Description: "Listings / Feeds 相关默认参数（不在代码中写死 Browse Node 等）。",
		Fields: []AppConfigField{
			{Name: "marketplace_id", Label: "Marketplace ID", Type: "text", Required: true, Sensitive: false},
			{Name: "product_type", Label: "Product Type", Type: "text", Required: true, Sensitive: false},
			{Name: "default_browse_node_id", Label: "默认 Browse Node ID", Type: "text", Required: false, Sensitive: false},
			{Name: "merchant_shipping_group", Label: "Merchant Shipping Group", Type: "text", Required: false, Sensitive: false},
			{Name: "condition_type", Label: "Condition Type", Type: "text", Required: false, Sensitive: false},
			{Name: "fulfillment_channel", Label: "Fulfillment Channel", Type: "text", Required: false, Sensitive: false, Help: "例如自填 MFN / 平台约定枚举文本"},
			{Name: "brand", Label: "Brand", Type: "text", Required: false, Sensitive: false},
			{Name: "manufacturer", Label: "Manufacturer", Type: "text", Required: false, Sensitive: false},
			{Name: "default_weight", Label: "默认重量", Type: "text", Required: false, Sensitive: false},
			{Name: "default_length", Label: "默认长度", Type: "text", Required: false, Sensitive: false},
			{Name: "default_width", Label: "默认宽度", Type: "text", Required: false, Sensitive: false},
			{Name: "default_height", Label: "默认高度", Type: "text", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认仅准备草稿清单", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func aliexpressPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_aliexpress",
		Title:       "AliExpress 商品刊登默认配置（预留）",
		Description: "平台对接仍规划中；可先保存默认刊登参数以备后续启用。",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "default_brand_id", Label: "默认品牌 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "shipping_template_id", Label: "运费模板 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "warehouse_id", Label: "仓库 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认发布为草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func shopifyPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_shopify",
		Title:       "Shopify 商品刊登默认配置（预留）",
		Description: "对接仍规划中。",
		Fields: []AppConfigField{
			{Name: "default_vendor", Label: "默认 Vendor", Type: "text", Required: false, Sensitive: false},
			{Name: "default_product_type", Label: "默认 Product type", Type: "text", Required: false, Sensitive: false},
			{Name: "default_tags", Label: "默认 Tags", Type: "textarea", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认发布为草稿状态", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func wooPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_woocommerce",
		Title:       "WooCommerce 刊登默认配置（预留）",
		Description: "",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID（站内涵义）", Type: "text", Required: false, Sensitive: false},
			{Name: "default_status", Label: "商品状态字符串", Type: "text", Required: false, Sensitive: false, Help: "如 draft/publish"},
			{Name: "publish_as_draft", Label: "默认草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func ebayPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_ebay",
		Title:       "eBay 刊登默认配置（预留）",
		Description: "",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "default_business_policies_hint", Label: "政策包/策略说明（摘录）", Type: "textarea", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认草稿/未刊登", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func temuPublishSchema() PlatformAppConfigSchema {
	return genericPlannedPublishSchemaWithKey("platform_publish_temu", "Temu")
}

func sheinPublishSchema() PlatformAppConfigSchema {
	return genericPlannedPublishSchemaWithKey("platform_publish_shein", "SHEIN")
}

func genericPlannedPublishSchema(platformID string) PlatformAppConfigSchema {
	slug := strings.ToLower(strings.TrimSpace(platformID))
	return genericPlannedPublishSchemaWithKey("platform_publish_"+slug, slug)
}

func genericPlannedPublishSchemaWithKey(groupSuffix, title string) PlatformAppConfigSchema {
	gk := groupSuffix
	if !strings.HasPrefix(groupSuffix, "platform_publish_") {
		gk = "platform_publish_" + groupSuffix
	}
	return PlatformAppConfigSchema{
		GroupKey:    gk,
		Title:       title + " 刊登默认参数（预留）",
		Description: "平台尚在规划；可先填默认类目/运费等占位。",
		Fields: []AppConfigField{
			{Name: "default_category_id", Label: "默认类目 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "default_brand_id", Label: "默认品牌 ID", Type: "text", Required: false, Sensitive: false},
			{Name: "shipping_hint", Label: "物流/履约说明摘录", Type: "textarea", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}

func genericCustomPublishSchema() PlatformAppConfigSchema {
	return PlatformAppConfigSchema{
		GroupKey:    "platform_publish_custom",
		Title:       "自定义平台刊登配置",
		Description: "",
		Fields: []AppConfigField{
			{Name: "endpoint_hint", Label: "对接 Endpoint 摘录", Type: "text", Required: false, Sensitive: false},
			{Name: "default_payload_hint", Label: "JSON 模板/字段说明摘录", Type: "textarea", Required: false, Sensitive: false},
			{Name: "publish_as_draft", Label: "默认草稿", Type: "switch", Required: false, Sensitive: false, DefaultValue: true},
		},
	}
}
