package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	PerfSystemAdminEmail = "p7v2-perf-admin@example.invalid"
	PerfTenantAdminEmail = "p7v2-perf-tenant-admin@example.invalid"
	PerfOperatorEmail    = "p7v2-perf-operator@example.invalid"
	PerfReadonlyEmail    = "p7v2-perf-readonly@example.invalid"
	PerfDisabledEmail    = "p7v2-perf-disabled@example.invalid"
)

// PerformanceBootstrapStats summarizes idempotent performance account provisioning.
type PerformanceBootstrapStats struct {
	UsersCreated     int   `json:"usersCreated"`
	UsersUpdated     int   `json:"usersUpdated"`
	GrantsCreated    int   `json:"grantsCreated"`
	GrantsUpdated    int   `json:"grantsUpdated"`
	DuplicateUsers   int   `json:"duplicateUsers"`
	DuplicateRoles   int   `json:"duplicateRoles"`
	DuplicateAssigns int   `json:"duplicateAssignments"`
	DefaultTenantID  int64 `json:"defaultTenantId"`
	ShopsGranted     int   `json:"shopsGranted"`
}

type perfAccountSpec struct {
	email    string
	role     string
	status   string
	tenantID int64
	password string
}

// EnsurePerformanceBootstrap provisions fixed performance test identities.
// Allowed only when APP_ENV=performance and PERFORMANCE_TEST_MODE=true.
func EnsurePerformanceBootstrap(ctx context.Context, db *gorm.DB, cfg *config.Config, log *slog.Logger, audit BootstrapAuditWriter) (*PerformanceBootstrapStats, error) {
	stats := &PerformanceBootstrapStats{}
	if db == nil || cfg == nil {
		return stats, fmt.Errorf("performance bootstrap: invalid deps")
	}
	if config.IsProduction(cfg.AppEnv) {
		return stats, fmt.Errorf("performance bootstrap: forbidden in production")
	}
	if cfg.AppEnv != config.EnvPerformance || !cfg.RuntimeLimits.PerformanceTestMode {
		return nil, nil
	}
	if audit == nil {
		return stats, fmt.Errorf("performance bootstrap: security audit unavailable")
	}

	tenantID := perfDefaultTenantID()
	stats.DefaultTenantID = tenantID

	specs := perfAccountSpecs(cfg)

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockBootstrapMutation(tx); err != nil {
			return fmt.Errorf("performance bootstrap lock: %w", err)
		}
		// Keep lock ordering aligned with admin user creation: tenant scopes,
		// then the global identity scope.
		if err := lockBootstrapTenantMutations(tx, 0); err != nil {
			return fmt.Errorf("performance bootstrap system tenant lock: %w", err)
		}
		if tenantID != 0 {
			if err := lockBootstrapTenantMutations(tx, tenantID); err != nil {
				return fmt.Errorf("performance bootstrap tenant lock: %w", err)
			}
		}
		if err := lockBootstrapAccountIdentities(tx); err != nil {
			return fmt.Errorf("performance bootstrap identity lock: %w", err)
		}

		for _, spec := range specs {
			if strings.TrimSpace(spec.password) == "" {
				return fmt.Errorf("performance bootstrap: missing password for %s", spec.email)
			}
			created, updated, err := upsertPerfUser(ctx, tx, spec, audit)
			if err != nil {
				return err
			}
			if created {
				stats.UsersCreated++
			}
			if updated {
				stats.UsersUpdated++
			}
		}

		var users []AdminUser
		if err := tx.WithContext(ctx).
			Where("LOWER(TRIM(email)) IN ?", []string{
				strings.ToLower(PerfOperatorEmail),
				strings.ToLower(PerfReadonlyEmail),
			}).
			Find(&users).Error; err != nil {
			return fmt.Errorf("performance bootstrap load scoped users: %w", err)
		}

		shopIDs, err := perfTenantShopIDs(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		stats.ShopsGranted = len(shopIDs)

		for _, u := range users {
			scope := StorePermScopeOperate
			if strings.EqualFold(u.Role, "readonly") {
				scope = StorePermScopeView
			}
			added, updated, err := ensureStoreGrants(ctx, tx, u, shopIDs, scope, audit)
			if err != nil {
				return err
			}
			stats.GrantsCreated += added
			stats.GrantsUpdated += updated
		}
		return nil
	})
	if err != nil {
		// Report only committed mutations. The whole bootstrap transaction rolls
		// back if any account, grant, or audit write fails.
		return &PerformanceBootstrapStats{DefaultTenantID: tenantID}, err
	}

	if log != nil {
		log.Info("performance_bootstrap_complete",
			"usersCreated", stats.UsersCreated,
			"usersUpdated", stats.UsersUpdated,
			"grantsCreated", stats.GrantsCreated,
			"defaultTenantId", stats.DefaultTenantID,
			"shopsGranted", stats.ShopsGranted,
		)
	}
	return stats, nil
}

// PerformanceBootstrapUserIDs returns performance harness account ids for cache invalidation.
func PerformanceBootstrapUserIDs(ctx context.Context, db *gorm.DB) ([]uuid.UUID, error) {
	if db == nil {
		return nil, fmt.Errorf("performance bootstrap: no db")
	}
	var ids []uuid.UUID
	err := db.WithContext(ctx).Model(&AdminUser{}).
		Where("LOWER(TRIM(email)) LIKE ?", "p7v2-perf-%@example.invalid").
		Pluck("id", &ids).Error
	return ids, err
}

func upsertPerfUser(ctx context.Context, tx *gorm.DB, spec perfAccountSpec, audit BootstrapAuditWriter) (created bool, updated bool, err error) {
	email := strings.ToLower(strings.TrimSpace(spec.email))

	var existing AdminUser
	findErr := tx.WithContext(ctx).Where("LOWER(TRIM(email)) = ?", email).First(&existing).Error
	if findErr != nil && findErr != gorm.ErrRecordNotFound {
		return false, false, findErr
	}

	if findErr == gorm.ErrRecordNotFound {
		hash, err := bcrypt.GenerateFromPassword([]byte(spec.password), bcrypt.DefaultCost)
		if err != nil {
			return false, false, fmt.Errorf("performance bootstrap hash: %w", err)
		}
		u := AdminUser{
			TenantID:     spec.tenantID,
			Username:     NewInternalUsername(),
			Email:        email,
			PasswordHash: string(hash),
			DisplayName:  email,
			Role:         spec.role,
			Status:       spec.status,
			TokenVersion: 1,
		}
		if err := tx.WithContext(ctx).Create(&u).Error; err != nil {
			return false, false, fmt.Errorf("performance bootstrap create %s: %w", email, err)
		}
		if err := writeBootstrapAudit(audit, ctx, tx, u.TenantID, "admin.performance_bootstrap.create", "admin_user", u.ID.String(), fmt.Sprintf("after={tenantId:%d,role:%s,status:%s} passwordChanged=true", u.TenantID, u.Role, u.Status)); err != nil {
			return false, false, err
		}
		return true, false, nil
	}

	updates := map[string]any{}
	beforeTenant, beforeRole, beforeStatus := existing.TenantID, strings.TrimSpace(existing.Role), strings.TrimSpace(existing.Status)
	passwordChanged := CheckPassword(existing.PasswordHash, spec.password) != nil
	if existing.TenantID != spec.tenantID {
		updates["tenant_id"] = spec.tenantID
	}
	if strings.TrimSpace(strings.ToLower(existing.Role)) != strings.TrimSpace(strings.ToLower(spec.role)) {
		updates["role"] = spec.role
	}
	wantStatus := strings.TrimSpace(strings.ToLower(spec.status))
	curStatus := strings.TrimSpace(strings.ToLower(existing.Status))
	if wantStatus != curStatus {
		updates["status"] = spec.status
	}
	if passwordChanged {
		// Performance harness treats env passwords as source of truth across restarts.
		hash, err := bcrypt.GenerateFromPassword([]byte(spec.password), bcrypt.DefaultCost)
		if err != nil {
			return false, false, fmt.Errorf("performance bootstrap hash: %w", err)
		}
		updates["password_hash"] = string(hash)
	}
	if len(updates) > 0 {
		updates["token_version"] = gorm.Expr("token_version + 1")
		if err := tx.WithContext(ctx).Model(&AdminUser{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return false, false, fmt.Errorf("performance bootstrap update %s: %w", email, err)
		}
		afterTenant := existing.TenantID
		if value, ok := updates["tenant_id"].(int64); ok {
			afterTenant = value
		}
		afterRole, _ := updates["role"].(string)
		if afterRole == "" {
			afterRole = existing.Role
		}
		afterStatus, _ := updates["status"].(string)
		if afterStatus == "" {
			afterStatus = existing.Status
		}
		message := fmt.Sprintf("before={tenantId:%d,role:%s,status:%s} after={tenantId:%d,role:%s,status:%s} passwordChanged=%t tokenVersionIncremented=true", beforeTenant, beforeRole, beforeStatus, afterTenant, afterRole, afterStatus, passwordChanged)
		if err := writeBootstrapAudit(audit, ctx, tx, existing.TenantID, "admin.performance_bootstrap.update", "admin_user", existing.ID.String(), message); err != nil {
			return false, false, err
		}
		return false, true, nil
	}
	return false, false, nil
}

func ensureStoreGrants(ctx context.Context, tx *gorm.DB, user AdminUser, shopIDs []uuid.UUID, scope string, audit BootstrapAuditWriter) (int, int, error) {
	if user.ID == uuid.Nil || len(shopIDs) == 0 {
		return 0, 0, nil
	}
	scope = NormalizeStorePermScope(scope)
	created, updated := 0, 0
	for _, sid := range shopIDs {
		if sid == uuid.Nil {
			continue
		}
		var grant UserStorePermission
		err := tx.WithContext(ctx).Where("user_id = ? AND store_id = ?", user.ID, sid).First(&grant).Error
		if err == nil {
			before := NormalizeStorePermScope(grant.PermissionScope)
			if before == scope {
				continue
			}
			if err := tx.WithContext(ctx).Model(&grant).Update("permission_scope", scope).Error; err != nil {
				return created, updated, err
			}
			if err := writeBootstrapAudit(audit, ctx, tx, user.TenantID, "admin.performance_bootstrap.store_grant", "user_store_permission", grant.ID.String(), fmt.Sprintf("userId=%s shopId=%s beforeScope=%s afterScope=%s", user.ID, sid, before, scope)); err != nil {
				return created, updated, err
			}
			updated++
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return created, updated, fmt.Errorf("performance bootstrap load store grant: %w", err)
		}
		row := UserStorePermission{
			UserID:          user.ID,
			StoreID:         sid,
			Platform:        "mock",
			PermissionScope: scope,
		}
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			return created, updated, err
		}
		if err := writeBootstrapAudit(audit, ctx, tx, user.TenantID, "admin.performance_bootstrap.store_grant", "user_store_permission", row.ID.String(), fmt.Sprintf("userId=%s shopId=%s afterScope=%s", user.ID, sid, scope)); err != nil {
			return created, updated, err
		}
		created++
	}
	return created, updated, nil
}

func perfTenantShopIDs(ctx context.Context, db *gorm.DB, tenantID int64) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := db.WithContext(ctx).
		Table("shops").
		Select("id").
		Where("tenant_id = ?", tenantID).
		Order("created_at ASC, id ASC").
		Limit(32).
		Pluck("id", &ids).Error
	return ids, err
}

func perfAccountSpecs(cfg *config.Config) []perfAccountSpec {
	tenantID := perfDefaultTenantID()
	systemPassword := strings.TrimSpace(cfg.BootstrapAdminPassword)
	if systemPassword == "" {
		systemPassword = perfSystemAdminPassword()
	}
	return []perfAccountSpec{
		{
			email:    perfSystemAdminEmail(),
			role:     RoleAdmin,
			status:   StatusActive,
			tenantID: 0,
			password: systemPassword,
		},
		{
			email:    PerfTenantAdminEmail,
			role:     RoleAdmin,
			status:   StatusActive,
			tenantID: tenantID,
			password: os.Getenv("P7V2_PERF_TENANT_ADMIN_PASSWORD"),
		},
		{
			email:    PerfOperatorEmail,
			role:     "operator",
			status:   StatusActive,
			tenantID: tenantID,
			password: os.Getenv("P7V2_PERF_OPERATOR_PASSWORD"),
		},
		{
			email:    PerfReadonlyEmail,
			role:     "readonly",
			status:   StatusActive,
			tenantID: tenantID,
			password: os.Getenv("P7V2_PERF_READONLY_PASSWORD"),
		},
		{
			email:    PerfDisabledEmail,
			role:     "operator",
			status:   "disabled",
			tenantID: tenantID,
			password: firstNonEmptyEnv("P7V2_PERF_DISABLED_PASSWORD", "P7V2_PERF_OPERATOR_PASSWORD"),
		},
	}
}

func perfDefaultTenantID() int64 {
	raw := strings.TrimSpace(os.Getenv("P7_PERF_DEFAULT_TENANT_ID"))
	if raw == "" {
		return 1
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 1
	}
	return n
}

func firstNonEmptyEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func perfSystemAdminEmail() string {
	if v := firstNonEmptyEnv("ADMIN_BOOTSTRAP_EMAIL"); v != "" {
		return strings.ToLower(strings.TrimSpace(v))
	}
	return PerfSystemAdminEmail
}

func perfSystemAdminPassword() string {
	if v := firstNonEmptyEnv("P7V2_PERF_ADMIN_PASSWORD", "ADMIN_BOOTSTRAP_PASSWORD"); v != "" {
		return v
	}
	return ""
}
