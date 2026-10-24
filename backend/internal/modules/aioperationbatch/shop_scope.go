package aioperationbatch

import (
	"context"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

// aiShopScope returns whether the request carries an explicit shop scope. A
// nil ShopScope is the deliberate all-store marker used by administrators and
// workers; an empty non-nil slice means no store grant.
func aiShopScope(ctx context.Context) (ids []uuid.UUID, restricted bool) {
	tc := security.FromContext(ctx)
	if tc == nil || tc.ShopScope == nil {
		return nil, false
	}
	return tc.ShopScope, true
}

func applyAIProductShopScope(tx *gorm.DB, ctx context.Context, tenantID int64, column string) *gorm.DB {
	if tx == nil {
		return tx
	}
	ids, restricted := aiShopScope(ctx)
	if !restricted {
		return tx
	}
	if len(ids) == 0 {
		return tx.Where("1 = 0")
	}
	if column == "" {
		column = "products.id"
	}
	return tx.Where(column+` IN (
		SELECT p.id FROM products p
		WHERE p.tenant_id = ? AND p.id IN (
			SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
			UNION
			SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
		)
	)`, tenantID, ids, ids)
}

func applyAIBatchShopScope(tx *gorm.DB, ctx context.Context, tenantID int64) *gorm.DB {
	if tx == nil {
		return tx
	}
	ids, restricted := aiShopScope(ctx)
	if !restricted {
		return tx
	}
	if len(ids) == 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(`ai_operation_batches.id IN (
		SELECT at.batch_id
		FROM ai_tasks at
		JOIN products p ON p.id = at.product_id
		WHERE p.tenant_id = ? AND p.id IN (
			SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
			UNION
			SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
		)
		UNION
		SELECT it.batch_id
		FROM image_tasks it
		JOIN products p2 ON p2.id = it.product_id
		WHERE p2.tenant_id = ? AND p2.id IN (
			SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
			UNION
			SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
		)
	)`, tenantID, ids, ids, tenantID, ids, ids)
}
