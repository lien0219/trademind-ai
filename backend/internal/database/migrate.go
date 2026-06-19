package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"gorm.io/gorm"
)

// migrateLegacyPublicationSKUColumns renames GORM-default product_sk_uid / external_sk_uid
// to product_sku_id / external_sku_id so raw SQL and API field names stay consistent.
func migrateLegacyPublicationSKUColumns(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("product_publication_skus") {
		return nil
	}
	dst := &productpublish.ProductPublicationSKU{}
	if db.Migrator().HasColumn(dst, "product_sk_uid") && !db.Migrator().HasColumn(dst, "product_sku_id") {
		if err := db.Migrator().RenameColumn(dst, "product_sk_uid", "product_sku_id"); err != nil {
			return fmt.Errorf("rename product_publication_skus.product_sk_uid: %w", err)
		}
	}
	if db.Migrator().HasColumn(dst, "external_sk_uid") && !db.Migrator().HasColumn(dst, "external_sku_id") {
		if err := db.Migrator().RenameColumn(dst, "external_sk_uid", "external_sku_id"); err != nil {
			return fmt.Errorf("rename product_publication_skus.external_sk_uid: %w", err)
		}
	}
	return nil
}

// migrateLegacyInventorySKUColumns renames early GORM typo columns (product_sk_uid / external_sk_uid)
// and ensures inventory / order SKU linkage columns exist before raw SQL aggregations run.
func migrateLegacyInventorySKUColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	type spec struct {
		model     any
		legacyCol string
		newCol    string
	}
	renames := []spec{
		{&inventory.InventorySyncTask{}, "product_sk_uid", "product_sku_id"},
		{&inventory.InventoryChangeLog{}, "product_sk_uid", "product_sku_id"},
		{&inventory.OrderInventoryEffect{}, "product_sk_uid", "product_sku_id"},
		{&order.OrderItem{}, "product_sk_uid", "product_sku_id"},
		{&order.OrderItem{}, "external_sk_uid", "external_sku_id"},
	}
	for _, r := range renames {
		if !db.Migrator().HasTable(r.model) {
			continue
		}
		if db.Migrator().HasColumn(r.model, r.legacyCol) && !db.Migrator().HasColumn(r.model, r.newCol) {
			if err := db.Migrator().RenameColumn(r.model, r.legacyCol, r.newCol); err != nil {
				return fmt.Errorf("rename %T.%s -> %s: %w", r.model, r.legacyCol, r.newCol, err)
			}
		}
	}
	// Ensure current models add any still-missing columns (product_sku_id, external_sku_id, …).
	return db.AutoMigrate(
		&inventory.InventorySyncTask{},
		&inventory.InventoryChangeLog{},
		&inventory.OrderInventoryEffect{},
		&order.OrderItem{},
	)
}

// migrateLegacyProductTextColumns ensures AI text columns exist on older product tables.
func migrateLegacyProductTextColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.AutoMigrate(&product.Product{})
}

// AutoMigrate applies schema for core foundation tables.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("auto migrate: db is nil")
	}
	if err := migrateLegacyPublicationSKUColumns(db); err != nil {
		return err
	}
	if err := migrateLegacyInventorySKUColumns(db); err != nil {
		return err
	}
	if err := migrateLegacyProductTextColumns(db); err != nil {
		return err
	}
	if err := migrateDouyinPhase102Indexes(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(
		&admin.AdminUser{},
		&settings.Setting{},
		&operationlog.OperationLog{},
		&files.FileRecord{},
		&imagetask.ImageTask{},
		&imagetask.ImageTaskItem{},
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&product.ProductPlatformPublishConfig{},
		&product.ProductAIContentApplication{},
		&productpublish.ProductPublishTask{},
		&productpublish.ProductPublishBatch{},
		&productpublish.ProductPublication{},
		&productpublish.ProductPublicationSKU{},
		&order.Order{},
		&order.OrderItem{},
		&order.OrderItemSKUMatch{},
		&orderexception.OrderExceptionMark{},
		&ordersync.OrderSyncTask{},
		&customersync.CustomerMessageSyncTask{},
		&inventory.InventorySyncBatch{},
		&inventory.InventorySyncTask{},
		&inventory.InventoryChangeLog{},
		&inventory.OrderInventoryEffect{},
		&shop.Shop{},
		&shop.ShopAuthToken{},
		&shop.PlatformCategory{},
		&shop.PlatformCategoryAttribute{},
		&worker.Instance{},
		&collect.CollectBatch{},
		&collect.CollectTask{},
		&collect.CollectTaskEvent{},
		&collectrule.CollectRule{},
		&collectbrowserprofile.CollectBrowserProfile{},
		&aiprompt.AIPrompt{},
		&aitask.AITask{},
		&aioperationbatch.AIOperationBatch{},
		&aiproducttext.AIProductTextBatch{},
		&aiproducttext.AIProductTextItem{},
		&customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{},
		&customerchat.CustomerReplySuggestion{},
		&taskcenter.TaskFailureMark{},
		&taskcenter.TaskAlert{},
		&taskcenter.TaskAlertNotification{},
	); err != nil {
		return err
	}
	return migratePublishBatchA21(db)
}
