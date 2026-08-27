package order

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func newWaveGinContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/v1/fulfillment-waves", nil)
	ctx.Set(ctxkey.TenantID, int64(41))
	ctx.Set(ctxkey.AdminID, uuid.NewString())
	return ctx, recorder
}

func newFulfillmentWaveFixture(t *testing.T) (*Service, *inventory.Service, *warehouse.Service, *FulfillmentWave, *Order, *FulfillmentWaveLine) {
	t.Helper()
	orders, inv, ctx, orderRow, _, warehouseRow, _ := newFulfillmentFixture(t)
	if err := orders.DB.AutoMigrate(
		&FulfillmentWave{}, &FulfillmentWaveOrder{}, &FulfillmentWaveLine{},
		&FulfillmentWaveAssignment{}, &FulfillmentWaveAction{}, &FulfillmentWavePickScan{},
		&FulfillmentWavePackVerification{}, &FulfillmentWavePackScan{},
		&warehouse.WarehouseLocation{}, &inventory.WarehouseSKUPlacement{},
	); err != nil {
		t.Fatal(err)
	}
	warehouses := &warehouse.Service{DB: orders.DB}
	orders.Warehouses = warehouses
	if _, err := inv.DeductInventoryForOrder(ctx.Request.Context(), orderRow.ID, inventory.OrderInventoryOptions{
		Reason: "wave_test_reserve", WarehouseID: &warehouseRow.ID, TenantID: &orderRow.TenantID,
	}); err != nil {
		t.Fatal(err)
	}
	wave, err := orders.CreateFulfillmentWave(ctx.Request.Context(), 41, nil, nil, CreateFulfillmentWaveInput{
		IdempotencyKey: "wave-create-test-1", WarehouseID: warehouseRow.ID, OrderIDs: []uuid.UUID{orderRow.ID}, Remark: "test wave",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(wave.Lines) != 1 {
		t.Fatalf("expected one wave line, got %#v", wave.Lines)
	}
	return orders, inv, warehouses, wave, orderRow, &wave.Lines[0]
}

func TestFulfillmentWaveRequiresPickAndPackBeforeLocalFulfillment(t *testing.T) {
	orders, inv, _, wave, orderRow, line := newFulfillmentWaveFixture(t)
	ctx, _ := newWaveGinContext(t)
	if !wave.PackingVerificationRequired {
		t.Fatal("new waves must require packing scan verification")
	}
	if err := orders.DB.Model(&FulfillmentWaveLine{}).Where("id = ?", line.ID).Update("barcode", "PACK-001").Error; err != nil {
		t.Fatal(err)
	}

	if _, err := orders.FulfillOrder(ctx, inv, orderRow.ID, FulfillOrderInput{
		IdempotencyKey: "direct-bypass-test", WarehouseID: orderRow.WarehouseID, Carrier: "Carrier", TrackingNo: "BYPASS-1",
	}, nil); !errors.Is(err, ErrFulfillmentWaveRequired) {
		t.Fatalf("active wave must block direct fulfillment, got %v", err)
	}

	started, err := orders.StartFulfillmentWave(ctx.Request.Context(), 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{ExpectedRevision: wave.Revision, IdempotencyKey: "wave-start-test-1"})
	if err != nil {
		t.Fatal(err)
	}
	picked, err := orders.RecordFulfillmentWavePick(ctx.Request.Context(), 41, nil, nil, wave.ID, RecordFulfillmentWavePickInput{
		ExpectedRevision: started.Revision, IdempotencyKey: "wave-pick-test-1",
		Lines: []FulfillmentWavePickLineInput{{LineID: line.ID, PickedQty: line.RequiredQty, ScannedBarcode: "PACK-001"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if picked.Status != FulfillmentWavePacking || picked.Orders[0].Status != FulfillmentWaveOrderReadyToPack {
		t.Fatalf("fully picked order must become ready to pack: %#v", picked)
	}
	if _, err := orders.PackFulfillmentWaveOrder(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, PackFulfillmentWaveOrderInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-pack-test-1", Carrier: "Carrier", TrackingNo: "WAVE-TRACK-1",
	}); !errors.Is(err, ErrFulfillmentWavePackVerificationRequired) {
		t.Fatalf("new waves must reject legacy packing, got %v", err)
	}
	if _, err := orders.VerifyFulfillmentWavePack(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, VerifyFulfillmentWavePackInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-verify-wrong-package", ScannedOrderNo: orderRow.OrderNo,
		Carrier: "Carrier", TrackingNo: "WAVE-TRACK-1", PackageCode: "WRONG",
		Lines: []FulfillmentWavePackLineInput{{LineID: line.ID, ScannedCode: "PACK-001", VerifiedQty: line.RequiredQty}},
	}); !errors.Is(err, ErrFulfillmentWavePackageMismatch) {
		t.Fatalf("wrong package scan must be rejected, got %v", err)
	}
	if _, err := orders.VerifyFulfillmentWavePack(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, VerifyFulfillmentWavePackInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-verify-wrong-order", ScannedOrderNo: "WRONG-ORDER",
		Carrier: "Carrier", TrackingNo: "WAVE-TRACK-1", PackageCode: "WAVE-TRACK-1",
		Lines: []FulfillmentWavePackLineInput{{LineID: line.ID, ScannedCode: "PACK-001", VerifiedQty: line.RequiredQty}},
	}); !errors.Is(err, ErrFulfillmentWaveOrderScanMismatch) {
		t.Fatalf("wrong order scan must be rejected, got %v", err)
	}
	if _, err := orders.VerifyFulfillmentWavePack(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, VerifyFulfillmentWavePackInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-verify-wrong-sku", ScannedOrderNo: orderRow.OrderNo,
		Carrier: "Carrier", TrackingNo: "WAVE-TRACK-1", PackageCode: "WAVE-TRACK-1",
		Lines: []FulfillmentWavePackLineInput{{LineID: line.ID, ScannedCode: "WRONG", VerifiedQty: line.RequiredQty}},
	}); !errors.Is(err, ErrFulfillmentWavePackScanMismatch) {
		t.Fatalf("wrong sku scan must be rejected, got %v", err)
	}
	var unchanged FulfillmentWave
	var rejectedAuditCount int64
	if err := orders.DB.First(&unchanged, "id = ?", wave.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := orders.DB.Model(&FulfillmentWavePackVerification{}).Where("wave_id = ?", wave.ID).Count(&rejectedAuditCount).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Revision != picked.Revision || rejectedAuditCount != 0 {
		t.Fatalf("rejected verification must not advance or create a success audit: wave=%#v audits=%d", unchanged, rejectedAuditCount)
	}
	weight := 850
	packed, err := orders.VerifyFulfillmentWavePack(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, VerifyFulfillmentWavePackInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-verify-pack-test-1", ScannedOrderNo: orderRow.OrderNo,
		Carrier: "Carrier", TrackingNo: "WAVE-TRACK-1", PackageCode: "wave-track-1", ActualWeightGrams: &weight,
		Lines: []FulfillmentWavePackLineInput{{LineID: line.ID, ScannedCode: "pack-001", VerifiedQty: line.RequiredQty}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(packed.PackVerifications) != 1 || len(packed.PackScans) != 1 || packed.Orders[0].PackageCode != "wave-track-1" ||
		packed.Orders[0].ActualWeightGrams == nil || *packed.Orders[0].ActualWeightGrams != weight {
		t.Fatalf("packing verification must preserve immutable scan and weight facts: %#v", packed)
	}
	replayed, err := orders.VerifyFulfillmentWavePack(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, VerifyFulfillmentWavePackInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-verify-pack-test-1", ScannedOrderNo: orderRow.OrderNo,
		Carrier: "Carrier", TrackingNo: "WAVE-TRACK-1", PackageCode: "wave-track-1", ActualWeightGrams: &weight,
		Lines: []FulfillmentWavePackLineInput{{LineID: line.ID, ScannedCode: "pack-001", VerifiedQty: line.RequiredQty}},
	})
	if err != nil || len(replayed.PackVerifications) != 1 || len(replayed.PackScans) != 1 {
		t.Fatalf("packing verification replay must not duplicate audit rows: wave=%#v err=%v", replayed, err)
	}
	completed, err := orders.CompleteFulfillmentWave(ctx, inv, 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{
		ExpectedRevision: packed.Revision, IdempotencyKey: "wave-complete-test-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Wave.Status != FulfillmentWaveCompleted || completed.Succeeded != 1 || completed.Failed != 0 {
		t.Fatalf("unexpected completion result: %#v", completed)
	}
	completedReplay, err := orders.CompleteFulfillmentWave(ctx, inv, 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{
		ExpectedRevision: packed.Revision, IdempotencyKey: "wave-complete-test-replay-after-refresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if completedReplay.Wave.Status != FulfillmentWaveCompleted || completedReplay.Succeeded != 1 {
		t.Fatalf("completion replay must return the persisted run result: %#v", completedReplay)
	}
	var assignmentCount, shipmentCount int64
	if err := orders.DB.Model(&FulfillmentWaveAssignment{}).Where("tenant_id = ? AND order_id = ?", 41, orderRow.ID).Count(&assignmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := orders.DB.Model(&OrderShipment{}).Where("order_id = ?", orderRow.ID).Count(&shipmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if assignmentCount != 0 || shipmentCount != 1 {
		t.Fatalf("completion must release assignment and create one shipment, assignments=%d shipments=%d", assignmentCount, shipmentCount)
	}
}

func TestFulfillmentWaveShortageBlocksPackingAndCancelKeepsReservation(t *testing.T) {
	orders, _, _, wave, orderRow, line := newFulfillmentWaveFixture(t)
	if err := orders.DB.Model(&FulfillmentWaveLine{}).Where("id = ?", line.ID).Update("barcode", "SHORTAGE-SKU").Error; err != nil {
		t.Fatal(err)
	}
	ctx, _ := newWaveGinContext(t)
	started, err := orders.StartFulfillmentWave(ctx.Request.Context(), 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{ExpectedRevision: wave.Revision, IdempotencyKey: "wave-start-shortage"})
	if err != nil {
		t.Fatal(err)
	}
	picked, err := orders.RecordFulfillmentWavePick(ctx.Request.Context(), 41, nil, nil, wave.ID, RecordFulfillmentWavePickInput{
		ExpectedRevision: started.Revision, IdempotencyKey: "wave-pick-shortage",
		Lines: []FulfillmentWavePickLineInput{{LineID: line.ID, PickedQty: line.RequiredQty - 1, ShortageQty: 1, ScannedBarcode: "SHORTAGE-SKU"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if picked.Orders[0].Status != FulfillmentWaveOrderBlocked || picked.ShortageQty != 1 {
		t.Fatalf("shortage must block packing: %#v", picked)
	}
	if _, err := orders.VerifyFulfillmentWavePack(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, VerifyFulfillmentWavePackInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-pack-shortage", ScannedOrderNo: orderRow.OrderNo,
		Carrier: "Carrier", TrackingNo: "SHORT-1", PackageCode: "SHORT-1",
		Lines: []FulfillmentWavePackLineInput{{LineID: line.ID, ScannedCode: "SHORTAGE-SKU", VerifiedQty: line.RequiredQty}},
	}); !errors.Is(err, ErrFulfillmentWavePackingIncomplete) {
		t.Fatalf("shortage order must not pack, got %v", err)
	}
	cancelled, err := orders.CancelFulfillmentWave(ctx.Request.Context(), 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{ExpectedRevision: picked.Revision, IdempotencyKey: "wave-cancel-shortage"})
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != FulfillmentWaveCancelled {
		t.Fatalf("unexpected cancelled wave: %#v", cancelled)
	}
	var reserveCount, assignmentCount int64
	if err := orders.DB.Model(&inventory.OrderInventoryEffect{}).Where("tenant_id = ? AND order_id = ? AND effect_type = ? AND status = ?", 41, orderRow.ID, inventory.EffectTypeReserve, inventory.InventoryEffectSuccess).Count(&reserveCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := orders.DB.Model(&FulfillmentWaveAssignment{}).Where("tenant_id = ? AND order_id = ?", 41, orderRow.ID).Count(&assignmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if reserveCount != 1 || assignmentCount != 0 {
		t.Fatalf("cancel must retain reservation and release only wave ownership, reserves=%d assignments=%d", reserveCount, assignmentCount)
	}
}

func TestFulfillmentWaveWriteRequiresStoreOperateGrant(t *testing.T) {
	orders, _, _, wave, orderRow, line := newFulfillmentWaveFixture(t)
	shopID := uuid.New()
	if err := orders.DB.Model(&FulfillmentWaveOrder{}).Where("tenant_id = ? AND wave_id = ?", 41, wave.ID).Update("shop_id", shopID).Error; err != nil {
		t.Fatal(err)
	}
	principal := &adminperm.Principal{
		Role: adminperm.RoleOperator,
		StoreGrants: []adminperm.StoreGrant{{
			StoreID: shopID, PermissionScope: "view",
		}},
	}
	if _, err := orders.StartFulfillmentWave(context.Background(), 41, principal, nil, wave.ID, FulfillmentWaveRevisionInput{
		ExpectedRevision: wave.Revision, IdempotencyKey: "wave-start-store-view-only",
	}); !errors.Is(err, ErrFulfillmentWaveStorePermission) {
		t.Fatalf("view-only store grant must not advance a wave, got %v", err)
	}
	if _, err := orders.VerifyFulfillmentWavePack(context.Background(), 41, principal, nil, wave.ID, orderRow.ID, VerifyFulfillmentWavePackInput{
		ExpectedRevision: wave.Revision, IdempotencyKey: "wave-pack-store-view-only", ScannedOrderNo: orderRow.OrderNo,
		Carrier: "Carrier", TrackingNo: "TRACK-1", PackageCode: "TRACK-1",
		Lines: []FulfillmentWavePackLineInput{{LineID: line.ID, ScannedCode: line.SKUCode, VerifiedQty: line.RequiredQty}},
	}); !errors.Is(err, ErrFulfillmentWaveStorePermission) {
		t.Fatalf("view-only store grant must not verify packing, got %v", err)
	}
}

func TestFulfillmentWavePickValidatesFrozenBarcodeAndLocation(t *testing.T) {
	orders, _, _, wave, _, line := newFulfillmentWaveFixture(t)
	if err := orders.DB.Model(&FulfillmentWaveLine{}).Where("id = ?", line.ID).Updates(map[string]any{
		"barcode": "SCAN-001", "location_code": "A-01", "location_name": "Rack A01",
	}).Error; err != nil {
		t.Fatal(err)
	}
	ctx, _ := newWaveGinContext(t)
	started, err := orders.StartFulfillmentWave(ctx.Request.Context(), 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{ExpectedRevision: wave.Revision, IdempotencyKey: "wave-start-scan"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orders.RecordFulfillmentWavePick(ctx.Request.Context(), 41, nil, nil, wave.ID, RecordFulfillmentWavePickInput{
		ExpectedRevision: started.Revision, IdempotencyKey: "wave-pick-scan-wrong",
		Lines: []FulfillmentWavePickLineInput{{LineID: line.ID, PickedQty: line.RequiredQty, ScannedBarcode: "WRONG", ScannedLocationCode: "A-01"}},
	}); !errors.Is(err, ErrFulfillmentWaveScanMismatch) {
		t.Fatalf("wrong barcode must be rejected, got %v", err)
	}
	var unchanged FulfillmentWave
	if err := orders.DB.First(&unchanged, "id = ?", wave.ID).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.Revision != started.Revision {
		t.Fatalf("rejected scan must not advance revision: %#v", unchanged)
	}
	picked, err := orders.RecordFulfillmentWavePick(ctx.Request.Context(), 41, nil, nil, wave.ID, RecordFulfillmentWavePickInput{
		ExpectedRevision: started.Revision, IdempotencyKey: "wave-pick-scan-right",
		Lines: []FulfillmentWavePickLineInput{{LineID: line.ID, PickedQty: line.RequiredQty, ScannedBarcode: "scan-001", ScannedLocationCode: "a-01"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if picked.Status != FulfillmentWavePacking || len(picked.PickScans) != 1 || picked.PickScans[0].Validated != true {
		t.Fatalf("successful scan must be audited: %#v", picked)
	}
	if picked.PickScans[0].ScannedBarcode != "scan-001" || picked.PickScans[0].ExpectedLocation != "A-01" {
		t.Fatalf("scan audit must preserve operator and expected values: %#v", picked.PickScans[0])
	}
}
