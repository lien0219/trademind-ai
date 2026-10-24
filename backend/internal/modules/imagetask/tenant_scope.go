package imagetask

import (
	"context"

	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// imageTaskTenantScope returns the authenticated tenant when one is attached
// to the context. Older internal callers may intentionally omit the context;
// in that case scoped is false so existing worker/unit-test call paths keep
// their legacy behavior. A present but invalid context always fails closed.
func imageTaskTenantScope(ctx context.Context) (tenantID int64, scoped bool, err error) {
	if security.FromContext(ctx) == nil {
		return 0, false, nil
	}
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return 0, true, err
	}
	return tenant.TenantID, true, nil
}

func scopeImageTaskQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	return tx.Where("image_tasks.tenant_id = ?", tenantID)
}

func scopeImageTaskItemQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	return tx.Where(`EXISTS (
		SELECT 1 FROM image_tasks
		WHERE image_tasks.id = image_task_items.task_id
		  AND image_tasks.tenant_id = ?
	)`, tenantID)
}

func scopeProductQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	return tx.Where("products.tenant_id = ?", tenantID)
}

func scopeProductImageQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	return tx.Where(`product_images.product_id IN (
		SELECT products.id FROM products WHERE products.tenant_id = ?
	)`, tenantID)
}

func scopeFileRecordQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	return tx.Where("files.tenant_id = ?", tenantID)
}
