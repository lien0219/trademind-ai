package warehousefee

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

type fakeFulfillmentReader struct {
	fact order.WarehouseFeeFact
}

func (r *fakeFulfillmentReader) ListWarehouseFeeFacts(_ context.Context, _ *gorm.DB, _ int64, _ order.WarehouseFeeScope, query order.WarehouseFeeFactQuery) (*order.WarehouseFeeFactList, error) {
	return &order.WarehouseFeeFactList{List: []order.WarehouseFeeFact{r.fact}, Page: query.Page, PageSize: query.PageSize, Total: 1, TotalPages: 1}, nil
}

func (r *fakeFulfillmentReader) GetWarehouseFeeFact(_ context.Context, _ *gorm.DB, _ int64, _ order.WarehouseFeeScope, orderID uuid.UUID, _ bool) (*order.WarehouseFeeFact, error) {
	if r.fact.OrderID != orderID {
		return nil, order.ErrFulfillmentWaveNotFound
	}
	fact := r.fact
	return &fact, nil
}

func newWarehouseFeeService(t *testing.T) (*Service, warehouse.Warehouse, *fakeFulfillmentReader) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:warehouse-fee-%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&warehouse.Warehouse{}, &RateCard{}, &RateCardRevision{}, &Snapshot{}, &Adjustment{}); err != nil {
		t.Fatal(err)
	}
	warehouseRow := warehouse.Warehouse{TenantID: 7, Code: "WH-A", Name: "A warehouse", Status: warehouse.StatusActive}
	if err := db.Create(&warehouseRow).Error; err != nil {
		t.Fatal(err)
	}
	verificationID := uuid.New()
	now := time.Date(2026, 9, 8, 5, 0, 0, 0, time.UTC)
	reader := &fakeFulfillmentReader{fact: order.WarehouseFeeFact{
		OrderID: uuid.New(), OrderNo: "SO-FEE-1", Currency: "CNY", WarehouseID: warehouseRow.ID,
		WarehouseCode: warehouseRow.Code, WarehouseName: warehouseRow.Name,
		WaveID: uuid.New(), WaveNo: "FW-1", WaveRevision: 8, WaveStatus: order.FulfillmentWaveCompleted,
		WaveOrderID: uuid.New(), WaveOrderStatus: order.FulfillmentWaveOrderFulfilled,
		PackVerificationID: &verificationID, PackageCode: "PKG-1", ItemQuantity: 3, PackageQuantity: 1,
		FulfilledAt: &now, VerifiedAt: &now,
	}}
	return &Service{DB: db, Fulfillment: reader, Clock: func() time.Time { return now }}, warehouseRow, reader
}

func TestRateCardRevisionsAndImmutableConfirmation(t *testing.T) {
	svc, warehouseRow, reader := newWarehouseFeeService(t)
	ctx := context.Background()
	card, err := svc.CreateRateCard(ctx, 7, nil, CreateRateCardInput{
		WarehouseID: warehouseRow.ID, Code: "local-a", Name: "Local A", Currency: "cny",
		OutboundBaseFeeMinor: 100, PickingFeePerItemMinor: 20, PackingFeePerPackageMinor: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Preview(ctx, 7, order.WarehouseFeeScope{}, PreviewInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	if preview.AmountMinor != 190 || preview.ItemQuantity != 3 || preview.PackageQuantity != 1 || len(preview.CalculationHash) != 64 {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	var before int64
	if err := svc.DB.Model(&Snapshot{}).Count(&before).Error; err != nil || before != 0 {
		t.Fatalf("preview wrote snapshots: count=%d err=%v", before, err)
	}
	confirmInput := ConfirmInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1, WaveRevision: preview.WaveRevision, CalculationHash: preview.CalculationHash, IdempotencyKey: "warehouse-fee-confirm-1"}
	confirmed, err := svc.Confirm(ctx, 7, order.WarehouseFeeScope{}, nil, confirmInput)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Replayed || confirmed.Snapshot.AmountMinor != 190 || confirmed.Snapshot.RateCardRevision != 1 {
		t.Fatalf("unexpected confirmation: %#v", confirmed)
	}
	replayed, err := svc.Confirm(ctx, 7, order.WarehouseFeeScope{}, nil, confirmInput)
	if err != nil || !replayed.Replayed || replayed.Snapshot.ID != confirmed.Snapshot.ID {
		t.Fatalf("confirmation replay = %#v, err=%v", replayed, err)
	}
	updated, err := svc.UpdateRateCard(ctx, 7, nil, card.ID, UpdateRateCardInput{ExpectedRevision: 1, Name: "Local A v2", Currency: "CNY", OutboundBaseFeeMinor: 110, PickingFeePerItemMinor: 25, PackingFeePerPackageMinor: 35, Status: StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := svc.GetRateCardDetail(ctx, 7, card.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || len(detail.Revisions) != 2 || confirmed.Snapshot.OutboundBaseFeeMinor != 100 {
		t.Fatalf("rate history or snapshot changed: updated=%#v detail=%#v snapshot=%#v", updated, detail, confirmed.Snapshot)
	}
	if _, err := svc.Confirm(ctx, 7, order.WarehouseFeeScope{}, nil, ConfirmInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 2, WaveRevision: preview.WaveRevision, CalculationHash: preview.CalculationHash, IdempotencyKey: "warehouse-fee-confirm-2"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale calculation must conflict, got %v", err)
	}
}

func TestPreviewAndConfirmationFailClosedWhenOrderCurrencyChanges(t *testing.T) {
	svc, warehouseRow, reader := newWarehouseFeeService(t)
	ctx := context.Background()
	card, err := svc.CreateRateCard(ctx, 7, nil, CreateRateCardInput{
		WarehouseID: warehouseRow.ID, Code: "CURRENCY", Name: "Currency guard", Currency: "CNY",
		OutboundBaseFeeMinor: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Preview(ctx, 7, order.WarehouseFeeScope{}, PreviewInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	reader.fact.Currency = "USD"
	if _, err := svc.Preview(ctx, 7, order.WarehouseFeeScope{}, PreviewInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1}); !errors.Is(err, ErrConflict) {
		t.Fatalf("currency-mismatched preview must conflict, got %v", err)
	}
	if _, err := svc.Confirm(ctx, 7, order.WarehouseFeeScope{}, nil, ConfirmInput{
		OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1,
		WaveRevision: preview.WaveRevision, CalculationHash: preview.CalculationHash,
		IdempotencyKey: "warehouse-fee-currency-confirm",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("currency changed after preview must conflict, got %v", err)
	}
	var count int64
	if err := svc.DB.Model(&Snapshot{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("currency conflict wrote snapshots: count=%d err=%v", count, err)
	}
}

func TestAdjustmentsAndReversalsRemainAppendOnly(t *testing.T) {
	svc, warehouseRow, reader := newWarehouseFeeService(t)
	ctx := context.Background()
	card, err := svc.CreateRateCard(ctx, 7, nil, CreateRateCardInput{WarehouseID: warehouseRow.ID, Code: "A", Name: "A", Currency: "CNY", OutboundBaseFeeMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Preview(ctx, 7, order.WarehouseFeeScope{}, PreviewInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := svc.Confirm(ctx, 7, order.WarehouseFeeScope{}, nil, ConfirmInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1, WaveRevision: preview.WaveRevision, CalculationHash: preview.CalculationHash, IdempotencyKey: "warehouse-fee-confirm-a"})
	if err != nil {
		t.Fatal(err)
	}
	adjusted, err := svc.CreateAdjustment(ctx, 7, order.WarehouseFeeScope{}, nil, confirmed.Snapshot.ID, CreateAdjustmentInput{AmountMinor: 25, Reason: "人工补收", IdempotencyKey: "warehouse-fee-adjust-a"})
	if err != nil {
		t.Fatal(err)
	}
	reversed, err := svc.ReverseAdjustment(ctx, 7, order.WarehouseFeeScope{}, nil, confirmed.Snapshot.ID, adjusted.Adjustment.ID, ReverseAdjustmentInput{Reason: "撤销误收", IdempotencyKey: "warehouse-fee-reverse-a"})
	if err != nil {
		t.Fatal(err)
	}
	if reversed.Adjustment.AmountMinor != -25 || reversed.Adjustment.ReversesAdjustmentID == nil {
		t.Fatalf("unexpected reversal: %#v", reversed)
	}
	detail, err := svc.GetSnapshot(ctx, 7, order.WarehouseFeeScope{}, confirmed.Snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.AdjustmentCount != 2 || detail.AdjustmentMinor != 0 || detail.NetAmountMinor != 100 || len(detail.Adjustments) != 2 {
		t.Fatalf("unexpected append-only detail: %#v", detail)
	}
	if _, err := svc.ReverseAdjustment(ctx, 7, order.WarehouseFeeScope{}, nil, confirmed.Snapshot.ID, adjusted.Adjustment.ID, ReverseAdjustmentInput{Reason: "再次冲正", IdempotencyKey: "warehouse-fee-reverse-b"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("double reversal must conflict, got %v", err)
	}
	facts, err := svc.ProfitabilityFeesForOrders(ctx, 7, []uuid.UUID{reader.fact.OrderID})
	if err != nil {
		t.Fatal(err)
	}
	if facts[reader.fact.OrderID].AmountMinor != 100 || facts[reader.fact.OrderID].Status != "confirmed" || len(facts[reader.fact.OrderID].AdjustmentIDs) != 2 {
		t.Fatalf("unexpected profitability fact: %#v", facts[reader.fact.OrderID])
	}
}

func TestSnapshotScopeFailsClosed(t *testing.T) {
	svc, _, _ := newWarehouseFeeService(t)
	shopID := uuid.New()
	snapshot := Snapshot{TenantID: 7, OrderID: uuid.New(), ShopID: &shopID, OrderNo: "SO-SCOPE", WarehouseID: uuid.New(), WarehouseCode: "WH", WarehouseName: "Warehouse", WaveID: uuid.New(), WaveNo: "FW", WaveRevision: 1, WaveOrderID: uuid.New(), PackVerificationID: uuid.New(), PackageCode: "PKG", ItemQuantity: 1, PackageQuantity: 1, RateCardID: uuid.New(), RateCardRevision: 1, RateCardCode: "A", RateCardName: "A", AmountMinor: 10, Currency: "CNY", CalculationHash: stringsOfLength("a", 64), IdempotencyKey: "scope-confirm-key", RequestHash: stringsOfLength("b", 64), ConfirmedAt: time.Now().UTC()}
	if err := svc.DB.Create(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	result, err := svc.ListSnapshots(context.Background(), 7, order.WarehouseFeeScope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{}}, SnapshotListQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 0 || len(result.List) != 0 {
		t.Fatalf("empty scope leaked snapshots: %#v", result)
	}
}

func TestIdempotentReplaysStillEnforceStoreScope(t *testing.T) {
	svc, warehouseRow, reader := newWarehouseFeeService(t)
	ctx := context.Background()
	shopID := uuid.New()
	reader.fact.ShopID = &shopID
	card, err := svc.CreateRateCard(ctx, 7, nil, CreateRateCardInput{WarehouseID: warehouseRow.ID, Code: "SCOPE", Name: "Scoped", Currency: "CNY", OutboundBaseFeeMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Preview(ctx, 7, order.WarehouseFeeScope{}, PreviewInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := svc.Confirm(ctx, 7, order.WarehouseFeeScope{}, nil, ConfirmInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1, WaveRevision: preview.WaveRevision, CalculationHash: preview.CalculationHash, IdempotencyKey: "warehouse-fee-scope-confirm"})
	if err != nil {
		t.Fatal(err)
	}
	adjusted, err := svc.CreateAdjustment(ctx, 7, order.WarehouseFeeScope{}, nil, confirmed.Snapshot.ID, CreateAdjustmentInput{AmountMinor: 10, Reason: "scope check", IdempotencyKey: "warehouse-fee-scope-adjust"})
	if err != nil {
		t.Fatal(err)
	}
	deniedScope := order.WarehouseFeeScope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{}}
	if result, err := svc.Confirm(ctx, 7, deniedScope, nil, ConfirmInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1, WaveRevision: preview.WaveRevision, CalculationHash: preview.CalculationHash, IdempotencyKey: "warehouse-fee-scope-confirm"}); result != nil || err == nil {
		t.Fatalf("out-of-scope confirmation replay leaked result=%#v err=%v", result, err)
	}
	if result, err := svc.CreateAdjustment(ctx, 7, deniedScope, nil, confirmed.Snapshot.ID, CreateAdjustmentInput{AmountMinor: 10, Reason: "scope check", IdempotencyKey: "warehouse-fee-scope-adjust"}); result != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("out-of-scope adjustment replay leaked result=%#v err=%v", result, err)
	}
	if adjusted.Adjustment.SnapshotID != confirmed.Snapshot.ID {
		t.Fatalf("unexpected adjustment snapshot: %#v", adjusted)
	}
}

func TestProfitabilityFailsClosedForInvalidAdjustmentFacts(t *testing.T) {
	svc, warehouseRow, reader := newWarehouseFeeService(t)
	ctx := context.Background()
	card, err := svc.CreateRateCard(ctx, 7, nil, CreateRateCardInput{WarehouseID: warehouseRow.ID, Code: "INVALID", Name: "Invalid fact guard", Currency: "CNY", OutboundBaseFeeMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := svc.Preview(ctx, 7, order.WarehouseFeeScope{}, PreviewInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := svc.Confirm(ctx, 7, order.WarehouseFeeScope{}, nil, ConfirmInput{OrderID: reader.fact.OrderID, RateCardID: card.ID, RateCardRevision: 1, WaveRevision: preview.WaveRevision, CalculationHash: preview.CalculationHash, IdempotencyKey: "warehouse-fee-invalid-confirm"})
	if err != nil {
		t.Fatal(err)
	}
	invalid := Adjustment{TenantID: 7, SnapshotID: confirmed.Snapshot.ID, OrderID: confirmed.Snapshot.OrderID, FactType: FactReversal, AmountMinor: -10, Currency: "CNY", Reason: "missing target", IdempotencyKey: "warehouse-fee-invalid-adjust", RequestHash: stringsOfLength("c", 64)}
	if err := svc.DB.Create(&invalid).Error; err != nil {
		t.Fatal(err)
	}
	facts, err := svc.ProfitabilityFeesForOrders(ctx, 7, []uuid.UUID{reader.fact.OrderID})
	if err != nil {
		t.Fatal(err)
	}
	if fact := facts[reader.fact.OrderID]; fact.Status != "blocked" || fact.AmountMinor != 0 || fact.ReasonCode != "warehouse_fee_facts_invalid" {
		t.Fatalf("invalid adjustment fact was not blocked: %#v", fact)
	}
}

func stringsOfLength(value string, count int) string {
	result := ""
	for len(result) < count {
		result += value
	}
	return result[:count]
}
