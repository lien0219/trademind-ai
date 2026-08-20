package warehouse

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidWarehouse  = errors.New("invalid warehouse")
	ErrWarehouseConflict = errors.New("warehouse conflict")
	warehouseCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
)

type CreateInput struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type Service struct{ DB *gorm.DB }

func (s *Service) Create(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateInput) (*Warehouse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("warehouse: db unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	name := strings.TrimSpace(in.Name)
	if tenantID <= 0 || !warehouseCodePattern.MatchString(code) || name == "" || len([]rune(name)) > 160 {
		return nil, ErrInvalidWarehouse
	}
	row := &Warehouse{TenantID: tenantID, Code: code, Name: name, Status: StatusActive, IsDefault: in.IsDefault, CreatedBy: actor}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&Warehouse{}).Where("tenant_id = ? AND code = ?", tenantID, code).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrWarehouseConflict
		}
		if in.IsDefault {
			if err := tx.Model(&Warehouse{}).Where("tenant_id = ? AND is_default = ?", tenantID, true).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("create warehouse: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) List(ctx context.Context, tenantID int64) ([]Warehouse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("warehouse: db unavailable")
	}
	var rows []Warehouse
	if err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("is_default DESC, code ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list warehouses: %w", err)
	}
	return rows, nil
}

func (s *Service) RequireActive(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID) (*Warehouse, error) {
	if tx == nil {
		tx = s.DB
	}
	var row Warehouse
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, StatusActive).First(&row).Error
	if err != nil {
		return nil, fmt.Errorf("warehouse unavailable: %w", err)
	}
	return &row, nil
}

func (s *Service) ValidateActive(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID) error {
	_, err := s.RequireActive(ctx, tx, tenantID, id)
	return err
}
