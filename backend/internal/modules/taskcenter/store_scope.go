package taskcenter

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// applyTaskAlertStoreScope applies the same tenant and store boundary to alerts
// and their notification history. Only known tenant-owned source types and
// known tenant-scoped shop task tables are eligible for non-admin results.
func applyTaskAlertStoreScope(q *gorm.DB, tenantID int64, restrict bool, allowed []uuid.UUID) *gorm.DB {
	if !restrict {
		return q
	}
	condition := `(
		(task_type = ? AND source_id IN (SELECT id::text FROM collect_tasks WHERE tenant_id = ?))
		OR (task_type = ? AND source_id IN (SELECT id::text FROM image_tasks WHERE tenant_id = ?))
		OR (task_type = ? AND source_id IN (SELECT i.id::text FROM ai_product_text_items i JOIN products p ON p.id = i.product_id WHERE p.tenant_id = ? AND p.deleted_at IS NULL))
		OR (task_type = ? AND source_id IN (SELECT i.id::text FROM ai_product_image_items i JOIN products p ON p.id = i.product_id WHERE p.tenant_id = ? AND p.deleted_at IS NULL))
		OR (task_type = ? AND source_id IN (SELECT id::text FROM customer_failure_events WHERE shop_id IS NULL AND conversation_id IN (SELECT id FROM customer_conversations WHERE tenant_id = ?)))
	)`
	args := []any{TaskTypeCollect, tenantID, TaskTypeImage, tenantID, TaskTypeAIText, tenantID, TaskTypeAIImage, tenantID, TaskTypeCustomerFailure, tenantID}
	if len(allowed) > 0 {
		condition += ` OR (task_type = ? AND source_id IN (SELECT id::text FROM order_sync_tasks WHERE tenant_id = ? AND shop_id IN (SELECT id FROM shops WHERE tenant_id = ? AND id IN ?)))`
		args = append(args, TaskTypeOrderSync, tenantID, tenantID, allowed)
		condition += ` OR (task_type = ? AND source_id IN (SELECT id::text FROM customer_message_sync_tasks WHERE tenant_id = ? AND shop_id IN (SELECT id FROM shops WHERE tenant_id = ? AND id IN ?)))`
		args = append(args, TaskTypeCustomerMessageSync, tenantID, tenantID, allowed)
		condition += ` OR (task_type = ? AND source_id IN (SELECT id::text FROM product_publish_tasks WHERE tenant_id = ? AND shop_id IN (SELECT id FROM shops WHERE tenant_id = ? AND id IN ?)))`
		args = append(args, TaskTypeProductPublish, tenantID, tenantID, allowed)
		condition += ` OR (task_type = ? AND source_id IN (SELECT id::text FROM inventory_sync_tasks WHERE tenant_id = ? AND shop_id IN (SELECT id FROM shops WHERE tenant_id = ? AND id IN ?)))`
		args = append(args, TaskTypeInventorySync, tenantID, tenantID, allowed)
		condition += ` OR (task_type = ? AND source_id IN (SELECT id::text FROM customer_failure_events WHERE shop_id IN (SELECT id FROM shops WHERE tenant_id = ? AND id IN ?) AND conversation_id IN (SELECT id FROM customer_conversations WHERE tenant_id = ?)))`
		args = append(args, TaskTypeCustomerFailure, tenantID, allowed, tenantID)
	}
	// Keep the source-tenant branches and every dynamically added store branch
	// in one predicate. Without this outer group, SQL evaluates
	// `tenant_id = ? AND source_scope OR store_scope`, allowing the OR branches
	// to return alerts from other tenants.
	return q.Where("("+condition+")", args...)
}
