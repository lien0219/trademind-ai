package productpublish

import (
	"fmt"
	"strconv"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// ApplyPublishOptions merges camelCase/snake overrides from the publish modal onto base settings values.
func ApplyPublishOptions(base map[string]string, options map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	if len(options) == 0 {
		return out
	}
	aliases := map[string]string{
		"categoryid":           "default_category_id",
		"shippingtemplateid":   "shipping_template_id",
		"brandid":              "default_brand_id",
		"warehouseid":          "warehouse_id",
		"logisticchannelid":    "logistic_channel_id",
		"marketplaceid":        "marketplace_id",
		"producttype":          "product_type",
		"draftonly":            "publish_as_draft",
		"merchantshippinggroup": "merchant_shipping_group",
		"browsenodeid":         "default_browse_node_id",
	}
	for rk, vv := range options {
		key := strings.ToLower(strings.TrimSpace(rk))
		tgt := strings.TrimSpace(rk)
		if alt, ok := aliases[key]; ok {
			tgt = alt
		}
		out[tgt] = coerceAnyToString(vv)
	}
	return out
}

func coerceAnyToString(v any) string {
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

func mergePublishBaseline(schema platformp.PlatformAppConfigSchema, cur map[string]string) map[string]string {
	out := map[string]string{}
	for _, f := range schema.Fields {
		out[f.Name] = publishPickField(cur, f.Name)
	}
	return out
}

func stringifyPublishMap(src map[string]string) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
