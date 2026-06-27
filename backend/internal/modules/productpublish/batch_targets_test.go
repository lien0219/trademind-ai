package productpublish

import (
	"testing"
)

func TestMergeEffectiveConfigPriority(t *testing.T) {
	common := map[string]any{"priceRule": "common", "remark": "all"}
	overrides := PublishConfigOverrides{
		Products:       map[string]map[string]any{"p1": {"priceRule": "product"}},
		Platforms:      map[string]map[string]any{"shopee": {"imageStrategy": "platform"}},
		Shops:          map[string]map[string]any{"shop1": {"packageWeight": 1.2}},
		ProductTargets: map[string]map[string]any{"p1:shopee:shop1": {"stockStrategy": "target"}},
	}
	eff := mergeEffectiveConfig(common, overrides, "p1", "shopee", "shop1")
	if eff.Config["priceRule"] != "product" {
		t.Fatalf("expected product override on priceRule, got %v", eff.Config["priceRule"])
	}
	if eff.Config["imageStrategy"] != "platform" {
		t.Fatalf("expected platform override on imageStrategy")
	}
	if eff.Config["packageWeight"] != 1.2 {
		t.Fatalf("expected shop override on packageWeight")
	}
	if eff.Config["stockStrategy"] != "target" {
		t.Fatalf("expected productTarget override on stockStrategy")
	}
	if eff.ConfigSources["priceRule"] != "productOverride" {
		t.Fatalf("expected priceRule source productOverride, got %s", eff.ConfigSources["priceRule"])
	}
}

func TestShouldCreateForCheck(t *testing.T) {
	ready := PublishTargetCheckResult{Status: statusReady, CanCreate: true}
	warn := PublishTargetCheckResult{Status: statusWarning, CanCreate: true}
	blocked := PublishTargetCheckResult{Status: statusBlocked, CanCreate: false}

	if !shouldCreateForCheck(ready, false, true) {
		t.Fatal("ready should create by default")
	}
	if !shouldCreateForCheck(warn, false, true) {
		t.Fatal("warning should create when includeWarnings=true")
	}
	if shouldCreateForCheck(warn, false, false) {
		t.Fatal("warning should skip when includeWarnings=false")
	}
	if shouldCreateForCheck(warn, true, true) {
		t.Fatal("warning should skip when onlyReady=true")
	}
	if shouldCreateForCheck(blocked, false, true) {
		t.Fatal("blocked should never create")
	}
}

func TestConfigHashStableForMaps(t *testing.T) {
	a := configHash(map[string]any{"b": 2, "a": 1})
	b := configHash(map[string]any{"a": 1, "b": 2})
	if a != b {
		t.Fatalf("config hash should be order-independent, got %s vs %s", a, b)
	}
}

func TestBatchIdempotencyKeyStable(t *testing.T) {
	targets := []PublishTargetRef{{Platform: "shopee", ShopID: ptrString("s1")}}
	k1 := batchIdempotencyKey("admin", []string{"p2", "p1"}, targets, nil, PublishConfigOverrides{})
	k2 := batchIdempotencyKey("admin", []string{"p1", "p2"}, targets, nil, PublishConfigOverrides{})
	if k1 != k2 {
		t.Fatalf("idempotency key should be order-independent: %s vs %s", k1, k2)
	}
	if k1 == "" {
		t.Fatal("expected non-empty idempotency key")
	}
}

func TestFinalizeBatchStatus(t *testing.T) {
	st, _ := finalizeBatchStatus(2, 1, 0)
	if st != BatchPartialSuccess {
		t.Fatalf("expected partial_success, got %s", st)
	}
	st, _ = finalizeBatchStatus(0, 2, 0)
	if st != BatchFailed {
		t.Fatalf("expected failed, got %s", st)
	}
	st, _ = finalizeBatchStatus(3, 0, 1)
	if st != BatchSuccess {
		t.Fatalf("expected success, got %s", st)
	}
}
