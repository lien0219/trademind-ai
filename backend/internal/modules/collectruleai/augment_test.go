package collectruleai

import (
	"encoding/json"
	"testing"
)

func TestAugmentRuleFromDigestAddsTitle(t *testing.T) {
	digest := &PageStructureDigest{
		Candidates: json.RawMessage(`{
			"title":[{"selector":".sku-name","confidence":0.92,"count":1}],
			"price":[{"selector":".p-price","confidence":0.8,"count":1}],
			"mainImages":[{"selector":"#spec-list img","confidence":0.85,"count":3}]
		}`),
	}
	rule := json.RawMessage(`{"attributes":{"mode":"pairs","rowSelector":"li","keySelector":"dt","valueSelector":"dd"}}`)
	out, warnings, err := augmentRuleFromDigest(rule, digest, []string{"title", "price", "mainImages", "attributes"})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected augment warnings")
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"title", "price", "mainImages"} {
		if _, ok := root[k]; !ok {
			t.Fatalf("missing %s after augment", k)
		}
	}
}

func TestPickSelectorsSkipsH1WhenBetterExists(t *testing.T) {
	cands := []digestCandidate{
		{Selector: "h1", Confidence: 0.9},
		{Selector: ".sku-name", Confidence: 0.92},
	}
	sels := pickSelectors(cands, 3, true)
	if len(sels) != 1 || sels[0] != ".sku-name" {
		t.Fatalf("expected .sku-name only, got %v", sels)
	}
}
