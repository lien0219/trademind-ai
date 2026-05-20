package collect

import (
	"encoding/json"
	"testing"
)

func TestLooksLikePriceText(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"¥59", true},
		{"CNY", false},
		{"59.00", true},
		{"", false},
	}
	for _, c := range cases {
		if got := looksLikePriceText(c.in); got != c.want {
			t.Fatalf("looksLikePriceText(%q) = %v want %v", c.in, got, c.want)
		}
	}
}

func TestParsePriceCurrency(t *testing.T) {
	p, c := parsePriceCurrency("¥59.00")
	if p != 59 || c != "CNY" {
		t.Fatalf("got price=%v currency=%q", p, c)
	}
}

func TestNormalizeCustomImportFixesCurrency(t *testing.T) {
	raw := json.RawMessage(`{
		"source":"custom",
		"title":"Test",
		"currency":"¥59",
		"mainImages":["https://example.com/a.jpg"],
		"raw":{"productPrice":null}
	}`)
	n, err := parseNormalized(raw)
	if err != nil {
		t.Fatal(err)
	}
	params, _ := normalizeCustomImport("custom", n, raw)
	if params.Currency != "CNY" {
		t.Fatalf("currency=%q want CNY", params.Currency)
	}
	if len(params.SKUs) != 1 || params.SKUs[0].Price == nil || *params.SKUs[0].Price != 59 {
		t.Fatalf("expected default sku price 59, got %+v", params.SKUs)
	}
}
