package pricing

import (
	"strconv"
	"strings"
)

// MarkupType controls how base price is adjusted.
const (
	MarkupPercent = "percent"
	MarkupFixed   = "fixed"
	MarkupNone    = "none"
)

// Rule is the pricing rule payload (request override or settings-derived).
type Rule struct {
	MarkupType       string   `json:"markupType"`
	MarkupPercent    float64  `json:"markupPercent"`
	MarkupAmount     float64  `json:"markupAmount"`
	MinPublishPrice  *float64 `json:"minPublishPrice,omitempty"`
	MinMarginPercent float64  `json:"minMarginPercent"`
	RoundingMode     string   `json:"roundingMode"`
	ExchangeRate     *float64 `json:"exchangeRate,omitempty"`
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
	out.Normalize()
	return out
}
