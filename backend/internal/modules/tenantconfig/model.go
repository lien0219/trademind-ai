package tenantconfig

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Setting has one versioned JSON document per tenant/module. No global
// credentials or platform configuration are stored in this table.
type Setting struct {
	TenantID  int64          `gorm:"primaryKey;not null" json:"tenantId"`
	Module    string         `gorm:"primaryKey;size:32;not null" json:"module"`
	Values    datatypes.JSON `gorm:"type:jsonb;not null" json:"values"`
	Version   int64          `gorm:"not null" json:"version"`
	UpdatedBy uuid.UUID      `gorm:"type:char(36);not null" json:"updatedBy"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

func (Setting) TableName() string { return "tenant_business_settings" }

// Revision is an immutable snapshot, allowing an audited restore that creates
// a new version rather than mutating old evidence.
type Revision struct {
	ID        uuid.UUID      `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID  int64          `gorm:"not null;index:idx_tenant_settings_revision,priority:1;uniqueIndex:ux_tenant_setting_revision,priority:1" json:"-"`
	Module    string         `gorm:"size:32;not null;index:idx_tenant_settings_revision,priority:2;uniqueIndex:ux_tenant_setting_revision,priority:2" json:"module"`
	Version   int64          `gorm:"not null;uniqueIndex:ux_tenant_setting_revision,priority:3" json:"version"`
	Values    datatypes.JSON `gorm:"type:jsonb;not null" json:"values"`
	ActorID   uuid.UUID      `gorm:"type:char(36);not null" json:"actorId"`
	Action    string         `gorm:"size:24;not null" json:"action"`
	CreatedAt time.Time      `json:"createdAt"`
}
func (Revision) TableName() string { return "tenant_business_setting_revisions" }
