package douyinshop

import "testing"

func TestCheckWorkerExecutionGrayRelease(t *testing.T) {
	cfg := RuntimeConfig{
		RealAPIEnabled:         true,
		WriteOperationsEnabled: true,
		GrayReleaseEnabled:     true,
		GrayShopIDs:            []string{"shop-a"},
		ProductDraftEnabled:    true,
	}
	rt := RuntimeState{Status: RuntimeNormal}
	in := WorkerGuardInput{Config: cfg, Runtime: rt, Feature: FeatureProductDraft, IsWrite: true, ShopID: "shop-b"}
	if err := CheckWorkerExecution(in); err == nil || err.Code != CodeDouyinShopNotInGrayList {
		t.Fatalf("expected shop not in gray list, got %v", err)
	}
	in.ShopID = "shop-a"
	if err := CheckWorkerExecution(in); err != nil {
		t.Fatalf("expected whitelisted shop allowed, got %v", err)
	}
}

func TestCheckWorkerExecutionWriteDisabled(t *testing.T) {
	cfg := RuntimeConfig{RealAPIEnabled: true, WriteOperationsEnabled: false, ProductDraftEnabled: true}
	rt := RuntimeState{Status: RuntimeNormal}
	in := WorkerGuardInput{Config: cfg, Runtime: rt, Feature: FeatureProductDraft, IsWrite: true, ShopID: "any"}
	if err := CheckWorkerExecution(in); err == nil || err.Code != CodeDouyinWriteOperationDisabled {
		t.Fatalf("expected write disabled, got %v", err)
	}
}

func TestCheckWorkerExecutionScheduledOrderSync(t *testing.T) {
	cfg := RuntimeConfig{
		RealAPIEnabled:            true,
		WriteOperationsEnabled:    true,
		OrderSyncEnabled:          true,
		ScheduledOrderSyncEnabled: false,
	}
	rt := RuntimeState{Status: RuntimeNormal}
	in := WorkerGuardInput{Config: cfg, Runtime: rt, Feature: FeatureOrderSync, IsWrite: true, IsScheduled: true}
	if err := CheckWorkerExecution(in); err == nil {
		t.Fatal("expected scheduled order sync blocked")
	}
}

func TestParseGrayShopIDs(t *testing.T) {
	ids := parseGrayShopIDs(`["a","b"]`)
	if len(ids) != 2 || ids[0] != "a" {
		t.Fatalf("unexpected ids: %v", ids)
	}
	ids2 := parseGrayShopIDs("x, y ,x")
	if len(ids2) != 2 {
		t.Fatalf("expected dedup comma list, got %v", ids2)
	}
}

func TestShopInGrayList(t *testing.T) {
	cfg := RuntimeConfig{GrayShopIDs: []string{"ABC"}}
	if !cfg.ShopInGrayList("abc") {
		t.Fatal("expected case-insensitive match")
	}
}
