package productpublish

import (
	"testing"
)

func TestValidatePriceConfigNegativeMarkup(t *testing.T) {
	svc := &Service{}
	err := svc.validateBatchPublishConfig(
		t.Context(),
		nil,
		[]PublishTargetRef{{Platform: "shopee", ShopID: ptrString("s1")}},
		map[string]any{
			"price": map[string]any{"strategy": "cost_plus_percent", "markupValue": -1},
		},
		PublishConfigOverrides{},
	)
	if err == nil {
		t.Fatal("expected validation error for negative markup")
	}
	pe, ok := err.(*PublishConfigInvalidError)
	if !ok {
		t.Fatalf("expected PublishConfigInvalidError, got %T", err)
	}
	if pe.Title != "刊登配置不正确" {
		t.Fatalf("unexpected title: %s", pe.Title)
	}
}

func TestValidateInventoryFixedQuantity(t *testing.T) {
	svc := &Service{}
	err := svc.validateBatchPublishConfig(
		t.Context(),
		nil,
		[]PublishTargetRef{{Platform: "shopee"}},
		map[string]any{
			"inventory": map[string]any{"strategy": "fixed_quantity", "fixedQuantity": -5},
		},
		PublishConfigOverrides{},
	)
	if err == nil {
		t.Fatal("expected validation error for negative inventory")
	}
}

func TestValidatePackageWeightPositive(t *testing.T) {
	svc := &Service{}
	err := svc.validateBatchPublishConfig(
		t.Context(),
		nil,
		[]PublishTargetRef{{Platform: "shopee"}},
		map[string]any{
			"package": map[string]any{"weight": 0},
		},
		PublishConfigOverrides{},
	)
	if err == nil {
		t.Fatal("expected validation error for zero weight")
	}
}

func TestValidateProductOverrideOutOfScope(t *testing.T) {
	svc := &Service{}
	pid := "11111111-1111-1111-1111-111111111111"
	err := svc.validateBatchPublishConfig(
		t.Context(),
		nil,
		[]PublishTargetRef{{Platform: "shopee", ShopID: ptrString("s1")}},
		nil,
		PublishConfigOverrides{
			Products: map[string]map[string]any{
				pid: {"price": map[string]any{"strategy": "use_current_price"}},
			},
		},
	)
	if err == nil {
		t.Fatal("expected out-of-scope product error")
	}
}

func TestValidatePlatformOverrideOutOfScope(t *testing.T) {
	svc := &Service{}
	err := svc.validateBatchPublishConfig(
		t.Context(),
		nil,
		[]PublishTargetRef{{Platform: "shopee", ShopID: ptrString("s1")}},
		nil,
		PublishConfigOverrides{
			Platforms: map[string]map[string]any{
				"tiktok": {"image": map[string]any{"mainImageStrategy": "use_current"}},
			},
		},
	)
	if err == nil {
		t.Fatal("expected out-of-scope platform error")
	}
}

func TestValidateProductTargetKeyFormat(t *testing.T) {
	svc := &Service{}
	err := svc.validateBatchPublishConfig(
		t.Context(),
		nil,
		[]PublishTargetRef{{Platform: "shopee"}},
		nil,
		PublishConfigOverrides{
			ProductTargets: map[string]map[string]any{
				"bad-key": {"remark": "x"},
			},
		},
	)
	if err == nil {
		t.Fatal("expected bad product target key error")
	}
}

func TestDeepMergeEffectiveConfigNested(t *testing.T) {
	common := map[string]any{
		"price":  map[string]any{"strategy": "cost_plus_percent", "markupValue": 20},
		"remark": "all",
	}
	overrides := PublishConfigOverrides{
		Products: map[string]map[string]any{
			"p1": {"price": map[string]any{"markupValue": 30}},
		},
	}
	eff := mergeEffectiveConfig(common, overrides, "p1", "shopee", "shop1")
	price, ok := eff.Config["price"].(map[string]any)
	if !ok {
		t.Fatal("expected nested price map")
	}
	if price["strategy"] != "cost_plus_percent" {
		t.Fatalf("expected inherited strategy, got %v", price["strategy"])
	}
	if price["markupValue"] != 30 {
		t.Fatalf("expected overridden markupValue, got %v", price["markupValue"])
	}
	if eff.ConfigSources["price.markupValue"] != "productOverride" {
		t.Fatalf("expected productOverride source for markupValue, got %s", eff.ConfigSources["price.markupValue"])
	}
}

func TestConfigHashStableNested(t *testing.T) {
	a := configHash(map[string]any{"price": map[string]any{"b": 2, "a": 1}})
	b := configHash(map[string]any{"price": map[string]any{"a": 1, "b": 2}})
	if a != b {
		t.Fatalf("nested config hash should be order-independent: %s vs %s", a, b)
	}
}
