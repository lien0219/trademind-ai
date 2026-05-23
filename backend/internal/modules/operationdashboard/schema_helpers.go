package operationdashboard

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// SQL fragments for child tables (HardDeleteBase — no deleted_at column).
const (
	sqlMainImageExists = `EXISTS (
		SELECT 1 FROM product_images pi
		WHERE pi.product_id = products.id AND pi.image_type = ?
	)`
	sqlSKUExists = `EXISTS (
		SELECT 1 FROM product_skus ps
		WHERE ps.product_id = products.id
	)`
	sqlInvalidPriceSKU = `EXISTS (
		SELECT 1 FROM product_skus ps2
		WHERE ps2.product_id = products.id AND (ps2.price IS NULL OR ps2.price <= 0)
	)`
)

type schemaFlags struct {
	once              sync.Once
	productSoftDelete bool
	productAIDesc     bool
}

func (s *Service) hasDBColumn(ctx context.Context, table, column string) bool {
	if s == nil || s.DB == nil || table == "" || column == "" {
		return false
	}
	var count int64
	err := s.DB.WithContext(ctx).Raw(`
SELECT COUNT(*)::bigint FROM information_schema.columns
WHERE table_schema = ANY (current_schemas(true))
  AND table_name = ?
  AND column_name = ?`, table, column).Scan(&count).Error
	return err == nil && count > 0
}

func (s *Service) schemaFlags(ctx context.Context) (softDelete, aiDesc bool) {
	if s == nil || s.DB == nil {
		return false, false
	}
	s.flags.once.Do(func() {
		s.flags.productSoftDelete = s.hasDBColumn(ctx, "products", "deleted_at")
		s.flags.productAIDesc = s.hasDBColumn(ctx, "products", "ai_description")
	})
	return s.flags.productSoftDelete, s.flags.productAIDesc
}

func (s *Service) productTimeScope(ctx context.Context, q Query) *gorm.DB {
	tx := s.DB.WithContext(ctx).Table("products AS products")
	if soft, _ := s.schemaFlags(ctx); soft {
		tx = tx.Where("products.deleted_at IS NULL")
	}
	return s.applyProductScope(tx, q)
}

func (s *Service) aiDescMissingExpr(ctx context.Context) string {
	_, hasAIDesc := s.schemaFlags(ctx)
	if hasAIDesc {
		return `TRIM(COALESCE(products.ai_description,'')) = ''`
	}
	return `TRIM(COALESCE(products.description,'')) = ''`
}

func (s *Service) aiDescDoneExpr(ctx context.Context) string {
	_, hasAIDesc := s.schemaFlags(ctx)
	if hasAIDesc {
		return `TRIM(COALESCE(products.ai_description,'')) <> ''`
	}
	return fmt.Sprintf(`LENGTH(TRIM(COALESCE(products.description,''))) >= %d`, descShort)
}

func (s *Service) aiDescPresentExpr(ctx context.Context) string {
	_, hasAIDesc := s.schemaFlags(ctx)
	if hasAIDesc {
		return `(TRIM(COALESCE(products.ai_description,'')) <> '' OR LENGTH(TRIM(COALESCE(products.description,''))) >= ?)`
	}
	return `LENGTH(TRIM(COALESCE(products.description,''))) >= ?`
}

// scanOptionalMaxTime reads MAX(updated_at); returns nil when no rows or SQL NULL.
func scanOptionalMaxTime(tx *gorm.DB) *time.Time {
	var v sql.NullTime
	if err := tx.Select("MAX(updated_at)").Scan(&v).Error; err != nil || !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}
