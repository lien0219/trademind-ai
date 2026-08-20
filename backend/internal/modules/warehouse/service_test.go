package warehouse

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newWarehouseTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:warehouse_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Warehouse{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	return &Service{DB: db}
}

func TestUpdateWarehouseKeepsDefaultTenantScoped(t *testing.T) {
	service := newWarehouseTestService(t)
	ctx := context.Background()
	first, err := service.Create(ctx, 1, nil, CreateInput{Code: "MAIN", Name: "Main", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, 1, nil, CreateInput{Code: "BACKUP", Name: "Backup"})
	if err != nil {
		t.Fatal(err)
	}
	otherTenant, err := service.Create(ctx, 2, nil, CreateInput{Code: "MAIN", Name: "Tenant 2", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := service.Update(ctx, 1, second.ID, UpdateInput{Name: "East warehouse", Status: StatusActive, IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.IsDefault || updated.Name != "East warehouse" {
		t.Fatalf("unexpected updated warehouse: %#v", updated)
	}
	rows, err := service.List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != second.ID || rows[1].ID != first.ID || rows[1].IsDefault {
		t.Fatalf("tenant default was not switched safely: %#v", rows)
	}
	otherRows, err := service.List(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherRows) != 1 || otherRows[0].ID != otherTenant.ID || !otherRows[0].IsDefault {
		t.Fatalf("other tenant default changed: %#v", otherRows)
	}
	if _, err := service.Update(ctx, 1, otherTenant.ID, UpdateInput{Name: "No access", Status: StatusActive}); !errors.Is(err, ErrWarehouseAbsent) {
		t.Fatalf("cross-tenant update must look absent, got %v", err)
	}
}

func TestUpdateWarehouseRejectsInactiveDefault(t *testing.T) {
	service := newWarehouseTestService(t)
	row, err := service.Create(context.Background(), 1, nil, CreateInput{Code: "MAIN", Name: "Main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(context.Background(), 1, row.ID, UpdateInput{Name: "Main", Status: StatusInactive, IsDefault: true}); !errors.Is(err, ErrInvalidWarehouse) {
		t.Fatalf("expected invalid warehouse, got %v", err)
	}
}
