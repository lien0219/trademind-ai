package pricing

import (
	"math"
	"strings"
)

// CalculateInput is input for publish price calculation.
type CalculateInput struct {
	BasePrice       float64
	CostPrice       *float64
	CurrentPrice    *float64
	MinPublishPrice *float64
	Rule            Rule
}

// CalculateResult is the outcome of publish price calculation.
type CalculateResult struct {
	BasePrice       float64  `json:"basePrice"`
	CostPrice       *float64 `json:"costPrice,omitempty"`
	CurrentPrice    *float64 `json:"currentPrice,omitempty"`
	CalculatedPrice float64  `json:"calculatedPrice"`
	Currency        string   `json:"currency,omitempty"`
}

// CalculatePublishPrice computes local publish price from base/cost and rule.
func CalculatePublishPrice(in CalculateInput, currency string) CalculateResult {
	rule := in.Rule
	rule.Normalize()

	base := in.BasePrice
	if in.CostPrice != nil && *in.CostPrice > 0 {
		base = *in.CostPrice
	}
	if base < 0 {
		base = 0
	}

	price := base
	switch rule.MarkupType {
	case MarkupFixed:
		price = base + rule.MarkupAmount
	case MarkupNone:
		price = base
	default: // percent
		pct := rule.MarkupPercent
		price = base * (1 + pct/100)
	}

	if rule.ExchangeRate != nil && *rule.ExchangeRate > 0 {
		price = price * (*rule.ExchangeRate)
	}

	if in.CostPrice != nil && *in.CostPrice > 0 && rule.MinMarginPercent > 0 {
		floor := *in.CostPrice * (1 + rule.MinMarginPercent/100)
		if price < floor {
			price = floor
		}
	}

	minPub := rule.MinPublishPrice
	if minPub == nil {
		minPub = in.MinPublishPrice
	}
	if minPub != nil && *minPub > 0 && price < *minPub {
		price = *minPub
	}

	price = applyRounding(price, rule.RoundingMode)
	if price < 0 {
		price = 0
	}
	price = roundMoney(price)

	out := CalculateResult{
		BasePrice:       roundMoney(base),
		CostPrice:       in.CostPrice,
		CurrentPrice:    in.CurrentPrice,
		CalculatedPrice: price,
		Currency:        currency,
	}
	return out
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

func applyRounding(price float64, mode string) float64 {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "integer", "int":
		return math.Round(price)
	case ".9", "0.9", "point9":
		return charmPrice(price, 0.9)
	case ".95", "0.95", "point95":
		return charmPrice(price, 0.95)
	case ".99", "0.99", "point99":
		return charmPrice(price, 0.99)
	case "none", "":
		return price
	default:
		return price
	}
}

func charmPrice(price, suffix float64) float64 {
	if price <= 0 {
		return suffix
	}
	ip := math.Floor(price)
	candidate := ip + suffix
	if candidate >= price {
		return candidate
	}
	return ip + 1 + suffix
}
