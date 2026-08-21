package inventory

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

type orderInventoryFixture struct {
	db        *gorm.DB
	service   *Service
	order     *orderMirror
	line      *orderLineMirror
	product   *product.Product
	sku       *product.ProductSKU
	warehouse *warehouse.Warehouse
}

func newOrderInventoryFixture(t *testing.T, platform string, withWarehouseBinding, withDefault bool) *orderInventoryFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_inventory_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&product.Product{}, &product.ProductSKU{}, &warehouse.Warehouse{}, &WarehouseStockBalance{}, &InventoryMovement{},
		&InventoryChangeLog{}, &OrderInventoryEffect{}, &orderMirror{}, &orderLineMirror{},
	); err != nil {
		t.Fatal(err)
	}
	warehouseService := &warehouse.Service{DB: db}
	warehouseRow, err := warehouseService.Create(context.Background(), 41, nil, warehouse.CreateInput{Code: "ORDER-MAIN", Name: "Order main", IsDefault: withDefault})
	if err != nil {
		t.Fatal(err)
	}
	productRow := &product.Product{TenantID: 41, Source: "manual", Status: product.StatusDraft, Title: "Order ledger product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatal(err)
	}
	stock := 10
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "ORDER-LEDGER-SKU", SKUName: "Order ledger SKU", Stock: &stock, WarningStock: 2}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	orderRow := &orderMirror{TenantID: 41, Platform: platform, OrderNo: "ORDER-LEDGER-1", Status: "paid", PaymentStatus: "paid", FulfillmentStatus: "unfulfilled"}
	if withWarehouseBinding {
		orderRow.WarehouseID = &warehouseRow.ID
	}
	if err := db.Create(orderRow).Error; err != nil {
		t.Fatal(err)
	}
	line := &orderLineMirror{OrderID: orderRow.ID, ProductID: &productRow.ID, ProductSKUID: &sku.ID, Quantity: 3}
	if err := db.Create(line).Error; err != nil {
		t.Fatal(err)
	}
	return &orderInventoryFixture{
		db: db, service: &Service{DB: db, Warehouses: warehouseService}, order: orderRow, line: line,
		product: productRow, sku: sku, warehouse: warehouseRow,
	}
}

func (fx *orderInventoryFixture) reloadStock(t *testing.T) (WarehouseStockBalance, product.ProductSKU) {
	t.Helper()
	var balance WarehouseStockBalance
	if err := fx.db.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 41, fx.warehouse.ID, fx.sku.ID).First(&balance).Error; err != nil {
		t.Fatal(err)
	}
	var sku product.ProductSKU
	if err := fx.db.First(&sku, "id = ?", fx.sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	return balance, sku
}

func TestOrderInventoryReservesCommitsAndRestoresWarehouseStock(t *testing.T) {
	fx := newOrderInventoryFixture(t, "manual", true, true)
	ctx := context.Background()

	reserved, err := fx.service.DeductInventoryForOrder(ctx, fx.order.ID, OrderInventoryOptions{Reason: "paid"})
	if err != nil {
		t.Fatal(err)
	}
	if reserved.Action != EffectTypeReserve || reserved.LinesSynced != 1 {
		t.Fatalf("unexpected reservation summary: %#v", reserved)
	}
	balance, sku := fx.reloadStock(t)
	if balance.OnHand != 10 || balance.Reserved != 3 || sku.Stock == nil || *sku.Stock != 10 {
		t.Fatalf("reservation must not change on-hand projection: balance=%#v sku=%#v", balance, sku)
	}
	if _, err := fx.service.AdjustWarehouseStock(ctx, 41, fx.product.ID, fx.sku.ID, AdjustStockBody{
		WarehouseID: fx.warehouse.ID, Stock: 2, IdempotencyKey: "reserved-floor-001",
	}, nil); !errors.Is(err, ErrInvalidWarehouseAdjustment) {
		t.Fatalf("expected adjustment below reserved stock to fail, got %v", err)
	}
	replay, err := fx.service.DeductInventoryForOrder(ctx, fx.order.ID, OrderInventoryOptions{Reason: "paid"})
	if err != nil || replay.LinesSynced != 0 || replay.LinesSkipped != 1 {
		t.Fatalf("expected idempotent reservation replay, summary=%#v err=%v", replay, err)
	}

	if err := fx.db.Model(&orderMirror{}).Where("id = ?", fx.order.ID).Updates(map[string]any{"status": "shipped", "fulfillment_status": "fulfilled"}).Error; err != nil {
		t.Fatal(err)
	}
	deducted, err := fx.service.DeductInventoryForOrder(ctx, fx.order.ID, OrderInventoryOptions{Reason: "shipped"})
	if err != nil {
		t.Fatal(err)
	}
	if deducted.Action != EffectTypeDeduct || deducted.LinesSynced != 1 {
		t.Fatalf("unexpected deduction summary: %#v", deducted)
	}
	balance, sku = fx.reloadStock(t)
	if balance.OnHand != 7 || balance.Reserved != 0 || sku.Stock == nil || *sku.Stock != 7 {
		t.Fatalf("deduction must consume reservation and projection: balance=%#v sku=%#v", balance, sku)
	}

	if err := fx.db.Model(&orderMirror{}).Where("id = ?", fx.order.ID).Updates(map[string]any{"status": "refunded", "payment_status": "refunded"}).Error; err != nil {
		t.Fatal(err)
	}
	restored, err := fx.service.RestoreInventoryForOrder(ctx, fx.order.ID, OrderInventoryOptions{Reason: "refund"})
	if err != nil {
		t.Fatal(err)
	}
	if restored.Action != EffectTypeRestore || restored.LinesSynced != 1 {
		t.Fatalf("unexpected restoration summary: %#v", restored)
	}
	balance, sku = fx.reloadStock(t)
	if balance.OnHand != 10 || balance.Reserved != 0 || sku.Stock == nil || *sku.Stock != 10 {
		t.Fatalf("restore must replenish warehouse and projection: balance=%#v sku=%#v", balance, sku)
	}

	var movements []InventoryMovement
	if err := fx.db.Where("product_sku_id = ?", fx.sku.ID).Order("created_at ASC, id ASC").Find(&movements).Error; err != nil {
		t.Fatal(err)
	}
	if len(movements) != 4 {
		t.Fatalf("expected legacy import plus reserve, deduct and restore movements, got %#v", movements)
	}
	byType := make(map[string]InventoryMovement, len(movements))
	for _, movement := range movements {
		if _, exists := byType[movement.MovementType]; exists {
			t.Fatalf("duplicate movement type in order lifecycle: %#v", movements)
		}
		byType[movement.MovementType] = movement
	}
	reserve, ok := byType[MovementOrderReserve]
	if !ok || reserve.BeforeReserved != 0 || reserve.AfterReserved != 3 {
		t.Fatalf("unexpected reservation movement: %#v", reserve)
	}
	deduct, ok := byType[MovementOrderDeduct]
	if !ok || deduct.BeforeOnHand != 10 || deduct.AfterOnHand != 7 || deduct.AfterReserved != 0 {
		t.Fatalf("unexpected deduction movement: %#v", deduct)
	}
	if _, ok := byType[MovementLegacyImport]; !ok {
		t.Fatalf("expected legacy import movement: %#v", movements)
	}
	if _, ok := byType[MovementOrderRestore]; !ok {
		t.Fatalf("expected restore movement: %#v", movements)
	}
}

func TestOrderInventoryAllowsLegacyTenantZero(t *testing.T) {
	fx := newOrderInventoryFixture(t, "manual", true, true)
	if err := fx.db.Model(&warehouse.Warehouse{}).Where("tenant_id = ?", 41).Update("tenant_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(&product.Product{}).Where("tenant_id = ?", 41).Update("tenant_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(&orderMirror{}).Where("tenant_id = ?", 41).Update("tenant_id", 0).Error; err != nil {
		t.Fatal(err)
	}
	result, err := fx.service.DeductInventoryForOrder(context.Background(), fx.order.ID, OrderInventoryOptions{Reason: "legacy-paid"})
	if err != nil {
		t.Fatalf("reserve legacy tenant order: %v", err)
	}
	if result.Action != EffectTypeReserve || result.LinesSynced != 1 {
		t.Fatalf("unexpected legacy tenant reservation: %#v", result)
	}
}

func TestOrderInventoryCancellationReleasesReservationWithoutChangingProjection(t *testing.T) {
	fx := newOrderInventoryFixture(t, "manual", true, true)
	ctx := context.Background()
	if _, err := fx.service.DeductInventoryForOrder(ctx, fx.order.ID, OrderInventoryOptions{Reason: "paid"}); err != nil {
		t.Fatal(err)
	}
	if uncompensated, err := fx.service.HasUncompensatedOrderInventory(ctx, fx.order.ID); err != nil || !uncompensated {
		t.Fatalf("active reservation must block destructive order changes, uncompensated=%v err=%v", uncompensated, err)
	}
	if err := fx.db.Model(&orderMirror{}).Where("id = ?", fx.order.ID).Updates(map[string]any{"status": "cancelled"}).Error; err != nil {
		t.Fatal(err)
	}
	released, err := fx.service.RestoreInventoryForOrder(ctx, fx.order.ID, OrderInventoryOptions{Reason: "cancelled"})
	if err != nil {
		t.Fatal(err)
	}
	if released.Action != EffectTypeRelease || released.LinesSynced != 1 {
		t.Fatalf("unexpected release summary: %#v", released)
	}
	uncompensated, err := fx.service.HasUncompensatedOrderInventory(ctx, fx.order.ID)
	if err != nil || uncompensated {
		t.Fatalf("released reservation must be fully compensated, uncompensated=%v err=%v", uncompensated, err)
	}
	balance, sku := fx.reloadStock(t)
	if balance.OnHand != 10 || balance.Reserved != 0 || sku.Stock == nil || *sku.Stock != 10 {
		t.Fatalf("release must leave on-hand projection unchanged: balance=%#v sku=%#v", balance, sku)
	}
	var restoreCount int64
	if err := fx.db.Model(&OrderInventoryEffect{}).Where("order_id = ? AND effect_type = ?", fx.order.ID, EffectTypeRestore).Count(&restoreCount).Error; err != nil {
		t.Fatal(err)
	}
	if restoreCount != 0 {
		t.Fatalf("pre-shipment cancellation must not create restore effect, got %d", restoreCount)
	}
}

func TestPlatformOrderInventoryBindsActiveDefaultWarehouse(t *testing.T) {
	fx := newOrderInventoryFixture(t, "douyin_shop", false, true)
	summary, err := fx.service.DeductInventoryForOrder(context.Background(), fx.order.ID, OrderInventoryOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Action != EffectTypeReserve || summary.LinesSynced != 1 {
		t.Fatalf("unexpected platform reservation: %#v", summary)
	}
	var orderRow orderMirror
	if err := fx.db.First(&orderRow, "id = ?", fx.order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if orderRow.WarehouseID == nil || *orderRow.WarehouseID != fx.warehouse.ID {
		t.Fatalf("expected persisted default warehouse binding: %#v", orderRow.WarehouseID)
	}
}

func TestPlatformTerminalSyncCompensatesWhenAutoDeductIsDisabled(t *testing.T) {
	fx := newOrderInventoryFixture(t, "douyin_shop", false, true)
	if err := fx.db.AutoMigrate(&settings.Setting{}); err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Create(&settings.Setting{TenantID: 0, GroupKey: "inventory", ItemKey: "auto_restore_cancelled_orders", ItemValue: "true"}).Error; err != nil {
		t.Fatal(err)
	}
	fx.service.Settings = &settings.Service{DB: fx.db}
	if _, err := fx.service.DeductInventoryForOrder(context.Background(), fx.order.ID, OrderInventoryOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(&orderMirror{}).Where("id = ?", fx.order.ID).Updates(map[string]any{"status": "cancelled"}).Error; err != nil {
		t.Fatal(err)
	}
	summary, err := fx.service.DeductInventoryForOrder(context.Background(), fx.order.ID, OrderInventoryOptions{PlatformAuto: true, CompensationOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Action != EffectTypeRelease || summary.LinesSynced != 1 {
		t.Fatalf("expected terminal platform release independent of auto deduct, got %#v", summary)
	}
}

func TestManualOrderInventoryRequiresExplicitWarehouse(t *testing.T) {
	fx := newOrderInventoryFixture(t, "manual", false, true)
	_, err := fx.service.DeductInventoryForOrder(context.Background(), fx.order.ID, OrderInventoryOptions{})
	if !errors.Is(err, ErrOrderWarehouseRequired) {
		t.Fatalf("expected manual warehouse requirement, got %v", err)
	}
}

func TestLegacyDeductionBindsWarehouseOnFirstRestoreWithoutDoubleDeduct(t *testing.T) {
	fx := newOrderInventoryFixture(t, "manual", false, true)
	stock := 7
	if err := fx.db.Model(&product.ProductSKU{}).Where("id = ?", fx.sku.ID).Update("stock", stock).Error; err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Model(&orderMirror{}).Where("id = ?", fx.order.ID).Updates(map[string]any{"status": "refunded", "payment_status": "refunded"}).Error; err != nil {
		t.Fatal(err)
	}
	legacy := &OrderInventoryEffect{
		OrderID: fx.order.ID, OrderItemID: fx.line.ID, ProductID: &fx.product.ID, ProductSKUID: fx.sku.ID,
		EffectType: EffectTypeDeduct, Quantity: fx.line.Quantity, Status: InventoryEffectSuccess, BeforeStock: intPtr(10), AfterStock: intPtr(7),
	}
	if err := fx.db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	restored, err := fx.service.RestoreInventoryForOrder(context.Background(), fx.order.ID, OrderInventoryOptions{WarehouseID: &fx.warehouse.ID, Reason: "legacy_refund"})
	if err != nil {
		t.Fatal(err)
	}
	if restored.Action != EffectTypeRestore || restored.LinesSynced != 1 {
		t.Fatalf("unexpected legacy restore: %#v", restored)
	}
	balance, sku := fx.reloadStock(t)
	if balance.OnHand != 10 || sku.Stock == nil || *sku.Stock != 10 {
		t.Fatalf("legacy restore must import post-deduct stock once then replenish: balance=%#v sku=%#v", balance, sku)
	}
	if err := fx.db.First(legacy, "id = ?", legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.TenantID != 41 || legacy.WarehouseID == nil || *legacy.WarehouseID != fx.warehouse.ID {
		t.Fatalf("legacy effect was not tenant/warehouse bound: %#v", legacy)
	}
}

func TestGlobalOrderInventoryEffectsAreTenantScoped(t *testing.T) {
	fx := newOrderInventoryFixture(t, "manual", true, true)
	if _, err := fx.service.DeductInventoryForOrder(context.Background(), fx.order.ID, OrderInventoryOptions{Reason: "paid"}); err != nil {
		t.Fatal(err)
	}
	foreign := &OrderInventoryEffect{
		TenantID: 99, OrderID: uuid.New(), OrderItemID: uuid.New(), WarehouseID: &fx.warehouse.ID,
		ProductSKUID: uuid.New(), EffectType: EffectTypeReserve, Quantity: 1, Status: InventoryEffectSuccess,
	}
	if err := fx.db.Create(foreign).Error; err != nil {
		t.Fatal(err)
	}
	page, err := fx.service.ListOrderEffectsGlobal(context.Background(), OrderEffectsQuery{TenantID: 41, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].OrderID != fx.order.ID || page.Items[0].WarehouseID == nil {
		t.Fatalf("unexpected tenant-scoped effects page: %#v", page)
	}
}
