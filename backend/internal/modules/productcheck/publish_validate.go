package productcheck

import (
	"fmt"
	"strconv"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func partnerValuePresent(val string) bool {
	v := strings.TrimSpace(val)
	if v == "" {
		return false
	}
	if strings.Contains(v, "****") {
		return true
	}
	return true
}

func ensurePartnerOpenConfigPlain(m map[string]string, sch platformp.PlatformAppConfigSchema) error {
	lower := map[string]string{}
	for k, v := range m {
		lower[strings.ToLower(strings.TrimSpace(k))] = v
	}
	for _, f := range sch.Fields {
		if !f.Required {
			continue
		}
		nk := strings.ToLower(strings.TrimSpace(f.Name))
		if partnerValuePresent(lower[nk]) {
			continue
		}
		return fmt.Errorf("platform config incomplete: please configure settings.%s first", strings.TrimSpace(sch.GroupKey))
	}
	return nil
}

func mergePublishBaselineFromSchema(schema platformp.PlatformAppConfigSchema, cur map[string]string) map[string]string {
	out := map[string]string{}
	for _, f := range schema.Fields {
		out[f.Name] = publishPickField(cur, f.Name)
	}
	return out
}

func publishPickField(cur map[string]string, fieldName string) string {
	want := strings.ToLower(strings.TrimSpace(fieldName))
	if cur == nil {
		return ""
	}
	for k, v := range cur {
		if strings.TrimSpace(strings.ToLower(k)) == want {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func validateMergedPublishAgainstSchema(schema platformp.PlatformAppConfigSchema, merged map[string]string) error {
	mm := loweredStringMap(merged)
	for _, f := range schema.Fields {
		if !f.Required {
			continue
		}
		nv := mm[strings.ToLower(strings.TrimSpace(f.Name))]
		if nv == "" {
			pubGK := strings.TrimSpace(schema.GroupKey)
			return fmt.Errorf("platform publish config incomplete: please configure settings.%s first", pubGK)
		}
	}
	return nil
}

func applyPublishOptionsStrings(base map[string]string, options map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	if len(options) == 0 {
		return out
	}
	aliases := map[string]string{
		"categoryid":            "default_category_id",
		"shippingtemplateid":    "shipping_template_id",
		"brandid":               "default_brand_id",
		"warehouseid":           "warehouse_id",
		"logisticchannelid":     "logistic_channel_id",
		"marketplaceid":         "marketplace_id",
		"producttype":           "product_type",
		"draftonly":             "publish_as_draft",
		"merchantshippinggroup": "merchant_shipping_group",
		"browsenodeid":          "default_browse_node_id",
	}
	for rk, vv := range options {
		key := strings.ToLower(strings.TrimSpace(rk))
		tgt := strings.TrimSpace(rk)
		if alt, ok := aliases[key]; ok {
			tgt = alt
		}
		out[tgt] = coerceAnyToStringReadiness(vv)
	}
	return out
}

func coerceAnyToStringReadiness(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatInt(int64(x), 10)
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

func loweredStringMap(in map[string]string) map[string]string {
	m := map[string]string{}
	for k, v := range in {
		kk := strings.ToLower(strings.TrimSpace(k))
		if kk == "" {
			continue
		}
		m[kk] = strings.TrimSpace(v)
	}
	return m
}

// supplementalPublishRequiredKeys are extra publish settings beyond schema Required flags.
func supplementalPublishRequiredKeys(platform string) []string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "shopee":
		return []string{"days_to_ship"}
	case "lazada":
		return []string{"package_weight", "package_length", "package_width", "package_height"}
	case "amazon":
		return []string{"condition_type", "fulfillment_channel"}
	default:
		return nil
	}
}
