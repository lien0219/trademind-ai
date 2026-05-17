package amazon

import (
	"context"
	"fmt"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func loadAmazonPublishPlain(ctx context.Context) map[string]string {
	if bridges == nil {
		return map[string]string{}
	}
	m, err := bridges.AmazonPublishSettings(ctx)
	if err != nil || m == nil {
		return map[string]string{}
	}
	return m
}

func stringMapToAny(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		out[kk] = v
	}
	return out
}

func stringFromOptsAmazon(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if m == nil {
			break
		}
		raw, ok := m[k]
		if !ok {
			continue
		}
		s := strings.TrimSpace(fmt.Sprint(raw))
		if s != "" {
			return s
		}
	}
	return ""
}

func resolveAmazonInventoryMarketplaceID(opts map[string]any, merged amazonPublishMerged, auth platformp.TestConnectionRequest, cfg RuntimeConfig) string {
	if v := stringFromOptsAmazon(opts,
		"marketplace_id", "marketplaceId", "MarketplaceID", "marketplaceIds",
	); v != "" {
		return v
	}
	if strings.TrimSpace(merged.MarketplaceID) != "" {
		return strings.TrimSpace(merged.MarketplaceID)
	}
	return EffectiveMarketplaceID(auth, cfg)
}

func resolveAmazonInventoryFulfillmentChannel(opts map[string]any, merged amazonPublishMerged) string {
	if v := stringFromOptsAmazon(opts,
		"fulfillment_channel", "fulfillmentChannel", "FulfillmentChannel",
	); v != "" {
		return v
	}
	return strings.TrimSpace(merged.FulfillmentChannel)
}

func resolveAmazonInventoryProductType(opts map[string]any, merged amazonPublishMerged) string {
	if v := stringFromOptsAmazon(opts, "product_type", "productType", "ProductType"); v != "" {
		return v
	}
	return strings.TrimSpace(merged.ProductType)
}
