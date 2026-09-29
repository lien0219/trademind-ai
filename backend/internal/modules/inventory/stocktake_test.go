package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func createStocktake(t *testing.T, fx *warehouseLedgerFixture, key string) *InventoryStocktake {
	t.Helper()
	row, err := fx.service.CreateInventoryStocktake(context.Background(), 1, nil, CreateInventoryStocktakeBody{
		IdempotencyKey: key,
		WarehouseID:    fx.main.ID,
		Reason:         "cycle count",
		Items:          []CreateInventoryStocktakeItem{{ProductSKUID: fx.sku.ID}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != StocktakeCounting || row.Revision != 1 || len(row.Items) != 1 {
		t.Fatalf("unexpected stocktake: %#v", row)
	}
	if row.Items[0].SnapshotOnHand != 10 || row.Items[0].SnapshotVersion != 1 {
		t.Fatalf("unexpected stocktake snapshot: %#v", row.Items[0])
	}
	return row
}

func TestInventoryStocktakeLifecyclePostsDifferenceIdempotently(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, true)
	ctx := context.Background()
	row := createStocktake(t, fx, "stocktake-create-001")
	itemID := row.Items[0].ID

	counted, err := fx.service.UpdateInventoryStocktakeItem(ctx, 1, row.ID, itemID, nil, InventoryStocktakeItemBody{
		ExpectedRevision: 1, IdempotencyKey: "stocktake-count-001", CountedOnHand: intPtr(8), Remark: "first count",
	})
	if err != nil || counted.Revision != 2 || counted.Items[0].CountedOnHand == nil || *counted.Items[0].CountedOnHand != 8 {
		t.Fatalf("unexpected first count: row=%#v err=%v", counted, err)
	}
	replay, err := fx.service.UpdateInventoryStocktakeItem(ctx, 1, row.ID, itemID, nil, InventoryStocktakeItemBody{
		ExpectedRevision: 1, IdempotencyKey: "stocktake-count-001", CountedOnHand: intPtr(8), Remark: "first count",
	})
	if err != nil || replay.Revision != 2 {
		t.Fatalf("expected idempotent count replay: row=%#v err=%v", replay, err)
	}
	counted, err = fx.service.UpdateInventoryStocktakeItem(ctx, 1, row.ID, itemID, nil, InventoryStocktakeItemBody{
		ExpectedRevision: 2, IdempotencyKey: "stocktake-count-002", CountedOnHand: intPtr(9), Remark: "verified count",
	})
	if err != nil || counted.Revision != 3 || *counted.Items[0].CountedOnHand != 9 {
		t.Fatalf("unexpected corrected count: row=%#v err=%v", counted, err)
	}
	if _, err := fx.service.SubmitInventoryStocktake(ctx, 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: 3, IdempotencyKey: "stocktake-submit-001"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.ApproveInventoryStocktake(ctx, 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: 4, IdempotencyKey: "stocktake-approve-001"}); err != nil {
		t.Fatal(err)
	}
	posted, err := fx.service.PostInventoryStocktake(ctx, 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: 5, IdempotencyKey: "stocktake-post-001"})
	if err != nil || posted.Status != StocktakePosted || posted.Revision != 6 {
		t.Fatalf("unexpected posted stocktake: row=%#v err=%v", posted, err)
	}
	postedReplay, err := fx.service.PostInventoryStocktake(ctx, 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: 5, IdempotencyKey: "stocktake-post-001"})
	if err != nil || postedReplay.Status != StocktakePosted || postedReplay.Revision != 6 {
		t.Fatalf("expected post replay: row=%#v err=%v", postedReplay, err)
	}

	var balance WarehouseStockBalance
	if err := fx.db.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 1, fx.main.ID, fx.sku.ID).First(&balance).Error; err != nil {
		t.Fatal(err)
	}
	if balance.OnHand != 9 || balance.Version != 2 {
		t.Fatalf("unexpected posted balance: %#v", balance)
	}
	var sku product.ProductSKU
	if err := fx.db.First(&sku, "id = ?", fx.sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if sku.Stock == nil || *sku.Stock != 9 {
		t.Fatalf("unexpected aggregate projection: %#v", sku)
	}
	var movementCount, logCount int64
	if err := fx.db.Model(&InventoryMovement{}).Where("source_type = ? AND source_id = ?", "inventory_stocktake", row.ID).Count(&movementCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(&InventoryChangeLog{}).Where("change_type = ? AND business_event_key = ?", ChangeStocktakeAdjust, "stocktake:"+row.ID.String()+":post:"+fx.sku.ID.String()).Count(&logCount).Error; err != nil {
		t.Fatal(err)
	}
	if movementCount != 1 || logCount != 1 {
		t.Fatalf("expected one stocktake fact, movements=%d logs=%d", movementCount, logCount)
	}
}

func TestInventoryStocktakeRejectsStaleSnapshotWithoutPartialPost(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, true)
	ctx := context.Background()
	row := createStocktake(t, fx, "stocktake-create-002")
	counted, err := fx.service.UpdateInventoryStocktakeItem(ctx, 1, row.ID, row.Items[0].ID, nil, InventoryStocktakeItemBody{ExpectedRevision: 1, IdempotencyKey: "stocktake-count-003", CountedOnHand: intPtr(11)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.AdjustWarehouseStock(ctx, 1, fx.product.ID, fx.sku.ID, AdjustStockBody{WarehouseID: fx.main.ID, Stock: 12, IdempotencyKey: "stocktake-external-adjust-001"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.SubmitInventoryStocktake(ctx, 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: counted.Revision, IdempotencyKey: "stocktake-submit-002"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.ApproveInventoryStocktake(ctx, 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: counted.Revision + 1, IdempotencyKey: "stocktake-approve-002"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fx.service.PostInventoryStocktake(ctx, 1, row.ID, nil, InventoryStocktakeActionBody{ExpectedRevision: counted.Revision + 2, IdempotencyKey: "stocktake-post-002"}); !errors.Is(err, ErrStocktakeSnapshot) {
		t.Fatalf("expected stale snapshot error, got %v", err)
	}
	stored, err := fx.service.GetInventoryStocktake(ctx, 1, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != StocktakeApproved || stored.Revision != counted.Revision+2 || stored.PostedAt != nil {
		t.Fatalf("stocktake post should roll back: %#v", stored)
	}
	var balance WarehouseStockBalance
	if err := fx.db.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 1, fx.main.ID, fx.sku.ID).First(&balance).Error; err != nil {
		t.Fatal(err)
	}
	if balance.OnHand != 12 {
		t.Fatalf("stale stocktake changed balance: %#v", balance)
	}
	var count int64
	if err := fx.db.Model(&InventoryMovement{}).Where("source_type = ? AND source_id = ?", "inventory_stocktake", row.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("stale stocktake wrote %d movements", count)
	}
}

func TestInventoryStocktakeCreateRejectsCrossTenantSKUAndConflictingReplay(t *testing.T) {
	fx := newWarehouseLedgerFixture(t, true)
	row := createStocktake(t, fx, "stocktake-create-003")
	conflict := CreateInventoryStocktakeBody{IdempotencyKey: "stocktake-create-003", WarehouseID: fx.main.ID, Items: []CreateInventoryStocktakeItem{{ProductSKUID: row.Items[0].ProductSKUID}}, Reason: "changed"}
	if _, err := fx.service.CreateInventoryStocktake(context.Background(), 1, nil, conflict); !errors.Is(err, ErrStocktakeIdempotency) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	if _, err := fx.service.CreateInventoryStocktake(context.Background(), 2, nil, CreateInventoryStocktakeBody{IdempotencyKey: "stocktake-cross-tenant", WarehouseID: fx.main.ID, Items: []CreateInventoryStocktakeItem{{ProductSKUID: fx.sku.ID}}}); !errors.Is(err, ErrStocktakeInvalidInput) {
		t.Fatalf("expected cross-tenant warehouse rejection, got %v", err)
	}
	if row.ID == uuid.Nil {
		t.Fatal("expected persisted stocktake")
	}
}
