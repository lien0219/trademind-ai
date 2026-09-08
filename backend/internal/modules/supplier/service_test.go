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

func TestSupplierAllowsLegacyTenantZero(t *testing.T) {
	service, _ := newSupplierTestService(t)
	ctx := context.Background()

	row, err := service.Create(ctx, 0, nil, CreateInput{
		Code: "SUP-12", Name: "Legacy supplier", ContactName: "Buyer",
		Phone: "13812345678", Email: "buyer@example.test",
	})
	if err != nil {
		t.Fatalf("create legacy tenant supplier: %v", err)
	}
	if row.TenantID != 0 || row.Code != "SUP-12" {
		t.Fatalf("unexpected legacy tenant supplier: %#v", row)
	}

	updated, err := service.Update(ctx, 0, row.ID, UpdateInput{Name: "Renamed supplier", Status: StatusActive, ContactName: "Buyer"})
	if err != nil {
		t.Fatalf("update legacy tenant supplier: %v", err)
	}
	if updated.TenantID != 0 || updated.Name != "Renamed supplier" {
		t.Fatalf("unexpected updated legacy tenant supplier: %#v", updated)
	}
}

func TestListActiveCostsUsesCallerTransactionAndTenantScope(t *testing.T) {
	service, db := newSupplierTestService(t)
	ctx := context.Background()
	productRow := &product.Product{TenantID: 1, Source: "manual", Status: product.StatusDraft, Title: "Cost source product"}
	if err := db.Create(productRow).Error; err != nil {
		t.Fatal(err)
	}
	sku := &product.ProductSKU{ProductID: productRow.ID, SKUCode: "COST-SKU", SKUName: "Cost SKU"}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	active, err := service.Create(ctx, 1, nil, CreateInput{Code: "ACTIVE-COST", Name: "Active cost supplier"})
	if err != nil {
		t.Fatal(err)
	}
	inactive, err := service.Create(ctx, 1, nil, CreateInput{Code: "INACTIVE-COST", Name: "Inactive cost supplier"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, 1, inactive.ID, UpdateInput{Name: inactive.Name, Status: StatusInactive}); err != nil {
		t.Fatal(err)
	}
	activeBinding, err := service.BindSKU(ctx, 1, active.ID, BindSKUInput{ProductSKUID: sku.ID, SupplierSKUCode: "ACTIVE-SKU", UnitCostMinor: 321, Currency: "CNY", MinOrderQty: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&SupplierSKU{TenantID: 1, SupplierID: inactive.ID, ProductSKUID: sku.ID, SupplierSKUCode: "INACTIVE-SKU", UnitCostMinor: 111, Currency: "CNY", MinOrderQty: 1}).Error; err != nil {
		t.Fatal(err)
	}

	var rows []ActiveCostFact
	if err := db.Transaction(func(tx *gorm.DB) error {
		var listErr error
		rows, listErr = service.ListActiveCosts(ctx, tx, 1, []uuid.UUID{sku.ID})
		return listErr
	}); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].SupplierSKUID != activeBinding.ID || rows[0].UnitCostMinor != 321 || rows[0].SupplierCode != active.Code {
		t.Fatalf("unexpected active cost facts: %#v", rows)
	}
	otherTenantRows, err := service.ListActiveCosts(ctx, nil, 2, []uuid.UUID{sku.ID})
	if err != nil || len(otherTenantRows) != 0 {
		t.Fatalf("cross-tenant costs = %#v err=%v", otherTenantRows, err)
	}
}
