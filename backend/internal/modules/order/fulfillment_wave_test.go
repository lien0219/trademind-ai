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
		Lines: []FulfillmentWavePickLineInput{{LineID: line.ID, PickedQty: line.RequiredQty}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if picked.Status != FulfillmentWavePacking || picked.Orders[0].Status != FulfillmentWaveOrderReadyToPack {
		t.Fatalf("fully picked order must become ready to pack: %#v", picked)
	}
	packed, err := orders.PackFulfillmentWaveOrder(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, PackFulfillmentWaveOrderInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-pack-test-1", Carrier: "Carrier", TrackingNo: "WAVE-TRACK-1",
	})
	if err != nil {
		t.Fatal(err)
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
	replayed, err := orders.CompleteFulfillmentWave(ctx, inv, 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{
		ExpectedRevision: packed.Revision, IdempotencyKey: "wave-complete-test-replay-after-refresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Wave.Status != FulfillmentWaveCompleted || replayed.Succeeded != 1 {
		t.Fatalf("completion replay must return the persisted run result: %#v", replayed)
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
	ctx, _ := newWaveGinContext(t)
	started, err := orders.StartFulfillmentWave(ctx.Request.Context(), 41, nil, nil, wave.ID, FulfillmentWaveRevisionInput{ExpectedRevision: wave.Revision, IdempotencyKey: "wave-start-shortage"})
	if err != nil {
		t.Fatal(err)
	}
	picked, err := orders.RecordFulfillmentWavePick(ctx.Request.Context(), 41, nil, nil, wave.ID, RecordFulfillmentWavePickInput{
		ExpectedRevision: started.Revision, IdempotencyKey: "wave-pick-shortage",
		Lines: []FulfillmentWavePickLineInput{{LineID: line.ID, PickedQty: line.RequiredQty - 1, ShortageQty: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if picked.Orders[0].Status != FulfillmentWaveOrderBlocked || picked.ShortageQty != 1 {
		t.Fatalf("shortage must block packing: %#v", picked)
	}
	if _, err := orders.PackFulfillmentWaveOrder(ctx.Request.Context(), 41, nil, nil, wave.ID, orderRow.ID, PackFulfillmentWaveOrderInput{
		ExpectedRevision: picked.Revision, IdempotencyKey: "wave-pack-shortage", Carrier: "Carrier", TrackingNo: "SHORT-1",
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
	orders, _, _, wave, _, _ := newFulfillmentWaveFixture(t)
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
