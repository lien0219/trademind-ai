package adminperm

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"gorm.io/gorm"
)

// IsRoot checks the unique persisted bootstrap binding and live user status.
// Email, role strings, JWT claims and PlatformOperator grants cannot replace it.
func IsRoot(ctx context.Context, db *gorm.DB, id uuid.UUID) (bool, error) {
	if db == nil || id == uuid.Nil {
		return false, nil
	}
	if !db.Migrator().HasTable(&admin.RootAccount{}) {
		return false, nil
	}
	var root admin.RootAccount
	if err := db.WithContext(ctx).Where("id = ? AND user_id = ?", 1, id).First(&root).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	var u admin.AdminUser
	if err := db.WithContext(ctx).Select("id", "tenant_id", "role", "status").First(&u, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return u.TenantID == 0 && strings.EqualFold(strings.TrimSpace(u.Role), RoleAdmin) &&
		strings.EqualFold(strings.TrimSpace(u.Status), admin.StatusActive), nil
}

func rootActiveTenant(ctx context.Context, db *gorm.DB, id int64) (bool, error) {
	if id <= 0 || db == nil {
		return false, nil
	}
	var t admin.Tenant
	err := db.WithContext(ctx).Select("status").First(&t, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return t.Status == admin.TenantActive, nil
}
