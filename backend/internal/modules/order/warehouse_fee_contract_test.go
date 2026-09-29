package order

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
)

func TestWarehouseFeeFactsUseLatestVerifiedFulfillmentAndStoreScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order-warehouse-fee-%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&shop.Shop{}, &warehouse.Warehouse{}, &Order{}, &FulfillmentWave{}, &FulfillmentWaveOrder{}, &FulfillmentWaveLine{}, &FulfillmentWavePackVerification{}); err != nil {
		t.Fatal(err)
	}
	shopRow := shop.Shop{TenantID: 7, Platform: "manual", ShopName: "Scoped shop", Status: "active", AuthStatus: "active"}
	warehouseRow := warehouse.Warehouse{TenantID: 7, Code: "WH-A", Name: "Warehouse A", Status: warehouse.StatusActive}
	if err := db.Create(&shopRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&warehouseRow).Error; err != nil {
		t.Fatal(err)
	}
	orderRow := Order{TenantID: 7, Platform: "manual", ShopID: &shopRow.ID, WarehouseID: &warehouseRow.ID, OrderNo: "SO-WF-1", CustomerName: "Customer", Status: "fulfilled", PaymentStatus: "paid", FulfillmentStatus: "fulfilled", Currency: "CNY"}
	if err := db.Create(&orderRow).Error; err != nil {
		t.Fatal(err)
	}
	wave := FulfillmentWave{TenantID: 7, WaveNo: "FW-WF-1", WarehouseID: warehouseRow.ID, Status: FulfillmentWaveCompleted, Revision: 8}
	if err := db.Create(&wave).Error; err != nil {
		t.Fatal(err)
	}
	fulfilledAt := time.Now().UTC()
	waveOrder := FulfillmentWaveOrder{TenantID: 7, WaveID: wave.ID, OrderID: orderRow.ID, ShopID: &shopRow.ID, OrderNo: orderRow.OrderNo, Status: FulfillmentWaveOrderFulfilled, PackageCode: "PKG-1", ShipmentID: uuidPointer(uuid.New()), FulfilledAt: &fulfilledAt}
	if err := db.Create(&waveOrder).Error; err != nil {
		t.Fatal(err)
	}
	for _, qty := range []int{2, 3} {
		line := FulfillmentWaveLine{TenantID: 7, WaveID: wave.ID, WaveOrderID: waveOrder.ID, OrderID: orderRow.ID, OrderItemID: uuid.New(), ProductSKUID: uuid.New(), RequiredQty: qty, PickedQty: qty, Status: FulfillmentWaveLinePicked}
		if err := db.Create(&line).Error; err != nil {
			t.Fatal(err)
		}
	}
	verification := FulfillmentWavePackVerification{TenantID: 7, WaveID: wave.ID, WaveOrderID: waveOrder.ID, OrderID: orderRow.ID, ActionID: uuid.New(), PackageCode: "PKG-1", ScannedOrderNo: orderRow.OrderNo, Carrier: "Local", TrackingNo: "TRACK-1", LineCount: 2, VerifiedQty: 5}
	if err := db.Create(&verification).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}

	denied, err := svc.ListWarehouseFeeFacts(context.Background(), db, 7, WarehouseFeeScope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{}}, WarehouseFeeFactQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if denied.Total != 0 || len(denied.List) != 0 {
		t.Fatalf("empty store scope leaked fulfillment facts: %#v", denied)
	}
	allowed, err := svc.ListWarehouseFeeFacts(context.Background(), db, 7, WarehouseFeeScope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{shopRow.ID}}, WarehouseFeeFactQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if allowed.Total != 1 || len(allowed.List) != 1 || allowed.List[0].ItemQuantity != 5 || allowed.List[0].PackageQuantity != 1 || allowed.List[0].PackVerificationID == nil || *allowed.List[0].PackVerificationID != verification.ID {
		t.Fatalf("unexpected fulfillment fee facts: %#v", allowed)
	}
	if _, err := svc.GetWarehouseFeeFact(context.Background(), db, 8, WarehouseFeeScope{}, orderRow.ID, false); err != ErrFulfillmentWaveNotFound {
		t.Fatalf("cross-tenant fact lookup = %v, want not found", err)
	}
}
