package order

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func newFulfillmentFixture(t *testing.T) (*Service, *inventory.Service, *gin.Context, *Order, *product.ProductSKU, *warehouse.Warehouse, *idempotency.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_fulfillment_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&Order{}, &OrderItem{}, &OrderShipment{}, &product.Product{}, &product.ProductSKU{},
		&warehouse.Warehouse{}, &inventory.WarehouseStockBalance{}, &inventory.InventoryMovement{},
		&inventory.InventoryChangeLog{}, &inventory.OrderInventoryEffect{}, &idempotency.Record{},
	); err != nil {
		t.Fatal(err)
	}
	warehouseSvc := &warehouse.Service{DB: db}
	warehouseRow, err := warehouseSvc.Create(context.Background(), 41, nil, warehouse.CreateInput{Code: "FULFILL-MAIN", Name: "Fulfillment main", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	productRow := &product.Product{TenantID: 41, Source: "manual", Status: product.StatusDraft, Title: "Fulfillment product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatal(err)
	}
	stock := 10
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "FULFILL-SKU", SKUName: "Fulfillment SKU", Stock: &stock}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	orderRow := &Order{
		Base: model.Base{ID: uuid.New()}, TenantID: 41, Platform: "manual", WarehouseID: &warehouseRow.ID,
		OrderNo: "FULFILL-ORDER-1", CustomerName: "Buyer", Status: StatusPaid, PaymentStatus: PaymentPaid,
		FulfillmentStatus: FulfillmentUnfulfilled, Currency: "CNY", TotalAmount: 99,
	}
	if err := db.Create(orderRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&OrderItem{OrderID: orderRow.ID, ProductID: &productRow.ID, ProductSKUID: &sku.ID, ProductTitle: "Fulfillment product", Quantity: 3}).Error; err != nil {
		t.Fatal(err)
	}
	idem := &idempotency.Service{DB: db}
	inv := &inventory.Service{DB: db, Warehouses: warehouseSvc, Idempotency: idem}
	orderSvc := &Service{DB: db, Idempotency: idem}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/api/v1/orders/"+orderRow.ID.String()+"/fulfill", nil)
	ctx.Set(ctxkey.TenantID, int64(41))
	ctx.Set(ctxkey.AdminID, uuid.New().String())
	return orderSvc, inv, ctx, orderRow, sku, warehouseRow, idem
}

func TestFulfillOrderCommitsShipmentAndInventoryAtomically(t *testing.T) {
	orderSvc, inv, ctx, orderRow, sku, warehouseRow, _ := newFulfillmentFixture(t)
	key := "fulfill-key-1"
	result, err := orderSvc.FulfillOrder(ctx, inv, orderRow.ID, FulfillOrderInput{
		IdempotencyKey: key, WarehouseID: &warehouseRow.ID, Carrier: "Carrier", TrackingNo: "TRACK-1",
		TrackingURL: "https://carrier.test/track/TRACK-1",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Shipment == nil || result.Order == nil || result.Order.Status != StatusShipped || result.Order.FulfillmentStatus != FulfillmentFulfilled {
		t.Fatalf("unexpected fulfillment result: %#v", result)
	}
	var balance inventory.WarehouseStockBalance
	if err := orderSvc.DB.Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", 41, warehouseRow.ID, sku.ID).First(&balance).Error; err != nil {
		t.Fatal(err)
	}
	if balance.OnHand != 7 || balance.Reserved != 0 {
		t.Fatalf("unexpected warehouse balance: %#v", balance)
	}
	var storedSKU product.ProductSKU
	if err := orderSvc.DB.First(&storedSKU, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedSKU.Stock == nil || *storedSKU.Stock != 7 {
		t.Fatalf("unexpected compatibility stock: %#v", storedSKU.Stock)
	}
	var shipmentCount int64
	if err := orderSvc.DB.Model(&OrderShipment{}).Where("order_id = ?", orderRow.ID).Count(&shipmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 1 {
		t.Fatalf("expected one shipment, got %d", shipmentCount)
	}
	var idemRow idempotency.Record
	if err := orderSvc.DB.Where("scope = ? AND idempotency_key = ?", orderFulfillmentScope, idempotency.OrderFulfillment(orderRow.ID.String(), key)).First(&idemRow).Error; err != nil {
		t.Fatal(err)
	}
	if idemRow.Status != idempotency.StatusSucceeded {
		t.Fatalf("fulfillment idempotency must complete with transaction: %#v", idemRow)
	}

	replay, err := orderSvc.FulfillOrder(ctx, inv, orderRow.ID, FulfillOrderInput{
		IdempotencyKey: key, WarehouseID: &warehouseRow.ID, Carrier: "Carrier", TrackingNo: "TRACK-1",
		TrackingURL: "https://carrier.test/track/TRACK-1",
	}, nil)
	if err != nil || replay.Shipment == nil || replay.Shipment.ID != result.Shipment.ID {
		t.Fatalf("same fulfillment key must replay shipment: result=%#v err=%v", replay, err)
	}
	if err := orderSvc.DB.Model(&OrderShipment{}).Where("order_id = ?", orderRow.ID).Count(&shipmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 1 {
		t.Fatalf("replay must not create another shipment, got %d", shipmentCount)
	}
}

func TestFulfillOrderRollsBackWhenStockIsInsufficient(t *testing.T) {
	orderSvc, inv, ctx, orderRow, sku, warehouseRow, _ := newFulfillmentFixture(t)
	short := 2
	if err := orderSvc.DB.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).Update("stock", short).Error; err != nil {
		t.Fatal(err)
	}

	_, err := orderSvc.FulfillOrder(ctx, inv, orderRow.ID, FulfillOrderInput{
		IdempotencyKey: "fulfill-insufficient",
		WarehouseID:    &warehouseRow.ID,
		Carrier:        "Carrier",
		TrackingNo:     "TRACK-INSUFFICIENT",
	}, nil)
	if !errors.Is(err, inventory.ErrInsufficientSKUStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}

	var stored Order
	if err := orderSvc.DB.First(&stored, "id = ?", orderRow.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != StatusPaid || stored.FulfillmentStatus != FulfillmentUnfulfilled {
		t.Fatalf("order lifecycle must roll back: %#v", stored)
	}
	var shipmentCount int64
	if err := orderSvc.DB.Model(&OrderShipment{}).Where("order_id = ?", orderRow.ID).Count(&shipmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if shipmentCount != 0 {
		t.Fatalf("failed fulfillment must not create shipment, got %d", shipmentCount)
	}
	var storedSKU product.ProductSKU
	if err := orderSvc.DB.First(&storedSKU, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedSKU.Stock == nil || *storedSKU.Stock != short {
		t.Fatalf("failed fulfillment must not change SKU stock: %#v", storedSKU.Stock)
	}
}

func TestFulfillmentOwnerIsUniquePerAttempt(t *testing.T) {
	actor := uuid.New()
	first := fulfillmentOwner(&actor)
	second := fulfillmentOwner(&actor)
	if first == second || !strings.HasPrefix(first, "order-fulfillment:"+actor.String()+":") {
		t.Fatalf("fulfillment owners must be unique and actor-scoped: %q %q", first, second)
	}
}

func TestFulfillOrderRejectsUnsafeTrackingURL(t *testing.T) {
	orderSvc, inv, ctx, orderRow, _, warehouseRow, _ := newFulfillmentFixture(t)
	_, err := orderSvc.FulfillOrder(ctx, inv, orderRow.ID, FulfillOrderInput{
		IdempotencyKey: "fulfill-unsafe-url",
		WarehouseID:    &warehouseRow.ID,
		Carrier:        "Carrier",
		TrackingNo:     "TRACK-UNSAFE",
		TrackingURL:    "javascript:alert(1)",
	}, nil)
	if !errors.Is(err, ErrFulfillmentTrackingURLInvalid) {
		t.Fatalf("expected unsafe tracking URL rejection, got %v", err)
	}
}

func TestFulfillOrderRejectsCompositeIdempotencyKeyOverflow(t *testing.T) {
	orderSvc, inv, ctx, orderRow, _, warehouseRow, _ := newFulfillmentFixture(t)
	_, err := orderSvc.FulfillOrder(ctx, inv, orderRow.ID, FulfillOrderInput{
		IdempotencyKey: strings.Repeat("k", 129),
		WarehouseID:    &warehouseRow.ID,
		Carrier:        "Carrier",
		TrackingNo:     "TRACK-LONG-KEY",
	}, nil)
	if !errors.Is(err, ErrFulfillmentInputTooLong) {
		t.Fatalf("expected bounded idempotency key rejection, got %v", err)
	}
}
