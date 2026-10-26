package tenantconfig

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type UserPreference struct {
	TenantID  int64          `gorm:"primaryKey;not null" json:"-"`
	UserID    uuid.UUID      `gorm:"primaryKey;type:char(36);not null" json:"-"`
	Values    datatypes.JSON `gorm:"type:jsonb;not null" json:"values"`
	Version   int64          `gorm:"not null" json:"version"`
	UpdatedAt time.Time      `json:"updatedAt"`
	CreatedAt time.Time      `json:"createdAt"`
}

func (UserPreference) TableName() string { return "tenant_user_preferences" }
