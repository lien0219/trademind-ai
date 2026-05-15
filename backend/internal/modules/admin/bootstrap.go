package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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

	rawEmail := strings.TrimSpace(cfg.BootstrapAdminEmail)
	rawPhone := strings.TrimSpace(cfg.BootstrapAdminPhone)
	hasEmail := rawEmail != ""
	hasPhone := rawPhone != ""

	if !hasEmail && !hasPhone {
		return fmt.Errorf("admin bootstrap: set ADMIN_BOOTSTRAP_EMAIL and/or ADMIN_BOOTSTRAP_PHONE")
	}

	var email, phone string
	if hasEmail {
		em, _, ok := ParseLoginAccount(rawEmail)
		if !ok {
			return fmt.Errorf("admin bootstrap: ADMIN_BOOTSTRAP_EMAIL invalid")
		}
		email = em
	}
	if hasPhone {
		phone = NormalizePhoneDigits(rawPhone)
		if len(phone) < 10 || len(phone) > 15 {
			return fmt.Errorf("admin bootstrap: invalid ADMIN_BOOTSTRAP_PHONE")
		}
	}

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

	disp := email
	if disp == "" {
		disp = phone
	}

	u := AdminUser{
		Username:     NewInternalUsername(),
		Email:        email,
		Phone:        phone,
		PasswordHash: string(hash),
		DisplayName:  disp,
	}

	if err := db.WithContext(ctx).Create(&u).Error; err != nil {
		return fmt.Errorf("admin bootstrap create: %w", err)
	}
	if log != nil {
		log.Info("admin_bootstrapped", "email", email, "phone", phone, "id", u.ID.String())
	}
	return nil
}
