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
}

func scopeFromContext(c *gin.Context, db *gorm.DB) Scope {
	if c == nil {
		return Scope{IsAdmin: true}
	}
	p, _ := adminperm.LoadPrincipal(c, db)
	if p == nil || p.IsAdmin() {
		return Scope{IsAdmin: true}
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
