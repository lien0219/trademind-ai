package supplier

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

type Supplier struct {
	model.Base
	TenantID    int64      `gorm:"not null;uniqueIndex:ux_supplier_tenant_code;index" json:"tenantId"`
	Code        string     `gorm:"size:64;not null;uniqueIndex:ux_supplier_tenant_code" json:"code"`
	Name        string     `gorm:"size:200;not null;index" json:"name"`
	Status      string     `gorm:"size:24;not null;default:active;index" json:"status"`
	ContactName string     `gorm:"size:120" json:"contactName,omitempty"`
	Phone       string     `gorm:"size:64" json:"phone,omitempty"`
	Email       string     `gorm:"size:254" json:"email,omitempty"`
	CreatedBy   *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Supplier) TableName() string { return "suppliers" }

type SupplierSKU struct {
	model.Base
	TenantID        int64     `gorm:"not null;uniqueIndex:ux_supplier_sku_binding;index" json:"tenantId"`
	SupplierID      uuid.UUID `gorm:"type:char(36);not null;uniqueIndex:ux_supplier_sku_binding;index" json:"supplierId"`
	ProductSKUID    uuid.UUID `gorm:"column:product_sku_id;type:char(36);not null;uniqueIndex:ux_supplier_sku_binding;index" json:"productSkuId"`
	SupplierSKUCode string    `gorm:"size:128;index" json:"supplierSkuCode,omitempty"`
	UnitCostMinor   int64     `gorm:"not null;default:0" json:"unitCostMinor"`
	Currency        string    `gorm:"size:8;not null;default:CNY" json:"currency"`
	MinOrderQty     int       `gorm:"not null;default:1" json:"minOrderQty"`
	LeadTimeDays    int       `gorm:"not null;default:0" json:"leadTimeDays"`
}

func (SupplierSKU) TableName() string { return "supplier_skus" }
