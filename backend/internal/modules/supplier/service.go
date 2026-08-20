package supplier

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidSupplier  = errors.New("invalid supplier")
	ErrSupplierConflict = errors.New("supplier conflict")
	supplierCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
)

type CreateInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	ContactName string `json:"contactName"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
}

type BindSKUInput struct {
	ProductSKUID    uuid.UUID `json:"productSkuId"`
	SupplierSKUCode string    `json:"supplierSkuCode"`
	UnitCostMinor   int64     `json:"unitCostMinor"`
	Currency        string    `json:"currency"`
	MinOrderQty     int       `json:"minOrderQty"`
	LeadTimeDays    int       `json:"leadTimeDays"`
}

type Service struct{ DB *gorm.DB }

func (s *Service) Create(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateInput) (*Supplier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	name := strings.TrimSpace(in.Name)
	contactName := strings.TrimSpace(in.ContactName)
	phone := strings.TrimSpace(in.Phone)
	email := strings.TrimSpace(in.Email)
	if tenantID <= 0 || !supplierCodePattern.MatchString(code) || name == "" || len([]rune(name)) > 200 || len([]rune(contactName)) > 120 || len([]rune(phone)) > 64 || len([]rune(email)) > 254 {
		return nil, ErrInvalidSupplier
	}
	row := &Supplier{TenantID: tenantID, Code: code, Name: name, Status: StatusActive, ContactName: contactName, Phone: phone, Email: email, CreatedBy: actor}
	var count int64
	if err := s.DB.WithContext(ctx).Model(&Supplier{}).Where("tenant_id = ? AND code = ?", tenantID, code).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSupplierConflict
	}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		return nil, fmt.Errorf("create supplier: %w", err)
	}
	return row, nil
}

func (s *Service) List(ctx context.Context, tenantID int64) ([]Supplier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	var rows []Supplier
	if err := s.DB.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("code ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list suppliers: %w", err)
	}
	return rows, nil
}

func (s *Service) BindSKU(ctx context.Context, tenantID int64, supplierID uuid.UUID, in BindSKUInput) (*SupplierSKU, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if tenantID <= 0 || supplierID == uuid.Nil || in.ProductSKUID == uuid.Nil || in.UnitCostMinor < 0 || in.MinOrderQty < 1 || in.LeadTimeDays < 0 || len(currency) != 3 {
		return nil, ErrInvalidSupplier
	}
	var supplierRow Supplier
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ? AND status = ?", supplierID, tenantID, StatusActive).First(&supplierRow).Error; err != nil {
		return nil, fmt.Errorf("supplier unavailable: %w", err)
	}
	var sku product.ProductSKU
	if err := s.DB.WithContext(ctx).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").Where("product_skus.id = ? AND products.tenant_id = ?", in.ProductSKUID, tenantID).First(&sku).Error; err != nil {
		return nil, fmt.Errorf("tenant SKU unavailable: %w", err)
	}
	row := &SupplierSKU{TenantID: tenantID, SupplierID: supplierID, ProductSKUID: in.ProductSKUID, SupplierSKUCode: strings.TrimSpace(in.SupplierSKUCode), UnitCostMinor: in.UnitCostMinor, Currency: currency, MinOrderQty: in.MinOrderQty, LeadTimeDays: in.LeadTimeDays}
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "supplier_id"}, {Name: "product_sku_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"supplier_sku_code", "unit_cost_minor", "currency", "min_order_qty", "lead_time_days", "updated_at"}),
	}).Create(row).Error
	if err != nil {
		return nil, fmt.Errorf("bind supplier SKU: %w", err)
	}
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND supplier_id = ? AND product_sku_id = ?", tenantID, supplierID, in.ProductSKUID).First(row).Error; err != nil {
		return nil, fmt.Errorf("load supplier SKU: %w", err)
	}
	return row, nil
}

func (s *Service) RequireActive(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID) (*Supplier, error) {
	if tx == nil {
		tx = s.DB
	}
	var row Supplier
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, StatusActive).First(&row).Error; err != nil {
		return nil, fmt.Errorf("supplier unavailable: %w", err)
	}
	return &row, nil
}

func (s *Service) ValidateActive(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID) error {
	_, err := s.RequireActive(ctx, tx, tenantID, id)
	return err
}

// ValidateBinding ensures an optional supplier SKU belongs to the selected
// supplier, tenant and product SKU. A nil binding is valid because supplier
// catalog mapping can be completed after the purchase order is created.
func (s *Service) ValidateBinding(ctx context.Context, tx *gorm.DB, tenantID int64, supplierID, productSKUID uuid.UUID, supplierSKUID *uuid.UUID) error {
	if supplierSKUID == nil || *supplierSKUID == uuid.Nil {
		return nil
	}
	if tx == nil {
		tx = s.DB
	}
	var count int64
	err := tx.WithContext(ctx).Model(&SupplierSKU{}).
		Where("id = ? AND tenant_id = ? AND supplier_id = ? AND product_sku_id = ?", *supplierSKUID, tenantID, supplierID, productSKUID).
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("validate supplier SKU: %w", err)
	}
	if count != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
