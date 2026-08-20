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
