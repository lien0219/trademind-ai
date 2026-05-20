package pricing

import "testing"

func TestCalculatePublishPrice_percent99(t *testing.T) {
	cost := 80.0
	rule := Rule{MarkupType: MarkupPercent, MarkupPercent: 30, RoundingMode: ".99"}
	out := CalculatePublishPrice(CalculateInput{
		BasePrice: cost,
		CostPrice: &cost,
		Rule:      rule,
	}, "CNY")
	// 80 * 1.3 = 104 -> charm .99 => 104.99
	if out.CalculatedPrice != 104.99 {
		t.Fatalf("got %v want 104.99", out.CalculatedPrice)
	}
}

func TestCalculatePublishPrice_minPublish(t *testing.T) {
	base := 10.0
	min := 15.0
	rule := Rule{MarkupType: MarkupNone, MinPublishPrice: &min, RoundingMode: "none"}
	out := CalculatePublishPrice(CalculateInput{BasePrice: base, Rule: rule}, "CNY")
	if out.CalculatedPrice != 15 {
		t.Fatalf("got %v want 15", out.CalculatedPrice)
	}
}
