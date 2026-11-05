package operationdashboard

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// Scope carries RBAC store filters for dashboard aggregation.
type Scope struct {
	AllowedShopIDs []uuid.UUID // nil = admin (all stores)
	IsAdmin        bool
	TenantID       *int64
}

func scopeFromContext(c *gin.Context, db *gorm.DB) Scope {
	if c == nil {
		return Scope{IsAdmin: true}
	}
	tenantID, tenantErr := adminperm.TenantIDFromGin(c)
	p, _ := adminperm.LoadPrincipal(c, db)
	if p == nil || p.IsAdmin() {
		if tenantErr == nil {
			return Scope{IsAdmin: true, TenantID: &tenantID}
		}
		return Scope{IsAdmin: true}
	}
	if tenantErr == nil {
		return Scope{AllowedShopIDs: p.AllowedStoreIDs(), TenantID: &tenantID}
	}
	return Scope{AllowedShopIDs: p.AllowedStoreIDs()}
}

func (sc Scope) applyShopColumn(tx *gorm.DB, column string) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if len(sc.AllowedShopIDs) == 0 {
		return tx.Where("1 = 0")
	}
	col := column
	if col == "" {
		col = "shop_id"
	}
	return tx.Where(col+" IN ?", sc.AllowedShopIDs)
}

func (sc Scope) applyProductScope(tx *gorm.DB) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if len(sc.AllowedShopIDs) == 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(`products.id IN (
		SELECT DISTINCT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
		UNION
		SELECT DISTINCT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
	)`, sc.AllowedShopIDs, sc.AllowedShopIDs)
}

// applyProductIDScope restricts a child query that carries a product ID but
// does not join the products table itself. Unassigned products stay hidden
// from scoped roles because they have no authorized shop linkage.
func (sc Scope) applyProductIDScope(tx *gorm.DB, tenantID int64, column string) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if len(sc.AllowedShopIDs) == 0 {
		return tx.Where("1 = 0")
	}
	if column == "" {
		column = "product_id"
	}
	return tx.Where(column+` IN (
		SELECT p.id FROM products p
		WHERE p.tenant_id = ? AND p.id IN (
			SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
			UNION
			SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
		)
	)`, tenantID, sc.AllowedShopIDs, sc.AllowedShopIDs)
}

// applyBatchScope restricts batches through their product-linked tasks. A
// batch is visible to a scoped role only when at least one task belongs to an
// authorized product in the current tenant.
func (sc Scope) applyBatchScope(tx *gorm.DB, tenantID int64) *gorm.DB {
	if tx == nil || sc.IsAdmin {
		return tx
	}
	if len(sc.AllowedShopIDs) == 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(`(
		EXISTS (
			SELECT 1 FROM ai_tasks at
			JOIN products p ON p.id = at.product_id
			WHERE at.batch_id = ai_operation_batches.id
			  AND p.tenant_id = ?
			  AND p.id IN (
				SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
				UNION
				SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
			  )
		)
		OR EXISTS (
			SELECT 1 FROM image_tasks it
			JOIN products p2 ON p2.id = it.product_id
			WHERE it.batch_id = ai_operation_batches.id
			  AND p2.tenant_id = ?
			  AND p2.id IN (
				SELECT product_id FROM product_platform_publish_configs WHERE shop_id IN ?
				UNION
				SELECT product_id FROM product_publications WHERE shop_id IN ? AND deleted_at IS NULL
			  )
		)
	)`, tenantID, sc.AllowedShopIDs, sc.AllowedShopIDs, tenantID, sc.AllowedShopIDs, sc.AllowedShopIDs)
}

func (sc Scope) shopIDStrings() []string {
	if len(sc.AllowedShopIDs) == 0 {
		return nil
	}
	out := make([]string, 0, len(sc.AllowedShopIDs))
	for _, id := range sc.AllowedShopIDs {
		out = append(out, id.String())
	}
	return out
}
