package settings

import (
	"strconv"
	"strings"
)

// PricingRuleFromMap builds a pricing rule from settings.pricing map and optional platform override.
func PricingRuleFromMap(m map[string]string, platform string) map[string]string {
	out := map[string]string{
		"markupType":       strings.TrimSpace(m["default_markup_type"]),
		"markupPercent":    strings.TrimSpace(m["default_markup_percent"]),
		"markupAmount":     strings.TrimSpace(m["default_markup_amount"]),
		"roundingMode":     strings.TrimSpace(m["default_rounding_mode"]),
		"minMarginPercent": strings.TrimSpace(m["default_min_margin_percent"]),
		"currency":         strings.TrimSpace(m["default_currency"]),
	}
	if out["markupType"] == "" {
		out["markupType"] = "percent"
	}
	if out["roundingMode"] == "" {
		out["roundingMode"] = ".99"
	}
	if out["currency"] == "" {
		out["currency"] = "CNY"
	}
	if !truthySetting(m["enable_platform_pricing_rules"]) {
		return out
	}
	plat := strings.TrimSpace(strings.ToLower(platform))
	if plat == "" {
		return out
	}
	key := plat + "_markup_percent"
	if v := strings.TrimSpace(m[key]); v != "" {
		out["markupType"] = "percent"
		out["markupPercent"] = v
	}
	return out
}

// PricingBatchMaxFromMap reads settings.pricing.batch_max_size (fallback 500).
func PricingBatchMaxFromMap(m map[string]string) int {
	max := 500
	if v := strings.TrimSpace(m["batch_max_size"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			return n
		}
	}
	return max
}

func truthySetting(v string) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	return s == "1" || s == "true" || s == "yes" || s == "on"
}
