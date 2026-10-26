package douyinpreflight

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPreflightRoutesEnforceRootAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name     string
		method   string
		path     string
		tenantID int64
		role     string
		status   string
		noAuth   bool
		wantCode int
	}{
		{name: "root administrator can run", method: http.MethodPost, path: "/platform/douyin/production-preflight", tenantID: 0, role: adminperm.RoleAdmin, status: admin.StatusActive, wantCode: http.StatusOK},
		{name: "root administrator can read", method: http.MethodGet, path: "/platform/douyin/production-preflight/latest", tenantID: 0, role: adminperm.RoleAdmin, status: admin.StatusActive, wantCode: http.StatusOK},
		{name: "non-Root readonly may not read global preflight", method: http.MethodGet, path: "/platform/douyin/production-preflight/latest", tenantID: 0, role: adminperm.RoleReadonly, status: admin.StatusActive, wantCode: http.StatusForbidden},
		{name: "other tenant cannot read global state", method: http.MethodGet, path: "/platform/douyin/production-preflight/latest", tenantID: 17, role: adminperm.RoleAdmin, status: admin.StatusActive, wantCode: http.StatusForbidden},
		{name: "other tenant cannot run global preflight", method: http.MethodPost, path: "/platform/douyin/production-preflight", tenantID: 17, role: adminperm.RoleAdmin, status: admin.StatusActive, wantCode: http.StatusForbidden},
		{name: "readonly root cannot run", method: http.MethodPost, path: "/platform/douyin/production-preflight", tenantID: 0, role: adminperm.RoleReadonly, status: admin.StatusActive, wantCode: http.StatusForbidden},
		{name: "unknown root role cannot read", method: http.MethodGet, path: "/platform/douyin/production-preflight/latest", tenantID: 0, role: "unexpected", status: admin.StatusActive, wantCode: http.StatusForbidden},
		{name: "disabled root administrator cannot read", method: http.MethodGet, path: "/platform/douyin/production-preflight/latest", tenantID: 0, role: adminperm.RoleAdmin, status: admin.StatusDisabled, wantCode: http.StatusForbidden},
		{name: "missing authentication is rejected", method: http.MethodPost, path: "/platform/douyin/production-preflight", noAuth: true, wantCode: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPreflightHTTPFixture(t)
			var actor *admin.AdminUser
			if !tc.noAuth {
				actor = f.addActor(t, tc.tenantID, tc.role, tc.status)
			}
			res := f.request(t, tc.method, tc.path, `{"liveTest":false}`, actor)
			require.Equal(t, tc.wantCode, res.Code, res.Body.String())
			if tc.method == http.MethodPost && tc.wantCode == http.StatusForbidden {
				var rows []operationlog.OperationLog
				require.NoError(t, f.db.Where("action = ?", "douyin.production.preflight.denied").Find(&rows).Error)
				require.Len(t, rows, 1)
				require.Equal(t, "denied", rows[0].Status)
			}
			if tc.wantCode != http.StatusOK {
				var count int64
				require.NoError(t, f.db.Model(&settings.Setting{}).Where("tenant_id = 0 AND group_key = ? AND item_key = ?", settingsGroup, settingsKey).Count(&count).Error)
				require.Zero(t, count, "unauthorized requests must not persist preflight results")
			}
		})
	}
}

func TestPreflightFailsClosedBeforeRunningWhenStartAuditFails(t *testing.T) {
	f := newPreflightHTTPFixture(t)
	actor := f.addActor(t, 0, adminperm.RoleAdmin, admin.StatusActive)
	require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register("test:fail_preflight_start_audit", func(tx *gorm.DB) {
		row, ok := tx.Statement.Dest.(*operationlog.OperationLog)
		if ok && row.Action == "douyin.production.preflight.started" {
			tx.AddError(errors.New("injected audit failure"))
		}
	}))

	res := f.request(t, http.MethodPost, "/platform/douyin/production-preflight", `{"liveTest":false}`, actor)
	require.Equal(t, http.StatusInternalServerError, res.Code, res.Body.String())
	require.NotContains(t, res.Body.String(), "injected audit failure")
	var count int64
	require.NoError(t, f.db.Model(&settings.Setting{}).Where("tenant_id = 0 AND group_key = ? AND item_key = ?", settingsGroup, settingsKey).Count(&count).Error)
	require.Zero(t, count, "preflight must not run when its durable start audit cannot be recorded")
	var failures []operationlog.OperationLog
	require.NoError(t, f.db.Where("action = ? AND status = ?", "douyin.production.preflight.denied", "failed").Find(&failures).Error)
	require.Len(t, failures, 1)
}

func TestPreflightResultAndCompletionAuditCommitTogether(t *testing.T) {
	f := newPreflightHTTPFixture(t)
	actor := f.addActor(t, 0, adminperm.RoleAdmin, admin.StatusActive)
	require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register("test:fail_preflight_completion_audit", func(tx *gorm.DB) {
		row, ok := tx.Statement.Dest.(*operationlog.OperationLog)
		if ok && row.Action == "douyin.production.preflight" {
			tx.AddError(errors.New("injected audit failure"))
		}
	}))

	res := f.request(t, http.MethodPost, "/platform/douyin/production-preflight", `{"liveTest":false}`, actor)
	require.Equal(t, http.StatusInternalServerError, res.Code, res.Body.String())
	require.NotContains(t, res.Body.String(), "injected audit failure")
	var count int64
	require.NoError(t, f.db.Model(&settings.Setting{}).Where("tenant_id = 0 AND group_key = ? AND item_key = ?", settingsGroup, settingsKey).Count(&count).Error)
	require.Zero(t, count, "result must roll back with its completion audit")
	var starts []operationlog.OperationLog
	require.NoError(t, f.db.Where("action = ? AND status = ?", "douyin.production.preflight.started", "started").Find(&starts).Error)
	require.Len(t, starts, 1, "durable start evidence must remain available for recovery")
	var failures []operationlog.OperationLog
	require.NoError(t, f.db.Where("action = ? AND status = ?", "douyin.production.preflight.denied", "failed").Find(&failures).Error)
	require.Len(t, failures, 1)
}

func TestPreflightDatabaseQueryErrorsAreFailedChecks(t *testing.T) {
	f := newPreflightHTTPFixture(t)
	service := &Service{DB: f.db, Settings: &settings.Service{DB: f.db}}
	require.NoError(t, f.db.Migrator().DropTable(&product.ProductSKU{}))

	result, err := service.Run(nil, RunRequest{})
	require.NoError(t, err)
	var skuCheck *CheckItem
	for i := range result.Checks {
		if result.Checks[i].Key == "data.skus" {
			skuCheck = &result.Checks[i]
			break
		}
	}
	require.NotNil(t, skuCheck)
	require.Equal(t, statusFailed, skuCheck.Status, "a failed database query must never be reported as an empty or healthy result")
}

func TestPreflightDoesNotExposeDatabaseErrorsInPersistableResults(t *testing.T) {
	f := newPreflightHTTPFixture(t)
	const internalError = "secret-preflight-test-error"
	injected := false
	require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register("test:fail_preflight_setting_reads", func(tx *gorm.DB) {
		if tx.Statement.Table == "settings" {
			injected = true
			tx.AddError(errors.New(internalError))
		}
	}))

	service := &Service{DB: f.db, Settings: &settings.Service{DB: f.db}}
	result, err := service.Run(nil, RunRequest{})
	require.NoError(t, err)
	require.True(t, injected, "the settings query failure must be exercised")

	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), internalError, "persistable results must not contain raw database errors")
}

func TestPreflightCredentialQueryFailureIsNotReportedAsMissingCredentials(t *testing.T) {
	f := newPreflightHTTPFixture(t)
	service := &Service{DB: f.db, Settings: &settings.Service{DB: f.db}}
	for _, item := range []settings.Setting{
		{TenantID: 0, GroupKey: "platform_douyin_shop", ItemKey: "app_key", ItemValue: "test-app-key"},
		{TenantID: 0, GroupKey: "platform_douyin_shop", ItemKey: "app_secret", ItemValue: "test-app-secret"},
	} {
		require.NoError(t, f.db.Create(&item).Error)
	}
	require.NoError(t, f.db.Migrator().DropTable(&shop.Shop{}))

	result, err := service.Run(nil, RunRequest{LiveTest: true})
	require.NoError(t, err)
	var credentialCheck *CheckItem
	for i := range result.Checks {
		if result.Checks[i].Key == "live.credentials" {
			credentialCheck = &result.Checks[i]
			break
		}
	}
	require.NotNil(t, credentialCheck, "database errors must produce a failed credential check")
	require.Equal(t, statusFailed, credentialCheck.Status)
	require.True(t, result.BlockedByReal, "live probes must remain blocked when credential state cannot be verified")
}

type preflightHTTPFixture struct {
	db *gorm.DB
}

func newPreflightHTTPFixture(t *testing.T) *preflightHTTPFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{},
		&settings.Setting{}, &operationlog.OperationLog{}, &shop.Shop{}, &shop.ShopAuthToken{},
		&product.Product{}, &product.ProductImage{}, &product.ProductSKU{},
		&productpublish.ProductPublication{}, &productpublish.ProductPublicationSKU{},
	))
	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		require.NoError(t, dbErr)
		require.NoError(t, sqlDB.Close())
	})
	return &preflightHTTPFixture{db: db}
}

func (f *preflightHTTPFixture) addActor(t *testing.T, tenantID int64, role, status string) *admin.AdminUser {
	t.Helper()
	actor := &admin.AdminUser{TenantID: tenantID, Username: admin.NewInternalUsername(), Role: role, Status: status, PasswordHash: "test-only", TokenVersion: 1}
	require.NoError(t, f.db.Create(actor).Error)
	return actor
}

func (f *preflightHTTPFixture) request(t *testing.T, method, path, body string, actor *admin.AdminUser) *httptest.ResponseRecorder {
	var tenantID int64
	if actor != nil { tenantID = actor.TenantID }
	return f.requestInTenant(t, method, path, body, actor, tenantID)
}
func (f *preflightHTTPFixture) requestInTenant(t *testing.T, method, path, body string, actor *admin.AdminUser, tenantID int64) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	if actor != nil {
		router.Use(func(c *gin.Context) {
			c.Set(ctxkey.AdminID, actor.ID.String())
			c.Set(ctxkey.TenantID, tenantID)
			c.Set(ctxkey.TraceID, "test-preflight-rbac")
			c.Next()
		})
	}
	logService := &operationlog.Service{DB: f.db}
	service := &Service{DB: f.db, Settings: &settings.Service{DB: f.db}}
	Register(router.Group(""), &Handler{Svc: service, OpLog: logService})
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestPreflightFromAuthenticatedRootBusinessWorkspace(t *testing.T) {
	f := newPreflightHTTPFixture(t)
	require.NoError(t, f.db.AutoMigrate(&admin.RootAccount{}, &admin.Tenant{}, &admin.TenantMembership{}))
	root := f.addActor(t, 0, adminperm.RoleAdmin, admin.StatusActive)
	require.NoError(t, f.db.Create(&admin.RootAccount{ID: 1, UserID: root.ID}).Error)
	require.NoError(t, f.db.Create(&admin.Tenant{ID: 35, Code: admin.RootWorkspaceCode, Name: admin.RootWorkspaceName, Status: admin.TenantActive}).Error)
	require.NoError(t, f.db.Create(&admin.Tenant{ID: 36, Code: "tenant-b", Name: "B", Status: admin.TenantActive}).Error)
	require.NoError(t, f.db.Create(&admin.TenantMembership{TenantID: 35, UserID: root.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive}).Error)
	other := f.addActor(t, 36, adminperm.RoleAdmin, admin.StatusActive)
	require.NoError(t, f.db.Create(&admin.TenantMembership{TenantID: 36, UserID: other.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive}).Error)

	for _, tc := range []struct{method, path string}{
		{http.MethodGet, "/platform/douyin/production-preflight/latest"},
		{http.MethodPost, "/platform/douyin/production-preflight"},
	} {
		t.Run(tc.method, func(t *testing.T) {
			allowed := f.requestInTenant(t, tc.method, tc.path, `{"liveTest":false}`, root, 35)
			require.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
			denied := f.requestInTenant(t, tc.method, tc.path, `{"liveTest":false}`, other, 36)
			require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
		})
	}
	require.NoError(t, f.db.Model(&admin.RootAccount{}).Where("id = ?", 1).Update("user_id", other.ID).Error)
	revoked := f.requestInTenant(t, http.MethodGet, "/platform/douyin/production-preflight/latest", "", root, 35)
	require.Equal(t, http.StatusForbidden, revoked.Code, revoked.Body.String())
}
