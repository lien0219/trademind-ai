package collectruleai

import (
	"encoding/json"
	"testing"
)

func TestIsTitleOnlyRule(t *testing.T) {
	rule := json.RawMessage(`{"title":{"selectors":["h1"],"attr":"text"},"fallbacks":{"meta":true}}`)
	targets := []string{"title", "price", "mainImages"}
	if !isTitleOnlyRule(rule, targets) {
		t.Fatal("expected title-only")
	}
}

func TestTitleUsesBroadSelector(t *testing.T) {
	rule := json.RawMessage(`{"title":{"selectors":["h1"],"attr":"text"}}`)
	ok, sel := titleUsesBroadSelector(rule)
	if !ok || sel != "h1" {
		t.Fatalf("expected broad h1, got ok=%v sel=%q", ok, sel)
	}
}

func TestMissingGeneratedFields(t *testing.T) {
	rule := json.RawMessage(`{"title":{"selectors":[".sku-name"],"attr":"text"},"price":{"selectors":[".p-price"],"attr":"text"}}`)
	missing := missingGeneratedFields(rule, []string{"title", "price", "mainImages", "attributes"})
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing, got %v", missing)
	}
}

func TestComputeQualityGateBlocksTitleOnly(t *testing.T) {
	rule := json.RawMessage(`{"title":{"selectors":["h1"],"attr":"text"}}`)
	gate := computeQualityGate(rule, []string{"title", "price", "mainImages"}, nil)
	if gate.AllowSaveEnabled {
		t.Fatal("title-only should not allow enable")
	}
	if gate.Score > 30 {
		t.Fatalf("score should be capped, got %d", gate.Score)
	}
}
