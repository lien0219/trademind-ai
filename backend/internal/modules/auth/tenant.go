package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// resolveLoginTenant restores a previously selected active tenant, otherwise
// chooses the lowest active membership. A migrated identity without a valid
// membership receives a tenant-independent recovery context (tenant 0).
func resolveLoginTenant(ctx context.Context, db *gorm.DB, user *admin.AdminUser) (int64, error) {
	if db == nil || user == nil {
		return 0, errors.New("auth: tenant resolution unavailable")
	}
	registry, memberships := adminperm.TenantSchemaState(db)
	root, err := adminperm.IsRoot(ctx, db, user.ID)
	if err != nil {
		return 0, err
	}
	if root && registry && memberships {
		var row admin.Tenant
		if user.LastTenantID > 0 {
			err := db.WithContext(ctx).Where("id = ? AND status = ?", user.LastTenantID, admin.TenantActive).First(&row).Error
			if err == nil {
				return row.ID, nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return 0, err
			}
		}
		// Deleted customer enterprises cannot strand Root in tenant zero.
		err := db.WithContext(ctx).Where("code = ? AND status = ?", admin.RootWorkspaceCode, admin.TenantActive).First(&row).Error
		if err == nil {
			return row.ID, nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
		err = db.WithContext(ctx).Where("status = ?", admin.TenantActive).Order("id ASC").First(&row).Error
		if err == nil {
			return row.ID, nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if !registry && !memberships {
		return user.TenantID, nil
	}
	if !registry || !memberships {
		return 0, nil
	}
	type membershipTenant struct {
		TenantID int64
	}
	query := func() *gorm.DB {
		return db.WithContext(ctx).Table("tenant_memberships AS m").
			Select("m.tenant_id").
			Joins("JOIN tenants AS t ON t.id = m.tenant_id").
			Where("m.user_id = ? AND m.status = ? AND t.status = ?", user.ID, admin.MembershipActive, admin.TenantActive)
	}
	if user.LastTenantID > 0 {
		var preferred membershipTenant
		err := query().Where("m.tenant_id = ?", user.LastTenantID).Take(&preferred).Error
		if err == nil {
			return preferred.TenantID, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}
	var fallback membershipTenant
	err = query().Order("m.tenant_id ASC").Take(&fallback).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return fallback.TenantID, nil
}

func validateTenantSession(ctx context.Context, db *gorm.DB, userID uuid.UUID, tenantID int64) error {
	if tenantID <= 0 {
		return nil
	}
	access, err := adminperm.ResolveTenantAccess(ctx, db, userID, tenantID)
	if err != nil {
		return err
	}
	if access.Schema && !access.Allowed {
		return errors.New(ErrSessionRevoked)
	}
	return nil
}

// ValidateTenantAccess is used by authentication middleware for legacy bearer
// tokens that do not carry a persisted session binding.
func ValidateTenantAccess(ctx context.Context, db *gorm.DB, userID uuid.UUID, tenantID int64) error {
	return validateTenantSession(ctx, db, userID, tenantID)
}
