package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/aiproductimage"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/exportmod"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"gorm.io/gorm"
)

// migrateTenantSecurity applies tenant columns, export jobs, and security worker indexes.
func migrateTenantSecurity(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate tenant security: db is nil")
	}
	if err := db.AutoMigrate(
		&inventory.InventorySyncTask{},
		&inventory.InventorySyncBatch{},
		&inventory.InventoryChangeLog{},
		&imagetask.ImageTask{},
		&ordersync.OrderSyncTask{},
		&customersync.CustomerMessageSyncTask{},
		&productpublish.ProductPublishTask{},
		&aiproducttext.AIProductTextBatch{},
		&aiproductimage.AIProductImageBatch{},
		&customerchat.CustomerConversation{},
		&collect.CollectTask{},
		&collect.CollectBatch{},
		&taskcenter.TaskFailureMark{},
		&taskcenter.TaskAlert{},
		&product.DouyinImageAsset{},
		&exportmod.ExportJob{},
		&files.FileRecord{},
	); err != nil {
		return err
	}
	if err := backfillTenantIDs(db); err != nil {
		return err
	}
	return migrateTenantSecurityIndexes(db)
}

func backfillTenantIDs(db *gorm.DB) error {
	stmts := []string{
		`UPDATE inventory_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE inventory_sync_batches t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE inventory_change_logs t SET tenant_id = p.tenant_id FROM products p WHERE t.product_id = p.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE image_tasks t SET tenant_id = p.tenant_id FROM products p WHERE t.product_id = p.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE order_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE customer_message_sync_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE product_publish_tasks t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE ai_product_text_batches t SET tenant_id = p.tenant_id FROM ai_product_text_items i JOIN products p ON i.product_id = p.id WHERE i.batch_id = t.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE ai_product_image_batches t SET tenant_id = p.tenant_id FROM ai_product_image_items i JOIN products p ON i.product_id = p.id WHERE i.batch_id = t.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE customer_conversations t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE collect_tasks t SET tenant_id = p.tenant_id FROM products p WHERE t.result_product_id = p.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
		`UPDATE douyin_image_assets t SET tenant_id = s.tenant_id FROM shops s WHERE t.shop_id = s.id AND (t.tenant_id IS NULL OR t.tenant_id = 0)`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			// SQLite dev may not support UPDATE FROM; skip non-fatal.
			_ = err
		}
	}
	return backfillCollectTenantIDs(db)
}

func backfillCollectTenantIDs(db *gorm.DB) error {
	stmts := []string{
		`UPDATE products SET tenant_id = (SELECT admin_users.tenant_id FROM admin_users WHERE admin_users.id = products.created_by AND admin_users.tenant_id > 0 LIMIT 1) WHERE tenant_id = 0 AND created_by IS NOT NULL AND EXISTS (SELECT 1 FROM admin_users WHERE admin_users.id = products.created_by AND admin_users.tenant_id > 0)`,
		`UPDATE collect_batches SET tenant_id = (SELECT admin_users.tenant_id FROM admin_users WHERE admin_users.id = collect_batches.created_by AND admin_users.tenant_id > 0 LIMIT 1) WHERE tenant_id = 0 AND created_by IS NOT NULL AND EXISTS (SELECT 1 FROM admin_users WHERE admin_users.id = collect_batches.created_by AND admin_users.tenant_id > 0)`,
		`UPDATE collect_tasks SET tenant_id = (SELECT admin_users.tenant_id FROM admin_users WHERE admin_users.id = collect_tasks.created_by AND admin_users.tenant_id > 0 LIMIT 1) WHERE tenant_id = 0 AND created_by IS NOT NULL AND EXISTS (SELECT 1 FROM admin_users WHERE admin_users.id = collect_tasks.created_by AND admin_users.tenant_id > 0)`,
		`UPDATE collect_tasks SET tenant_id = (SELECT collect_batches.tenant_id FROM collect_batches WHERE collect_batches.id = collect_tasks.batch_id AND collect_batches.tenant_id > 0 LIMIT 1) WHERE tenant_id = 0 AND batch_id IS NOT NULL AND EXISTS (SELECT 1 FROM collect_batches WHERE collect_batches.id = collect_tasks.batch_id AND collect_batches.tenant_id > 0)`,
		`UPDATE collect_tasks SET tenant_id = (SELECT products.tenant_id FROM products WHERE products.id = collect_tasks.result_product_id AND products.tenant_id > 0 LIMIT 1) WHERE tenant_id = 0 AND result_product_id IS NOT NULL AND EXISTS (SELECT 1 FROM products WHERE products.id = collect_tasks.result_product_id AND products.tenant_id > 0)`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("collect tenant backfill: %w", err)
		}
	}
	return nil
}

func migrateTenantSecurityIndexes(db *gorm.DB) error {
	for _, legacy := range []string{
		"uq_task_alert_type_src_cat", "uq_task_alert_tenant_type_src_cat",
		"uniq_task_failure_mark", "uniq_task_failure_mark_tenant",
	} {
		if err := db.Exec("DROP INDEX IF EXISTS " + legacy).Error; err != nil {
			return fmt.Errorf("drop legacy tenant index %s: %w", legacy, err)
		}
	}
	type idx struct {
		table string
		name  string
		sql   string
	}
	indexes := []idx{
		{"task_alerts", "uq_task_alert_tenant_type_src_cat", "CREATE UNIQUE INDEX IF NOT EXISTS uq_task_alert_tenant_type_src_cat ON task_alerts (tenant_id, task_type, source_id, failure_category)"},
		{"task_failure_marks", "uniq_task_failure_mark_tenant", "CREATE UNIQUE INDEX IF NOT EXISTS uniq_task_failure_mark_tenant ON task_failure_marks (tenant_id, task_type, source_id, mark_type)"},
		{"image_tasks", "idx_image_tasks_tenant", "CREATE INDEX IF NOT EXISTS idx_image_tasks_tenant ON image_tasks (tenant_id, updated_at)"},
		{"inventory_sync_tasks", "idx_inv_sync_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_inv_sync_tenant_shop ON inventory_sync_tasks (tenant_id, shop_id)"},
		{"order_sync_tasks", "idx_order_sync_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_order_sync_tenant_shop ON order_sync_tasks (tenant_id, shop_id)"},
		{"product_publish_tasks", "idx_publish_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_publish_tenant_shop ON product_publish_tasks (tenant_id, shop_id)"},
		{"ai_product_text_batches", "idx_ai_text_tenant", "CREATE INDEX IF NOT EXISTS idx_ai_text_tenant ON ai_product_text_batches (tenant_id, created_at)"},
		{"ai_product_image_batches", "idx_ai_image_tenant", "CREATE INDEX IF NOT EXISTS idx_ai_image_tenant ON ai_product_image_batches (tenant_id, created_at)"},
		{"export_jobs", "idx_export_jobs_tenant", "CREATE INDEX IF NOT EXISTS idx_export_jobs_tenant ON export_jobs (tenant_id, created_at)"},
		{"files", "idx_files_tenant_security", "CREATE INDEX IF NOT EXISTS idx_files_tenant_security ON files (tenant_id, security_status)"},
		{"task_failure_marks", "idx_task_failure_tenant", "CREATE INDEX IF NOT EXISTS idx_task_failure_tenant ON task_failure_marks (tenant_id, task_type)"},
		{"douyin_image_assets", "idx_douyin_img_tenant_shop", "CREATE INDEX IF NOT EXISTS idx_douyin_img_tenant_shop ON douyin_image_assets (tenant_id, shop_id)"},
	}
	for _, i := range indexes {
		if !db.Migrator().HasTable(i.table) {
			continue
		}
		if err := db.Exec(i.sql).Error; err != nil {
			return fmt.Errorf("tenant security index %s: %w", i.name, err)
		}
	}
	return nil
}
