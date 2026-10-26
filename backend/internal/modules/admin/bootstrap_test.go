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
	if err := db.AutoMigrate(&AdminUser{}, &AdminMutationLock{}, &PlatformOperator{}); err != nil {
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
	if len(audit.calls) != 2 || audit.calls[0].action != "admin.bootstrap.create" || audit.calls[1].action != "admin.bootstrap.platform_grant" || audit.calls[1].resourceID != u.ID.String() {
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
	if len(audit.calls) != 2 || audit.calls[0].action != "admin.bootstrap.platform_grant" || audit.calls[1].action != "admin.bootstrap.privileges_sync" {
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

// New installations must persist a platform operator grant exactly once.
func TestEnsureBootstrapAdmin_platformGrantIdempotentAndRestored(t *testing.T) {
	db := testBootstrapDB(t)
	cfg := &config.Config{AppEnv: config.EnvDevelopment, BootstrapAdminEmail: "root@example.com", BootstrapAdminPassword: "secret"}
	audit := &recordingBootstrapAuditWriter{}
	ctx := context.Background()
	if err := EnsureBootstrapAdmin(ctx, db, cfg, slog.Default(), audit); err != nil {
		t.Fatal(err)
	}
	var u AdminUser
	if err := db.Where("email = ?", cfg.BootstrapAdminEmail).First(&u).Error; err != nil {
		t.Fatal(err)
	}
	var grant PlatformOperator
	if err := db.First(&grant, "user_id = ?", u.ID).Error; err != nil || !grant.Enabled {
		t.Fatalf("grant=%+v err=%v", grant, err)
	}
	count := len(audit.calls)
	if err := EnsureBootstrapAdmin(ctx, db, cfg, slog.Default(), audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.calls) != count {
		t.Fatal("idempotent startup must not create extra audit events")
	}
	if err := db.Model(&grant).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	beforeVersion := u.TokenVersion
	if err := EnsureBootstrapAdmin(ctx, db, cfg, slog.Default(), audit); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&grant, "user_id = ?", u.ID).Error; err != nil || !grant.Enabled {
		t.Fatalf("grant not restored: %+v %v", grant, err)
	}
	if err := db.First(&u, "id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if u.TokenVersion != beforeVersion+1 {
		t.Fatalf("grant change did not revoke old tokens: %d", u.TokenVersion)
	}
}

func TestEnsureBootstrapAdmin_neverPromotesTenantLocalUser(t *testing.T) {
	db := testBootstrapDB(t)
	u := AdminUser{TenantID: 42, Username: NewInternalUsername(), Email: "owner@example.com", PasswordHash: "hash", Role: RoleAdmin, Status: StatusActive}
	if err := db.Create(&u).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvDevelopment, BootstrapAdminEmail: u.Email}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), &recordingBootstrapAuditWriter{}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&PlatformOperator{}).Where("user_id = ?", u.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("tenant-local user was promoted to platform operator")
	}
}

func TestBootstrapRootSingletonProvisionAndEnvRebindingDenied(t *testing.T) {
	db := testBootstrapDB(t)
	if err := db.AutoMigrate(&RootAccount{}, &Tenant{}, &TenantMembership{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvDevelopment, BootstrapAdminEmail: "root@example.test", BootstrapAdminPassword: "long-temporary-test-password"}
	audit := &recordingBootstrapAuditWriter{}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), audit); err != nil {
		t.Fatal(err)
	}
	var root RootAccount
	if err := db.First(&root, "id = ?", 1).Error; err != nil {
		t.Fatal(err)
	}
	var tenant Tenant
	if err := db.First(&tenant, "code = ?", RootWorkspaceCode).Error; err != nil {
		t.Fatal(err)
	}
	if tenant.Status != TenantActive || tenant.OwnerUserID == nil || *tenant.OwnerUserID != root.UserID {
		t.Fatalf("invalid default owner %+v", tenant)
	}
	var member TenantMembership
	if err := db.Where("tenant_id = ? AND user_id = ?", tenant.ID, root.UserID).First(&member).Error; err != nil {
		t.Fatal(err)
	}
	if member.Role != RoleAdmin || member.Status != MembershipActive {
		t.Fatal("root must own the default workspace")
	}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), audit); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&Tenant{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("not idempotent count=%d err=%v", count, err)
	}
	second := AdminUser{Username: NewInternalUsername(), Email: "new-root@example.test", Role: RoleAdmin, Status: StatusActive, PasswordHash: "not-used"}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	cfg.BootstrapAdminEmail = second.Email
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), audit); err == nil {
		t.Fatal("changing .env cannot reassign persisted root")
	}
	var retained RootAccount
	if err := db.First(&retained, "id = ?", 1).Error; err != nil {
		t.Fatal(err)
	}
	if retained.UserID != root.UserID {
		t.Fatal("root singleton changed")
	}
}

func TestBootstrapRootWorkspaceWithDeletedCustomerTenant(t *testing.T) {
	db := testBootstrapDB(t)
	if err := db.AutoMigrate(&RootAccount{}, &Tenant{}, &TenantMembership{}); err != nil {
		t.Fatal(err)
	}
	deleted := Tenant{Code: "deleted-customer", Name: "Old tenant", Status: TenantDeleted, SettingsJSON: "{}", Version: 2}
	if err := db.Create(&deleted).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvDevelopment, BootstrapAdminEmail: "root-tombstone@example.test", BootstrapAdminPassword: "local-test-password"}
	if err := EnsureBootstrapAdmin(context.Background(), db, cfg, slog.Default(), &recordingBootstrapAuditWriter{}); err != nil {
		t.Fatal(err)
	}
	var ws Tenant
	if err := db.Where("code = ?", RootWorkspaceCode).First(&ws).Error; err != nil {
		t.Fatal(err)
	}
	if ws.Status != TenantActive || ws.ID == deleted.ID {
		t.Fatal("invalid Root workspace")
	}
	var old Tenant
	if err := db.First(&old, "id = ?", deleted.ID).Error; err != nil {
		t.Fatal(err)
	}
	if old.Status != TenantDeleted {
		t.Fatal("customer tombstone modified")
	}
	var root AdminUser
	if err := db.Where("email = ?", cfg.BootstrapAdminEmail).First(&root).Error; err != nil {
		t.Fatal(err)
	}
	if root.LastTenantID != ws.ID {
		t.Fatalf("wrong initial Root workspace: %d != %d", root.LastTenantID, ws.ID)
	}
}
