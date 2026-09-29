package warehouse

import (
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// Warehouse is a tenant-owned physical or logical stock location.
type Warehouse struct {
	model.Base
	TenantID  int64      `gorm:"not null;uniqueIndex:ux_warehouse_tenant_code;index" json:"tenantId"`
	Code      string     `gorm:"size:64;not null;uniqueIndex:ux_warehouse_tenant_code" json:"code"`
	Name      string     `gorm:"size:160;not null" json:"name"`
	Status    string     `gorm:"size:24;not null;default:active;index" json:"status"`
	IsDefault bool       `gorm:"not null;default:false;index" json:"isDefault"`
	CreatedBy *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (Warehouse) TableName() string { return "warehouses" }

// WarehouseLocation is a tenant-owned physical slot inside a warehouse.
// Codes are stable identifiers used by operators and barcode workflows.
type WarehouseLocation struct {
	model.Base
	TenantID    int64      `gorm:"not null;uniqueIndex:ux_warehouse_location_code,priority:1;index" json:"tenantId"`
	WarehouseID uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:ux_warehouse_location_code,priority:2;index" json:"warehouseId"`
	Code        string     `gorm:"size:64;not null;uniqueIndex:ux_warehouse_location_code,priority:3" json:"code"`
	Name        string     `gorm:"size:160;not null" json:"name"`
	Zone        string     `gorm:"size:64;index" json:"zone,omitempty"`
	Status      string     `gorm:"size:24;not null;default:active;index" json:"status"`
	CreatedBy   *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (WarehouseLocation) TableName() string { return "warehouse_locations" }
