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

func newPlacementTestService(t *testing.T) (*Service, *warehouse.Service, *warehouse.Warehouse, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:warehouse_placement_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &product.ProductSKU{}, &warehouse.Warehouse{}, &warehouse.WarehouseLocation{}, &WarehouseSKUPlacement{}); err != nil {
		t.Fatal(err)
	}
	warehouses := &warehouse.Service{DB: db}
	main, err := warehouses.Create(context.Background(), 11, nil, warehouse.CreateInput{Code: "MAIN", Name: "Main", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	location, err := warehouses.CreateLocation(context.Background(), 11, main.ID, nil, warehouse.CreateLocationInput{Code: "A-01", Name: "Rack A01"})
	if err != nil {
		t.Fatal(err)
	}
	prod := &product.Product{TenantID: 11, Source: "manual", Status: product.StatusDraft, Title: "Placement product"}
	if err := db.Create(prod).Error; err != nil {
		t.Fatal(err)
	}
	sku := &product.ProductSKU{ProductID: prod.ID, SKUCode: "PLACEMENT-SKU", SKUName: "Placement SKU"}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	return &Service{DB: db, Warehouses: warehouses}, warehouses, main, location.ID, sku.ID
}

func TestWarehouseSKUPlacementValidatesTenantAndBarcode(t *testing.T) {
	service, _, main, locationID, skuID := newPlacementTestService(t)
	ctx := context.Background()
	row, err := service.CreateWarehouseSKUPlacement(ctx, 11, nil, WarehouseSKUPlacementInput{
		WarehouseID: main.ID, ProductSKUID: skuID, LocationID: &locationID, Barcode: "abc-001", Status: PlacementStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if row.Barcode != "ABC-001" {
		t.Fatalf("barcode must normalize: %#v", row)
	}
	if _, err := service.CreateWarehouseSKUPlacement(ctx, 11, nil, WarehouseSKUPlacementInput{WarehouseID: main.ID, ProductSKUID: uuid.New(), Barcode: "OTHER", Status: PlacementStatusActive}); !errors.Is(err, ErrPlacementAbsent) {
		t.Fatalf("unknown sku must be rejected, got %v", err)
	}
	if _, err := service.CreateWarehouseSKUPlacement(ctx, 11, nil, WarehouseSKUPlacementInput{WarehouseID: main.ID, ProductSKUID: uuid.New(), Barcode: "ABC-001", Status: PlacementStatusActive}); !errors.Is(err, ErrPlacementAbsent) {
		t.Fatalf("unknown sku must fail before barcode reuse, got %v", err)
	}
	rows, err := service.ListWarehouseSKUPlacements(ctx, 11, main.ID, nil, false)
	if err != nil || len(rows) != 1 || rows[0].LocationCode != "A-01" || rows[0].SKUCode != "PLACEMENT-SKU" {
		t.Fatalf("unexpected placement view: %#v %v", rows, err)
	}
}

func TestWarehouseSKUPlacementSnapshotIsReadInsideTransaction(t *testing.T) {
	service, _, main, locationID, skuID := newPlacementTestService(t)
	ctx := context.Background()
	if _, err := service.CreateWarehouseSKUPlacement(ctx, 11, nil, WarehouseSKUPlacementInput{WarehouseID: main.ID, ProductSKUID: skuID, LocationID: &locationID, Barcode: "SNAP-1", Status: PlacementStatusActive}); err != nil {
		t.Fatal(err)
	}
	got := make(map[uuid.UUID]WarehouseSKUPlacementSnapshot)
	if err := service.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		got, err = service.LoadWarehouseSKUPlacementSnapshots(ctx, tx, 11, main.ID, []uuid.UUID{skuID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, ok := got[skuID]
	if !ok || snapshot.Barcode != "SNAP-1" || snapshot.LocationCode != "A-01" || snapshot.LocationID == nil || *snapshot.LocationID != locationID {
		t.Fatalf("unexpected snapshot: %#v", got)
	}
}
