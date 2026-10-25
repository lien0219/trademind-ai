package admin

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

func perfBootstrapDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AdminUser{}, &AdminMutationLock{}, &PlatformOperator{}, &UserStorePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS shops (
		id char(36) PRIMARY KEY,
		tenant_id INTEGER NOT NULL DEFAULT 0,
		platform TEXT,
		shop_name TEXT,
		shop_code TEXT,
		status TEXT,
		auth_status TEXT,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("shops table: %v", err)
	}
	sid := uuid.New().String()
	if err := db.Exec(
		`INSERT INTO shops (id, tenant_id, platform, shop_name, shop_code, status, auth_status, created_at, updated_at)
		 VALUES (?, 1, 'mock', 'Perf Shop', 'perf-shop-1', 'active', 'mock_authorized', datetime('now'), datetime('now'))`,
		sid,
	).Error; err != nil {
		t.Fatalf("seed shop: %v", err)
	}
	return db
}

func TestEnsurePerformanceBootstrap_idempotent(t *testing.T) {
	t.Setenv("P7V2_PERF_TENANT_ADMIN_PASSWORD", "TenantAdmin-Test-2026!")
	t.Setenv("P7V2_PERF_OPERATOR_PASSWORD", "Operator-Test-2026!")
	t.Setenv("P7V2_PERF_READONLY_PASSWORD", "Readonly-Test-2026!")
	t.Setenv("P7V2_PERF_ADMIN_PASSWORD", "SystemAdmin-Test-2026!")
	t.Setenv("ADMIN_BOOTSTRAP_EMAIL", PerfSystemAdminEmail)

	db := perfBootstrapDB(t)
	cfg := &config.Config{
		AppEnv: config.EnvPerformance,
		RuntimeLimits: config.RuntimeLimitsConfig{
			PerformanceTestMode:     true,
			AllowPerformanceDataset: true,
			ExternalProviderMode:    "mock",
		},
		BootstrapAdminEmail:    PerfSystemAdminEmail,
		BootstrapAdminPassword: "SystemAdmin-Test-2026!",
	}

	ctx := context.Background()
	audit := &recordingBootstrapAuditWriter{}
	if err := EnsureBootstrapAdmin(ctx, db, cfg, slog.Default(), audit); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}
	s1, err := EnsurePerformanceBootstrap(ctx, db, cfg, slog.Default(), audit)
	if err != nil {
		t.Fatalf("perf bootstrap run1: %v", err)
	}
	if s1.UsersCreated < 4 {
		t.Fatalf("expected at least 4 users created, got %+v", s1)
	}
	callsBeforeRerun := len(audit.calls)
	s2, err := EnsurePerformanceBootstrap(ctx, db, cfg, slog.Default(), audit)
	if err != nil {
		t.Fatalf("perf bootstrap run2: %v", err)
	}
	if s2.UsersCreated != 0 {
		t.Fatalf("expected no new users on rerun, got %+v", s2)
	}
	if len(audit.calls) != callsBeforeRerun {
		t.Fatalf("idempotent rerun wrote %d additional audits", len(audit.calls)-callsBeforeRerun)
	}

	var disabled AdminUser
	if err := db.Where("LOWER(TRIM(email)) = ?", PerfDisabledEmail).First(&disabled).Error; err != nil {
		t.Fatalf("load disabled: %v", err)
	}
	if strings.TrimSpace(strings.ToLower(disabled.Status)) != "disabled" {
		t.Fatalf("disabled status=%q", disabled.Status)
	}

	var system AdminUser
	if err := db.Where("LOWER(TRIM(email)) = ?", PerfSystemAdminEmail).First(&system).Error; err != nil {
		t.Fatalf("load system admin: %v", err)
	}
	if err := CheckPassword(system.PasswordHash, cfg.BootstrapAdminPassword); err != nil {
		t.Fatalf("system admin password mismatch: %v", err)
	}
}

func TestEnsurePerformanceBootstrap_rejectsProduction(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AppEnv: config.EnvProduction, RuntimeLimits: config.RuntimeLimitsConfig{PerformanceTestMode: true}}
	_, err := EnsurePerformanceBootstrap(context.Background(), perfBootstrapDB(t), cfg, nil, &recordingBootstrapAuditWriter{})
	if err == nil {
		t.Fatal("expected production rejection")
	}
}
