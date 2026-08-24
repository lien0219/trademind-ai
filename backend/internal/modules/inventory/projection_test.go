package inventory

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

func newProjectionFixture(t *testing.T) (*gorm.DB, *product.ProductSKU) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:inventory_projection_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&product.Product{}, &product.ProductSKU{}); err != nil {
		t.Fatal(err)
	}
	row := &product.Product{TenantID: 7, Source: "manual", Status: product.StatusDraft, Title: "Projection product"}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	stock := 10
	sku := &product.ProductSKU{ProductID: row.ID, SKUCode: "PROJECTION-1", SKUName: "Projection SKU", Stock: &stock, WarningStock: 5, SafetyStock: 2}
	if err := db.Create(sku).Error; err != nil {
		t.Fatal(err)
	}
	return db, sku
}

func TestWriteSKUStockProjectionTxUpdatesProjectionAndStatus(t *testing.T) {
	db, sku := newProjectionFixture(t)
	if err := writeSKUStockProjectionTx(context.Background(), db, sku, 2); err != nil {
		t.Fatal(err)
	}

	var stored product.ProductSKU
	if err := db.First(&stored, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Stock == nil || *stored.Stock != 2 || stored.StockStatus != product.StockStatusBelowSafetyStock {
		t.Fatalf("unexpected compatibility projection: %#v", stored)
	}
}

func TestWriteSKUStockProjectionTxFailsClosedWhenProductScopeDoesNotMatch(t *testing.T) {
	db, sku := newProjectionFixture(t)
	wrongProduct := &product.Product{TenantID: 7, Source: "manual", Status: product.StatusDraft, Title: "Other product"}
	if err := db.Create(wrongProduct).Error; err != nil {
		t.Fatal(err)
	}

	wrongScope := *sku
	wrongScope.ProductID = wrongProduct.ID
	err := writeSKUStockProjectionTx(context.Background(), db, &wrongScope, 1)
	if !errors.Is(err, ErrSKUStockProjection) {
		t.Fatalf("expected projection scope error, got %v", err)
	}

	var stored product.ProductSKU
	if err := db.First(&stored, "id = ?", sku.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Stock == nil || *stored.Stock != 10 {
		t.Fatalf("projection changed after scope mismatch: %#v", stored.Stock)
	}
}
