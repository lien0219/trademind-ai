package database

import (
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"gorm.io/gorm"
)

// migrateAIOperationBatchTenant adds the batch ownership boundary and fills it
// only when all persisted child evidence points to one non-zero tenant.
// Ambiguous or orphaned legacy rows stay at tenant zero and are intentionally
// invisible to authenticated non-zero tenants.
func migrateAIOperationBatchTenant(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&aioperationbatch.AIOperationBatch{}) ||
		!db.Migrator().HasColumn(&aioperationbatch.AIOperationBatch{}, "tenant_id") {
		return nil
	}

	return db.Transaction(backfillAIOperationBatchTenants)
}

func backfillAIOperationBatchTenants(db *gorm.DB) error {
	type ownership struct {
		BatchID  string `gorm:"column:batch_id"`
		TenantID int64  `gorm:"column:tenant_id"`
	}
	// Consider both child kinds together. Dropping unknown owners before the
	// aggregation, or choosing one child kind first, could assign mixed-tenant
	// evidence to the wrong tenant. Missing product evidence remains unknown.
	var childOwners []ownership
	if err := db.Raw(`
		SELECT evidence.batch_id, MIN(evidence.tenant_id) AS tenant_id
		FROM (
			SELECT t.batch_id, COALESCE(p.tenant_id, 0) AS tenant_id
			FROM ai_tasks t
			JOIN ai_operation_batches b ON b.id = t.batch_id AND b.tenant_id = 0
			LEFT JOIN products p ON p.id = t.product_id
			UNION ALL
			SELECT i.batch_id, COALESCE(i.tenant_id, 0) AS tenant_id
			FROM image_tasks i
			JOIN ai_operation_batches b ON b.id = i.batch_id AND b.tenant_id = 0
		) evidence
		GROUP BY evidence.batch_id
		HAVING MIN(evidence.tenant_id) > 0 AND COUNT(DISTINCT evidence.tenant_id) = 1
	`).Scan(&childOwners).Error; err != nil {
		return fmt.Errorf("scan ai batch child ownership: %w", err)
	}
	for _, owner := range childOwners {
		if err := db.Table("ai_operation_batches").
			Where("id = ? AND tenant_id = 0", owner.BatchID).
			Update("tenant_id", owner.TenantID).Error; err != nil {
			return fmt.Errorf("backfill ai batch child tenant: %w", err)
		}
	}

	// A childless legacy batch can be assigned from its creator. Do not use
	// creator fallback when child rows exist: their ownership is stronger
	// evidence and may be ambiguous.
	var creatorOwners []ownership
	if err := db.Table("ai_operation_batches AS b").
		Select("b.id AS batch_id, u.tenant_id").
		Joins("JOIN admin_users AS u ON u.id = b.created_by").
		Where("b.tenant_id = 0 AND u.tenant_id <> 0").
		Where("NOT EXISTS (SELECT 1 FROM ai_tasks t WHERE t.batch_id = b.id)").
		Where("NOT EXISTS (SELECT 1 FROM image_tasks i WHERE i.batch_id = b.id)").
		Scan(&creatorOwners).Error; err != nil {
		return fmt.Errorf("scan childless ai batch ownership: %w", err)
	}
	for _, owner := range creatorOwners {
		if err := db.Table("ai_operation_batches").
			Where("id = ? AND tenant_id = 0", owner.BatchID).
			Update("tenant_id", owner.TenantID).Error; err != nil {
			return fmt.Errorf("backfill childless ai batch tenant: %w", err)
		}
	}
	return nil
}
