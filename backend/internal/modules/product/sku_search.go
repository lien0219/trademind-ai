package product

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

var (
	errSKUSearchAuthenticationRequired = errors.New("authentication_required")
	errSKUSearchPermissionDenied       = errors.New("permission_denied")
)

// ProductSKUSearchHit is a trimmed row for admin SKU picker.
type ProductSKUSearchHit struct {
	ProductID    string `json:"productId"`
	ProductTitle string `json:"productTitle"`
	ProductSKUID string `json:"productSkuId"`
	SKUCode      string `json:"skuCode"`
	SKUName      string `json:"skuName"`
	Stock        *int   `json:"stock,omitempty"`
	Attrs        any    `json:"attrs,omitempty"`
}

// SearchSKUsQuery GET /product-skus/search
type SearchSKUsQuery struct {
	Keyword   string
	ProductID *string
	Limit     int
}

type skuSearchRepository interface {
	SearchSKUs(ctx context.Context, tenantID int64, q SearchSKUsQuery) ([]ProductSKUSearchHit, error)
}

type gormSKUSearchRepository struct {
	db *gorm.DB
}

// RequireSKUSearchTenant validates the trusted authenticated tenant membership.
func (s *Service) RequireSKUSearchTenant(c *gin.Context) (int64, error) {
	if c == nil {
		return 0, errSKUSearchAuthenticationRequired
	}
	tenantValue, ok := c.Get(ctxkey.TenantID)
	if !ok {
		return 0, errSKUSearchAuthenticationRequired
	}
	var tenantID int64
	switch value := tenantValue.(type) {
	case int64:
		tenantID = value
	case int:
		tenantID = int64(value)
	default:
		return 0, errSKUSearchAuthenticationRequired
	}
	if tenantID < 0 {
		return 0, errSKUSearchAuthenticationRequired
	}

	tenantContext := security.FromGin(c)
	if tenantContext == nil || tenantContext.TenantID < 0 || tenantContext.UserID == uuid.Nil ||
		(tenantContext.TenantID == 0 && tenantContext.AuthSource != security.AuthSourceLegacyDevZero) {
		return 0, errSKUSearchAuthenticationRequired
	}
	if tenantContext.TenantID != tenantID {
		return 0, errSKUSearchPermissionDenied
	}

	actorValue, ok := c.Get(ctxkey.AdminID)
	if !ok {
		return 0, errSKUSearchAuthenticationRequired
	}
	actorRaw, ok := actorValue.(string)
	if !ok {
		return 0, errSKUSearchAuthenticationRequired
	}
	actorID, err := uuid.Parse(strings.TrimSpace(actorRaw))
	if err != nil || actorID == uuid.Nil {
		return 0, errSKUSearchAuthenticationRequired
	}
	if actorID != tenantContext.UserID {
		return 0, errSKUSearchPermissionDenied
	}
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("product: no db")
	}

	var user admin.AdminUser
	err = s.DB.WithContext(c.Request.Context()).
		Select("id", "tenant_id", "status").
		First(&user, "id = ?", actorID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, errSKUSearchPermissionDenied
	}
	if err != nil {
		return 0, err
	}
	if user.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusActive) {
		return 0, errSKUSearchPermissionDenied
	}
	return tenantID, nil
}

// SearchSKUs runs tenant-scoped keyword search across sku_code, sku_name, and product.title.
func (s *Service) SearchSKUs(ctx context.Context, tenantID int64, q SearchSKUsQuery) ([]ProductSKUSearchHit, error) {
	if tenantID < 0 {
		return nil, errSKUSearchAuthenticationRequired
	}
	if s == nil {
		return nil, fmt.Errorf("product: no service")
	}
	repo := s.skuSearchRepo
	if repo == nil {
		if s.DB == nil {
			return nil, fmt.Errorf("product: no db")
		}
		repo = &gormSKUSearchRepository{db: s.DB}
	}
	return repo.SearchSKUs(ctx, tenantID, q)
}

func (r *gormSKUSearchRepository) SearchSKUs(ctx context.Context, tenantID int64, q SearchSKUsQuery) ([]ProductSKUSearchHit, error) {
	if tenantID < 0 {
		return nil, errSKUSearchAuthenticationRequired
	}
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("product: no db")
	}
	kw := strings.TrimSpace(q.Keyword)
	lim := q.Limit
	if lim <= 0 {
		lim = 20
	}
	if lim > 50 {
		lim = 50
	}
	tx := r.db.WithContext(ctx).
		Table("product_skus AS sk").
		Select(`sk.id AS sku_id, sk.product_id AS product_id, sk.sku_code AS sku_code, sk.sku_name AS sku_name, sk.stock AS stock, sk.attrs AS attrs,
			p.title AS product_title`).
		Joins("JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL").
		Where("p.tenant_id = ?", tenantID)
	if q.ProductID != nil && strings.TrimSpace(*q.ProductID) != "" {
		tx = tx.Where("sk.product_id = ?", strings.TrimSpace(*q.ProductID))
	}
	if kw != "" {
		pat := "%" + strings.ToLower(kw) + "%"
		tx = tx.Where(`(LOWER(sk.sku_code) LIKE ? OR LOWER(sk.sku_name) LIKE ? OR LOWER(p.title) LIKE ?)`, pat, pat, pat)
	}
	type row struct {
		SKUID        string `gorm:"column:sku_id"`
		ProductID    string `gorm:"column:product_id"`
		SKUCode      string `gorm:"column:sku_code"`
		SKUName      string `gorm:"column:sku_name"`
		Stock        *int   `gorm:"column:stock"`
		Attrs        any    `gorm:"column:attrs"`
		ProductTitle string `gorm:"column:product_title"`
	}
	var rows []row
	if err := tx.Order("p.updated_at DESC, sk.created_at ASC").Limit(lim).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]ProductSKUSearchHit, 0, len(rows))
	for _, item := range rows {
		out = append(out, ProductSKUSearchHit{
			ProductID:    item.ProductID,
			ProductTitle: item.ProductTitle,
			ProductSKUID: item.SKUID,
			SKUCode:      item.SKUCode,
			SKUName:      item.SKUName,
			Stock:        item.Stock,
			Attrs:        item.Attrs,
		})
	}
	return out, nil
}
