package aiproductimage

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// requestTenantID returns the tenant established by the authenticated request.
// AI image operations fail closed when the middleware did not establish one.
func requestTenantID(c *gin.Context) (int64, error) {
	return adminperm.TenantIDFromGin(c)
}

// optionalRequestTenantID keeps low-level idempotency tests and trusted internal
// calls usable while every HTTP service entry point remains tenant-scoped.
func optionalRequestTenantID(c *gin.Context) (*int64, bool) {
	tid, err := requestTenantID(c)
	if err != nil {
		return nil, false
	}
	return &tid, true
}

func scopeBatchQuery(q *gorm.DB, tenantID int64) *gorm.DB {
	return q.Where("tenant_id = ?", tenantID)
}

func scopeItemQuery(q *gorm.DB, tenantID int64) *gorm.DB {
	return q.Where("batch_id IN (SELECT id FROM ai_product_image_batches WHERE tenant_id = ? AND deleted_at IS NULL)", tenantID)
}

func scopeProductQuery(q *gorm.DB, tenantID int64) *gorm.DB {
	return q.Where("tenant_id = ?", tenantID)
}

func scopeImageQuery(q *gorm.DB, tenantID int64) *gorm.DB {
	return q.Where("product_id IN (SELECT id FROM products WHERE tenant_id = ? AND deleted_at IS NULL)", tenantID)
}

func scopeTaskQuery(q *gorm.DB, tenantID int64) *gorm.DB {
	return q.Where("tenant_id = ?", tenantID)
}

func scopeApplicationQuery(q *gorm.DB, tenantID int64) *gorm.DB {
	return q.Where("product_id IN (SELECT id FROM products WHERE tenant_id = ? AND deleted_at IS NULL)", tenantID)
}

func scopeOptional(q *gorm.DB, tenantID *int64, scope func(*gorm.DB, int64) *gorm.DB) *gorm.DB {
	if tenantID == nil {
		return q
	}
	return scope(q, *tenantID)
}

// validateItemTenant verifies every resource reference before a write can
// reach the image task or product services. Related tables without a tenant
// column are constrained through their tenant-bearing parent.
func (s *Service) validateItemTenant(ctx context.Context, item *AIProductImageItem, tenantID int64) error {
	if item == nil {
		return gorm.ErrRecordNotFound
	}
	var p product.Product
	if err := scopeProductQuery(s.DB.WithContext(ctx), tenantID).Where("id = ?", item.ProductID).First(&p).Error; err != nil {
		return err
	}
	if item.ImageID != nil {
		var img product.ProductImage
		if err := scopeImageQuery(s.DB.WithContext(ctx), tenantID).
			Where("id = ? AND product_id = ?", *item.ImageID, item.ProductID).First(&img).Error; err != nil {
			return err
		}
	}
	if item.ImageTaskID != nil {
		var task imagetask.ImageTask
		if err := scopeTaskQuery(s.DB.WithContext(ctx), tenantID).Where("id = ?", *item.ImageTaskID).First(&task).Error; err != nil {
			return err
		}
		if task.ProductID == nil || *task.ProductID != item.ProductID {
			return fmt.Errorf("image task product mismatch")
		}
	}
	if item.ApplicationID != nil {
		var app product.ProductImageApplication
		if err := scopeApplicationQuery(s.DB.WithContext(ctx), tenantID).
			Where("id = ? AND product_id = ?", *item.ApplicationID, item.ProductID).First(&app).Error; err != nil {
			return err
		}
	}
	return nil
}
