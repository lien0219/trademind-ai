package admin

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/gorm"
)

const (
	StorePermScopeView    = "view"
	StorePermScopeOperate = "operate"
	StorePermScopeManage  = "manage"
)

// UserStorePermission binds an admin user to a shop with a scope.
type UserStorePermission struct {
	ID              uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	UserID          uuid.UUID  `gorm:"type:char(36);index;not null" json:"userId"`
	StoreID         uuid.UUID  `gorm:"type:char(36);index;not null" json:"storeId"`
	Platform        string     `gorm:"size:32;index" json:"platform,omitempty"`
	PermissionScope string     `gorm:"size:32;not null;default:'operate'" json:"permissionScope"`
	CreatedBy       *uuid.UUID `gorm:"type:char(36)" json:"createdBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// TableName keeps a stable table name for migrations.
func (UserStorePermission) TableName() string {
	return "user_store_permissions"
}

// NormalizeStorePermScope returns a valid scope or operate.
func NormalizeStorePermScope(scope string) string {
	switch strings.TrimSpace(strings.ToLower(scope)) {
	case StorePermScopeView, StorePermScopeOperate, StorePermScopeManage:
		return strings.TrimSpace(strings.ToLower(scope))
	default:
		return StorePermScopeOperate
	}
}

// ScopeAllowsOperate returns true when scope permits write operations on the store.
func ScopeAllowsOperate(scope string) bool {
	s := NormalizeStorePermScope(scope)
	return s == StorePermScopeOperate || s == StorePermScopeManage
}

// BeforeCreate assigns a UUID when id is zero.
func (u *UserStorePermission) BeforeCreate(tx *gorm.DB) error {
	id.Ensure(&u.ID)
	return nil
}
