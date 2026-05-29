package imagetask

import "testing"

func TestIsLikelyHallucinatedMarketingText(t *testing.T) {
	cases := []struct {
		text       string
		confidence float64
		want       bool
	}{
		{"限时抢购", 0.8, true},
		{"原价: ¥128", 0.85, true},
		{"金属底座 折叠支架", 0.88, false},
		{"手机/平板", 0.9, false},
		{"Flash Sale", 0.7, true},
	}
	for _, c := range cases {
		got := isLikelyHallucinatedMarketingText(c.text, c.confidence)
		if got != c.want {
			t.Fatalf("text=%q conf=%v got=%v want=%v", c.text, c.confidence, got, c.want)
		}
	}
}

func TestFilterOCRBlocksHeuristic(t *testing.T) {
	blocks := []translateTextBlock{
		{Text: "金属底座 折叠支架", Confidence: 0.9},
		{Text: "限时抢购", Confidence: 0.8},
		{Text: "原价: ¥128", Confidence: 0.85},
		{Text: "手机/平板", Confidence: 0.92},
	}
	out, filtered := filterOCRBlocksHeuristic(blocks)
	if filtered != 2 {
		t.Fatalf("expected 2 filtered, got %d", filtered)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(out))
	}
}
