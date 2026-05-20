package collectrule

import (
	"encoding/json"
	"testing"
)

func TestNormalizeFieldSpecJSON_StringShorthand(t *testing.T) {
	raw := json.RawMessage(`".sku-name"`)
	out, err := normalizeFieldSpecJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	sels, ok := m["selectors"].([]any)
	if !ok || len(sels) != 1 || sels[0] != ".sku-name" {
		t.Fatalf("unexpected selectors: %v", m["selectors"])
	}
}

func TestNormalizeFieldSpecJSON_SelectorsAsString(t *testing.T) {
	raw := json.RawMessage(`{"selectors":".p-price","attr":"text"}`)
	out, err := normalizeFieldSpecJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Selectors []string `json:"selectors"`
	}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Selectors) != 1 || m.Selectors[0] != ".p-price" {
		t.Fatalf("got %v", m.Selectors)
	}
}

func TestNormalizeRuleJSON_AIShapes(t *testing.T) {
	raw := []byte(`{
		"title": ".sku-name",
		"price": {"selectors": ".p-price", "attr": "text"},
		"mainImages": {
			"selectors": ["#spec-list img"],
			"attr": "src",
			"multiple": true,
			"attrs": ["src","data-src"],
			"filters": {"minWidth": 300}
		},
		"fallbacks": {"meta": true}
	}`)
	out, err := NormalizeRuleJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateRuleJSON(out); err != nil {
		t.Fatalf("validate: %v; out=%s", err, string(out))
	}
}
