package demoseed

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

func TestSeedFullProjectEdgeCases_blocksProduction(t *testing.T) {
	s := &Service{AppEnv: "production"}
	_, err := s.SeedFullProjectEdgeCases(context.Background(), nil, 0)
	if !errors.Is(err, ErrProductionForbidden) {
		t.Fatalf("expected ErrProductionForbidden, got %v", err)
	}
}

func TestSeedFullProjectEdgeCases_createsSamples(t *testing.T) {
	db := openDemoSeedTestDB(t)
	if db == nil {
		return
	}
	s := &Service{DB: db, AppEnv: "development"}
	adminID := uuid.New()
	const tenantID int64 = 42
	out, err := s.SeedFullProjectEdgeCases(context.Background(), &adminID, tenantID)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if len(out.Samples) < 5 {
		t.Fatalf("expected >=5 samples, got %d", len(out.Samples))
	}
	var orderCnt, invCnt, failCnt int64
	db.Model(&ordersync.OrderSyncTask{}).Where("status = ?", "partial_success").Count(&orderCnt)
	db.Model(&inventory.InventorySyncTask{}).Where("status = ?", "failed").Count(&invCnt)
	db.Model(&customerchat.CustomerFailureEvent{}).Count(&failCnt)
	if orderCnt < 1 || invCnt < 1 || failCnt < 1 {
		t.Fatalf("counts order=%d inv=%d fail=%d", orderCnt, invCnt, failCnt)
	}
	var shops int64
	db.Model(&shop.Shop{}).Count(&shops)
	if shops < 1 {
		t.Fatal("expected demo shop")
	}
	var tenantShopCount, defaultTenantInventoryCount int64
	db.Model(&shop.Shop{}).Where("tenant_id = ?", tenantID).Count(&tenantShopCount)
	db.Model(&inventory.InventorySyncTask{}).Where("tenant_id = ?", 0).Count(&defaultTenantInventoryCount)
	if tenantShopCount < 1 || defaultTenantInventoryCount != 0 {
		t.Fatalf("expected tenant-scoped demo rows, tenant shops=%d default inventory tasks=%d", tenantShopCount, defaultTenantInventoryCount)
	}
}

func openDemoSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
		return nil
	}
	if err := db.AutoMigrate(
		&shop.Shop{},
		&product.Product{},
		&product.ProductSKU{},
		&product.ProductPlatformPublishConfig{},
		&ordersync.OrderSyncTask{},
		&inventory.InventorySyncTask{},
		&customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{},
		&customerchat.CustomerFailureEvent{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}
