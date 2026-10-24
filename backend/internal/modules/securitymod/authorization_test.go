package securitymod_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

func rotationHTTPFixture(t *testing.T, tenant int64, role, status string, identity, tenantClaim bool) (*gorm.DB, *securitymod.Service, *gin.Engine, uuid.UUID) {
	t.Helper()
	db := openRotationTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &securitymod.KeyRotationJob{}, &operationlog.OperationLog{}))
	actor := admin.AdminUser{Username: admin.NewInternalUsername(), Email: uuid.NewString() + "@example.test", PasswordHash: "test-only", Role: role, Status: status, TenantID: tenant, TokenVersion: 1}
	require.NoError(t, db.Create(&actor).Error)
	if adminperm.IsCustomRole(role) {
		require.NoError(t, db.Create(&admin.RoleDefinition{TenantID: tenant, Code: role, Name: "legacy rotation grant", Enabled: true, PermissionsJSON: `["security.key.rotate"]`}).Error)
	}
	svc := &securitymod.Service{DB: db, Cfg: testSecurityCfg(testKeyRing(t)), OpLogs: &operationlog.Service{DB: db}}
	r := gin.New()
	r.Use(gin.Recovery(), func(c *gin.Context) {
		if identity {
			c.Set(ctxkey.AdminID, actor.ID.String())
		}
		if tenantClaim {
			c.Set(ctxkey.TenantID, tenant)
		}
		c.Request = c.Request.WithContext(security.WithTenantContext(c.Request.Context(), &security.TenantContext{TenantID: tenant, UserID: actor.ID, Role: role, AuthSource: security.AuthSourceAccessToken}))
		c.Next()
	})
	securitymod.RegisterRoutes(r.Group("/api/v1"), &securitymod.Handler{DB: db, Svc: svc})
	return db, svc, r, actor.ID
}

func rotationRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/api/v1/security"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestGlobalRotationRejectsNonGlobalAdministrators(t *testing.T) {
	for _, actor := range []struct {
		name, role, status string
		tenant             int64
		identity, claim    bool
	}{
		{"tenant_admin", "admin", admin.StatusActive, 17, true, true},
		{"custom_role", "r_0123456789abcdef01234567", admin.StatusActive, 0, true, true},
		{"unknown_role", "unknown", admin.StatusActive, 0, true, true},
		{"disabled_admin", "admin", "disabled", 0, true, true},
		{"missing_identity", "admin", admin.StatusActive, 0, false, true},
		{"missing_tenant", "admin", admin.StatusActive, 0, true, false},
	} {
		t.Run(actor.name, func(t *testing.T) {
			db, _, r, _ := rotationHTTPFixture(t, actor.tenant, actor.role, actor.status, actor.identity, actor.claim)
			job := securitymod.KeyRotationJob{ActiveKeyID: "active", Scope: "global", Status: securitymod.RotationRunning}
			require.NoError(t, db.Create(&job).Error)
			for _, route := range []struct{ method, path, body string }{
				{http.MethodPost, "/keys/rotation/prepare", `{"confirmPhrase":"ROTATE-KEYS-DRY-RUN"}`},
				{http.MethodGet, "/keys/rotation/status", ""},
				{http.MethodPost, "/keys/rotation/start", `{"confirmPhrase":"ROTATE-KEYS-START"}`},
				{http.MethodGet, "/keys/rotation/" + job.ID.String(), ""},
				{http.MethodGet, "/keys/rotation/" + job.ID.String() + "/progress", ""},
				{http.MethodPost, "/keys/rotation/" + job.ID.String() + "/pause", "{}"},
				{http.MethodPost, "/keys/rotation/" + job.ID.String() + "/resume", "{}"},
				{http.MethodPost, "/keys/rotation/" + job.ID.String() + "/verify", "{}"},
				{http.MethodGet, "/keys/references", ""},
			} {
				t.Run(route.method+route.path, func(t *testing.T) {
					w := rotationRequest(r, route.method, route.path, route.body)
					require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
				})
			}
		})
	}
}

func TestRotationChangesRequireAtomicAudit(t *testing.T) {
	for _, action := range []string{"start", "pause", "resume", "verify"} {
		for _, auditMode := range []string{"success", "missing", "database_failure"} {
			t.Run(action+"/"+auditMode, func(t *testing.T) {
				db, svc, r, actorID := rotationHTTPFixture(t, 0, "admin", admin.StatusActive, true, true)
				initialStatus := securitymod.RotationRunning
				if action == "resume" {
					initialStatus = securitymod.RotationPaused
				}
				job := securitymod.KeyRotationJob{ActiveKeyID: "active", Scope: "global", Status: initialStatus}
				path, body := "/keys/rotation/start", `{"confirmPhrase":"ROTATE-KEYS-START"}`
				if action != "start" {
					require.NoError(t, db.Create(&job).Error)
					path, body = "/keys/rotation/"+job.ID.String()+"/"+action, "{}"
				}
				if auditMode == "missing" {
					svc.OpLogs = nil
				} else if auditMode == "database_failure" {
					require.NoError(t, db.Exec(`CREATE TRIGGER reject_rotation_audit BEFORE INSERT ON operation_logs BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`).Error)
				}
				w := rotationRequest(r, http.MethodPost, path, body)
				var jobs []securitymod.KeyRotationJob
				require.NoError(t, db.Find(&jobs).Error)
				var logs []operationlog.OperationLog
				require.NoError(t, db.Where("action = ? AND status = ?", "key_rotation_"+action, "success").Find(&logs).Error)
				if auditMode != "success" {
					require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
					require.Empty(t, logs)
					if action == "start" {
						require.Empty(t, jobs)
					} else {
						require.Len(t, jobs, 1)
						require.Equal(t, initialStatus, jobs[0].Status)
					}
					return
				}
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Len(t, jobs, 1)
				require.Len(t, logs, 1)
				require.Equal(t, jobs[0].ID.String(), logs[0].ResourceID)
				require.Equal(t, &actorID, logs[0].AdminUserID)
				require.Equal(t, adminperm.PermSecurityKeyRotate, logs[0].Permission)
				require.NotEmpty(t, logs[0].EntryHash)
				expected := map[string]string{"start": securitymod.RotationRunning, "pause": securitymod.RotationPaused, "resume": securitymod.RotationRunning, "verify": securitymod.RotationVerified}
				require.Equal(t, expected[action], jobs[0].Status)
			})
		}
	}
}

func TestRotationDeniedRequestIsAudited(t *testing.T) {
	db, _, r, actorID := rotationHTTPFixture(t, 17, "admin", admin.StatusActive, true, true)
	w := rotationRequest(r, http.MethodPost, "/keys/rotation/"+uuid.NewString()+"/pause", "{}")
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	var rows []operationlog.OperationLog
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, int64(17), rows[0].TenantID)
	require.Equal(t, &actorID, rows[0].AdminUserID)
	require.Equal(t, "denied", rows[0].Status)
}

func TestRotationPauseAndResumeRejectMissingOrStaleJobs(t *testing.T) {
	db, _, r, _ := rotationHTTPFixture(t, 0, "admin", admin.StatusActive, true, true)
	running := securitymod.KeyRotationJob{ActiveKeyID: "active", Scope: "global", Status: securitymod.RotationRunning}
	require.NoError(t, db.Create(&running).Error)

	paused := rotationRequest(r, http.MethodPost, "/keys/rotation/"+running.ID.String()+"/resume", "{}")
	require.Equal(t, http.StatusNotFound, paused.Code, paused.Body.String())
	var stored securitymod.KeyRotationJob
	require.NoError(t, db.First(&stored, "id = ?", running.ID).Error)
	require.Equal(t, securitymod.RotationRunning, stored.Status)

	missing := rotationRequest(r, http.MethodPost, "/keys/rotation/"+uuid.NewString()+"/pause", "{}")
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	require.NoError(t, db.First(&stored, "id = ?", running.ID).Error)
	require.Equal(t, securitymod.RotationRunning, stored.Status)

	var successes int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action IN ? AND status = ?", []string{"key_rotation_pause", "key_rotation_resume"}, "success").Count(&successes).Error)
	require.Zero(t, successes, "stale controls must not create success audit evidence")
}

func TestRotationVerificationReturnsPersistenceError(t *testing.T) {
	db, svc, _, _ := rotationHTTPFixture(t, 0, "admin", admin.StatusActive, true, true)
	job := securitymod.KeyRotationJob{ActiveKeyID: "active", Scope: "global", Status: securitymod.RotationCompleted}
	require.NoError(t, db.Create(&job).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_rotation_update BEFORE UPDATE ON key_rotation_jobs BEGIN SELECT RAISE(ABORT, 'job unavailable'); END`).Error)
	ok, _, err := svc.VerifyRotation(context.Background(), job.ID)
	require.Error(t, err)
	require.False(t, ok)
}

func TestAuditIntegrityUsesCurrentTenant(t *testing.T) {
	db, svc, _, _ := rotationHTTPFixture(t, 17, "admin", admin.StatusActive, true, true)
	for _, tenant := range []int64{0, 17} {
		require.NoError(t, svc.OpLogs.WriteBackground(context.Background(), operationlog.WriteOpts{TenantID: tenant, Action: "fixture", Status: "success"}))
	}
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("tenant_id = ?", 0).Update("message", "tampered").Error)
	ctx := security.WithTenantContext(context.Background(), &security.TenantContext{TenantID: 17})
	n, err := svc.VerifyAuditIntegrity(ctx, 7)
	require.NoError(t, err, "tenant 17 must not inspect tenant 0 chain")
	require.Equal(t, 1, n, "exactly one tenant 17 record must be checked")
	_, err = svc.VerifyAuditIntegrity(context.Background(), 7)
	require.Error(t, err, "missing tenant context must fail closed")
}
