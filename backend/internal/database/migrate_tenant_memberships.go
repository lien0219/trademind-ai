package database

import (
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// backfillTenantMemberships turns the pre-membership AdminUser.tenant_id
// evidence into explicit memberships. Tenant zero is intentionally untouched:
// it is a system/control-plane scope, not an inferred business tenant.
func backfillTenantMemberships(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&admin.AdminUser{}) || !db.Migrator().HasTable(&admin.TenantMembership{}) {
		return nil
	}
	var users []admin.AdminUser
	if err := db.Select("id", "tenant_id", "role", "status").Where("tenant_id > 0").Order("tenant_id ASC, created_at ASC").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		var tenant admin.Tenant
		err := db.Where("id = ?", user.TenantID).First(&tenant).Error
		if errorsIsNotFound(err) {
			code := fmt.Sprintf("legacy-tenant-%d", user.TenantID)
			tenant = admin.Tenant{ID: user.TenantID, Code: code, Name: code, Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&tenant).Error; err != nil {
				return fmt.Errorf("create legacy tenant %d: %w", user.TenantID, err)
			}
		} else if err != nil {
			return err
		}
		role := strings.ToLower(strings.TrimSpace(user.Role))
		switch role {
		case adminperm.RoleAdmin, adminperm.RoleOperator, adminperm.RoleReviewer, adminperm.RoleReadonly:
		default:
			// Custom roles remain tenant-scoped only when their code is valid;
			// unknown historical values fail closed as readonly.
			if !adminperm.IsCustomRole(role) {
				role = adminperm.RoleReadonly
			}
		}
		membershipStatus := admin.MembershipActive
		if strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusDisabled) || strings.EqualFold(strings.TrimSpace(user.Status), "inactive") {
			membershipStatus = admin.MembershipDisabled
		}
		membership := admin.TenantMembership{TenantID: user.TenantID, UserID: user.ID, Role: role, Status: membershipStatus}
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "user_id"}}, DoNothing: true}).Create(&membership).Error; err != nil {
			return fmt.Errorf("backfill membership %s: %w", user.ID, err)
		}
	}
	if db.Dialector.Name() == "postgres" {
		// Legacy tenants retain their AdminUser.tenant_id as the primary key.
		// Advance PostgreSQL's generated-ID sequence before provisioning can
		// create a new tenant, or the next insert can collide with a backfill.
		if err := db.Exec(`SELECT setval(pg_get_serial_sequence('tenants', 'id'), COALESCE(MAX(id), 1), MAX(id) IS NOT NULL) FROM tenants`).Error; err != nil {
			return fmt.Errorf("synchronize tenant ID sequence: %w", err)
		}
	}
	return nil
}

func errorsIsNotFound(err error) bool { return err == gorm.ErrRecordNotFound }
