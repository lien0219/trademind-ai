package admin

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"gorm.io/gorm"
)

type bootstrapAuditCall struct {
	tenantID   int64
	action     string
	resource   string
	resourceID string
	message    string
}

type recordingBootstrapAuditWriter struct {
	err   error
	calls []bootstrapAuditCall
}

func (w *recordingBootstrapAuditWriter) WriteBootstrapAuditTx(_ context.Context, _ *gorm.DB, tenantID int64, action, resource, resourceID, message string) error {
	w.calls = append(w.calls, bootstrapAuditCall{tenantID: tenantID, action: action, resource: resource, resourceID: resourceID, message: message})
	return w.err
}

func testBootstrapDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Skipf("sqlite unavailable: %v", err)
	}
	if err := db.AutoMigrate(&AdminUser{}, &AdminMutationLock{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestEnsureBootstrapAdmin_createsAdminRole(t *testing.T) {
	t.Parallel()
	db := testBootstrapDB(t)
	cfg := &config.Config{
		AppEnv:                 config.EnvDevelopment,
		BootstrapAdminEmail:    "admin@example.com",
		BootstrapAdminPassword: "secret",
	}
	audit := &recordingBootstrapAuditWriter{}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), audit); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	var u AdminUser
	if err := db.Where("email = ?", "admin@example.com").First(&u).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	if u.Role != RoleAdmin || u.Status != StatusActive {
		t.Fatalf("got role=%q status=%q", u.Role, u.Status)
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "admin.bootstrap.create" || audit.calls[0].resourceID != u.ID.String() {
		t.Fatalf("unexpected bootstrap audit calls: %+v", audit.calls)
	}
}

func TestEnsureBootstrapAdmin_syncsExistingOperatorToAdmin(t *testing.T) {
	t.Parallel()
	db := testBootstrapDB(t)
	existing := AdminUser{
		Username:     NewInternalUsername(),
		Email:        "ops@example.com",
		PasswordHash: "hash",
		Role:         "operator",
		Status:       StatusActive,
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := &config.Config{
		AppEnv:              config.EnvDevelopment,
		BootstrapAdminEmail: "ops@example.com",
	}
	audit := &recordingBootstrapAuditWriter{}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), audit); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	var u AdminUser
	if err := db.First(&u, "id = ?", existing.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if u.Role != RoleAdmin {
		t.Fatalf("expected admin role, got %q", u.Role)
	}
	if u.TokenVersion != existing.TokenVersion+1 {
		t.Fatalf("token version = %d, want %d", u.TokenVersion, existing.TokenVersion+1)
	}
	if len(audit.calls) != 1 || audit.calls[0].action != "admin.bootstrap.privileges_sync" {
		t.Fatalf("unexpected bootstrap audit calls: %+v", audit.calls)
	}
}

func TestEnsureBootstrapAdmin_auditFailureRollsBackCreation(t *testing.T) {
	t.Parallel()
	db := testBootstrapDB(t)
	cfg := &config.Config{AppEnv: config.EnvDevelopment, BootstrapAdminEmail: "admin@example.com", BootstrapAdminPassword: "secret"}
	audit := &recordingBootstrapAuditWriter{err: errors.New("audit unavailable")}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), audit); err == nil {
		t.Fatal("expected audit failure")
	}
	var count int64
	if err := db.Model(&AdminUser{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("created %d users despite audit failure", count)
	}
}

func TestEnsureBootstrapAdmin_auditFailureRollsBackPrivilegeAndTokenChanges(t *testing.T) {
	t.Parallel()
	db := testBootstrapDB(t)
	existing := AdminUser{Username: NewInternalUsername(), Email: "ops@example.com", PasswordHash: "hash", Role: "operator", Status: StatusDisabled, TokenVersion: 8}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvDevelopment, BootstrapAdminEmail: existing.Email}
	audit := &recordingBootstrapAuditWriter{err: errors.New("audit unavailable")}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), audit); err == nil {
		t.Fatal("expected audit failure")
	}
	var got AdminUser
	if err := db.First(&got, "id = ?", existing.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Role != "operator" || got.Status != StatusDisabled || got.TokenVersion != 8 {
		t.Fatalf("privilege change escaped failed audit transaction: role=%q status=%q tokenVersion=%d", got.Role, got.Status, got.TokenVersion)
	}
}
