package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	// RoleAdmin is the global administrator role with all permissions.
	RoleAdmin = "admin"
	// StatusActive allows login and API access.
	StatusActive = "active"
	// StatusDisabled revokes login and API access while retaining the account.
	StatusDisabled = "disabled"
)

type bootstrapIdentity struct {
	email string
	phone string
}

// BootstrapAuditWriter persists startup security changes in the same SQL
// transaction as their subject changes. admin depends only on this contract;
// operationlog implements it without creating a package cycle.
type BootstrapAuditWriter interface {
	WriteBootstrapAuditTx(ctx context.Context, tx *gorm.DB, tenantID int64, action, resource, resourceID, message string) error
}

// EnsureBootstrapAdmin creates the first admin when the table is empty and keeps
// the configured bootstrap account (ADMIN_BOOTSTRAP_EMAIL / PHONE) at admin role.
func EnsureBootstrapAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config, log *slog.Logger, audit BootstrapAuditWriter) error {
	if db == nil || cfg == nil || audit == nil {
		return fmt.Errorf("admin bootstrap: invalid deps")
	}
	id, err := parseBootstrapIdentity(cfg)
	if err != nil {
		var n int64
		if countErr := db.WithContext(ctx).Model(&AdminUser{}).Count(&n).Error; countErr != nil {
			return fmt.Errorf("admin bootstrap count: %w", countErr)
		}
		if n == 0 {
			return err
		}
		id = bootstrapIdentity{}
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockBootstrapMutation(tx); err != nil {
			return fmt.Errorf("admin bootstrap lock: %w", err)
		}
		var n int64
		if err := tx.Model(&AdminUser{}).Count(&n).Error; err != nil {
			return fmt.Errorf("admin bootstrap count: %w", err)
		}
		if n == 0 {
			if err := lockBootstrapTenantMutations(tx, 0); err != nil {
				return fmt.Errorf("admin bootstrap tenant lock: %w", err)
			}
			if err := lockBootstrapAccountIdentities(tx); err != nil {
				return fmt.Errorf("admin bootstrap identity lock: %w", err)
			}
			if err := createBootstrapAdmin(ctx, tx, cfg, id, log, audit); err != nil {
				return err
			}
		}
		return ensureBootstrapAdminPrivileges(ctx, tx, cfg, id, log, audit)
	})
}

func lockBootstrapMutation(tx *gorm.DB) error {
	return lockBootstrapMutationScope(tx, "trademind:admin-bootstrap")
}

func lockBootstrapTenantMutations(tx *gorm.DB, tenantID int64) error {
	return lockBootstrapMutationScope(tx, fmt.Sprintf("trademind:admin-users:%d", tenantID))
}

func lockBootstrapAccountIdentities(tx *gorm.DB) error {
	return lockBootstrapMutationScope(tx, "trademind:admin-account-identities")
}

func lockBootstrapMutationScope(tx *gorm.DB, scope string) error {
	key := sha256.Sum256([]byte(scope))
	lock := AdminMutationLock{KeyHash: hex.EncodeToString(key[:])}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&lock).Error; err != nil {
		return err
	}
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key_hash = ?", lock.KeyHash).Take(&lock).Error
}

func parseBootstrapIdentity(cfg *config.Config) (bootstrapIdentity, error) {
	rawEmail := strings.TrimSpace(cfg.BootstrapAdminEmail)
	rawPhone := strings.TrimSpace(cfg.BootstrapAdminPhone)
	hasEmail := rawEmail != ""
	hasPhone := rawPhone != ""

	if !hasEmail && !hasPhone {
		return bootstrapIdentity{}, fmt.Errorf("admin bootstrap: set ADMIN_BOOTSTRAP_EMAIL and/or ADMIN_BOOTSTRAP_PHONE")
	}

	var out bootstrapIdentity
	if hasEmail {
		em, _, ok := ParseLoginAccount(rawEmail)
		if !ok {
			return bootstrapIdentity{}, fmt.Errorf("admin bootstrap: ADMIN_BOOTSTRAP_EMAIL invalid")
		}
		out.email = em
	}
	if hasPhone {
		phone := NormalizePhoneDigits(rawPhone)
		if len(phone) < 10 || len(phone) > 15 {
			return bootstrapIdentity{}, fmt.Errorf("admin bootstrap: invalid ADMIN_BOOTSTRAP_PHONE")
		}
		out.phone = phone
	}
	return out, nil
}

func createBootstrapAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config, id bootstrapIdentity, log *slog.Logger, audit BootstrapAuditWriter) error {
	password := cfg.BootstrapAdminPassword
	if password == "" {
		if cfg.AppEnv == "production" {
			return fmt.Errorf("admin bootstrap: no admins exist; set ADMIN_BOOTSTRAP_PASSWORD for first boot")
		}
		password = "changeme"
		if log != nil {
			log.Warn("admin_bootstrap_default_password", "hint", "set ADMIN_BOOTSTRAP_PASSWORD for non-dev")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("admin bootstrap hash: %w", err)
	}

	disp := id.email
	if disp == "" {
		disp = id.phone
	}

	u := AdminUser{
		Username:     NewInternalUsername(),
		Email:        id.email,
		Phone:        id.phone,
		PasswordHash: string(hash),
		DisplayName:  disp,
		Role:         RoleAdmin,
		Status:       StatusActive,
	}

	if err := db.WithContext(ctx).Create(&u).Error; err != nil {
		return fmt.Errorf("admin bootstrap create: %w", err)
	}
	if err := writeBootstrapAudit(audit, ctx, db, 0, "admin.bootstrap.create", "admin_user", u.ID.String(), "role=admin status=active"); err != nil {
		return err
	}
	if log != nil {
		log.Info("admin_bootstrapped", "email", id.email, "phone", id.phone, "id", u.ID.String(), "role", RoleAdmin)
	}
	return nil
}

func ensureBootstrapAdminPrivileges(ctx context.Context, db *gorm.DB, cfg *config.Config, id bootstrapIdentity, log *slog.Logger, audit BootstrapAuditWriter) error {
	if id.email == "" && id.phone == "" {
		return nil
	}
	var u AdminUser
	found := false
	if id.email != "" {
		err := db.WithContext(ctx).Where("LOWER(TRIM(email)) = ?", id.email).First(&u).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("admin bootstrap find email: %w", err)
		}
	}
	if !found && id.phone != "" {
		err := db.WithContext(ctx).Where("phone = ?", id.phone).First(&u).Error
		if err == nil {
			found = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("admin bootstrap find phone: %w", err)
		}
	}
	if !found {
		if cfg.RuntimeLimits.PerformanceTestMode && cfg.AppEnv == "performance" {
			if err := lockBootstrapTenantMutations(db, 0); err != nil {
				return fmt.Errorf("admin bootstrap tenant lock: %w", err)
			}
			if err := lockBootstrapAccountIdentities(db); err != nil {
				return fmt.Errorf("admin bootstrap identity lock: %w", err)
			}
			return createBootstrapAdmin(ctx, db, cfg, id, log, audit)
		}
		return nil
	}
	if err := lockBootstrapTenantMutations(db, u.TenantID); err != nil {
		return fmt.Errorf("admin bootstrap tenant lock: %w", err)
	}
	if err := db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND tenant_id = ?", u.ID, u.TenantID).First(&u).Error; err != nil {
		return fmt.Errorf("admin bootstrap reload account: %w", err)
	}

	updates := map[string]any{}
	beforeRole, beforeStatus := strings.TrimSpace(u.Role), strings.TrimSpace(u.Status)
	passwordChanged := false
	if strings.TrimSpace(strings.ToLower(u.Role)) != RoleAdmin {
		updates["role"] = RoleAdmin
	}
	if st := strings.TrimSpace(strings.ToLower(u.Status)); st != StatusActive {
		updates["status"] = StatusActive
	}
	if cfg.RuntimeLimits.PerformanceTestMode && cfg.AppEnv == config.EnvPerformance {
		password := strings.TrimSpace(cfg.BootstrapAdminPassword)
		if password == "" {
			password = perfSystemAdminPassword()
		}
		if password != "" {
			if err := CheckPassword(u.PasswordHash, password); err != nil {
				hash, herr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
				if herr != nil {
					return fmt.Errorf("admin bootstrap password hash: %w", herr)
				}
				updates["password_hash"] = string(hash)
				passwordChanged = true
			}
		}
	}
	if len(updates) == 0 {
		return nil
	}
	updates["token_version"] = gorm.Expr("token_version + 1")
	if err := db.WithContext(ctx).Model(&AdminUser{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("admin bootstrap privileges: %w", err)
	}
	message := fmt.Sprintf("before={role:%s,status:%s} after={role:%s,status:%s} passwordChanged=%t", beforeRole, beforeStatus, valueOr(updates, "role", u.Role), valueOr(updates, "status", u.Status), passwordChanged)
	if err := writeBootstrapAudit(audit, ctx, db, u.TenantID, "admin.bootstrap.privileges_sync", "admin_user", u.ID.String(), message); err != nil {
		return err
	}
	if log != nil {
		log.Info("admin_bootstrap_privileges_synced", "email", u.Email, "phone", u.Phone, "id", u.ID.String(), "role", RoleAdmin)
	}
	return nil
}

func valueOr(values map[string]any, key, fallback string) string {
	if value, ok := values[key].(string); ok {
		return value
	}
	return fallback
}

func writeBootstrapAudit(audit BootstrapAuditWriter, ctx context.Context, tx *gorm.DB, tenantID int64, action, resource, resourceID, message string) error {
	if audit == nil {
		return fmt.Errorf("admin bootstrap: security audit unavailable")
	}
	if err := audit.WriteBootstrapAuditTx(ctx, tx, tenantID, action, resource, resourceID, message); err != nil {
		return fmt.Errorf("admin bootstrap: persist security audit: %w", err)
	}
	return nil
}
