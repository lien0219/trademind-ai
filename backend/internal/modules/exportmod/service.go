package exportmod

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tenantquery"
	"gorm.io/gorm"
)

// Service manages tenant-scoped export jobs.
type Service struct {
	DB *gorm.DB
}

// CreateJobInput is the export request payload.
type CreateJobInput struct {
	ExportType string
	ShopID     *uuid.UUID
	MaskedPII  bool
	Filters    map[string]any
}

// CreateJob persists a tenant-scoped export job in pending state.
func (s *Service) CreateJob(c *gin.Context, in CreateJobInput) (*ExportJob, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("export: unavailable")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if in.ShopID != nil && *in.ShopID != uuid.Nil {
		if !adminperm.RequireStoreView(c, s.DB, *in.ShopID) {
			return nil, fmt.Errorf("shop scope denied")
		}
	}
	var adminID *uuid.UUID
	if pr, err := adminperm.LoadPrincipal(c, s.DB); err == nil && pr != nil {
		adminID = &pr.UserID
	}
	exp := time.Now().UTC().Add(72 * time.Hour)
	row := &ExportJob{
		TenantID:   tid,
		ExportType: in.ExportType,
		Status:     ExportStatusPending,
		ShopID:     in.ShopID,
		MaskedPII:  in.MaskedPII,
		CreatedBy:  adminID,
		ExpiresAt:  &exp,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// GetJob loads one export job with tenant scope.
func (s *Service) GetJob(c *gin.Context, id uuid.UUID) (*ExportJob, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("export: unavailable")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var row ExportJob
	if err := repository.FindByID(c.Request.Context(), s.DB, &row, tid, id); err != nil {
		return nil, err
	}
	if row.ShopID != nil && *row.ShopID != uuid.Nil {
		if !adminperm.RequireStoreView(c, s.DB, *row.ShopID) {
			return nil, fmt.Errorf("shop scope denied")
		}
	}
	return &row, nil
}

// ListJobs returns paginated tenant-scoped export jobs.
func (s *Service) ListJobs(c *gin.Context, page, pageSize int) ([]ExportJob, int64, error) {
	if s == nil || s.DB == nil {
		return nil, 0, fmt.Errorf("export: unavailable")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	tx, tid, err := repository.ApplyTenantScope(c, nil, s.DB.WithContext(c.Request.Context()).Model(&ExportJob{}))
	if err != nil {
		return nil, 0, err
	}
	if scoped, err := adminperm.ApplyStoreScope(c, s.DB, tx, "shop_id"); err != nil {
		return nil, 0, err
	} else {
		tx = scoped
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ExportJob
	if err := tx.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	_ = tid
	return rows, total, nil
}

// FindByIDTenant is for workers with explicit tenant context.
func (s *Service) FindByIDTenant(ctx context.Context, tenantID int64, id uuid.UUID) (*ExportJob, error) {
	var row ExportJob
	if err := tenantquery.FindByIDTenant(ctx, s.DB, &row, tenantID, id); err != nil {
		return nil, err
	}
	return &row, nil
}
