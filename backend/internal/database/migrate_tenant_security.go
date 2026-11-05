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
	if db == nil {
		return fmt.Errorf("backfill tenant IDs: db is nil")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		stmts := []struct {
			table string
			sql   string
		}{
			{"inventory_sync_tasks", `UPDATE inventory_sync_tasks SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = inventory_sync_tasks.shop_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = inventory_sync_tasks.shop_id AND s.tenant_id > 0)`},
			{"inventory_sync_batches", `UPDATE inventory_sync_batches SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = inventory_sync_batches.shop_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = inventory_sync_batches.shop_id AND s.tenant_id > 0)`},
			{"inventory_change_logs", `UPDATE inventory_change_logs SET tenant_id = (SELECT p.tenant_id FROM products p WHERE p.id = inventory_change_logs.product_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM products p WHERE p.id = inventory_change_logs.product_id AND p.tenant_id > 0)`},
			{"image_tasks", `UPDATE image_tasks SET tenant_id = (SELECT p.tenant_id FROM products p WHERE p.id = image_tasks.product_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM products p WHERE p.id = image_tasks.product_id AND p.tenant_id > 0)`},
			{"order_sync_tasks", `UPDATE order_sync_tasks SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = order_sync_tasks.shop_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = order_sync_tasks.shop_id AND s.tenant_id > 0)`},
			{"customer_message_sync_tasks", `UPDATE customer_message_sync_tasks SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = customer_message_sync_tasks.shop_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = customer_message_sync_tasks.shop_id AND s.tenant_id > 0)`},
			{"product_publish_tasks", `UPDATE product_publish_tasks SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = product_publish_tasks.shop_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = product_publish_tasks.shop_id AND s.tenant_id > 0)`},
			{"customer_conversations", `UPDATE customer_conversations SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = customer_conversations.shop_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = customer_conversations.shop_id AND s.tenant_id > 0)`},
			{"douyin_image_assets", `UPDATE douyin_image_assets SET tenant_id = (SELECT s.tenant_id FROM shops s WHERE s.id = douyin_image_assets.shop_id) WHERE (tenant_id IS NULL OR tenant_id = 0) AND EXISTS (SELECT 1 FROM shops s WHERE s.id = douyin_image_assets.shop_id AND s.tenant_id > 0)`},
		}
		for _, stmt := range stmts {
			if !tx.Migrator().HasTable(stmt.table) {
				continue
			}
			if err := tx.Exec(stmt.sql).Error; err != nil {
				return fmt.Errorf("backfill tenant IDs for %s: %w", stmt.table, err)
			}
		}

		if err := backfillConsistentBatchTenants(tx, "ai_product_text_batches", "ai_product_text_items"); err != nil {
			return err
		}
		if err := backfillConsistentBatchTenants(tx, "ai_product_image_batches", "ai_product_image_items"); err != nil {
			return err
		}
		return backfillCollectTenantIDs(tx)
	})
}

func backfillConsistentBatchTenants(db *gorm.DB, batchTable, itemTable string) error {
	if !db.Migrator().HasTable(batchTable) || !db.Migrator().HasTable(itemTable) {
		return nil
	}
	sql := fmt.Sprintf(`
		WITH evidence AS (
			SELECT b.id AS batch_id, COALESCE(p.tenant_id, 0) AS tenant_id
			FROM %s b
			JOIN %s i ON i.batch_id = b.id
			LEFT JOIN products p ON p.id = i.product_id
			WHERE b.tenant_id = 0
			UNION ALL
			SELECT b.id AS batch_id, COALESCE(u.tenant_id, 0) AS tenant_id
			FROM %s b
			LEFT JOIN admin_users u ON u.id = b.created_by
			WHERE b.tenant_id = 0 AND b.created_by IS NOT NULL
			  AND NOT EXISTS (SELECT 1 FROM %s i WHERE i.batch_id = b.id)
		), owners AS (
			SELECT batch_id, MIN(tenant_id) AS tenant_id
			FROM evidence
			GROUP BY batch_id
			HAVING MIN(tenant_id) > 0 AND COUNT(DISTINCT tenant_id) = 1
		)
		UPDATE %s
		SET tenant_id = (SELECT owners.tenant_id FROM owners WHERE owners.batch_id = %s.id)
		WHERE tenant_id = 0 AND id IN (SELECT batch_id FROM owners)
	`, batchTable, itemTable, batchTable, itemTable, batchTable, batchTable)
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("backfill tenant IDs for %s from %s evidence: %w", batchTable, itemTable, err)
	}
	return nil
}

func backfillCollectTenantIDs(db *gorm.DB) error {
	stmts := []struct {
		table string
		sql   string
	}{
		{"products", `UPDATE products SET tenant_id = (SELECT admin_users.tenant_id FROM admin_users WHERE admin_users.id = products.created_by) WHERE tenant_id = 0 AND created_by IS NOT NULL AND EXISTS (SELECT 1 FROM admin_users WHERE admin_users.id = products.created_by AND admin_users.tenant_id > 0)`},
		{"collect_batches", `UPDATE collect_batches SET tenant_id = (SELECT admin_users.tenant_id FROM admin_users WHERE admin_users.id = collect_batches.created_by) WHERE tenant_id = 0 AND created_by IS NOT NULL AND EXISTS (SELECT 1 FROM admin_users WHERE admin_users.id = collect_batches.created_by AND admin_users.tenant_id > 0)`},
	}
	for _, stmt := range stmts {
		if !db.Migrator().HasTable(stmt.table) {
			continue
		}
		if err := db.Exec(stmt.sql).Error; err != nil {
			return fmt.Errorf("backfill tenant IDs for %s: %w", stmt.table, err)
		}
	}
	if !db.Migrator().HasTable("collect_tasks") {
		return nil
	}
	const collectTaskSQL = `
		WITH evidence AS (
			SELECT t.id AS task_id, COALESCE(u.tenant_id, 0) AS tenant_id
			FROM collect_tasks t
			LEFT JOIN admin_users u ON u.id = t.created_by
			WHERE t.tenant_id = 0 AND t.created_by IS NOT NULL
			UNION ALL
			SELECT t.id AS task_id, COALESCE(b.tenant_id, 0) AS tenant_id
			FROM collect_tasks t
			LEFT JOIN collect_batches b ON b.id = t.batch_id
			WHERE t.tenant_id = 0 AND t.batch_id IS NOT NULL
			UNION ALL
			SELECT t.id AS task_id, COALESCE(p.tenant_id, 0) AS tenant_id
			FROM collect_tasks t
			LEFT JOIN products p ON p.id = t.result_product_id
			WHERE t.tenant_id = 0 AND t.result_product_id IS NOT NULL
		), owners AS (
			SELECT task_id, MIN(tenant_id) AS tenant_id
			FROM evidence
			GROUP BY task_id
			HAVING MIN(tenant_id) > 0 AND COUNT(DISTINCT tenant_id) = 1
		)
		UPDATE collect_tasks
		SET tenant_id = (SELECT owners.tenant_id FROM owners WHERE owners.task_id = collect_tasks.id)
		WHERE tenant_id = 0 AND id IN (SELECT task_id FROM owners)
	`
	if err := db.Exec(collectTaskSQL).Error; err != nil {
		return fmt.Errorf("backfill tenant IDs for collect_tasks from creator, batch, and product evidence: %w", err)
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
