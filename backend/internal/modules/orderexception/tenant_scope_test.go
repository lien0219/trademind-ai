package orderexception

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"gorm.io/gorm"
)

func TestExceptionSourcesAndMarksAreTenantScoped(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:order_exception_tenant_scope?mode=memory&cache=shared"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&order.Order{}, &order.OrderItem{}, &order.OrderItemSKUMatch{},
		&inventory.OrderInventoryEffect{}, &inventory.InventorySyncTask{},
		&ordersync.OrderSyncTask{}, &OrderExceptionMark{},
	); err != nil {
		t.Fatal(err)
	}

	orderRow := &order.Order{TenantID: 101, Platform: "douyin_shop", OrderNo: "TENANT-EXC-101", CustomerName: "A", Status: order.StatusPaid, PaymentStatus: order.PaymentPaid, FulfillmentStatus: order.FulfillmentUnfulfilled, Currency: "USD"}
	if err := db.Create(orderRow).Error; err != nil {
		t.Fatal(err)
	}
	item := &order.OrderItem{OrderID: orderRow.ID, ProductTitle: "Tenant scoped item", Quantity: 1}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}

	svc := &Service{DB: db}
	foreignTenant := int64(202)
	if _, err := svc.ResolveOrderItemForBind(context.Background(), &foreignTenant, SourceOrderItem, item.ID.String()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign tenant must not resolve order item, got %v", err)
	}
	if _, err := svc.GetOrderExceptionDetail(context.Background(), &foreignTenant, SourceOrderItem, item.ID.String()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign tenant must not read exception detail, got %v", err)
	}
	if err := svc.UpsertMark(context.Background(), &foreignTenant, TypeSKUUnmatched, SourceOrderItem, item.ID.String(), MarkHandled, "foreign", nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign tenant must not mark exception, got %v", err)
	}

	ownerTenant := int64(101)
	if err := svc.UpsertMark(context.Background(), &ownerTenant, TypeSKUUnmatched, SourceOrderItem, item.ID.String(), MarkHandled, "owner", nil); err != nil {
		t.Fatal(err)
	}
	var mark OrderExceptionMark
	if err := db.Where("source_id = ?", item.ID.String()).First(&mark).Error; err != nil {
		t.Fatal(err)
	}
	if mark.TenantID != ownerTenant {
		t.Fatalf("mark tenant mismatch: got %d", mark.TenantID)
	}
	if err := svc.DeleteMarks(context.Background(), &foreignTenant, SourceOrderItem, item.ID.String()); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&mark, "id = ?", mark.ID).Error; err != nil {
		t.Fatalf("foreign delete must preserve mark: %v", err)
	}
	if err := svc.DeleteMarks(context.Background(), &ownerTenant, SourceOrderItem, item.ID.String()); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(db.First(&mark, "id = ?", mark.ID).Error, gorm.ErrRecordNotFound) {
		t.Fatal("owner tenant should delete its own mark")
	}
}
