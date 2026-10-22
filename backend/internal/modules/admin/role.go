package admin

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

// RoleDefinition defines a tenant-owned, non-system role. Its code is stable
// and deliberately short enough for admin_users.role (varchar 32).
type RoleDefinition struct {
	ID uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID int64 `gorm:"not null;uniqueIndex:ux_admin_role_tenant_code,priority:1;index" json:"tenantId"`
	Code string `gorm:"size:32;not null;uniqueIndex:ux_admin_role_tenant_code,priority:2" json:"code"`
	Name string `gorm:"size:80;not null" json:"name"`
	Description string `gorm:"size:400" json:"description"`
	PermissionsJSON string `gorm:"type:text;not null" json:"-"`
	Enabled bool `gorm:"not null;default:true" json:"enabled"`
	CreatedBy *uuid.UUID `gorm:"type:char(36)" json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (RoleDefinition) TableName() string { return "admin_role_definitions" }

func (r *RoleDefinition) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&r.ID)
	return nil
}
