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
	BasePrice           float64  `json:"basePrice"`
	CostPrice           *float64 `json:"costPrice,omitempty"`
	CurrentPrice        *float64 `json:"currentPrice,omitempty"`
	LandedCost          float64  `json:"landedCost"`
	ShippingCost        float64  `json:"shippingCost"`
	CommissionFee       float64  `json:"commissionFee"`
	CalculatedPrice     float64  `json:"calculatedPrice"`
	EstimatedProfit     float64  `json:"estimatedProfit"`
	ProfitMarginPercent float64  `json:"profitMarginPercent"`
	Currency            string   `json:"currency,omitempty"`
}

// CalculatePublishPrice computes local publish price from base/cost and rule.
func CalculatePublishPrice(in CalculateInput, currency string) CalculateResult {
	rule := in.Rule
	rule.Normalize()

	base := in.BasePrice
	if in.CostPrice != nil && *in.CostPrice > 0 {
		base = *in.CostPrice
	}
	if rule.CostSource == "manual" && rule.ManualCostPrice != nil && *rule.ManualCostPrice >= 0 {
		base = *rule.ManualCostPrice
	}
	if base < 0 {
		base = 0
	}
	shipping := rule.ShippingCost
	if rule.Weight != nil && *rule.Weight > 0 && rule.ShippingCostPerWeight > 0 {
		shipping += (*rule.Weight) * rule.ShippingCostPerWeight
	}
	if shipping < 0 {
		shipping = 0
	}
	landedCost := base + shipping

	price := landedCost
	switch rule.MarkupType {
	case MarkupFixed:
		price = landedCost + rule.MarkupAmount
	case MarkupMultiplier:
		mul := rule.MarkupMultiplier
		if mul <= 0 {
			mul = 1
		}
		price = landedCost * mul
	case MarkupNone:
		price = landedCost
	default: // percent
		pct := rule.MarkupPercent
		price = landedCost * (1 + pct/100)
	}

	rate := 1.0
	if rule.ExchangeRate != nil && *rule.ExchangeRate > 0 {
		rate = *rule.ExchangeRate
		price = price * rate
	}
	landedCostTarget := landedCost * rate

	commissionPct := rule.PlatformCommissionPercent
	if commissionPct < 0 {
		commissionPct = 0
	}
	if commissionPct > 95 {
		commissionPct = 95
	}
	if rule.MinProfit > 0 {
		floor := (landedCostTarget + rule.MinProfit) / (1 - commissionPct/100)
		if price < floor {
			price = floor
		}
	}
	if rule.MinMarginPercent > 0 {
		denom := 1 - commissionPct/100 - rule.MinMarginPercent/100
		if denom > 0 {
			floor := landedCostTarget / denom
			if price < floor {
				price = floor
			}
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
	commissionFee := roundMoney(price * commissionPct / 100)
	profit := roundMoney(price - landedCostTarget - commissionFee)
	margin := 0.0
	if price > 0 {
		margin = roundMoney(profit / price * 100)
	}

	out := CalculateResult{
		BasePrice:           roundMoney(base),
		CostPrice:           in.CostPrice,
		CurrentPrice:        in.CurrentPrice,
		LandedCost:          roundMoney(landedCostTarget),
		ShippingCost:        roundMoney(shipping * rate),
		CommissionFee:       commissionFee,
		CalculatedPrice:     price,
		EstimatedProfit:     profit,
		ProfitMarginPercent: margin,
		Currency:            currency,
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
	case "9.99":
		return ladderCharmPrice(price, 10, 9, 0.99)
	case "19.90", "19.9":
		return ladderCharmPrice(price, 20, 19, 0.90)
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

func ladderCharmPrice(price float64, step int, tailInt int, suffix float64) float64 {
	if price <= 0 {
		return float64(tailInt) + suffix
	}
	ip := int(math.Floor(price))
	base := (ip / step) * step
	for i := 0; i < step*4; i++ {
		candidate := float64(base+tailInt+i*step) + suffix
		if candidate >= price {
			return candidate
		}
	}
	return price
}
