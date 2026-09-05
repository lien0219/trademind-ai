package profitability

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	ordermod "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	basemodel "github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestGormRepositoryAppliesTenantAndStoreScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:profitability_scope?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&shop.Shop{}, &warehouse.Warehouse{}, &ordermod.Order{}, &ordermod.OrderItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	shopA := shop.Shop{TenantID: 41, Platform: "manual", ShopName: "A", Status: "active", AuthStatus: "active"}
	shopB := shop.Shop{TenantID: 41, Platform: "manual", ShopName: "B", Status: "active", AuthStatus: "active"}
	shopOther := shop.Shop{TenantID: 42, Platform: "manual", ShopName: "Other", Status: "active", AuthStatus: "active"}
	for _, row := range []*shop.Shop{&shopA, &shopB, &shopOther} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create shop: %v", err)
		}
	}
	now := time.Now().UTC()
	orders := []ordermod.Order{
		{Base: basemodel.Base{CreatedAt: now}, TenantID: 41, Platform: "manual", ShopID: &shopA.ID, OrderNo: "A-ORDER", CustomerName: "A", Status: "pending", PaymentStatus: "paid", FulfillmentStatus: "unfulfilled", Currency: "CNY", TotalAmount: 12.34},
		{Base: basemodel.Base{CreatedAt: now.Add(-time.Minute)}, TenantID: 41, Platform: "manual", ShopID: &shopB.ID, OrderNo: "B-ORDER", CustomerName: "B", Status: "pending", PaymentStatus: "paid", FulfillmentStatus: "unfulfilled", Currency: "CNY", TotalAmount: 20},
		{Base: basemodel.Base{CreatedAt: now.Add(-2 * time.Minute)}, TenantID: 42, Platform: "manual", ShopID: &shopOther.ID, OrderNo: "OTHER", CustomerName: "Other", Status: "pending", PaymentStatus: "paid", FulfillmentStatus: "unfulfilled", Currency: "CNY", TotalAmount: 99},
	}
	for i := range orders {
		if err := db.Create(&orders[i]).Error; err != nil {
			t.Fatalf("create order: %v", err)
		}
	}
	repo := &GormRepository{DB: db}

	rows, total, err := repo.ListOrders(context.Background(), 41, Scope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{shopA.ID}}, ListQuery{}, 0, 20)
	if err != nil {
		t.Fatalf("ListOrders() error = %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].OrderNo != "A-ORDER" || rows[0].TotalAmountText != "12.34" {
		t.Fatalf("scoped rows = %#v total=%d", rows, total)
	}
	rows, total, err = repo.ListOrders(context.Background(), 41, Scope{RestrictStoreScope: true}, ListQuery{}, 0, 20)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("empty store scope rows=%#v total=%d err=%v", rows, total, err)
	}
	if _, err := repo.GetOrder(context.Background(), 41, Scope{RestrictStoreScope: true, AllowedShopIDs: []uuid.UUID{shopA.ID}}, orders[1].ID); err != gorm.ErrRecordNotFound {
		t.Fatalf("out-of-scope GetOrder() error = %v, want record not found", err)
	}
}
