package amazon

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

type amazonPublishMerged struct {
	MarketplaceID         string
	ProductType           string
	DefaultBrowseNodeID   string
	MerchantShippingGroup string
	ConditionType         string
	FulfillmentChannel    string
	Brand                 string
	Manufacturer          string
	DefaultWeight         string
	DefaultLength         string
	DefaultWidth          string
	DefaultHeight         string
	WeightUnit            string
	DimensionUnit         string
	Requirements          string
	IssueLocale           string
	PublishAsDraft        bool
	ExtraAmazonAttributes map[string]any
	VariationTheme        string
	ParentSellerSKU       string
}

func mergeAmazonPublish(pub map[string]any, opt map[string]any) (amazonPublishMerged, error) {
	base := loweredScalarMapAmazon(pub)
	ov := loweredScalarMapAmazon(opt)
	merged := overlayStringMapsAmazon(base, ov)
	out := amazonPublishMerged{
		MarketplaceID:         merged["marketplace_id"],
		ProductType:           merged["product_type"],
		DefaultBrowseNodeID:   merged["default_browse_node_id"],
		MerchantShippingGroup: merged["merchant_shipping_group"],
		ConditionType:         merged["condition_type"],
		FulfillmentChannel:    merged["fulfillment_channel"],
		Brand:                 merged["brand"],
		Manufacturer:          merged["manufacturer"],
		DefaultWeight:         merged["default_weight"],
		DefaultLength:         merged["default_length"],
		DefaultWidth:          merged["default_width"],
		DefaultHeight:         merged["default_height"],
		WeightUnit:            merged["weight_unit"],
		DimensionUnit:         merged["dimension_unit"],
		Requirements:          merged["requirements"],
		IssueLocale:           merged["issue_locale"],
		PublishAsDraft:        truthyAmazon(merged["publish_as_draft"]),
		VariationTheme:        merged["variation_theme"],
		ParentSellerSKU:       merged["parent_seller_sku"],
	}
	if out.Requirements == "" {
		out.Requirements = "LISTING"
	}
	attrs, err := parseAmazonAttributes(pub, opt)
	if err != nil {
		return out, err
	}
	out.ExtraAmazonAttributes = attrs
	return out, nil
}

func validateAmazonPublishConfig(m amazonPublishMerged) error {
	var missing []string
	if strings.TrimSpace(m.MarketplaceID) == "" {
		missing = append(missing, "marketplace_id")
	}
	if strings.TrimSpace(m.ProductType) == "" {
		missing = append(missing, "product_type")
	}
	if strings.TrimSpace(m.Brand) == "" {
		missing = append(missing, "brand")
	}
	if strings.TrimSpace(m.Manufacturer) == "" {
		missing = append(missing, "manufacturer")
	}
	if len(missing) > 0 {
		return fmt.Errorf("platform publish config incomplete: missing %s", strings.Join(missing, " / "))
	}
	return nil
}

func validateAmazonDraft(d platformp.PlatformProductDraft) error {
	if strings.TrimSpace(d.Title) == "" {
		return fmt.Errorf("product title is required")
	}
	if strings.TrimSpace(d.Description) == "" {
		return fmt.Errorf("product description is required")
	}
	if strings.TrimSpace(d.Currency) == "" {
		return fmt.Errorf("product currency is required")
	}
	if len(d.SKUs) == 0 {
		return fmt.Errorf("product SKU is required for publish")
	}
	mainCandidates := orderedAmazonListingImages(d.Images)
	if len(mainCandidates) == 0 {
		return fmt.Errorf("product main image required for publish")
	}
	hasMain := false
	for _, im := range mainCandidates {
		if strings.TrimSpace(strings.ToLower(im.Type)) == "main" {
			hasMain = true
			break
		}
	}
	if !hasMain {
		return fmt.Errorf("product main image required for publish")
	}
	for _, im := range mainCandidates {
		u := strings.TrimSpace(im.URL)
		if u == "" {
			continue
		}
		if !httppublic.IsPublicHTTPURL(u) {
			return fmt.Errorf("amazon product image must be publicly accessible")
		}
	}
	return nil
}

func buildAmazonListingBody(d platformp.PlatformProductDraft, sku platformp.PlatformProductSKU, cfg amazonPublishMerged, imageURLs []string, skuIndex int) (map[string]any, error) {
	attrs := map[string]any{}
	for k, v := range cfg.ExtraAmazonAttributes {
		if strings.TrimSpace(k) != "" && v != nil {
			attrs[k] = v
		}
	}
	mp := strings.TrimSpace(cfg.MarketplaceID)
	putAttrIfMissing(attrs, "item_name", amazonValueAttr(strings.TrimSpace(d.Title), mp))
	putAttrIfMissing(attrs, "brand", amazonValueAttr(strings.TrimSpace(cfg.Brand), mp))
	putAttrIfMissing(attrs, "manufacturer", amazonValueAttr(strings.TrimSpace(cfg.Manufacturer), mp))
	putAttrIfMissing(attrs, "product_description", amazonValueAttr(strings.TrimSpace(d.Description), mp))
	if strings.TrimSpace(cfg.DefaultBrowseNodeID) != "" {
		putAttrIfMissing(attrs, "recommended_browse_nodes", []any{map[string]any{"value": strings.TrimSpace(cfg.DefaultBrowseNodeID), "marketplace_id": mp}})
	}
	if strings.TrimSpace(cfg.ConditionType) != "" {
		putAttrIfMissing(attrs, "condition_type", amazonValueAttr(strings.TrimSpace(cfg.ConditionType), mp))
	}
	if strings.TrimSpace(cfg.MerchantShippingGroup) != "" {
		putAttrIfMissing(attrs, "merchant_shipping_group", amazonValueAttr(strings.TrimSpace(cfg.MerchantShippingGroup), mp))
	}
	if len(imageURLs) > 0 {
		putAttrIfMissing(attrs, "main_product_image_locator", []any{map[string]any{"media_location": imageURLs[0], "marketplace_id": mp}})
		for i, u := range imageURLs[1:] {
			if i >= 8 {
				break
			}
			key := fmt.Sprintf("other_product_image_locator_%d", i+1)
			putAttrIfMissing(attrs, key, []any{map[string]any{"media_location": u, "marketplace_id": mp}})
		}
	}
	if bullets := amazonBulletPoints(d.Attributes, cfg.ExtraAmazonAttributes); len(bullets) > 0 {
		rows := make([]any, 0, len(bullets))
		for _, b := range bullets {
			rows = append(rows, map[string]any{"value": b, "marketplace_id": mp})
		}
		putAttrIfMissing(attrs, "bullet_point", rows)
	}
	if sku.Price < 0 {
		return nil, fmt.Errorf("amazon product publish: invalid sku price")
	}
	if sku.Price > 0 {
		putAttrIfMissing(attrs, "purchasable_offer", []any{map[string]any{
			"marketplace_id": mp,
			"currency":       strings.ToUpper(strings.TrimSpace(d.Currency)),
			"our_price": []any{map[string]any{
				"schedule": []any{map[string]any{"value_with_tax": sku.Price}},
			}},
		}})
	}
	fc := strings.TrimSpace(cfg.FulfillmentChannel)
	if fc != "" {
		putAttrIfMissing(attrs, "fulfillment_availability", []any{map[string]any{
			"fulfillment_channel_code": fc,
			"quantity":                 maxAmazonInt(0, sku.Stock),
		}})
	}
	addPackageAttrs(attrs, cfg, mp)

	if len(d.SKUs) > 1 {
		parent := strings.TrimSpace(cfg.ParentSellerSKU)
		if parent == "" {
			parent = "parent-" + d.ProductID.String()
		}
		putAttrIfMissing(attrs, "parentage_level", amazonValueAttr("child", mp))
		putAttrIfMissing(attrs, "child_parent_sku_relationship", []any{map[string]any{"parent_sku": parent, "marketplace_id": mp}})
		if theme := strings.TrimSpace(cfg.VariationTheme); theme != "" {
			putAttrIfMissing(attrs, "variation_theme", amazonValueAttr(theme, mp))
		}
		if skuName := strings.TrimSpace(firstNonEmptyAmazonSKUAttr(sku, "variation_name", "option_name")); skuName != "" {
			putAttrIfMissing(attrs, "variation_name", amazonValueAttr(skuName, mp))
		} else if strings.TrimSpace(sku.SKUName) != "" {
			putAttrIfMissing(attrs, "variation_name", amazonValueAttr(strings.TrimSpace(sku.SKUName), mp))
		} else if skuIndex >= 0 {
			putAttrIfMissing(attrs, "variation_name", amazonValueAttr(fmt.Sprintf("Option %d", skuIndex+1), mp))
		}
	}

	return map[string]any{
		"productType":  strings.TrimSpace(cfg.ProductType),
		"requirements": strings.TrimSpace(cfg.Requirements),
		"attributes":   attrs,
	}, nil
}

func addPackageAttrs(attrs map[string]any, cfg amazonPublishMerged, marketplaceID string) {
	weight := strings.TrimSpace(cfg.DefaultWeight)
	weightUnit := strings.TrimSpace(cfg.WeightUnit)
	if weight != "" && weightUnit != "" {
		if val, ok := parseAmazonFloat(weight); ok {
			putAttrIfMissing(attrs, "item_package_weight", []any{map[string]any{
				"value":          val,
				"unit":           weightUnit,
				"marketplace_id": marketplaceID,
			}})
		}
	}
	dimUnit := strings.TrimSpace(cfg.DimensionUnit)
	if dimUnit == "" {
		return
	}
	length, lok := parseAmazonFloat(cfg.DefaultLength)
	width, wok := parseAmazonFloat(cfg.DefaultWidth)
	height, hok := parseAmazonFloat(cfg.DefaultHeight)
	if lok && wok && hok {
		putAttrIfMissing(attrs, "item_package_dimensions", []any{map[string]any{
			"length":         map[string]any{"value": length, "unit": dimUnit},
			"width":          map[string]any{"value": width, "unit": dimUnit},
			"height":         map[string]any{"value": height, "unit": dimUnit},
			"marketplace_id": marketplaceID,
		}})
	}
}

func amazonValueAttr(value string, marketplaceID string) []any {
	return []any{map[string]any{"value": strings.TrimSpace(value), "marketplace_id": strings.TrimSpace(marketplaceID)}}
}

func putAttrIfMissing(attrs map[string]any, key string, val any) {
	if attrs == nil || strings.TrimSpace(key) == "" || val == nil {
		return
	}
	if _, exists := attrs[key]; exists {
		return
	}
	attrs[key] = val
}

func amazonSellerSKU(sku platformp.PlatformProductSKU) string {
	if v := strings.TrimSpace(sku.SKUCode); v != "" {
		return v
	}
	if sku.LocalSKUID != uuid.Nil {
		return sku.LocalSKUID.String()
	}
	return ""
}

func orderedAmazonListingImages(images []platformp.PlatformProductImage) []platformp.PlatformProductImage {
	type tagged struct {
		im   platformp.PlatformProductImage
		rank int
		ord  int
	}
	var rows []tagged
	for _, im := range images {
		t := strings.ToLower(strings.TrimSpace(im.Type))
		rank := 3
		switch t {
		case "main":
			rank = 0
		case "detail", "description":
			rank = 1
		case "sku":
			rank = 2
		}
		rows = append(rows, tagged{im: im, rank: rank, ord: im.SortOrder})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return rows[i].ord < rows[j].ord
	})
	out := make([]platformp.PlatformProductImage, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.im)
	}
	return out
}

func collectAmazonPublicImageURLs(images []platformp.PlatformProductImage) ([]string, error) {
	out := make([]string, 0, len(images))
	for _, im := range images {
		u := strings.TrimSpace(im.URL)
		if u == "" {
			continue
		}
		if !httppublic.IsPublicHTTPURL(u) {
			return nil, fmt.Errorf("amazon product image must be publicly accessible")
		}
		out = append(out, u)
		if len(out) >= 9 {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("product main image required for publish")
	}
	return out, nil
}

func parseAmazonAttributes(pub map[string]any, opt map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for _, src := range []map[string]any{pub, opt} {
		if src == nil {
			continue
		}
		for _, key := range []string{"amazon_attributes", "attributes"} {
			raw, ok := pickAnyCI(src, key)
			if !ok || raw == nil {
				continue
			}
			m, err := anyToStringAnyMap(raw)
			if err != nil {
				return nil, fmt.Errorf("amazon_attributes must be a JSON object")
			}
			for k, v := range m {
				if strings.TrimSpace(k) != "" && v != nil {
					out[k] = v
				}
			}
		}
	}
	return out, nil
}

func amazonBulletPoints(productAttrs map[string]any, extra map[string]any) []string {
	for _, src := range []map[string]any{extra, productAttrs} {
		if src == nil {
			continue
		}
		for _, key := range []string{"bullet_points", "bullet_point", "highlights"} {
			if raw, ok := pickAnyCI(src, key); ok {
				return stringSliceFromAny(raw, 5, 500)
			}
		}
	}
	return nil
}

func loweredScalarMapAmazon(in map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk == "" {
			continue
		}
		out[kk] = coerceAmazonScalar(v)
	}
	return out
}

func overlayStringMapsAmazon(base, over map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		if strings.TrimSpace(v) != "" {
			out[k] = strings.TrimSpace(v)
		}
	}
	return out
}

func coerceAmazonScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", x))
	}
}

func truthyAmazon(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func pickAnyCI(m map[string]any, key string) (any, bool) {
	want := strings.ToLower(strings.TrimSpace(key))
	for k, v := range m {
		if strings.ToLower(strings.TrimSpace(k)) == want {
			return v, true
		}
	}
	return nil, false
}

func anyToStringAnyMap(raw any) (map[string]any, error) {
	switch v := raw.(type) {
	case map[string]any:
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return map[string]any{}, nil
		}
		var out map[string]any
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			return nil, err
		}
		return out, nil
	default:
		body, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var out map[string]any
		if err := json.Unmarshal(body, &out); err != nil {
			return nil, err
		}
		return out, nil
	}
}

func stringSliceFromAny(raw any, limit int, maxLen int) []string {
	var vals []string
	switch v := raw.(type) {
	case []string:
		vals = append(vals, v...)
	case []any:
		for _, it := range v {
			vals = append(vals, strings.TrimSpace(fmt.Sprint(it)))
		}
	case string:
		for _, p := range strings.Split(v, "\n") {
			vals = append(vals, strings.TrimSpace(p))
		}
	}
	out := make([]string, 0, len(vals))
	for _, s := range vals {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		r := []rune(s)
		if maxLen > 0 && len(r) > maxLen {
			s = string(r[:maxLen])
		}
		out = append(out, s)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func parseAmazonFloat(s string) (float64, bool) {
	v := strings.TrimSpace(s)
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 0, false
	}
	return f, true
}

func maxAmazonInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstNonEmptyAmazonSKUAttr(sku platformp.PlatformProductSKU, keys ...string) string {
	for _, k := range keys {
		if v, ok := pickAnyCI(sku.Attrs, k); ok {
			if s := strings.TrimSpace(fmt.Sprint(v)); s != "" {
				return s
			}
		}
	}
	return ""
}
