package inventory

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

func newTransferFixture(t *testing.T) (*Service, *product.Product, *product.ProductSKU, *warehouse.Warehouse, *warehouse.Warehouse) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:warehouse_transfer_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &product.ProductSKU{}, &warehouse.Warehouse{}, &WarehouseStockBalance{}, &InventoryMovement{}, &InventoryChangeLog{}, &WarehouseTransfer{}, &WarehouseTransferItem{}, &WarehouseTransferAction{}); err != nil {
		t.Fatal(err)
	}
	ws := &warehouse.Service{DB: db}
	source, err := ws.Create(context.Background(), 7, nil, warehouse.CreateInput{Code: "MAIN", Name: "Main", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	target, err := ws.Create(context.Background(), 7, nil, warehouse.CreateInput{Code: "EAST", Name: "East"})
	if err != nil {
		t.Fatal(err)
	}
	p := &product.Product{TenantID: 7, Source: "manual", Status: product.StatusDraft, Title: "Transfer product"}
	if err := db.Create(p).Error; err != nil {
		t.Fatal(err)
	}
	stock := 10
	sku := &product.ProductSKU{ProductID: p.ID, SKUCode: "TRANSFER-1", SKUName: "Transfer SKU", Stock: &stock}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	return &Service{DB: db, Warehouses: ws}, p, sku, source, target
}

func transferActionKey(action string) string { return "transfer-" + action + "-001" }

func TestWarehouseTransferLifecycleIsIdempotent(t *testing.T) {
	svc, _, sku, source, target := newTransferFixture(t)
	ctx := context.Background()
	row, err := svc.CreateWarehouseTransfer(ctx, 7, nil, CreateWarehouseTransferBody{IdempotencyKey: "transfer-create-001", SourceWarehouseID: source.ID, TargetWarehouseID: target.ID, Items: []CreateWarehouseTransferItem{{ProductSKUID: sku.ID, Quantity: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := svc.CreateWarehouseTransfer(ctx, 7, nil, CreateWarehouseTransferBody{IdempotencyKey: "transfer-create-001", SourceWarehouseID: source.ID, TargetWarehouseID: target.ID, Items: []CreateWarehouseTransferItem{{ProductSKUID: sku.ID, Quantity: 4}}})
	if err != nil || replay.ID != row.ID {
		t.Fatalf("expected create replay, row=%#v err=%v", replay, err)
	}
	if _, err := svc.SubmitWarehouseTransfer(ctx, 7, row.ID, nil, WarehouseTransferActionBody{ExpectedRevision: 1, IdempotencyKey: transferActionKey("submit")}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ApproveWarehouseTransfer(ctx, 7, row.ID, nil, WarehouseTransferActionBody{ExpectedRevision: 2, IdempotencyKey: transferActionKey("approve")}); err != nil {
		t.Fatal(err)
	}
	dispatched, err := svc.DispatchWarehouseTransfer(ctx, 7, row.ID, nil, WarehouseTransferActionBody{ExpectedRevision: 3, IdempotencyKey: transferActionKey("dispatch")})
	if err != nil {
		t.Fatal(err)
	}
	if dispatched.Status != TransferInTransit || dispatched.Revision != 4 {
		t.Fatalf("unexpected dispatch: %#v", dispatched)
	}
	replayedDispatch, err := svc.DispatchWarehouseTransfer(ctx, 7, row.ID, nil, WarehouseTransferActionBody{ExpectedRevision: 3, IdempotencyKey: transferActionKey("dispatch")})
	if err != nil || replayedDispatch.ID != row.ID {
		t.Fatalf("expected dispatch replay: %#v %v", replayedDispatch, err)
	}
	received, err := svc.ReceiveWarehouseTransfer(ctx, 7, row.ID, nil, WarehouseTransferActionBody{ExpectedRevision: 4, IdempotencyKey: transferActionKey("receive")})
	if err != nil {
		t.Fatal(err)
	}
	if received.Status != TransferReceived || received.Revision != 5 {
		t.Fatalf("unexpected receive: %#v", received)
	}
	var sourceBalance, targetBalance WarehouseStockBalance
	if err := svc.DB.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 7, source.ID, sku.ID).First(&sourceBalance).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 7, target.ID, sku.ID).First(&targetBalance).Error; err != nil {
		t.Fatal(err)
	}
	if sourceBalance.OnHand != 6 || sourceBalance.InTransit != 0 || targetBalance.OnHand != 4 {
		t.Fatalf("unexpected balances: source=%#v target=%#v", sourceBalance, targetBalance)
	}
	var movementCount int64
	if err := svc.DB.Model(&InventoryMovement{}).Where("source_type = ? AND source_id = ?", "warehouse_transfer", row.ID).Count(&movementCount).Error; err != nil {
		t.Fatal(err)
	}
	if movementCount != 2 {
		t.Fatalf("expected dispatch and receive movements, got %d", movementCount)
	}
}

func TestWarehouseTransferRejectsRevisionAndInvalidReplay(t *testing.T) {
	svc, _, sku, source, target := newTransferFixture(t)
	row, err := svc.CreateWarehouseTransfer(context.Background(), 7, nil, CreateWarehouseTransferBody{IdempotencyKey: "transfer-create-002", SourceWarehouseID: source.ID, TargetWarehouseID: target.ID, Items: []CreateWarehouseTransferItem{{ProductSKUID: sku.ID, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitWarehouseTransfer(context.Background(), 7, row.ID, nil, WarehouseTransferActionBody{ExpectedRevision: 9, IdempotencyKey: transferActionKey("submit")}); !errors.Is(err, ErrTransferRevision) {
		t.Fatalf("expected revision error, got %v", err)
	}
	conflict := CreateWarehouseTransferBody{IdempotencyKey: "transfer-create-002", SourceWarehouseID: source.ID, TargetWarehouseID: target.ID, Items: []CreateWarehouseTransferItem{{ProductSKUID: sku.ID, Quantity: 2}}}
	if _, err := svc.CreateWarehouseTransfer(context.Background(), 7, nil, conflict); !errors.Is(err, ErrTransferIdempotency) {
		t.Fatalf("expected create idempotency error, got %v", err)
	}
}

func TestListWarehouseTransfersIncludesWarehouseLabelsAndItemCount(t *testing.T) {
	svc, _, sku, source, target := newTransferFixture(t)
	_, err := svc.CreateWarehouseTransfer(context.Background(), 7, nil, CreateWarehouseTransferBody{
		IdempotencyKey:    "transfer-list-001",
		SourceWarehouseID: source.ID,
		TargetWarehouseID: target.ID,
		Items:             []CreateWarehouseTransferItem{{ProductSKUID: sku.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.ListWarehouseTransfers(context.Background(), 7, 1, 20, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.List) != 1 {
		t.Fatalf("unexpected list result: %#v", result)
	}
	row := result.List[0]
	if row.SourceWarehouseCode != "MAIN" || row.TargetWarehouseCode != "EAST" || row.ItemCount != 1 {
		t.Fatalf("unexpected list row: %#v", row)
	}
}
