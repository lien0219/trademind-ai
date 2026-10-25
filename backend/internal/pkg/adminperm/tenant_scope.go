package adminperm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTenantPermissionDenied = errors.New("tenant permission denied")

// CheckTenantPermission reads current SQL authorization without request or
// process caches. Security mutations call it after taking the tenant lock.
func CheckTenantPermission(ctx context.Context, db *gorm.DB, tenantID int64, userID uuid.UUID, permission string) error {
	if db == nil || tenantID <= 0 || userID == uuid.Nil {
		return ErrTenantPermissionDenied
	}
	var user admin.AdminUser
	if err := db.WithContext(ctx).Select("status").First(&user, "id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTenantPermissionDenied
		}
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(user.Status), admin.StatusActive) {
		return ErrTenantPermissionDenied
	}
	access, err := ResolveTenantAccess(ctx, db, userID, tenantID)
	if err != nil {
		return err
	}
	if access == nil || !access.Schema || !access.Allowed {
		return ErrTenantPermissionDenied
	}
	role := normalizeRole(access.Membership.Role)
	if permission == RoleAdmin {
		if role == RoleAdmin {
			return nil
		}
		return ErrTenantPermissionDenied
	}
	if IsCustomRole(role) {
		var definition admin.RoleDefinition
		if err := db.WithContext(ctx).Where("tenant_id = ? AND code = ? AND enabled = ?", tenantID, role, true).First(&definition).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTenantPermissionDenied
			}
			return err
		}
		var decoded []string
		if err := json.Unmarshal([]byte(definition.PermissionsJSON), &decoded); err != nil {
			return ErrTenantPermissionDenied
		}
		grants, err := ValidateCustomPermissions(decoded)
		if err == nil && permissionIn(grants, permission) {
			return nil
		}
	} else if HasPermission(role, permission) {
		return nil
	}
	return ErrTenantPermissionDenied
}

// TenantIDFromGin returns trusted tenant id from request context.
// tenant_id=0 is valid for legacy single-tenant development data.
func TenantIDFromGin(c *gin.Context) (int64, error) {
	if c == nil {
		return 0, errTenantContextMissing
	}
	v, ok := c.Get(ctxkey.TenantID)
	if !ok {
		return 0, errTenantContextMissing
	}
	switch tid := v.(type) {
	case int64:
		return tid, nil
	case int:
		return int64(tid), nil
	default:
		return 0, errTenantContextMissing
	}
}

// ApplyTenantScope restricts query to current tenant for all roles.
func ApplyTenantScope(c *gin.Context, tx *gorm.DB) (*gorm.DB, int64, error) {
	if tx == nil {
		return tx, 0, nil
	}
	tid, err := TenantIDFromGin(c)
	if err != nil {
		return nil, 0, err
	}
	return tx.Where("tenant_id = ?", tid), tid, nil
}

// TenantSchemaState reports whether the durable tenant registry is available.
// Legacy SQLite fixtures are allowed to use AdminUser.TenantID only when both
// tenant tables are absent. A partially migrated database fails closed.
func TenantSchemaState(db *gorm.DB) (registry, membership bool) {
	if db == nil {
		return false, false
	}
	return db.Migrator().HasTable(&admin.Tenant{}), db.Migrator().HasTable(&admin.TenantMembership{})
}

// TenantAccess is the authoritative tenant-scoped identity decision.
type TenantAccess struct {
	Tenant     admin.Tenant
	Membership admin.TenantMembership
	Schema     bool
	Allowed    bool
}

// ResolveTenantAccess resolves a user's membership and tenant state. It never
// falls back to AdminUser.Role once either tenant table exists.
func ResolveTenantAccess(ctx context.Context, db *gorm.DB, userID uuid.UUID, tenantID int64) (*TenantAccess, error) {
	if db == nil || userID == uuid.Nil || tenantID <= 0 {
		return &TenantAccess{}, nil
	}
	registry, memberships := TenantSchemaState(db)
	if !registry && !memberships {
		return &TenantAccess{Schema: false}, nil
	}
	access := &TenantAccess{Schema: true}
	if !registry || !memberships {
		return access, nil
	}
	if err := db.WithContext(ctx).First(&access.Tenant, "id = ?", tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return access, nil
		}
		return nil, err
	}
	if err := db.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&access.Membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return access, nil
		}
		return nil, err
	}
	access.Allowed = access.Tenant.Status == admin.TenantActive && access.Membership.Status == admin.MembershipActive
	return access, nil
}

// LockTenantMutation serializes tenant role and membership changes with the
// existing admin-user mutation lock namespace.
func LockTenantMutation(tx *gorm.DB, tenantID int64) error {
	if tx == nil || tenantID <= 0 {
		return errors.New("tenant mutation lock unavailable")
	}
	keyHash := sha256.Sum256([]byte(fmt.Sprintf("trademind:admin-users:%d", tenantID)))
	key := hex.EncodeToString(keyHash[:])
	row := &admin.AdminMutationLock{KeyHash: key}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return err
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key_hash = ?", key).Take(row).Error
}
