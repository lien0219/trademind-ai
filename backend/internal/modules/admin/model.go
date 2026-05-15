package admin

import (
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// AdminUser is a console administrator account.
type AdminUser struct {
	model.Base
	Username     string `gorm:"size:64;not null;uniqueIndex" json:"username"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	DisplayName  string `gorm:"size:128" json:"displayName"`
}

// TableName keeps a stable table name for migrations.
func (AdminUser) TableName() string {
	return "admin_users"
}
