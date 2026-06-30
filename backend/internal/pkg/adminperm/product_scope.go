package adminperm

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApplyProductScope restricts product queries to authorized stores for non-admin principals.
// Products are visible when linked via product_platform_publish_configs.shop_id or
// product_publications.shop_id. Unassigned drafts (no shop link) are admin-only.
func ApplyProductScope(c *gin.Context, db *gorm.DB, tx *gorm.DB) (*gorm.DB, error) {
	if tx == nil {
		return tx, nil
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return nil, err
	}
	if p.IsAdmin() {
		return tx, nil
	}
	ids := p.AllowedStoreIDs()
	if len(ids) == 0 {
		return tx.Where("1 = 0"), nil
	}
	return tx.Where(`products.id IN (
		SELECT DISTINCT product_id FROM product_platform_publish_configs
		WHERE shop_id IN ?
		UNION
		SELECT DISTINCT product_id FROM product_publications
		WHERE shop_id IN ? AND deleted_at IS NULL
	)`, ids, ids), nil
}

// EnsureProductVisible returns gorm.ErrRecordNotFound when product is out of scope.
func EnsureProductVisible(c *gin.Context, db *gorm.DB, productID uuid.UUID) error {
	if productID == uuid.Nil {
		return gorm.ErrRecordNotFound
	}
	p, err := LoadPrincipal(c, db)
	if err != nil {
		return err
	}
	if p.IsAdmin() {
		return nil
	}
	ids := p.AllowedStoreIDs()
	if len(ids) == 0 {
		return gorm.ErrRecordNotFound
	}
	var count int64
	err = db.WithContext(c.Request.Context()).Raw(`
SELECT COUNT(*)::bigint FROM (
	SELECT product_id FROM product_platform_publish_configs WHERE product_id = ? AND shop_id IN ?
	UNION
	SELECT product_id FROM product_publications WHERE product_id = ? AND shop_id IN ? AND deleted_at IS NULL
) scoped`, productID, ids, productID, ids).Scan(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
