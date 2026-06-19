package database

import (
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"gorm.io/gorm"
)

type duplicatePublishBatchIdempotencyRow struct {
	IdempotencyKey string `gorm:"column:idempotency_key"`
	Count          int64  `gorm:"column:cnt"`
}

// migratePublishBatchA21 hardens product_publish_batches / product_publish_tasks for Phase A2.1.
// Safe to re-run (IF NOT EXISTS). Rollback: see docs/PUBLISH_BATCH_MIGRATION.md
func migratePublishBatchA21(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate publish batch a21: db is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	if !db.Migrator().HasTable("product_publish_batches") {
		return nil
	}
	if err := ensurePublishBatchProductIDNullable(db); err != nil {
		return err
	}
	if err := createPublishBatchA21Indexes(db); err != nil {
		return err
	}
	return nil
}

func ensurePublishBatchProductIDNullable(db *gorm.DB) error {
	var nullable string
	err := db.Raw(`
SELECT is_nullable FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = 'product_publish_batches'
  AND column_name = 'product_id'`).Scan(&nullable).Error
	if err != nil {
		return fmt.Errorf("publish batch a21 product_id nullable check: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(nullable), "YES") {
		return nil
	}
	if err := db.Exec(`ALTER TABLE product_publish_batches ALTER COLUMN product_id DROP NOT NULL`).Error; err != nil {
		return fmt.Errorf("publish batch a21 product_id drop not null: %w", err)
	}
	return nil
}

func createPublishBatchA21Indexes(db *gorm.DB) error {
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS ix_publish_batches_created_at ON product_publish_batches (created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS ix_publish_batches_status ON product_publish_batches (status)`,
	}
	if db.Migrator().HasTable("product_publish_tasks") {
		taskModel := &productpublish.ProductPublishTask{}
		if db.Migrator().HasColumn(taskModel, "batch_id") {
			stmts = append(stmts, `CREATE INDEX IF NOT EXISTS ix_publish_tasks_batch_id ON product_publish_tasks (batch_id)`)
		}
		if db.Migrator().HasColumn(taskModel, "target_key") {
			stmts = append(stmts, `CREATE INDEX IF NOT EXISTS ix_publish_tasks_target_key ON product_publish_tasks (target_key)`)
		}
		if db.Migrator().HasColumn(taskModel, "batch_id") && db.Migrator().HasColumn(taskModel, "status") {
			stmts = append(stmts, `CREATE INDEX IF NOT EXISTS ix_publish_tasks_batch_status ON product_publish_tasks (batch_id, status)`)
		}
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("publish batch a21 index: %w", err)
		}
	}
	batchModel := &productpublish.ProductPublishBatch{}
	if !db.Migrator().HasColumn(batchModel, "idempotency_key") {
		return nil
	}
	dupes, err := checkPublishBatchIdempotencyDuplicates(db)
	if err != nil {
		return err
	}
	if len(dupes) > 0 {
		// Skip unique index; operators must dedupe manually (documented in PUBLISH_BATCH_MIGRATION.md).
		return nil
	}
	if err := db.Exec(`
CREATE UNIQUE INDEX IF NOT EXISTS ux_publish_batches_idempotency_active
 ON product_publish_batches (idempotency_key)
 WHERE idempotency_key <> '' AND status NOT IN ('failed','cancelled')`).Error; err != nil {
		return fmt.Errorf("publish batch a21 idempotency unique index: %w", err)
	}
	return nil
}

func checkPublishBatchIdempotencyDuplicates(db *gorm.DB) ([]duplicatePublishBatchIdempotencyRow, error) {
	var rows []duplicatePublishBatchIdempotencyRow
	err := db.Raw(`
SELECT idempotency_key, COUNT(*) AS cnt
FROM product_publish_batches
WHERE idempotency_key <> ''
  AND status NOT IN ('failed','cancelled')
GROUP BY idempotency_key
HAVING COUNT(*) > 1
LIMIT 20`).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("publish batch a21 duplicate idempotency check: %w", err)
	}
	return rows, nil
}
