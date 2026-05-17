package platform

// Bootstrap registers built-in platform providers (safe to call once at startup).
func Bootstrap() {
	Register(newManualProvider())
	Register(newMockProvider())

	Register(newPlannedProvider("aliexpress", "AliExpress", StatusPlanned, "oauth2", []Capability{
		CapOrderSync, CapProductPublish, CapCustomerMessage,
	}, nil, AliExpressAppConfigSchema()))

	Register(newPlannedProvider("shopify", "Shopify", StatusPlanned, "oauth2", []Capability{
		CapOrderSync, CapProductPublish, CapInventorySync,
	}, nil, ShopifyAppConfigSchema()))

	Register(newPlannedProvider("woocommerce", "WooCommerce", StatusPlanned, "api_key", []Capability{
		CapOrderSync, CapProductPublish,
	}, woocommerceAuthFields(), WooCommerceAppConfigSchema()))

	Register(newPlannedProvider("ebay", "eBay", StatusPlanned, "oauth2", []Capability{
		CapOrderSync, CapProductPublish, CapCustomerMessage,
	}, nil, EbayAppConfigSchema()))

	Register(newPlannedProvider("temu", "Temu", StatusPlanned, "oauth2", []Capability{
		CapOrderSync, CapProductPublish,
	}, nil, GenericPlannedAppSchema("temu", "Temu")))

	Register(newPlannedProvider("shein", "SHEIN", StatusPlanned, "oauth2", []Capability{
		CapOrderSync, CapProductPublish,
	}, nil, GenericPlannedAppSchema("shein", "SHEIN")))

	Register(newPlannedProvider("custom", "自定义平台", StatusPlanned, "custom", []Capability{
		CapManualManage, CapProductPublish,
	}, customAuthFields(), GenericPlannedAppSchema("custom", "自定义平台")))
}

func woocommerceAuthFields() []AuthField {
	return []AuthField{
		{Name: "siteUrl", Label: "Shop URL", Type: "text", Required: false, Sensitive: false},
		{Name: "consumerKey", Label: "Consumer Key", Type: "text", Required: false, Sensitive: true},
		{Name: "consumerSecret", Label: "Consumer Secret", Type: "password", Required: false, Sensitive: true},
	}
}

func customAuthFields() []AuthField {
	return []AuthField{
		{Name: "endpoint", Label: "API Base URL", Type: "text", Required: false, Sensitive: false},
		{Name: "apiKey", Label: "API Key", Type: "password", Required: false, Sensitive: true},
	}
}
