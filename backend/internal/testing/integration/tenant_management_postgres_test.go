//go:build integration || inventorypostgres

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/tenantmgmt"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const tenantPGPassword = "Test-Only-Membership2026!"

type tenantPGFixture struct {
	db       *gorm.DB
	svc      *tenantmgmt.Service
	sessions *auth.SessionService
	operator admin.AdminUser
	router   *gin.Engine
}

func newTenantPGFixture(t *testing.T) *tenantPGFixture {
	t.Helper()
	db := postgrestest.Require(t).DB
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &admin.AdminMutationLock{}, &admin.Tenant{}, &admin.TenantMembership{},
		&admin.TenantInvitation{}, &admin.PlatformOperator{}, &admin.RoleDefinition{}, &admin.UserStorePermission{},
		&auth.AuthSession{}, &auth.AuthRefreshToken{}, &auth.AuthLoginAttempt{}, &operationlog.OperationLog{},
		&shop.Shop{}, &product.Product{}, &product.ProductSKU{}, &product.ProductImage{}, &product.ProductAIContentApplication{},
		&product.ProductPlatformPublishConfig{}, &productpublish.ProductPublication{},
	))
	cfg := &config.Config{AppEnv: "test", JWTSecret: "postgres-tenant-integration-test-secret-at-least-32-bytes",
		Auth: config.AuthConfig{SessionMode: config.AuthSessionModeSecure, AccessTokenTTLMinutes: 15, RefreshTokenTTLDays: 7}}
	audit := &operationlog.Service{DB: db}
	f := &tenantPGFixture{db: db, svc: &tenantmgmt.Service{DB: db, Audit: audit}}
	f.sessions = &auth.SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}, OpLog: audit}
	f.operator = f.user(t, "platform")
	require.NoError(t, db.Create(&admin.PlatformOperator{UserID: f.operator.ID, Enabled: true}).Error)
	gin.SetMode(gin.TestMode)
	f.router = gin.New()
	g := f.router.Group("/api/v1", middleware.BearerAuthWithDB(cfg, db, f.sessions))
	tenantmgmt.Register(g, &tenantmgmt.Handler{Service: f.svc})
	(&auth.Handler{Cfg: cfg, DB: db, Sessions: f.sessions, Admins: &admin.Store{DB: db}, OpLog: audit}).RegisterTenantRoutes(g)
	products := &product.Handler{Svc: &product.Service{DB: db}, Files: &files.Service{DB: db}}
	g.PUT("/products/:id/platform-configs/douyin_shop/mapping", products.PutDouyinDraftMapping)
	g.GET("/products/:id/platform-configs/douyin_shop/images/status", products.GetDouyinImageStatus)
	g.POST("/products/:id/platform-configs/douyin_shop/images/upload", products.UploadDouyinImages)
	g.POST("/products/:id/platform-configs/douyin_shop/images/:imageKey/retry", products.RetryDouyinImage)
	g.POST("/products/:id/sync-images", products.SyncImages)
	g.POST("/products/:id/undo-ai-title", products.UndoAITitle)
	return f
}

func (f *tenantPGFixture) user(t *testing.T, label string) admin.AdminUser {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(tenantPGPassword), bcrypt.MinCost)
	require.NoError(t, err)
	u := admin.AdminUser{Username: admin.NewInternalUsername(), Email: label + "-" + uuid.NewString() + "@example.test",
		PasswordHash: string(hash), Role: "admin", Status: admin.StatusActive, TokenVersion: 1}
	require.NoError(t, f.db.Create(&u).Error)
	return u
}

func (f *tenantPGFixture) tenant(t *testing.T, owner admin.AdminUser, label string) *admin.Tenant {
	t.Helper()
	row, err := f.svc.Create(context.Background(), f.operator.ID, tenantmgmt.CreateTenantInput{
		Code: label, Name: label, OwnerEmail: owner.Email, IdempotencyKey: uuid.NewString()})
	require.NoError(t, err)
	return row
}

func (f *tenantPGFixture) login(t *testing.T, user admin.AdminUser) *auth.LoginSessionResult {
	t.Helper()
	result, err := f.sessions.CreateSession(context.Background(), user.Email, tenantPGPassword, "127.0.0.1", "tenant-postgres-test")
	require.NoError(t, err)
	return result
}

func (f *tenantPGFixture) request(t *testing.T, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	return res
}

func TestTenantSystemAdministrationPostgresHTTP(t *testing.T) {
	for _, environment := range []string{"production", "test-with-fallback"} {
		t.Run(environment, func(t *testing.T) {
			f := newTenantPGFixture(t)
			if environment == "production" {
				f.sessions.Cfg.AppEnv = "production"
			} else {
				f.sessions.Cfg.Tenant.EnableDevDefaultTenant = true
				f.sessions.Cfg.Tenant.DevDefaultTenantID = 98765
			}
			require.NoError(t, f.db.AutoMigrate(&settings.Setting{}, &securitymod.KeyRotationJob{}))
			g := f.router.Group("/api/v1", middleware.BearerAuthWithDB(f.sessions.Cfg, f.db, f.sessions))
			settingsHandler := &settings.Handler{DB: f.db, Svc: &settings.Service{DB: f.db}, OpLog: f.svc.Audit}
			g.GET("/settings", settingsHandler.List)
			g.PUT("/settings", settingsHandler.Put)
			securitymod.RegisterRoutes(g, &securitymod.Handler{DB: f.db, Svc: &securitymod.Service{DB: f.db, OpLogs: f.svc.Audit}})
			g.GET("/auth/profile", (&auth.Handler{DB: f.db, Admins: &admin.Store{DB: f.db}}).Profile)
			root := f.user(t, "system-administrator")
			require.Greater(t, len(root.LoginLabel()), 64, "audit must accept the full persisted login label")
			rootSession := f.login(t, root)
			owner := f.user(t, "enterprise-administrator")
			tenant := f.tenant(t, owner, "system-scope-tenant")
			require.NoError(t, f.db.Model(&owner).Update("tenant_id", tenant.ID).Error)
			job := securitymod.KeyRotationJob{ActiveKeyID: "test-active-key", Scope: "global", Status: securitymod.RotationPrepared}
			require.NoError(t, f.db.Create(&job).Error)
			keyPath := "/security/keys/rotation/" + job.ID.String()
			put := f.request(t, rootSession.AccessToken, "PUT", "/settings", map[string]any{
				"items": []map[string]any{{"groupKey": "test", "itemKey": "system_label", "itemValue": "system-scope-marker"}},
			})
			require.Equal(t, http.StatusOK, put.Code, put.Body.String())
			res := f.request(t, rootSession.AccessToken, "GET", "/settings", nil)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())
			require.Contains(t, res.Body.String(), "system-scope-marker")
			require.Equal(t, http.StatusOK, f.request(t, rootSession.AccessToken, "GET", keyPath, nil).Code)
			require.Equal(t, http.StatusForbidden, f.request(t, rootSession.AccessToken, "GET", "/tenant/current", nil).Code)
			profile := f.request(t, rootSession.AccessToken, "GET", "/auth/profile", nil)
			require.Equal(t, http.StatusOK, profile.Code)
			var profileEnvelope struct {
				Data struct {
					TenantID    int64    `json:"tenantId"`
					Permissions []string `json:"permissions"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(profile.Body.Bytes(), &profileEnvelope))
			require.Zero(t, profileEnvelope.Data.TenantID, "a fallback must never rewrite an explicit system context")
			require.Contains(t, profileEnvelope.Data.Permissions, adminperm.PermSettingsManage)
			require.Contains(t, profileEnvelope.Data.Permissions, adminperm.PermSecurityKeyRotate)
			require.NotContains(t, profileEnvelope.Data.Permissions, adminperm.PermProductView)
			for _, actor := range []admin.AdminUser{owner, f.operator} {
				if actor.ID == f.operator.ID {
					require.NoError(t, f.db.Model(&actor).Update("role", adminperm.RoleReadonly).Error)
				}
				login := f.login(t, actor)
				claims, err := auth.ParseAccessToken(f.sessions.Cfg, nil, login.AccessToken)
				require.NoError(t, err)
				keys, err := auth.BuildKeySet(f.sessions.Cfg)
				require.NoError(t, err)
				forgedScope, _, err := auth.MintAccessToken(f.sessions.Cfg, keys, auth.MintAccessInput{
					UserID: actor.ID, Username: actor.LoginLabel(), TenantID: 0, TokenVersion: claims.TokenVersion,
				})
				require.NoError(t, err)
				for _, token := range []string{login.AccessToken, forgedScope} {
					require.Equal(t, http.StatusForbidden, f.request(t, token, "GET", "/settings", nil).Code)
					require.Equal(t, http.StatusForbidden, f.request(t, token, "GET", keyPath, nil).Code)
				}
				require.Equal(t, http.StatusForbidden, f.request(t, login.AccessToken, "POST", "/auth/tenants/switch", map[string]any{"tenantId": 0}).Code)
			}
			require.NoError(t, f.db.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: root.ID, Role: "readonly", Status: admin.MembershipActive}).Error)
			list := f.request(t, rootSession.AccessToken, "GET", "/auth/tenants", nil)
			require.Equal(t, http.StatusOK, list.Code)
			require.Contains(t, list.Body.String(), "系统控制台")
			claims, err := auth.ParseAccessToken(f.sessions.Cfg, nil, rootSession.AccessToken)
			require.NoError(t, err)
			enterprise, err := f.sessions.SwitchTenant(context.Background(), root.ID, rootSession.SessionID, claims.TokenVersion, tenant.ID, "127.0.0.1", "test")
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, f.request(t, enterprise.AccessToken, "GET", "/settings", nil).Code)
			require.Equal(t, http.StatusOK, f.request(t, enterprise.AccessToken, "GET", "/tenant/current", nil).Code)
			switched := f.request(t, enterprise.AccessToken, "POST", "/auth/tenants/switch", map[string]any{"tenantId": 0})
			require.Equal(t, http.StatusOK, switched.Code, switched.Body.String())
			var switchEnvelope struct {
				Data struct {
					Token string `json:"token"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(switched.Body.Bytes(), &switchEnvelope))
			require.Equal(t, http.StatusOK, f.request(t, switchEnvelope.Data.Token, "GET", "/settings", nil).Code)
			require.Equal(t, http.StatusUnauthorized, f.request(t, enterprise.AccessToken, "GET", "/tenant/current", nil).Code)
			_, err = f.sessions.RotateRefresh(context.Background(), enterprise.RefreshToken, "127.0.0.1", "test")
			require.Error(t, err)
		})
	}
}

func TestTenantProvisioningPostgresConcurrencyAndRollback(t *testing.T) {
	f := newTenantPGFixture(t)
	owner := f.user(t, "owner")
	in := tenantmgmt.CreateTenantInput{Code: "concurrent-tenant", Name: "Concurrent tenant", OwnerEmail: owner.Email, IdempotencyKey: uuid.NewString()}
	type result struct {
		row *admin.Tenant
		err error
	}
	done := make(chan result, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			row, err := f.svc.Create(context.Background(), f.operator.ID, in)
			done <- result{row, err}
		}()
	}
	close(start)
	first, second := <-done, <-done
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.Equal(t, first.row.ID, second.row.ID)
	var count int64
	require.NoError(t, f.db.Model(&admin.TenantMembership{}).Where("tenant_id = ?", first.row.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, f.db.Model(&operationlog.OperationLog{}).Where("tenant_id = ? AND action = ?", first.row.ID, "tenant.create").Count(&count).Error)
	require.EqualValues(t, 1, count)
	in.Name = "Changed request"
	_, err := f.svc.Create(context.Background(), f.operator.ID, in)
	require.Error(t, err)

	// Audit failure must roll back the tenant, identity, and membership together.
	require.NoError(t, f.db.Exec("ALTER TABLE operation_logs ADD CONSTRAINT test_reject_creation CHECK (action <> 'tenant.create') NOT VALID").Error)
	_, err = f.svc.Create(context.Background(), f.operator.ID, tenantmgmt.CreateTenantInput{
		Code: "rollback-tenant", Name: "Rollback tenant", OwnerEmail: "rollback-owner@example.test", OwnerPassword: tenantPGPassword})
	require.Error(t, err)
	require.NoError(t, f.db.Model(&admin.Tenant{}).Where("code = ?", "rollback-tenant").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, f.db.Model(&admin.AdminUser{}).Where("email = ?", "rollback-owner@example.test").Count(&count).Error)
	require.Zero(t, count)
}

func TestTenantMembershipSessionAndRBACPostgres(t *testing.T) {
	f := newTenantPGFixture(t)
	ownerA, ownerB := f.user(t, "owner-a"), f.user(t, "owner-b")
	a, b := f.tenant(t, ownerA, "tenant-alpha"), f.tenant(t, ownerB, "tenant-beta")
	inv, token, err := f.svc.CreateInvitation(context.Background(), ownerB.ID, b.ID, tenantmgmt.InvitationInput{Email: ownerA.Email, Role: "readonly"})
	require.NoError(t, err)
	require.NotEqual(t, token, inv.TokenHash)
	_, err = f.svc.AcceptInvitation(context.Background(), ownerA.ID, token, ownerA.Email)
	require.NoError(t, err)
	_, err = f.svc.AcceptInvitation(context.Background(), ownerA.ID, token, ownerA.Email)
	require.Error(t, err, "invitation replay must fail")
	old := f.login(t, ownerA)
	require.Equal(t, http.StatusOK, f.request(t, old.AccessToken, "GET", "/tenant/members", nil).Code)
	require.Equal(t, http.StatusForbidden, f.request(t, old.AccessToken, "GET", "/platform/tenants", nil).Code)
	claims, err := auth.ParseAccessToken(f.sessions.Cfg, nil, old.AccessToken)
	require.NoError(t, err)
	require.Equal(t, a.ID, claims.TenantID)

	type result struct {
		row *auth.LoginSessionResult
		err error
	}
	done := make(chan result, 2)
	start := make(chan struct{})
	for range 2 {
		go func() {
			<-start
			row, err := f.sessions.SwitchTenant(context.Background(), ownerA.ID, old.SessionID, claims.TokenVersion, b.ID, "127.0.0.1", "concurrent-switch")
			done <- result{row, err}
		}()
	}
	close(start)
	var switched *auth.LoginSessionResult
	successes := 0
	for range 2 {
		r := <-done
		if r.err == nil {
			successes++
			switched = r.row
		}
	}
	require.Equal(t, 1, successes, "only one switch may consume a session")
	require.Equal(t, http.StatusUnauthorized, f.request(t, old.AccessToken, "GET", "/tenant/current", nil).Code)
	_, err = f.sessions.RotateRefresh(context.Background(), old.RefreshToken, "127.0.0.1", "test")
	require.Error(t, err)
	require.Equal(t, http.StatusOK, f.request(t, switched.AccessToken, "GET", "/tenant/current", nil).Code)
	require.Equal(t, http.StatusForbidden, f.request(t, switched.AccessToken, "GET", "/tenant/members", nil).Code)
	require.Equal(t, http.StatusForbidden, f.request(t, switched.AccessToken, "PATCH", "/tenant/current", map[string]any{"name": "forged", "version": b.Version}).Code)
	var membership admin.TenantMembership
	require.NoError(t, f.db.Where("tenant_id = ? AND user_id = ?", b.ID, ownerA.ID).First(&membership).Error)
	// A's tenant administrator cannot mutate B's membership through either ID.
	require.Error(t, f.svc.UpdateMember(context.Background(), ownerA.ID, b.ID, membership.ID, "admin", "active", nil))
	require.Error(t, f.svc.UpdateMember(context.Background(), ownerA.ID, a.ID, membership.ID, "admin", "active", nil))

	b, err = f.svc.SetStatus(context.Background(), f.operator.ID, b.ID, admin.TenantSuspended, b.Version)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, f.request(t, switched.AccessToken, "GET", "/tenant/current", nil).Code)
	b, err = f.svc.SetStatus(context.Background(), f.operator.ID, b.ID, admin.TenantActive, b.Version)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, f.request(t, switched.AccessToken, "GET", "/tenant/current", nil).Code, "resuming must not revive revoked sessions")
	fresh := f.login(t, ownerA)
	require.NoError(t, f.svc.UpdateMember(context.Background(), ownerB.ID, b.ID, membership.ID, "readonly", admin.MembershipDisabled, nil))
	require.Equal(t, http.StatusUnauthorized, f.request(t, fresh.AccessToken, "GET", "/tenant/current", nil).Code)
	restored := f.login(t, ownerA)
	res := f.request(t, restored.AccessToken, "POST", "/auth/tenants/switch", map[string]any{"tenantId": b.ID})
	require.Equal(t, http.StatusForbidden, res.Code)
	require.Equal(t, http.StatusOK, f.request(t, restored.AccessToken, "GET", "/tenant/current", nil).Code, "a denied switch must preserve the existing session")
}

type invitationAcceptContextKey struct{}

type tenantSessionRaceContextKey struct{}

func TestTenantSwitchConcurrentRefreshPostgres(t *testing.T) {
	for _, replay := range []bool{false, true} {
		t.Run(fmt.Sprintf("replay=%t", replay), func(t *testing.T) {
			f := newTenantPGFixture(t)
			owner := f.user(t, "session-owner")
			f.tenant(t, owner, "switch-source")
			target := f.tenant(t, owner, "switch-target")
			old := f.login(t, owner)
			if replay {
				_, err := f.sessions.RotateRefresh(context.Background(), old.RefreshToken, "127.0.0.1", "test")
				require.NoError(t, err)
			}
			claims, err := auth.ParseAccessToken(f.sessions.Cfg, nil, old.AccessToken)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			locked, release := make(chan struct{}), make(chan struct{})
			var holdOnce, releaseOnce sync.Once
			unlock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unlock()
			appName := "tenant-refresh-race-" + uuid.NewString()
			require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register("tenant_refresh_connection", func(tx *gorm.DB) {
				if tx.Statement.Context.Value(tenantSessionRaceContextKey{}) == "refresh" {
					if err := tx.Session(&gorm.Session{NewDB: true}).Exec("SELECT set_config('application_name', ?, true)", appName).Error; err != nil {
						_ = tx.AddError(err)
					}
				}
			}))
			require.NoError(t, f.db.Callback().Query().After("gorm:query").Register("tenant_switch_holds_session", func(tx *gorm.DB) {
				if tx.Statement.Context.Value(tenantSessionRaceContextKey{}) == "switch" && tx.Statement.Table == "auth_sessions" && tx.Error == nil {
					holdOnce.Do(func() {
						close(locked)
						select {
						case <-release:
						case <-ctx.Done():
							_ = tx.AddError(ctx.Err())
						}
					})
				}
			}))
			t.Cleanup(func() {
				require.NoError(t, f.db.Callback().Query().Remove("tenant_refresh_connection"))
				require.NoError(t, f.db.Callback().Query().Remove("tenant_switch_holds_session"))
			})
			switched, refreshed := make(chan error, 1), make(chan error, 1)
			go func() {
				_, err := f.sessions.SwitchTenant(context.WithValue(ctx, tenantSessionRaceContextKey{}, "switch"), owner.ID, old.SessionID, claims.TokenVersion, target.ID, "127.0.0.1", "test")
				switched <- err
			}()
			select {
			case <-locked:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			go func() {
				_, err := f.sessions.RotateRefresh(context.WithValue(ctx, tenantSessionRaceContextKey{}, "refresh"), old.RefreshToken, "127.0.0.1", "test")
				refreshed <- err
			}()
			// Observe the actual PostgreSQL lock wait before allowing the switch
			// to revoke the refresh family. No sleeps or assumed scheduling.
			require.Eventually(t, func() bool {
				var waiting int64
				err := f.db.WithContext(ctx).Raw("SELECT count(*) FROM pg_stat_activity WHERE application_name = ? AND wait_event_type = 'Lock'", appName).Scan(&waiting).Error
				return err == nil && waiting == 1
			}, 5*time.Second, 10*time.Millisecond)
			unlock()
			require.NoError(t, <-switched, "tenant switching must not deadlock with token rotation")
			require.ErrorContains(t, <-refreshed, auth.ErrRefreshTokenRevoked)
		})
	}
}

func TestTenantInvitationRechecksStatusUnderPostgresMutationLock(t *testing.T) {
	f := newTenantPGFixture(t)
	owner, invited := f.user(t, "owner"), f.user(t, "invited")
	row := f.tenant(t, owner, "invitation-race")
	_, token, err := f.svc.CreateInvitation(context.Background(), owner.ID, row.ID, tenantmgmt.InvitationInput{Email: invited.Email, Role: "operator"})
	require.NoError(t, err)
	// Pre-create the lock row so the barrier observes SELECT FOR UPDATE.
	require.NoError(t, f.db.Transaction(func(tx *gorm.DB) error { return adminperm.LockTenantMutation(tx, row.ID) }))
	locking := make(chan struct{})
	var once sync.Once
	require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register("test_accept_waiting_for_tenant_lock", func(tx *gorm.DB) {
		if tx.Statement.Context.Value(invitationAcceptContextKey{}) == true && tx.Statement.Table == "admin_mutation_locks" {
			once.Do(func() { close(locking) })
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.db.Callback().Query().Remove("test_accept_waiting_for_tenant_lock")) })
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), invitationAcceptContextKey{}, true), 10*time.Second)
	defer cancel()
	err = f.db.Transaction(func(tx *gorm.DB) error {
		if err := adminperm.LockTenantMutation(tx, row.ID); err != nil {
			return err
		}
		go func() { _, err := f.svc.AcceptInvitation(ctx, invited.ID, token, invited.Email); done <- err }()
		select {
		case <-locking:
		case <-ctx.Done():
			return ctx.Err()
		}
		return tx.Model(&admin.Tenant{}).Where("id = ?", row.ID).Update("status", admin.TenantSuspended).Error
	})
	require.NoError(t, err)
	require.Error(t, <-done, "an invitation must not be accepted after suspension commits")
	var count int64
	require.NoError(t, f.db.Model(&admin.TenantMembership{}).Where("tenant_id = ? AND user_id = ?", row.ID, invited.ID).Count(&count).Error)
	require.Zero(t, count)
}

// Both administrator self-leave attempts are denied, even under concurrency.
// Transferring administration must use the explicit managed tenant workflow.
func TestTenantConcurrentAdministratorsCannotLeavePostgres(t *testing.T) {
	f := newTenantPGFixture(t)
	first, second := f.user(t, "first-admin"), f.user(t, "second-admin")
	row := f.tenant(t, first, "last-admin-race")
	require.NoError(t, f.db.Create(&admin.TenantMembership{TenantID: row.ID, UserID: second.ID, Role: "admin", Status: admin.MembershipActive}).Error)
	start, done := make(chan struct{}), make(chan error, 2)
	for _, user := range []admin.AdminUser{first, second} {
		go func(user admin.AdminUser) { <-start; done <- f.svc.Leave(context.Background(), user.ID, row.ID) }(user)
	}
	close(start)
	successes := 0
	for range 2 {
		if <-done == nil {
			successes++
		}
	}
	require.Zero(t, successes, "administrators must never self-leave a tenant")
	var count int64
	require.NoError(t, f.db.Model(&admin.TenantMembership{}).Where("tenant_id = ? AND role = ? AND status = ?", row.ID, "admin", admin.MembershipActive).Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestTenantMappingRejectsUnassignedShopPostgresHTTP(t *testing.T) {
	f := newTenantPGFixture(t)
	owner, operator := f.user(t, "owner"), f.user(t, "operator")
	row := f.tenant(t, owner, "mapping-scope")
	require.NoError(t, f.db.Create(&admin.TenantMembership{TenantID: row.ID, UserID: operator.ID, Role: "operator", Status: admin.MembershipActive}).Error)
	draft := product.Product{TenantID: row.ID, CreatedBy: &operator.ID, Source: "test", Title: "Operator draft", Status: product.StatusDraft}
	require.NoError(t, f.db.Create(&draft).Error)
	store := shop.Shop{TenantID: row.ID, Platform: "douyin_shop", ShopName: "Unassigned shop", Status: "active"}
	require.NoError(t, f.db.Create(&store).Error)
	assigned := shop.Shop{TenantID: row.ID, Platform: "douyin_shop", ShopName: "Assigned shop", Status: "active"}
	require.NoError(t, f.db.Create(&assigned).Error)
	require.NoError(t, f.db.Create(&admin.UserStorePermission{UserID: operator.ID, StoreID: assigned.ID, Platform: assigned.Platform, PermissionScope: admin.StorePermScopeOperate}).Error)
	require.NoError(t, f.db.Create(&productpublish.ProductPublication{TenantID: row.ID, ProductID: draft.ID, ShopID: assigned.ID, Platform: assigned.Platform, Status: "active", PublishStatus: "success"}).Error)
	login := f.login(t, operator)
	res := f.request(t, login.AccessToken, "PUT", fmt.Sprintf("/products/%s/platform-configs/douyin_shop/mapping", draft.ID), map[string]any{"shopId": store.ID.String()})
	require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, res.Code, res.Body.String())
	var count int64
	require.NoError(t, f.db.Model(&product.ProductPlatformPublishConfig{}).Where("product_id = ?", draft.ID).Count(&count).Error)
	require.Zero(t, count)
}

func TestTenantProductShopScopeProtectsImagesAndUndoPostgresHTTP(t *testing.T) {
	f := newTenantPGFixture(t)
	owner, operator := f.user(t, "image-owner"), f.user(t, "image-operator")
	row := f.tenant(t, owner, "image-shop-scope")
	require.NoError(t, f.db.Create(&admin.TenantMembership{TenantID: row.ID, UserID: operator.ID, Role: "operator", Status: admin.MembershipActive}).Error)
	draft := product.Product{TenantID: row.ID, Source: "test", Title: "Private product", AITitle: "Applied AI title", Status: product.StatusDraft}
	require.NoError(t, f.db.Create(&draft).Error)
	store := shop.Shop{TenantID: row.ID, Platform: "douyin_shop", ShopName: "Private shop", Status: "active"}
	require.NoError(t, f.db.Create(&store).Error)
	require.NoError(t, f.db.Create(&product.ProductPlatformPublishConfig{ProductID: draft.ID, Platform: store.Platform, ShopID: &store.ID, MappedTitle: "Private mapping"}).Error)
	application := product.ProductAIContentApplication{ProductID: draft.ID, FieldType: product.AIContentFieldTitle, PreviousValue: "Previous title", AppliedValue: draft.AITitle, AppliedAt: time.Now().UTC(), Status: product.AIContentApplyStatusApplied}
	require.NoError(t, f.db.Create(&application).Error)
	login := f.login(t, operator)
	base := fmt.Sprintf("/products/%s", draft.ID)
	for _, endpoint := range []struct{ method, path string }{
		{"GET", "/platform-configs/douyin_shop/images/status"},
		{"POST", "/platform-configs/douyin_shop/images/upload"},
		{"POST", "/platform-configs/douyin_shop/images/private/retry"},
		{"POST", "/sync-images"},
		{"POST", "/undo-ai-title"},
	} {
		t.Run("no-grant"+endpoint.path, func(t *testing.T) {
			res := f.request(t, login.AccessToken, endpoint.method, base+endpoint.path, map[string]any{})
			require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound}, res.Code, res.Body.String())
		})
	}
	var unchanged product.Product
	require.NoError(t, f.db.First(&unchanged, "id = ?", draft.ID).Error)
	require.Equal(t, draft.AITitle, unchanged.AITitle, "a rejected undo must not commit a field change")
	var unchangedApplication product.ProductAIContentApplication
	require.NoError(t, f.db.First(&unchangedApplication, "id = ?", application.ID).Error)
	require.Equal(t, product.AIContentApplyStatusApplied, unchangedApplication.Status)

	grant := admin.UserStorePermission{UserID: operator.ID, StoreID: store.ID, Platform: store.Platform, PermissionScope: admin.StorePermScopeView}
	require.NoError(t, f.db.Create(&grant).Error)
	adminperm.InvalidateUserPermissionCache(operator.ID)
	require.Equal(t, http.StatusOK, f.request(t, login.AccessToken, "GET", base+"/platform-configs/douyin_shop/images/status", nil).Code)
	for _, action := range []string{"upload", "private/retry"} {
		res := f.request(t, login.AccessToken, "POST", base+"/platform-configs/douyin_shop/images/"+action, map[string]any{})
		require.Equal(t, http.StatusForbidden, res.Code, "view permission must not permit platform image writes: %s", res.Body.String())
	}
	other := shop.Shop{TenantID: row.ID, Platform: store.Platform, ShopName: "Unassigned mapping shop", Status: "active"}
	require.NoError(t, f.db.Create(&other).Error)
	require.NoError(t, f.db.Create(&productpublish.ProductPublication{TenantID: row.ID, ProductID: draft.ID, ShopID: store.ID, Platform: store.Platform}).Error)
	require.NoError(t, f.db.Model(&product.ProductPlatformPublishConfig{}).Where("product_id = ?", draft.ID).Update("shop_id", other.ID).Error)
	res := f.request(t, login.AccessToken, "GET", base+"/platform-configs/douyin_shop/images/status", nil)
	require.Equal(t, http.StatusNotFound, res.Code, "product visibility cannot expose a different shop's mapping")
}

func TestTenantRemovedMembershipRequiresNewInvitationPostgres(t *testing.T) {
	f := newTenantPGFixture(t)
	owner, member := f.user(t, "owner"), f.user(t, "departed-member")
	row := f.tenant(t, owner, "member-consent")
	membership := admin.TenantMembership{TenantID: row.ID, UserID: member.ID, Role: "operator", Status: admin.MembershipActive}
	require.NoError(t, f.db.Create(&membership).Error)
	require.NoError(t, f.svc.Leave(context.Background(), member.ID, row.ID))
	var before admin.AdminUser
	require.NoError(t, f.db.First(&before, "id = ?", member.ID).Error)
	err := f.svc.UpdateMember(context.Background(), owner.ID, row.ID, membership.ID, "operator", admin.MembershipActive, nil)
	require.Error(t, err, "the administrator must not force a departed identity to rejoin")
	var after admin.AdminUser
	require.NoError(t, f.db.First(&after, "id = ?", member.ID).Error)
	require.Equal(t, before.TokenVersion, after.TokenVersion)
	require.NoError(t, f.db.First(&membership, "id = ?", membership.ID).Error)
	require.Equal(t, admin.MembershipRemoved, membership.Status)
	_, token, err := f.svc.CreateInvitation(context.Background(), owner.ID, row.ID, tenantmgmt.InvitationInput{Email: member.Email, Role: "operator"})
	require.NoError(t, err)
	_, err = f.svc.AcceptInvitation(context.Background(), member.ID, token, member.Email)
	require.NoError(t, err)
}
