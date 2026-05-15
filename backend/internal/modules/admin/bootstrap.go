package admin

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// EnsureBootstrapAdmin creates the first admin when the table is empty.
func EnsureBootstrapAdmin(ctx context.Context, db *gorm.DB, cfg *config.Config, log *slog.Logger) error {
	if db == nil || cfg == nil {
		return fmt.Errorf("admin bootstrap: invalid deps")
	}
	var n int64
	if err := db.WithContext(ctx).Model(&AdminUser{}).Count(&n).Error; err != nil {
		return fmt.Errorf("admin bootstrap count: %w", err)
	}
	if n > 0 {
		return nil
	}

	username := cfg.BootstrapAdminUsername
	password := cfg.BootstrapAdminPassword
	if username == "" {
		username = "admin"
	}
	if password == "" {
		if cfg.AppEnv == "production" {
			return fmt.Errorf("admin bootstrap: no admins exist; set ADMIN_BOOTSTRAP_PASSWORD for first boot")
		}
		password = "changeme"
		if log != nil {
			log.Warn("admin_bootstrap_default_password", "username", username, "hint", "set ADMIN_BOOTSTRAP_PASSWORD for non-dev")
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("admin bootstrap hash: %w", err)
	}

	u := AdminUser{
		Username:     username,
		PasswordHash: string(hash),
		DisplayName:  username,
	}

	if err := db.WithContext(ctx).Create(&u).Error; err != nil {
		return fmt.Errorf("admin bootstrap create: %w", err)
	}
	if log != nil {
		log.Info("admin_bootstrapped", "username", username, "id", u.ID.String())
	}
	return nil
}
