package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
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

// AutoMigrate applies schema for core foundation tables.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("auto migrate: db is nil")
	}
	if err := migrateLegacyPublicationSKUColumns(db); err != nil {
		return err
	}
	return db.AutoMigrate(
		&admin.AdminUser{},
		&settings.Setting{},
		&operationlog.OperationLog{},
		&files.FileRecord{},
		&imagetask.ImageTask{},
		&product.Product{},
		&product.ProductImage{},
		&product.ProductSKU{},
		&productpublish.ProductPublishTask{},
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
		&worker.Instance{},
		&collect.CollectBatch{},
		&collect.CollectTask{},
		&collect.CollectTaskEvent{},
		&collectrule.CollectRule{},
		&collectbrowserprofile.CollectBrowserProfile{},
		&aiprompt.AIPrompt{},
		&aitask.AITask{},
		&aioperationbatch.AIOperationBatch{},
		&customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{},
		&customerchat.CustomerReplySuggestion{},
		&taskcenter.TaskFailureMark{},
		&taskcenter.TaskAlert{},
		&taskcenter.TaskAlertNotification{},
	)
}
