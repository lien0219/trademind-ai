package database

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
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
	"github.com/trademind-ai/trademind/backend/internal/modules/inventorysync"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationtask"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/performance"
	"github.com/trademind-ai/trademind/backend/internal/modules/procurement"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/supplier"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
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

// migrateLegacyInventorySKUColumns renames early GORM typo columns (*_sk_uid)
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
		{&inventory.InventorySyncTask{}, "publication_sk_uid", "publication_sku_id"},
		{&inventory.InventoryChangeLog{}, "product_sk_uid", "product_sku_id"},
		{&inventory.OrderInventoryEffect{}, "product_sk_uid", "product_sku_id"},
		{&order.OrderItem{}, "product_sk_uid", "product_sku_id"},
		{&order.OrderItem{}, "external_sk_uid", "external_sku_id"},
		{&order.OrderItemSKUMatch{}, "product_sku_i_d", "product_sku_id"},
		{&order.OrderItemSKUMatch{}, "external_sk_uid", "external_sku_id"},
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
		&order.OrderItemSKUMatch{},
	)
}

// migrateLegacyProductTextColumns ensures AI text columns exist on older product tables.
func migrateLegacyProductTextColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	return db.AutoMigrate(&product.Product{})
}

func backfillOrderInventoryEffectTenants(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&inventory.OrderInventoryEffect{}) || !db.Migrator().HasTable(&order.Order{}) {
		return nil
	}
	return db.Exec(`
		UPDATE order_inventory_effects
		SET tenant_id = COALESCE((
			SELECT orders.tenant_id
			FROM orders
			WHERE orders.id = order_inventory_effects.order_id
		), 0)
		WHERE tenant_id = 0
	`).Error
}

// backfillOrderExceptionMarkTenants preserves existing workbench marks when
// tenant_id is added to the overlay table. Unknown legacy sources remain at
// tenant zero and are therefore invisible to authenticated non-zero tenants.
func backfillOrderExceptionMarkTenants(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&orderexception.OrderExceptionMark{}) ||
		!db.Migrator().HasColumn(&orderexception.OrderExceptionMark{}, "tenant_id") {
		return nil
	}
	type markRef struct {
		ID         uuid.UUID `gorm:"column:id"`
		SourceType string    `gorm:"column:source_type"`
		SourceID   string    `gorm:"column:source_id"`
	}
	var refs []markRef
	if err := db.Table("order_exception_marks").Select("id, source_type, source_id").Where("tenant_id = 0").Find(&refs).Error; err != nil {
		return err
	}
	for _, ref := range refs {
		var tenantID int64
		var err error
		switch ref.SourceType {
		case orderexception.SourceOrder:
			err = db.Table("orders").Select("tenant_id").Where("id = ?", ref.SourceID).Scan(&tenantID).Error
		case orderexception.SourceOrderItem:
			err = db.Raw("SELECT o.tenant_id FROM orders o JOIN order_items oi ON oi.order_id = o.id WHERE oi.id = ?", ref.SourceID).Scan(&tenantID).Error
		case orderexception.SourceOrderItemSKUMatch:
			err = db.Raw("SELECT o.tenant_id FROM orders o JOIN order_item_sku_matches m ON m.order_id = o.id WHERE m.id = ?", ref.SourceID).Scan(&tenantID).Error
		case orderexception.SourceOrderInventoryEffect:
			err = db.Table("order_inventory_effects").Select("tenant_id").Where("id = ?", ref.SourceID).Scan(&tenantID).Error
		case orderexception.SourceInventorySyncTask:
			err = db.Table("inventory_sync_tasks").Select("tenant_id").Where("id = ?", ref.SourceID).Scan(&tenantID).Error
		case orderexception.SourceOrderSyncTask:
			err = db.Table("order_sync_tasks").Select("tenant_id").Where("id = ?", ref.SourceID).Scan(&tenantID).Error
		}
		if err != nil {
			return err
		}
		if tenantID != 0 {
			if err := db.Table("order_exception_marks").Where("id = ? AND tenant_id = 0", ref.ID).Update("tenant_id", tenantID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func dropLegacyOrderExceptionMarkIndex(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&orderexception.OrderExceptionMark{}) {
		return nil
	}
	const legacyIndex = "ux_order_exception_mark_quad"
	if db.Migrator().HasIndex(&orderexception.OrderExceptionMark{}, legacyIndex) {
		if err := db.Migrator().DropIndex(&orderexception.OrderExceptionMark{}, legacyIndex); err != nil {
			return fmt.Errorf("drop legacy %s: %w", legacyIndex, err)
		}
	}
	return nil
}

// AutoMigrate applies schema for core foundation tables.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("auto migrate: db is nil")
	}
	if err := migrateLegacyTableNames(db, inventoryLegacyTableRenames); err != nil {
		return err
	}
	if err := migrateLegacyTableNames(db, imageTaskTableRenames); err != nil {
		return err
	}
	if err := migrateLegacyDatabaseObjectNames(db); err != nil {
		return err
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
	if err := db.AutoMigrate(
		&admin.AdminUser{},
		&admin.UserStorePermission{},
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
		&product.ProductImageApplication{},
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
		&warehouse.Warehouse{},
		&inventory.WarehouseStockBalance{},
		&inventory.InventoryMovement{},
		&inventory.WarehouseTransfer{},
		&inventory.WarehouseTransferItem{},
		&inventory.WarehouseTransferAction{},
		&supplier.Supplier{},
		&supplier.SupplierSKU{},
		&procurement.PurchaseOrder{},
		&procurement.PurchaseOrderItem{},
		&procurement.GoodsReceipt{},
		&procurement.GoodsReceiptItem{},
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
		&aiproductimage.AIProductImageBatch{},
		&aiproductimage.AIProductImageItem{},
		&customerchat.CustomerConversation{},
		&customerchat.CustomerMessage{},
		&customerchat.CustomerReplySuggestion{},
		&customerchat.CustomerFailureEvent{},
		&customerchat.CustomerAutoReplySetting{},
		&customerchat.CustomerAutoReplyPolicy{},
		&customerchat.CustomerAutoReplyRun{},
		&taskcenter.TaskFailureMark{},
		&taskcenter.TaskAlert{},
		&taskcenter.TaskAlertNotification{},
		&performance.TestRun{},
		&performance.Regression{},
		&performance.CapacitySnapshot{},
		&performance.RateLimitPolicy{},
		&performance.QuotaPolicy{},
	); err != nil {
		return err
	}
	if err := backfillOrderInventoryEffectTenants(db); err != nil {
		return fmt.Errorf("backfill order inventory effect tenants: %w", err)
	}
	if err := dropLegacyOrderExceptionMarkIndex(db); err != nil {
		return fmt.Errorf("drop legacy order exception mark index: %w", err)
	}
	if err := backfillOrderExceptionMarkTenants(db); err != nil {
		return fmt.Errorf("backfill order exception mark tenants: %w", err)
	}
	if err := operationtask.Migrate(db); err != nil {
		return err
	}
	if err := migrateProductPublishTenant(db); err != nil {
		return err
	}
	if err := inventorysync.Migrate(db); err != nil {
		return err
	}
	if err := migrateDouyinOrderIdempotencyIndexes(db); err != nil {
		return err
	}
	if err := migratePublishBatchIndexes(db); err != nil {
		return err
	}
	if err := migrateReliabilitySchema(db); err != nil {
		return err
	}
	if err := migrateTaskExecutionTracking(db); err != nil {
		return err
	}
	if err := migrateWorkerRecoveryIndexes(db); err != nil {
		return err
	}
	if err := migrateDouyinPlatform(db); err != nil {
		return err
	}
	if err := migrateDouyinOrderRevision(db); err != nil {
		return err
	}
	if err := migrateWebhookRouting(db); err != nil {
		return err
	}
	if err := migrateAuthAuditSecurity(db); err != nil {
		return err
	}
	if err := migrateKeyRotationSecurity(db); err != nil {
		return err
	}
	if err := migrateTenantSecurity(db); err != nil {
		return err
	}
	if err := migrateObservability(db); err != nil {
		return err
	}
	if err := migratePerformance(db); err != nil {
		return err
	}
	return migrateCustomerAutoReplyReliability(db)
}
