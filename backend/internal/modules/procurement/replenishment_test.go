package procurement

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/supplier"
)

func TestListReplenishmentSuggestionsUsesAllIncomingQuantitiesAndMOQ(t *testing.T) {
	fx := newProcurementFixture(t)
	if err := fx.DB.AutoMigrate(&inventory.WarehouseTransfer{}, &inventory.WarehouseTransferItem{}); err != nil {
		t.Fatalf("migrate transfers: %v", err)
	}
	stock := 3
	if err := fx.DB.Model(&product.ProductSKU{}).Where("id = ?", fx.ProductSKU.ID).Updates(map[string]any{"stock": stock, "warning_stock": 10, "safety_stock": 4}).Error; err != nil {
		t.Fatalf("update sku thresholds: %v", err)
	}
	if err := fx.DB.Model(&supplier.SupplierSKU{}).Where("id = ?", fx.SupplierSKU.ID).Update("min_order_qty", 4).Error; err != nil {
		t.Fatalf("update supplier MOQ: %v", err)
	}
	if err := fx.DB.Create(&inventory.WarehouseStockBalance{TenantID: 1, WarehouseID: fx.Warehouse.ID, ProductSKUID: fx.ProductSKU.ID, OnHand: 3, Version: 1}).Error; err != nil {
		t.Fatalf("create balance: %v", err)
	}
	transfer := &inventory.WarehouseTransfer{TenantID: 1, TransferNo: "TRF-REPLENISH", SourceWarehouseID: uuid.New(), TargetWarehouseID: fx.Warehouse.ID, Status: inventory.TransferInTransit, Revision: 1, IdempotencyKey: "transfer-replenish-001", PayloadHash: "hash"}
	if err := fx.DB.Create(transfer).Error; err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	if err := fx.DB.Create(&inventory.WarehouseTransferItem{TenantID: 1, TransferID: transfer.ID, ProductID: fx.ProductSKU.ProductID, ProductSKUID: fx.ProductSKU.ID, Quantity: 2, ReceivedQuantity: 0}).Error; err != nil {
		t.Fatalf("create transfer item: %v", err)
	}
	po := &PurchaseOrder{TenantID: 1, PurchaseOrderNo: "PO-REPLENISH", IdempotencyKey: "po-replenish-001", PayloadHash: "hash", SupplierID: fx.Supplier.ID, WarehouseID: fx.Warehouse.ID, Status: StatusApproved, Currency: "CNY", Revision: 2}
	if err := fx.DB.Create(po).Error; err != nil {
		t.Fatalf("create purchase order: %v", err)
	}
	if err := fx.DB.Create(&PurchaseOrderItem{TenantID: 1, PurchaseOrderID: po.ID, ProductSKUID: fx.ProductSKU.ID, Quantity: 3, ReceivedQuantity: 0, UnitCostMinor: 2500, LineAmountMinor: 7500}).Error; err != nil {
		t.Fatalf("create purchase item: %v", err)
	}

	result, err := fx.Service.ListReplenishmentSuggestions(context.Background(), 1, ReplenishmentQuery{WarehouseID: fx.Warehouse.ID, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("list suggestions: %v", err)
	}
	if len(result.List) != 1 {
		t.Fatalf("expected one suggestion, got %#v", result.List)
	}
	row := result.List[0]
	if row.Status != "actionable" || row.AvailableStock != 3 || row.InTransitTransfer != 2 || row.PendingPurchase != 3 || row.Deficit != 2 || row.SuggestedQuantity != 4 || row.MinOrderQty != 4 {
		t.Fatalf("unexpected replenishment calculation: %#v", row)
	}
	if row.SupplierID == nil || *row.SupplierID != fx.Supplier.ID || row.SupplierSKUID == nil || *row.SupplierSKUID != fx.SupplierSKU.ID || row.SuggestionHash == "" {
		t.Fatalf("replenishment supplier identity/hash should be explicit: %#v", row)
	}
}

func TestCreatePurchaseOrderFromReplenishmentRevalidatesAndReplaysDraft(t *testing.T) {
	fx := newProcurementFixture(t)
	if err := fx.DB.AutoMigrate(&inventory.WarehouseTransfer{}, &inventory.WarehouseTransferItem{}); err != nil {
		t.Fatalf("migrate transfers: %v", err)
	}
	stock := 0
	if err := fx.DB.Model(&product.ProductSKU{}).Where("id = ?", fx.ProductSKU.ID).Updates(map[string]any{"stock": stock, "warning_stock": 7}).Error; err != nil {
		t.Fatalf("update sku thresholds: %v", err)
	}
	if err := fx.DB.Create(&inventory.WarehouseStockBalance{TenantID: 1, WarehouseID: fx.Warehouse.ID, ProductSKUID: fx.ProductSKU.ID, OnHand: 0, Version: 1}).Error; err != nil {
		t.Fatalf("create balance: %v", err)
	}
	ctx := context.Background()
	suggestions, err := fx.Service.ListReplenishmentSuggestions(ctx, 1, ReplenishmentQuery{WarehouseID: fx.Warehouse.ID, Page: 1, PageSize: 20})
	if err != nil || len(suggestions.List) != 1 {
		t.Fatalf("load actionable suggestion: result=%#v err=%v", suggestions, err)
	}
	suggestion := suggestions.List[0]
	supplierSKU := fx.SupplierSKU.ID
	row, err := fx.Service.CreatePurchaseOrderFromReplenishment(ctx, 1, nil, CreateReplenishmentPurchaseOrderInput{
		IdempotencyKey: "replenishment-po-001",
		WarehouseID:    fx.Warehouse.ID,
		SupplierID:     fx.Supplier.ID,
		Remark:         "补货建议人工确认",
		Items: []CreateReplenishmentPurchaseOrderItemInput{{
			ProductSKUID: fx.ProductSKU.ID, SupplierSKUID: &supplierSKU, Quantity: suggestion.SuggestedQuantity, SuggestionHash: suggestion.SuggestionHash,
		}},
	})
	if err != nil {
		t.Fatalf("create local draft: %v", err)
	}
	if row.Status != StatusDraft || row.WarehouseID != fx.Warehouse.ID || row.SupplierID != fx.Supplier.ID || len(row.Items) != 1 || row.Items[0].Quantity != suggestion.SuggestedQuantity || row.Items[0].SupplierSKUID == nil || *row.Items[0].SupplierSKUID != supplierSKU {
		t.Fatalf("unexpected replenishment draft: %#v", row)
	}
	replay, err := fx.Service.CreatePurchaseOrderFromReplenishment(ctx, 1, nil, CreateReplenishmentPurchaseOrderInput{
		IdempotencyKey: "replenishment-po-001", WarehouseID: fx.Warehouse.ID, SupplierID: fx.Supplier.ID, Remark: "补货建议人工确认",
		Items: []CreateReplenishmentPurchaseOrderItemInput{{ProductSKUID: fx.ProductSKU.ID, SupplierSKUID: &supplierSKU, Quantity: suggestion.SuggestedQuantity, SuggestionHash: suggestion.SuggestionHash}},
	})
	if err != nil || replay.ID != row.ID {
		t.Fatalf("same request should replay the draft: row=%#v err=%v", replay, err)
	}
	if err := fx.DB.Model(&product.ProductSKU{}).Where("id = ?", fx.ProductSKU.ID).Update("warning_stock", 9).Error; err != nil {
		t.Fatalf("change suggestion input: %v", err)
	}
	_, err = fx.Service.CreatePurchaseOrderFromReplenishment(ctx, 1, nil, CreateReplenishmentPurchaseOrderInput{
		IdempotencyKey: "replenishment-po-002", WarehouseID: fx.Warehouse.ID, SupplierID: fx.Supplier.ID,
		Items: []CreateReplenishmentPurchaseOrderItemInput{{ProductSKUID: fx.ProductSKU.ID, SupplierSKUID: &supplierSKU, Quantity: suggestion.SuggestedQuantity, SuggestionHash: suggestion.SuggestionHash}},
	})
	if !errors.Is(err, ErrReplenishmentStale) {
		t.Fatalf("stale suggestion should be rejected, got %v", err)
	}
}

func TestListReplenishmentSuggestionsBlocksLedgerAndSupplierAmbiguity(t *testing.T) {
	fx := newProcurementFixture(t)
	if err := fx.DB.AutoMigrate(&inventory.WarehouseTransfer{}, &inventory.WarehouseTransferItem{}); err != nil {
		t.Fatalf("migrate transfers: %v", err)
	}
	if err := fx.DB.Model(&product.ProductSKU{}).Where("id = ?", fx.ProductSKU.ID).Updates(map[string]any{"stock": 0, "warning_stock": 10}).Error; err != nil {
		t.Fatalf("update sku: %v", err)
	}
	if err := fx.DB.Create(&inventory.WarehouseStockBalance{TenantID: 1, WarehouseID: fx.Warehouse.ID, ProductSKUID: fx.ProductSKU.ID, OnHand: 1, Version: 1}).Error; err != nil {
		t.Fatalf("create balance: %v", err)
	}
	result, err := fx.Service.ListReplenishmentSuggestions(context.Background(), 1, ReplenishmentQuery{WarehouseID: fx.Warehouse.ID, Page: 1, PageSize: 20, Status: "blocked_inventory_mismatch"})
	if err != nil || len(result.List) != 1 || result.List[0].BlockReasonCode != "inventory_mismatch" {
		t.Fatalf("expected mismatch block, result=%#v err=%v", result, err)
	}

	secondProduct := &product.Product{TenantID: 1, Source: "manual", Status: product.StatusDraft, Title: "Unmigrated"}
	if err := fx.DB.Create(secondProduct).Error; err != nil {
		t.Fatalf("create second product: %v", err)
	}
	zero := 0
	secondSKU := &product.ProductSKU{ProductID: secondProduct.ID, SKUCode: "SKU-UNMIGRATED", SKUName: "Unmigrated", Stock: &zero, WarningStock: 5}
	if err := fx.DB.Create(secondSKU).Error; err != nil {
		t.Fatalf("create second sku: %v", err)
	}
	result, err = fx.Service.ListReplenishmentSuggestions(context.Background(), 1, ReplenishmentQuery{WarehouseID: fx.Warehouse.ID, Page: 1, PageSize: 20, Status: "blocked_inventory_unmigrated"})
	if err != nil || len(result.List) != 1 || result.List[0].ProductSKUID != secondSKU.ID {
		t.Fatalf("expected unmigrated block, result=%#v err=%v", result, err)
	}

	secondSupplier, err := (&supplier.Service{DB: fx.DB}).Create(context.Background(), 1, nil, supplier.CreateInput{Code: "SUP-002", Name: "Second supplier"})
	if err != nil {
		t.Fatalf("create second supplier: %v", err)
	}
	if _, err := (&supplier.Service{DB: fx.DB}).BindSKU(context.Background(), 1, secondSupplier.ID, supplier.BindSKUInput{ProductSKUID: fx.ProductSKU.ID, UnitCostMinor: 3000, Currency: "CNY", MinOrderQty: 2}); err != nil {
		t.Fatalf("bind second supplier: %v", err)
	}
	if err := fx.DB.Model(&product.ProductSKU{}).Where("id = ?", fx.ProductSKU.ID).Update("stock", 1).Error; err != nil {
		t.Fatalf("reset sku stock: %v", err)
	}
	result, err = fx.Service.ListReplenishmentSuggestions(context.Background(), 1, ReplenishmentQuery{WarehouseID: fx.Warehouse.ID, Page: 1, PageSize: 20, Status: "blocked_supplier_selection"})
	if err != nil || len(result.List) != 1 || result.List[0].BlockReasonCode != "multiple_suppliers" {
		t.Fatalf("expected supplier selection block, result=%#v err=%v", result, err)
	}
	ambiguous := result.List[0]
	var secondOption *ReplenishmentSupplierOption
	for index := range ambiguous.SupplierOptions {
		if ambiguous.SupplierOptions[index].SupplierID == secondSupplier.ID {
			secondOption = &ambiguous.SupplierOptions[index]
			break
		}
	}
	if secondOption == nil {
		t.Fatalf("expected second supplier option: %#v", ambiguous.SupplierOptions)
	}
	minimum := ((ambiguous.Deficit + secondOption.MinOrderQty - 1) / secondOption.MinOrderQty) * secondOption.MinOrderQty
	_, err = fx.Service.CreatePurchaseOrderFromReplenishment(context.Background(), 1, nil, CreateReplenishmentPurchaseOrderInput{
		IdempotencyKey: "replenishment-manual-supplier-quantity",
		WarehouseID:    fx.Warehouse.ID,
		SupplierID:     secondSupplier.ID,
		Items: []CreateReplenishmentPurchaseOrderItemInput{{
			ProductSKUID: fx.ProductSKU.ID, SupplierSKUID: &secondOption.SupplierSKUID,
			Quantity: minimum - 1, SuggestionHash: ambiguous.SuggestionHash,
		}},
	})
	if !errors.Is(err, ErrReplenishmentQuantityInvalid) {
		t.Fatalf("manual supplier quantity below MOQ-rounded deficit should fail, got %v", err)
	}
	draft, err := fx.Service.CreatePurchaseOrderFromReplenishment(context.Background(), 1, nil, CreateReplenishmentPurchaseOrderInput{
		IdempotencyKey: "replenishment-manual-supplier-valid",
		WarehouseID:    fx.Warehouse.ID,
		SupplierID:     secondSupplier.ID,
		Items: []CreateReplenishmentPurchaseOrderItemInput{{
			ProductSKUID: fx.ProductSKU.ID, SupplierSKUID: &secondOption.SupplierSKUID,
			Quantity: minimum, SuggestionHash: ambiguous.SuggestionHash,
		}},
	})
	if err != nil || draft.SupplierID != secondSupplier.ID || draft.Status != StatusDraft || len(draft.Items) != 1 || draft.Items[0].Quantity != minimum {
		t.Fatalf("manual supplier selection should create one valid draft: draft=%#v err=%v", draft, err)
	}

	noSupplierProduct := &product.Product{TenantID: 1, Source: "manual", Status: product.StatusDraft, Title: "No supplier"}
	if err := fx.DB.Create(noSupplierProduct).Error; err != nil {
		t.Fatalf("create no-supplier product: %v", err)
	}
	noSupplierStock := 0
	noSupplierSKU := &product.ProductSKU{ProductID: noSupplierProduct.ID, SKUCode: "SKU-NO-SUPPLIER", SKUName: "No supplier", Stock: &noSupplierStock, WarningStock: 5}
	if err := fx.DB.Create(noSupplierSKU).Error; err != nil {
		t.Fatalf("create no-supplier sku: %v", err)
	}
	if err := fx.DB.Create(&inventory.WarehouseStockBalance{TenantID: 1, WarehouseID: fx.Warehouse.ID, ProductSKUID: noSupplierSKU.ID, OnHand: 0, Version: 1}).Error; err != nil {
		t.Fatalf("create no-supplier balance: %v", err)
	}
	result, err = fx.Service.ListReplenishmentSuggestions(context.Background(), 1, ReplenishmentQuery{WarehouseID: fx.Warehouse.ID, Page: 1, PageSize: 20, Status: "blocked_supplier_missing"})
	if err != nil || len(result.List) != 1 || result.List[0].ProductSKUID != noSupplierSKU.ID || result.List[0].BlockReasonCode != "supplier_missing" {
		t.Fatalf("expected missing supplier block, result=%#v err=%v", result, err)
	}
}

func TestListReplenishmentSuggestionsReconcilesDamagedStockAsNonSellable(t *testing.T) {
	fx := newProcurementFixture(t)
	if err := fx.DB.AutoMigrate(&inventory.WarehouseTransfer{}, &inventory.WarehouseTransferItem{}); err != nil {
		t.Fatalf("migrate transfers: %v", err)
	}
	projection := 8
	if err := fx.DB.Model(&product.ProductSKU{}).Where("id = ?", fx.ProductSKU.ID).
		Updates(map[string]any{"stock": projection, "warning_stock": 10, "safety_stock": 4}).Error; err != nil {
		t.Fatalf("update sku thresholds: %v", err)
	}
	if err := fx.DB.Create(&inventory.WarehouseStockBalance{
		TenantID: 1, WarehouseID: fx.Warehouse.ID, ProductSKUID: fx.ProductSKU.ID,
		OnHand: 10, Reserved: 1, Damaged: 2, Version: 1,
	}).Error; err != nil {
		t.Fatalf("create damaged balance: %v", err)
	}

	result, err := fx.Service.ListReplenishmentSuggestions(context.Background(), 1, ReplenishmentQuery{
		WarehouseID: fx.Warehouse.ID, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list suggestions: %v", err)
	}
	if len(result.List) != 1 {
		t.Fatalf("expected one suggestion, got %#v", result.List)
	}
	row := result.List[0]
	if row.Status != "actionable" || row.BlockReasonCode != "" || row.AvailableStock != 7 || row.InventoryOnHandTotal != 10 || row.InventorySellableTotal != 8 {
		t.Fatalf("unexpected damaged-stock replenishment result: %#v", row)
	}
}
