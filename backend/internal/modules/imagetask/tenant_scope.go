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
	tx = tx.Where("image_tasks.tenant_id = ?", tenantID)
	return scopeImageShopQuery(tx, security.FromContext(tx.Statement.Context), tenantID, "image_tasks.product_id")
}

func scopeImageTaskItemQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	tc := security.FromContext(tx.Statement.Context)
	if tc == nil || tc.ShopScope == nil {
		return tx.Where(`EXISTS (
			SELECT 1 FROM image_tasks
			WHERE image_tasks.id = image_task_items.task_id
			  AND image_tasks.tenant_id = ?
		)`, tenantID)
	}
	if len(tc.ShopScope) == 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(`EXISTS (
		SELECT 1 FROM image_tasks
		WHERE image_tasks.id = image_task_items.task_id
		  AND image_tasks.tenant_id = ?
		  AND image_tasks.product_id IN (
			SELECT p.id FROM products p
			WHERE p.tenant_id = ? AND p.id IN (
				SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
				UNION
				SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
			)
		  )
	)`, tenantID, tenantID, tc.ShopScope, tc.ShopScope)
}

func scopeProductQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	tx = tx.Where("products.tenant_id = ?", tenantID)
	return scopeImageShopQuery(tx, security.FromContext(tx.Statement.Context), tenantID, "products.id")
}

func scopeProductImageQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	tx = tx.Where(`product_images.product_id IN (
		SELECT products.id FROM products WHERE products.tenant_id = ?
	)`, tenantID)
	return scopeImageShopQuery(tx, security.FromContext(tx.Statement.Context), tenantID, "product_images.product_id")
}

func scopeFileRecordQuery(tx *gorm.DB, tenantID int64, scoped bool) *gorm.DB {
	if tx == nil || !scoped {
		return tx
	}
	if tc := security.FromContext(tx.Statement.Context); tc != nil && tc.ShopScope != nil {
		// File records do not carry a shop or product foreign key. A scoped
		// role therefore cannot prove ownership of an arbitrary tenant file;
		// fail closed and require a product image linked to an authorized shop.
		return tx.Where("1 = 0")
	}
	return tx.Where("files.tenant_id = ?", tenantID)
}

// scopeImageShopQuery applies the request's explicit store grants to a
// product-bearing query. A nil ShopScope deliberately keeps worker/admin
// all-store behavior; an empty scope fails closed.
func scopeImageShopQuery(tx *gorm.DB, tc *security.TenantContext, tenantID int64, productColumn string) *gorm.DB {
	if tx == nil || tc == nil || tc.ShopScope == nil {
		return tx
	}
	if len(tc.ShopScope) == 0 {
		return tx.Where("1 = 0")
	}
	if productColumn == "" {
		productColumn = "product_id"
	}
	return tx.Where(productColumn+` IN (
		SELECT p.id FROM products p
		WHERE p.tenant_id = ? AND p.id IN (
			SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
			UNION
			SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
		)
	)`, tenantID, tc.ShopScope, tc.ShopScope)
}
