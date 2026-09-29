package inventory

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PlacementStatusActive   = "active"
	PlacementStatusInactive = "inactive"
)

var (
	ErrInvalidPlacement  = errors.New("invalid warehouse sku placement")
	ErrPlacementConflict = errors.New("warehouse sku placement conflict")
	ErrPlacementAbsent   = errors.New("warehouse sku placement not found")
	barcodePattern       = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._/-]{0,127}$`)
)

func isPlacementUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") ||
		strings.Contains(message, "duplicate") ||
		strings.Contains(message, "constraint failed")
}

type WarehouseSKUPlacementInput struct {
	WarehouseID  uuid.UUID  `json:"warehouseId"`
	ProductSKUID uuid.UUID  `json:"productSkuId"`
	LocationID   *uuid.UUID `json:"locationId,omitempty"`
	Barcode      string     `json:"barcode,omitempty"`
	Status       string     `json:"status"`
}

type WarehouseSKUPlacementUpdateInput struct {
	LocationID *uuid.UUID `json:"locationId,omitempty"`
	Barcode    string     `json:"barcode,omitempty"`
	Status     string     `json:"status"`
}

type WarehouseSKUPlacementView struct {
	WarehouseSKUPlacement
	SKUCode      string `json:"skuCode,omitempty"`
	SKUName      string `json:"skuName,omitempty"`
	ProductTitle string `json:"productTitle,omitempty"`
	LocationCode string `json:"locationCode,omitempty"`
	LocationName string `json:"locationName,omitempty"`
	LocationZone string `json:"locationZone,omitempty"`
}

// WarehouseSKUPlacementSnapshot is the read-only data copied into a wave line.
type WarehouseSKUPlacementSnapshot struct {
	ProductSKUID uuid.UUID
	Barcode      string
	LocationID   *uuid.UUID
	LocationCode string
	LocationName string
}

func normalizePlacementBarcode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func validatePlacementInput(in WarehouseSKUPlacementInput) (string, string, error) {
	barcode := normalizePlacementBarcode(in.Barcode)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if in.WarehouseID == uuid.Nil || in.ProductSKUID == uuid.Nil ||
		(status != PlacementStatusActive && status != PlacementStatusInactive) {
		return "", "", ErrInvalidPlacement
	}
	if barcode != "" && !barcodePattern.MatchString(barcode) {
		return "", "", ErrInvalidPlacement
	}
	if in.LocationID != nil && *in.LocationID == uuid.Nil {
		return "", "", ErrInvalidPlacement
	}
	return barcode, status, nil
}

func validatePlacementUpdate(in WarehouseSKUPlacementUpdateInput) (string, string, error) {
	return validatePlacementInput(WarehouseSKUPlacementInput{WarehouseID: uuid.New(), ProductSKUID: uuid.New(), LocationID: in.LocationID, Barcode: in.Barcode, Status: in.Status})
}

func (s *Service) validatePlacementRefs(ctx context.Context, tx *gorm.DB, tenantID int64, warehouseID uuid.UUID, skuID uuid.UUID, locationID *uuid.UUID, requireActiveWarehouse bool) error {
	if requireActiveWarehouse {
		if s.Warehouses == nil {
			return ErrInvalidPlacement
		}
		if _, err := s.Warehouses.RequireActive(ctx, tx, tenantID, warehouseID); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return ErrPlacementAbsent
		}
	} else {
		var count int64
		if err := tx.Model(&warehouse.Warehouse{}).Where("id = ? AND tenant_id = ?", warehouseID, tenantID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrPlacementAbsent
		}
	}
	var skuCount int64
	if err := tx.Table("product_skus ps").Joins("JOIN products p ON p.id = ps.product_id").Where("ps.id = ? AND p.tenant_id = ? AND p.deleted_at IS NULL", skuID, tenantID).Count(&skuCount).Error; err != nil {
		return err
	}
	if skuCount != 1 {
		return ErrPlacementAbsent
	}
	if locationID != nil {
		var count int64
		if err := tx.Model(&warehouse.WarehouseLocation{}).Where("id = ? AND tenant_id = ? AND warehouse_id = ? AND status = ?", *locationID, tenantID, warehouseID, warehouse.StatusActive).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrInvalidPlacement
		}
	}
	return nil
}

func (s *Service) ensurePlacementBarcodeAvailable(tx *gorm.DB, tenantID int64, warehouseID uuid.UUID, barcode string, excludeID uuid.UUID) error {
	if barcode == "" {
		return nil
	}
	var count int64
	q := tx.Model(&WarehouseSKUPlacement{}).Where("tenant_id = ? AND warehouse_id = ? AND barcode = ? AND status = ?", tenantID, warehouseID, barcode, PlacementStatusActive)
	if excludeID != uuid.Nil {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrPlacementConflict
	}
	return nil
}

func (s *Service) CreateWarehouseSKUPlacement(ctx context.Context, tenantID int64, actor *uuid.UUID, in WarehouseSKUPlacementInput) (*WarehouseSKUPlacement, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrInvalidPlacement
	}
	barcode, status, err := validatePlacementInput(in)
	if err != nil {
		return nil, err
	}
	row := &WarehouseSKUPlacement{TenantID: tenantID, WarehouseID: in.WarehouseID, ProductSKUID: in.ProductSKUID, LocationID: in.LocationID, Barcode: barcode, Status: status, CreatedBy: actor, UpdatedBy: actor}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.validatePlacementRefs(ctx, tx, tenantID, in.WarehouseID, in.ProductSKUID, in.LocationID, true); err != nil {
			return err
		}
		if err := s.ensurePlacementBarcodeAvailable(tx, tenantID, in.WarehouseID, barcode, uuid.Nil); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&WarehouseSKUPlacement{}).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, in.WarehouseID, in.ProductSKUID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrPlacementConflict
		}
		if err := tx.Create(row).Error; err != nil {
			if isPlacementUniqueViolation(err) {
				return ErrPlacementConflict
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

func (s *Service) UpdateWarehouseSKUPlacement(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in WarehouseSKUPlacementUpdateInput) (*WarehouseSKUPlacement, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil {
		return nil, ErrInvalidPlacement
	}
	barcode, status, err := validatePlacementUpdate(in)
	if err != nil {
		return nil, err
	}
	var row WarehouseSKUPlacement
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPlacementAbsent
			}
			return err
		}
		if err := s.validatePlacementRefs(ctx, tx, tenantID, row.WarehouseID, row.ProductSKUID, in.LocationID, false); err != nil {
			return err
		}
		if err := s.ensurePlacementBarcodeAvailable(tx, tenantID, row.WarehouseID, barcode, id); err != nil {
			return err
		}
		err := tx.Model(&WarehouseSKUPlacement{}).Where("id = ? AND tenant_id = ?", id, tenantID).Updates(map[string]any{"location_id": in.LocationID, "barcode": barcode, "status": status, "updated_by": actor}).Error
		if isPlacementUniqueViolation(err) {
			return ErrPlacementConflict
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	row.LocationID, row.Barcode, row.Status, row.UpdatedBy = in.LocationID, barcode, status, actor
	return &row, nil
}

func (s *Service) ListWarehouseSKUPlacements(ctx context.Context, tenantID int64, warehouseID uuid.UUID, skuID *uuid.UUID, includeInactive bool) ([]WarehouseSKUPlacementView, error) {
	if s == nil || s.DB == nil || tenantID < 0 || warehouseID == uuid.Nil {
		return nil, ErrInvalidPlacement
	}
	var warehouseCount int64
	if err := s.DB.WithContext(ctx).Model(&warehouse.Warehouse{}).Where("id = ? AND tenant_id = ?", warehouseID, tenantID).Count(&warehouseCount).Error; err != nil {
		return nil, err
	}
	if warehouseCount != 1 {
		return nil, ErrPlacementAbsent
	}
	q := s.DB.WithContext(ctx).Table("warehouse_sku_placements wsp").
		Select("wsp.*, ps.sku_code, ps.sku_name, p.title AS product_title, wl.code AS location_code, wl.name AS location_name, wl.zone AS location_zone").
		Joins("JOIN product_skus ps ON ps.id = wsp.product_sku_id").
		Joins("JOIN products p ON p.id = ps.product_id").
		Joins("LEFT JOIN warehouse_locations wl ON wl.id = wsp.location_id AND wl.tenant_id = wsp.tenant_id").
		Where("wsp.tenant_id = ? AND wsp.warehouse_id = ? AND p.deleted_at IS NULL", tenantID, warehouseID)
	if skuID != nil && *skuID != uuid.Nil {
		q = q.Where("wsp.product_sku_id = ?", *skuID)
	}
	if !includeInactive {
		q = q.Where("wsp.status = ?", PlacementStatusActive)
	}
	var rows []WarehouseSKUPlacementView
	if err := q.Order("ps.sku_code ASC, wsp.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// LoadWarehouseSKUPlacementSnapshots reads active bindings inside the caller's
// transaction so wave creation can freeze the exact operator-facing data.
func LoadWarehouseSKUPlacementSnapshots(ctx context.Context, tx *gorm.DB, tenantID int64, warehouseID uuid.UUID, skuIDs []uuid.UUID) (map[uuid.UUID]WarehouseSKUPlacementSnapshot, error) {
	out := make(map[uuid.UUID]WarehouseSKUPlacementSnapshot)
	if tx == nil || tenantID < 0 || warehouseID == uuid.Nil || len(skuIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ProductSKUID uuid.UUID  `gorm:"column:product_sku_id"`
		Barcode      string     `gorm:"column:barcode"`
		LocationID   *uuid.UUID `gorm:"column:location_id"`
		LocationCode string     `gorm:"column:location_code"`
		LocationName string     `gorm:"column:location_name"`
	}
	err := tx.WithContext(ctx).Table("warehouse_sku_placements wsp").
		Select("wsp.product_sku_id, wsp.barcode, wsp.location_id, wl.code AS location_code, wl.name AS location_name").
		Joins("LEFT JOIN warehouse_locations wl ON wl.id = wsp.location_id AND wl.tenant_id = wsp.tenant_id").
		Where("wsp.tenant_id = ? AND wsp.warehouse_id = ? AND wsp.product_sku_id IN ? AND wsp.status = ?", tenantID, warehouseID, skuIDs, PlacementStatusActive).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProductSKUID] = WarehouseSKUPlacementSnapshot{ProductSKUID: row.ProductSKUID, Barcode: row.Barcode, LocationID: row.LocationID, LocationCode: row.LocationCode, LocationName: row.LocationName}
	}
	return out, nil
}

// LoadWarehouseSKUPlacementSnapshots keeps the service-shaped API available to
// callers while the transaction-only read remains an explicit module function.
func (s *Service) LoadWarehouseSKUPlacementSnapshots(ctx context.Context, tx *gorm.DB, tenantID int64, warehouseID uuid.UUID, skuIDs []uuid.UUID) (map[uuid.UUID]WarehouseSKUPlacementSnapshot, error) {
	return LoadWarehouseSKUPlacementSnapshots(ctx, tx, tenantID, warehouseID, skuIDs)
}
