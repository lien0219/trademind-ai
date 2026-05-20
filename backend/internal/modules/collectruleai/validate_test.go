package collectruleai

import (
	"encoding/json"
	"testing"
)

func TestNormalizeAndValidateRule_AISelectorString(t *testing.T) {
	raw := json.RawMessage(`{
		"title": {"selectors": ".sku-name", "attr": "text"},
		"price": {"selectors": [".p-price"], "attr": "text"},
		"mainImages": {
			"selectors": "#spec-list img",
			"attr": "src",
			"multiple": true,
			"attrs": ["src","data-src"],
			"filters": {"minWidth": 300, "minHeight": 300, "dedupeByImageKey": true}
		},
		"attributes": {"mode": "pairs", "rowSelector": ".Ptable-item dl", "keySelector": "dt", "valueSelector": "dd"},
		"fallbacks": {"meta": true, "jsonLd": true, "openGraph": true}
	}`)
	out, err := normalizeAndValidateRule(raw)
	if err != nil {
		t.Fatalf("normalizeAndValidateRule: %v", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	if _, ok := root["title"]; !ok {
		t.Fatal("missing title")
	}
}
