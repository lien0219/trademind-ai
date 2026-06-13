package database

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type duplicateOrderRow struct {
	ShopID          string `gorm:"column:shop_id"`
	Platform        string `gorm:"column:platform"`
	ExternalOrderID string `gorm:"column:external_order_id"`
	Count           int64  `gorm:"column:cnt"`
}

type duplicateOrderItemRow struct {
	OrderID        string `gorm:"column:order_id"`
	ExternalItemID string `gorm:"column:external_item_id"`
	Count          int64  `gorm:"column:cnt"`
}

type duplicateSampleID struct {
	ID string `gorm:"column:id"`
}

// migrateDouyinPhase102Indexes adds idempotency indexes for Douyin production hardening (Phase 10.2/10.3).
// Safe to re-run (IF NOT EXISTS). Rollback: DROP INDEX IF EXISTS ux_orders_shop_platform_ext_order;
func migrateDouyinPhase102Indexes(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("migrate phase102: db is nil")
	}
	if db.Dialector.Name() != "postgres" {
		return nil
	}
	if err := checkDouyinDuplicateOrders(db); err != nil {
		return err
	}
	if err := checkDouyinDuplicateOrderItems(db); err != nil {
		return err
	}
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_orders_shop_platform_ext_order
		 ON orders (shop_id, platform, external_order_id)
		 WHERE external_order_id IS NOT NULL AND external_order_id <> '' AND deleted_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_order_items_order_ext_item
		 ON order_items (order_id, external_item_id)
		 WHERE external_item_id IS NOT NULL AND external_item_id <> ''`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("phase102 index: %w", err)
		}
	}
	return nil
}

func checkDouyinDuplicateOrders(db *gorm.DB) error {
	var rows []duplicateOrderRow
	err := db.Raw(`
SELECT shop_id, platform, external_order_id, COUNT(*) AS cnt
FROM orders
WHERE deleted_at IS NULL
  AND external_order_id IS NOT NULL
  AND external_order_id <> ''
GROUP BY shop_id, platform, external_order_id
HAVING COUNT(*) > 1
LIMIT 20`).Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("phase102 duplicate order check failed: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	var samples []duplicateSampleID
	_ = db.Raw(`
SELECT id FROM orders
WHERE deleted_at IS NULL AND shop_id = ? AND platform = ? AND external_order_id = ?
ORDER BY created_at ASC LIMIT 5`,
		rows[0].ShopID, rows[0].Platform, rows[0].ExternalOrderID).Scan(&samples).Error
	ids := make([]string, 0, len(samples))
	for _, s := range samples {
		if strings.TrimSpace(s.ID) != "" {
			ids = append(ids, s.ID)
		}
	}
	return fmt.Errorf("phase102 blocked: found %d duplicate order groups (example shop=%s platform=%s external_order_id=%s count=%d sample_ids=%v); see docs/DOUYIN_DUPLICATE_DATA_REPAIR.md",
		len(rows), rows[0].ShopID, rows[0].Platform, rows[0].ExternalOrderID, rows[0].Count, ids)
}

func checkDouyinDuplicateOrderItems(db *gorm.DB) error {
	var rows []duplicateOrderItemRow
	err := db.Raw(`
SELECT order_id, external_item_id, COUNT(*) AS cnt
FROM order_items
WHERE external_item_id IS NOT NULL
  AND external_item_id <> ''
GROUP BY order_id, external_item_id
HAVING COUNT(*) > 1
LIMIT 20`).Scan(&rows).Error
	if err != nil {
		return fmt.Errorf("phase102 duplicate order item check failed: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	var samples []duplicateSampleID
	_ = db.Raw(`
SELECT id FROM order_items
WHERE order_id = ? AND external_item_id = ?
ORDER BY created_at ASC LIMIT 5`, rows[0].OrderID, rows[0].ExternalItemID).Scan(&samples).Error
	ids := make([]string, 0, len(samples))
	for _, s := range samples {
		if strings.TrimSpace(s.ID) != "" {
			ids = append(ids, s.ID)
		}
	}
	return fmt.Errorf("phase102 blocked: found %d duplicate order_item groups (example order_id=%s external_item_id=%s count=%d sample_ids=%v); see docs/DOUYIN_DUPLICATE_DATA_REPAIR.md",
		len(rows), rows[0].OrderID, rows[0].ExternalItemID, rows[0].Count, ids)
}
