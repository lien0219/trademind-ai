package supplier

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func newSupplierTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:supplier_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &product.ProductSKU{}, &Supplier{}, &SupplierSKU{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return &Service{DB: db}, db
}

func TestSupplierUpdateAndSKUListAreTenantScoped(t *testing.T) {
	service, db := newSupplierTestService(t)
	ctx := context.Background()
	supplierRow, err := service.Create(ctx, 1, nil, CreateInput{Code: "SUP-1", Name: "Old supplier"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.Update(ctx, 1, supplierRow.ID, UpdateInput{
		Name: "Primary supplier", Status: StatusActive, ContactName: "Buyer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Primary supplier" || updated.Phone != "" || updated.Email != "" {
		t.Fatalf("unexpected supplier update: %#v", updated)
	}
	phone, email := "13812345678", "buyer@example.test"
	updated, err = service.Update(ctx, 1, supplierRow.ID, UpdateInput{
		Name: "Primary supplier", Status: StatusActive, ContactName: "Buyer", Phone: &phone, Email: &email,
	})
	if err != nil || updated.Phone != phone || updated.Email != email {
		t.Fatalf("sensitive supplier fields were not updated explicitly: row=%#v err=%v", updated, err)
	}
	updated, err = service.Update(ctx, 1, supplierRow.ID, UpdateInput{
		Name: "Renamed supplier", Status: StatusActive, ContactName: "Buyer",
	})
	if err != nil || updated.Phone != phone || updated.Email != email {
		t.Fatalf("omitted sensitive fields must be preserved: row=%#v err=%v", updated, err)
	}
	productRow := &product.Product{TenantID: 1, Source: "manual", Status: product.StatusDraft, Title: "E2E product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatal(err)
	}
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "SKU-RED", SKUName: "Red"}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	bound, err := service.BindSKU(ctx, 1, supplierRow.ID, BindSKUInput{
		ProductSKUID: sku.ID, SupplierSKUCode: "VENDOR-RED", UnitCostMinor: 1234, Currency: "cny", MinOrderQty: 2, LeadTimeDays: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := service.ListSKUs(ctx, 1, supplierRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != bound.ID || rows[0].ProductTitle != "E2E product" || rows[0].SKUName != "Red" {
		t.Fatalf("unexpected supplier SKU list: %#v", rows)
	}
	if _, err := service.ListSKUs(ctx, 2, supplierRow.ID); !errors.Is(err, ErrSupplierAbsent) {
		t.Fatalf("cross-tenant supplier list must look absent, got %v", err)
	}
	if _, err := service.Update(ctx, 2, supplierRow.ID, UpdateInput{Name: "No access", Status: StatusActive}); !errors.Is(err, ErrSupplierAbsent) {
		t.Fatalf("cross-tenant supplier update must look absent, got %v", err)
	}
}
