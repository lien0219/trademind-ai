package warehouse

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidWarehouse  = errors.New("invalid warehouse")
	ErrWarehouseConflict = errors.New("warehouse conflict")
	ErrWarehouseAbsent   = errors.New("warehouse not found")
	ErrInvalidLocation   = errors.New("invalid warehouse location")
	ErrLocationConflict  = errors.New("warehouse location conflict")
	ErrLocationAbsent    = errors.New("warehouse location not found")
	warehouseCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
	locationCodePattern  = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{0,63}$`)
)

type CreateInput struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type UpdateInput struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	IsDefault bool   `json:"isDefault"`
}

type CreateLocationInput struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Zone string `json:"zone,omitempty"`
}

type UpdateLocationInput struct {
	Name   string `json:"name"`
	Zone   string `json:"zone,omitempty"`
	Status string `json:"status"`
}

type Service struct{ DB *gorm.DB }

func (s *Service) Create(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateInput) (*Warehouse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("warehouse: db unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	name := strings.TrimSpace(in.Name)
	if tenantID < 0 || !warehouseCodePattern.MatchString(code) || name == "" || len([]rune(name)) > 160 {
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

func (s *Service) Update(ctx context.Context, tenantID int64, id uuid.UUID, in UpdateInput) (*Warehouse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("warehouse: db unavailable")
	}
	name := strings.TrimSpace(in.Name)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if tenantID < 0 || id == uuid.Nil || name == "" || len([]rune(name)) > 160 ||
		(status != StatusActive && status != StatusInactive) || (in.IsDefault && status != StatusActive) {
		return nil, ErrInvalidWarehouse
	}

	var row Warehouse
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", id, tenantID).
			First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWarehouseAbsent
		}
		if err != nil {
			return fmt.Errorf("load warehouse: %w", err)
		}
		if in.IsDefault {
			if err := tx.Model(&Warehouse{}).
				Where("tenant_id = ? AND id <> ? AND is_default = ?", tenantID, id, true).
				Update("is_default", false).Error; err != nil {
				return fmt.Errorf("clear default warehouse: %w", err)
			}
		}
		if err := tx.Model(&Warehouse{}).
			Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(map[string]any{"name": name, "status": status, "is_default": in.IsDefault}).Error; err != nil {
			return fmt.Errorf("update warehouse: %w", err)
		}
		return tx.Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
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

func normalizeLocationCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func validateLocationNameZone(name, zone string) bool {
	return name != "" && len([]rune(name)) <= 160 && len([]rune(zone)) <= 64
}

func (s *Service) CreateLocation(ctx context.Context, tenantID int64, warehouseID uuid.UUID, actor *uuid.UUID, in CreateLocationInput) (*WarehouseLocation, error) {
	if s == nil || s.DB == nil || tenantID < 0 || warehouseID == uuid.Nil {
		return nil, ErrInvalidLocation
	}
	code := normalizeLocationCode(in.Code)
	name := strings.TrimSpace(in.Name)
	zone := strings.TrimSpace(in.Zone)
	if !locationCodePattern.MatchString(code) || !validateLocationNameZone(name, zone) {
		return nil, ErrInvalidLocation
	}
	row := &WarehouseLocation{TenantID: tenantID, WarehouseID: warehouseID, Code: code, Name: name, Zone: zone, Status: StatusActive, CreatedBy: actor}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.RequireActive(ctx, tx, tenantID, warehouseID); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return ErrWarehouseAbsent
		}
		var count int64
		if err := tx.Model(&WarehouseLocation{}).Where("tenant_id = ? AND warehouse_id = ? AND code = ?", tenantID, warehouseID, code).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrLocationConflict
		}
		if err := tx.Create(row).Error; err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "duplicate") {
				return ErrLocationConflict
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) ListLocations(ctx context.Context, tenantID int64, warehouseID uuid.UUID, includeInactive bool) ([]WarehouseLocation, error) {
	if s == nil || s.DB == nil || tenantID < 0 || warehouseID == uuid.Nil {
		return nil, ErrInvalidLocation
	}
	if err := s.DB.WithContext(ctx).Model(&Warehouse{}).Where("id = ? AND tenant_id = ?", warehouseID, tenantID).First(&Warehouse{}).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWarehouseAbsent
		}
		return nil, err
	}
	q := s.DB.WithContext(ctx).Where("tenant_id = ? AND warehouse_id = ?", tenantID, warehouseID)
	if !includeInactive {
		q = q.Where("status = ?", StatusActive)
	}
	var rows []WarehouseLocation
	if err := q.Order("zone ASC, code ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) UpdateLocation(ctx context.Context, tenantID int64, warehouseID, id uuid.UUID, in UpdateLocationInput) (*WarehouseLocation, error) {
	if s == nil || s.DB == nil || tenantID < 0 || warehouseID == uuid.Nil || id == uuid.Nil {
		return nil, ErrInvalidLocation
	}
	name := strings.TrimSpace(in.Name)
	zone := strings.TrimSpace(in.Zone)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if !validateLocationNameZone(name, zone) || (status != StatusActive && status != StatusInactive) {
		return nil, ErrInvalidLocation
	}
	var row WarehouseLocation
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ? AND warehouse_id = ?", id, tenantID, warehouseID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLocationAbsent
		}
		if err != nil {
			return err
		}
		return tx.Model(&WarehouseLocation{}).Where("id = ? AND tenant_id = ? AND warehouse_id = ?", id, tenantID, warehouseID).Updates(map[string]any{
			"name": name, "zone": zone, "status": status, "updated_at": time.Now().UTC(),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	row.Name, row.Zone, row.Status = name, zone, status
	return &row, nil
}
