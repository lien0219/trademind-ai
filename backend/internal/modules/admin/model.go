package admin

import (
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
)

// AdminUser is a console administrator account.
// Username is an internal stable id (UUID hex, no dashes), not used for login.
// Login uses Email and/or Phone (see ParseLoginAccount / ByLoginAccount).
type AdminUser struct {
	model.Base
	Username     string `gorm:"size:40;not null;uniqueIndex" json:"username"`
	Email        string `gorm:"size:128" json:"email"`
	Phone        string `gorm:"size:32" json:"phone"`
	PasswordHash string `gorm:"size:255;not null" json:"-"`
	DisplayName  string `gorm:"size:128" json:"displayName"`
	Role         string `gorm:"size:32;default:'admin'" json:"role"`
	Status       string `gorm:"size:32;default:'active'" json:"status"`
}

// TableName keeps a stable table name for migrations.
func (AdminUser) TableName() string {
	return "admin_users"
}

// NewInternalUsername returns a DB-unique opaque id (never used as a login credential).
func NewInternalUsername() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

// LoginLabel returns a human-facing session label for JWT / logs (email preferred, then phone).
func (u *AdminUser) LoginLabel() string {
	if u == nil {
		return ""
	}
	if e := strings.TrimSpace(u.Email); e != "" {
		return e
	}
	return strings.TrimSpace(u.Phone)
}
