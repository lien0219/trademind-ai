package supplier

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidSupplier  = errors.New("invalid supplier")
	ErrSupplierConflict = errors.New("supplier conflict")
	ErrSupplierAbsent   = errors.New("supplier not found")
	supplierCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
)

type CreateInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	ContactName string `json:"contactName"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
}

type UpdateInput struct {
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	ContactName string  `json:"contactName"`
	Phone       *string `json:"phone"`
	Email       *string `json:"email"`
}

type BindSKUInput struct {
	ProductSKUID    uuid.UUID `json:"productSkuId"`
	SupplierSKUCode string    `json:"supplierSkuCode"`
	UnitCostMinor   int64     `json:"unitCostMinor"`
	Currency        string    `json:"currency"`
	MinOrderQty     int       `json:"minOrderQty"`
	LeadTimeDays    int       `json:"leadTimeDays"`
}

type SupplierSKUListItem struct {
	ID              uuid.UUID `json:"id"`
	SupplierID      uuid.UUID `json:"supplierId"`
	ProductSKUID    uuid.UUID `json:"productSkuId"`
	ProductTitle    string    `json:"productTitle"`
	SKUCode         string    `json:"skuCode"`
	SKUName         string    `json:"skuName"`
	SupplierSKUCode string    `json:"supplierSkuCode,omitempty"`
	UnitCostMinor   int64     `json:"unitCostMinor"`
	Currency        string    `json:"currency"`
	MinOrderQty     int       `json:"minOrderQty"`
	LeadTimeDays    int       `json:"leadTimeDays"`
}

// ActiveCostFact is the supplier-owned read contract used by downstream
// transactions that need to freeze the current catalog state.
type ActiveCostFact struct {
	SupplierSKUID   uuid.UUID `gorm:"column:supplier_sku_id"`
	SupplierID      uuid.UUID `gorm:"column:supplier_id"`
	ProductSKUID    uuid.UUID `gorm:"column:product_sku_id"`
	SupplierCode    string    `gorm:"column:supplier_code"`
	SupplierName    string    `gorm:"column:supplier_name"`
	SupplierSKUCode string    `gorm:"column:supplier_sku_code"`
	UnitCostMinor   int64     `gorm:"column:unit_cost_minor"`
	Currency        string    `gorm:"column:currency"`
	SourceUpdatedAt time.Time `gorm:"column:source_updated_at"`
}

type Service struct{ DB *gorm.DB }

// ListActiveCosts returns all active supplier catalog entries for the supplied
// tenant-local SKUs. When tx is provided, the read participates in the caller's
// transaction so a downstream immutable snapshot can be committed atomically.
func (s *Service) ListActiveCosts(ctx context.Context, tx *gorm.DB, tenantID int64, skuIDs []uuid.UUID) ([]ActiveCostFact, error) {
	rows := make([]ActiveCostFact, 0)
	if s == nil || (s.DB == nil && tx == nil) {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	if tenantID < 0 {
		return nil, ErrInvalidSupplier
	}
	if len(skuIDs) == 0 {
		return rows, nil
	}
	if tx == nil {
		tx = s.DB
	}
	err := tx.WithContext(ctx).Table("supplier_skus AS supplier_sku").
		Select(`supplier_sku.id AS supplier_sku_id, supplier_sku.supplier_id, supplier_sku.product_sku_id,
			supplier.code AS supplier_code, supplier.name AS supplier_name,
			supplier_sku.supplier_sku_code, supplier_sku.unit_cost_minor, supplier_sku.currency,
			supplier_sku.updated_at AS source_updated_at`).
		Joins("JOIN suppliers AS supplier ON supplier.id = supplier_sku.supplier_id AND supplier.tenant_id = ? AND supplier.status = ? AND supplier.deleted_at IS NULL", tenantID, StatusActive).
		Where("supplier_sku.tenant_id = ? AND supplier_sku.deleted_at IS NULL AND supplier_sku.product_sku_id IN ?", tenantID, skuIDs).
		Order("supplier_sku.product_sku_id ASC, supplier.code ASC, supplier_sku.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list active supplier costs: %w", err)
	}
	return rows, nil
}

func (s *Service) Create(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateInput) (*Supplier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	name := strings.TrimSpace(in.Name)
	contactName := strings.TrimSpace(in.ContactName)
	phone := strings.TrimSpace(in.Phone)
	email := strings.TrimSpace(in.Email)
	if tenantID < 0 || !supplierCodePattern.MatchString(code) || name == "" || len([]rune(name)) > 200 || len([]rune(contactName)) > 120 || len([]rune(phone)) > 64 || len([]rune(email)) > 254 {
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

func (s *Service) Update(ctx context.Context, tenantID int64, id uuid.UUID, in UpdateInput) (*Supplier, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	name := strings.TrimSpace(in.Name)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	contactName := strings.TrimSpace(in.ContactName)
	if tenantID < 0 || id == uuid.Nil || name == "" || len([]rune(name)) > 200 ||
		(status != StatusActive && status != StatusInactive) || len([]rune(contactName)) > 120 {
		return nil, ErrInvalidSupplier
	}
	updates := map[string]any{"name": name, "status": status, "contact_name": contactName}
	if in.Phone != nil {
		phone := strings.TrimSpace(*in.Phone)
		if len([]rune(phone)) > 64 {
			return nil, ErrInvalidSupplier
		}
		updates["phone"] = phone
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		if len([]rune(email)) > 254 {
			return nil, ErrInvalidSupplier
		}
		updates["email"] = email
	}
	result := s.DB.WithContext(ctx).Model(&Supplier{}).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update supplier: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrSupplierAbsent
	}
	var row Supplier
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error; err != nil {
		return nil, fmt.Errorf("load supplier: %w", err)
	}
	return &row, nil
}

func (s *Service) ListSKUs(ctx context.Context, tenantID int64, supplierID uuid.UUID) ([]SupplierSKUListItem, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	if tenantID < 0 || supplierID == uuid.Nil {
		return nil, ErrInvalidSupplier
	}
	var supplierCount int64
	if err := s.DB.WithContext(ctx).Model(&Supplier{}).
		Where("id = ? AND tenant_id = ?", supplierID, tenantID).
		Count(&supplierCount).Error; err != nil {
		return nil, fmt.Errorf("load supplier: %w", err)
	}
	if supplierCount == 0 {
		return nil, ErrSupplierAbsent
	}
	var rows []SupplierSKUListItem
	err := s.DB.WithContext(ctx).Table("supplier_skus AS ss").
		Select("ss.id, ss.supplier_id, ss.product_sku_id, products.title AS product_title, product_skus.sku_code, product_skus.sku_name, ss.supplier_sku_code, ss.unit_cost_minor, ss.currency, ss.min_order_qty, ss.lead_time_days").
		Joins("JOIN product_skus ON product_skus.id = ss.product_sku_id").
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("ss.tenant_id = ? AND ss.supplier_id = ? AND products.tenant_id = ?", tenantID, supplierID, tenantID).
		Order("products.title ASC, product_skus.sku_code ASC, ss.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list supplier SKUs: %w", err)
	}
	return rows, nil
}

func (s *Service) BindSKU(ctx context.Context, tenantID int64, supplierID uuid.UUID, in BindSKUInput) (*SupplierSKU, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("supplier: db unavailable")
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if tenantID < 0 || supplierID == uuid.Nil || in.ProductSKUID == uuid.Nil || in.UnitCostMinor < 0 || in.MinOrderQty < 1 || in.LeadTimeDays < 0 || len(currency) != 3 || len([]rune(strings.TrimSpace(in.SupplierSKUCode))) > 128 {
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
