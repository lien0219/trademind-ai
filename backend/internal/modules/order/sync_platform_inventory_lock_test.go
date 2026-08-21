package order

import (
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"gorm.io/gorm"
)

func TestPlatformSyncPreservesInventoryLockedOrderItemIdentityAndQuantity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_sync_inventory_lock_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Order{}, &OrderItem{}, &OrderShipment{}, &inventory.OrderInventoryEffect{}); err != nil {
		t.Fatal(err)
	}
	orderRow := &Order{TenantID: 1, Platform: "test", OrderNo: "LOCKED-ORDER", CustomerName: "Buyer", Status: StatusPaid, PaymentStatus: PaymentPaid, FulfillmentStatus: FulfillmentUnfulfilled, Currency: "USD"}
	if err := db.Create(orderRow).Error; err != nil {
		t.Fatal(err)
	}
	externalItemID := "platform-line-1"
	skuID := uuid.New()
	item := &OrderItem{OrderID: orderRow.ID, ExternalItemID: &externalItemID, ProductSKUID: &skuID, ProductTitle: "Original", Quantity: 3}
	if err := db.Create(item).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&inventory.OrderInventoryEffect{
		TenantID: 1, OrderID: orderRow.ID, OrderItemID: item.ID, ProductSKUID: skuID,
		EffectType: inventory.EffectTypeReserve, Quantity: 3, Status: inventory.InventoryEffectSuccess,
	}).Error; err != nil {
		t.Fatal(err)
	}

	payload := SyncedOrderPayload{Items: []SyncedOrderItemPayload{{ExternalItemID: externalItemID, ProductTitle: "Updated", Quantity: 5}}}
	if err := db.Transaction(func(tx *gorm.DB) error { return replaceSyncedChildren(tx, orderRow.ID, payload) }); err != nil {
		t.Fatal(err)
	}
	var reloaded OrderItem
	if err := db.First(&reloaded, "id = ?", item.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.Quantity != 3 || reloaded.ProductTitle != "Updated" {
		t.Fatalf("inventory-locked line must retain identity and quantity while refreshing metadata: %#v", reloaded)
	}

	if err := db.Transaction(func(tx *gorm.DB) error { return replaceSyncedChildren(tx, orderRow.ID, SyncedOrderPayload{}) }); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&OrderItem{}).Where("id = ?", item.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("inventory-locked line must not disappear from a later platform payload, got %d", count)
	}
}
