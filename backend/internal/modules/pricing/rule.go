package pricing

import (
	"strconv"
	"strings"
)

// MarkupType controls how base price is adjusted.
const (
	MarkupPercent    = "percent"
	MarkupFixed      = "fixed"
	MarkupMultiplier = "multiplier"
	MarkupNone       = "none"
)

// Rule is the pricing rule payload (request override or settings-derived).
type Rule struct {
	CostSource                string   `json:"costSource,omitempty"`
	ManualCostPrice           *float64 `json:"manualCostPrice,omitempty"`
	MarkupType                string   `json:"markupType"`
	MarkupPercent             float64  `json:"markupPercent"`
	MarkupAmount              float64  `json:"markupAmount"`
	MarkupMultiplier          float64  `json:"markupMultiplier"`
	ShippingCost              float64  `json:"shippingCost"`
	Weight                    *float64 `json:"weight,omitempty"`
	ShippingCostPerWeight     float64  `json:"shippingCostPerWeight"`
	PlatformCommissionPercent float64  `json:"platformCommissionPercent"`
	MinProfit                 float64  `json:"minProfit"`
	MinPublishPrice           *float64 `json:"minPublishPrice,omitempty"`
	MinMarginPercent          float64  `json:"minMarginPercent"`
	RoundingMode              string   `json:"roundingMode"`
	ExchangeRate              *float64 `json:"exchangeRate,omitempty"`
}

// Normalize fills defaults and clamps known fields.
func (r *Rule) Normalize() {
	if r == nil {
		return
	}
	r.MarkupType = strings.TrimSpace(strings.ToLower(r.MarkupType))
	if r.MarkupType == "" {
		r.MarkupType = MarkupPercent
	}
	if r.MarkupType == "multiple" || r.MarkupType == "倍数" {
		r.MarkupType = MarkupMultiplier
	}
	if r.MarkupType == MarkupMultiplier && r.MarkupMultiplier <= 0 {
		r.MarkupMultiplier = 1
	}
	r.CostSource = strings.TrimSpace(strings.ToLower(r.CostSource))
	if r.CostSource == "" {
		r.CostSource = "collected"
	}
	r.RoundingMode = strings.TrimSpace(strings.ToLower(r.RoundingMode))
	if r.RoundingMode == "" {
		r.RoundingMode = ".99"
	}
}

// RuleFromSettingsMap converts string map from settings.PricingRuleFromMap into Rule.
func RuleFromSettingsMap(m map[string]string) Rule {
	r := Rule{
		MarkupType:   strings.TrimSpace(m["markupType"]),
		RoundingMode: strings.TrimSpace(m["roundingMode"]),
	}
	if v := strings.TrimSpace(m["markupPercent"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.MarkupPercent = f
		}
	}
	if v := strings.TrimSpace(m["markupAmount"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.MarkupAmount = f
		}
	}
	if v := strings.TrimSpace(m["markupMultiplier"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.MarkupMultiplier = f
		}
	}
	if v := strings.TrimSpace(m["shippingCost"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.ShippingCost = f
		}
	}
	if v := strings.TrimSpace(m["shippingCostPerWeight"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.ShippingCostPerWeight = f
		}
	}
	if v := strings.TrimSpace(m["platformCommissionPercent"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.PlatformCommissionPercent = f
		}
	}
	if v := strings.TrimSpace(m["minProfit"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.MinProfit = f
		}
	}
	if v := strings.TrimSpace(m["exchangeRate"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			r.ExchangeRate = &f
		}
	}
	if v := strings.TrimSpace(m["minMarginPercent"]); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			r.MinMarginPercent = f
		}
	}
	r.Normalize()
	return r
}

// MergeRule overlays non-zero request fields onto defaults.
func MergeRule(base, override Rule) Rule {
	out := base
	override.Normalize()
	if strings.TrimSpace(override.MarkupType) != "" {
		out.MarkupType = override.MarkupType
	}
	if override.MarkupPercent != 0 {
		out.MarkupPercent = override.MarkupPercent
	}
	if override.MarkupAmount != 0 {
		out.MarkupAmount = override.MarkupAmount
	}
	if override.MarkupMultiplier != 0 {
		out.MarkupMultiplier = override.MarkupMultiplier
	}
	if override.ShippingCost != 0 {
		out.ShippingCost = override.ShippingCost
	}
	if override.Weight != nil {
		out.Weight = override.Weight
	}
	if override.ShippingCostPerWeight != 0 {
		out.ShippingCostPerWeight = override.ShippingCostPerWeight
	}
	if override.PlatformCommissionPercent != 0 {
		out.PlatformCommissionPercent = override.PlatformCommissionPercent
	}
	if override.MinProfit != 0 {
		out.MinProfit = override.MinProfit
	}
	if override.MinMarginPercent != 0 {
		out.MinMarginPercent = override.MinMarginPercent
	}
	if strings.TrimSpace(override.RoundingMode) != "" {
		out.RoundingMode = override.RoundingMode
	}
	if override.MinPublishPrice != nil {
		out.MinPublishPrice = override.MinPublishPrice
	}
	if override.ExchangeRate != nil && *override.ExchangeRate > 0 {
		out.ExchangeRate = override.ExchangeRate
	}
	if strings.TrimSpace(override.CostSource) != "" {
		out.CostSource = override.CostSource
	}
	if override.ManualCostPrice != nil && *override.ManualCostPrice >= 0 {
		out.ManualCostPrice = override.ManualCostPrice
	}
	out.Normalize()
	return out
}
