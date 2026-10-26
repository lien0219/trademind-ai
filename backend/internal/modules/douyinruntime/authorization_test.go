package douyinruntime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	shopmodel "github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestRuntimeMutationRequiresRootSettingsAdmin(t *testing.T) {
	for _, path := range []string{
		"/platform/douyin/runtime-status/pause",
		"/platform/douyin/runtime-status/resume",
		"/platform/douyin/runtime-status/emergency-disable",
	} {
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				name      string
				tenantID  int64
				role      string
				status    string
				noContext bool
			}{
				{name: "readonly root", tenantID: 0, role: adminperm.RoleReadonly, status: admin.StatusActive},
				{name: "other tenant administrator", tenantID: 17, role: adminperm.RoleAdmin, status: admin.StatusActive},
				{name: "disabled administrator", tenantID: 0, role: adminperm.RoleAdmin, status: "disabled"},
				{name: "unknown role", tenantID: 0, role: "future-role", status: admin.StatusActive},
				{name: "missing authentication context", noContext: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					f := newRuntimeHTTPFixture(t, true)
					before := seedRuntimeStatus(t, f.db)
					var actor *admin.AdminUser
					if !tc.noContext {
						actor = f.addActor(t, tc.tenantID, tc.role, tc.status)
					}
					res := f.request(http.MethodPost, path, `{"reason":"operator requested"}`, actor, !tc.noContext)
					wantStatus := http.StatusForbidden
					if tc.noContext {
						wantStatus = http.StatusUnauthorized
					}
					require.Equal(t, wantStatus, res.Code, res.Body.String())
					requireRuntimeStatus(t, f.db, before)
					var rows []operationlog.OperationLog
					require.NoError(t, f.db.Find(&rows).Error)
					require.Len(t, rows, 1, "denied mutation must be audited")
					require.Equal(t, "denied", rows[0].Status)
					require.NotContains(t, rows[0].Message, "operator requested")
				})
			}
		})
	}
}

func TestRuntimeReadsRequireRootStoreViewAndDoNotWrite(t *testing.T) {
	paths := []string{
		"/platform/douyin/runtime-status",
		"/platform/douyin/health",
		"/platform/douyin/metrics-summary",
		"/platform/douyin/release-gate",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			f := newRuntimeHTTPFixture(t, true)
			readonly := f.addActor(t, 0, adminperm.RoleReadonly, admin.StatusActive)
			res := f.request(http.MethodGet, path, "", readonly, true)
			require.Equal(t, http.StatusOK, res.Code, res.Body.String())
			var count int64
			require.NoError(t, f.db.Model(&settings.Setting{}).Where("item_key = ?", "health_snapshot").Count(&count).Error)
			require.Zero(t, count, "read endpoints must not persist health snapshots")

			otherTenant := f.addActor(t, 17, adminperm.RoleAdmin, admin.StatusActive)
			denied := f.request(http.MethodGet, path, "", otherTenant, true)
			require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
			missing := f.request(http.MethodGet, path, "", nil, false)
			require.Equal(t, http.StatusUnauthorized, missing.Code, missing.Body.String())
		})
	}
}

func TestRuntimeStatusMutationCommitsSettingsWithTraceableAudit(t *testing.T) {
	f := newRuntimeHTTPFixture(t, true)
	actor := f.addActor(t, 0, adminperm.RoleAdmin, admin.StatusActive)
	res := f.request(http.MethodPost, "/platform/douyin/runtime-status/pause", `{"reason":"scheduled maintenance"}`, actor, true)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	var rows []settings.Setting
	require.NoError(t, f.db.Where("tenant_id = 0 AND group_key = ?", groupKey).Find(&rows).Error)
	require.Len(t, rows, 3)
	require.Equal(t, "paused", settingValue(rows, "platform_runtime_status"))
	require.Equal(t, "scheduled maintenance", settingValue(rows, "platform_runtime_status_reason"))

	var audits []operationlog.OperationLog
	require.NoError(t, f.db.Order("action asc").Find(&audits).Error)
	require.Len(t, audits, 2)
	for _, audit := range audits {
		require.Equal(t, actor.ID, *audit.AdminUserID)
		require.Equal(t, int64(0), audit.TenantID)
		require.Equal(t, "success", audit.Status)
		require.NotContains(t, audit.Message, "scheduled maintenance")
		require.NotEmpty(t, audit.RequestID)
	}
}

func TestRuntimeStatusMutationRollsBackWhenAuditCannotBeWritten(t *testing.T) {
	f := newRuntimeHTTPFixture(t, false)
	before := seedRuntimeStatus(t, f.db)
	actor := f.addActor(t, 0, adminperm.RoleAdmin, admin.StatusActive)
	res := f.request(http.MethodPost, "/platform/douyin/runtime-status/pause", `{"reason":"must not persist"}`, actor, true)
	require.Equal(t, http.StatusInternalServerError, res.Code, res.Body.String())
	require.NotContains(t, res.Body.String(), "must not persist")
	requireRuntimeStatus(t, f.db, before)
}

func TestRuntimeHealthCheckRequiresRootSettingsAdmin(t *testing.T) {
	f := newRuntimeHTTPFixture(t, true)
	readonly := f.addActor(t, 0, adminperm.RoleReadonly, admin.StatusActive)
	res := f.request(http.MethodPost, "/platform/douyin/run-health-check", "{}", readonly, true)
	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	var count int64
	require.NoError(t, f.db.Model(&settings.Setting{}).Where("item_key = ?", "health_snapshot").Count(&count).Error)
	require.Zero(t, count)
	var audits []operationlog.OperationLog
	require.NoError(t, f.db.Find(&audits).Error)
	require.Len(t, audits, 1)
	require.Equal(t, "denied", audits[0].Status)
}

func TestRuntimeAuthorizationRejectsMissingTrustedTenantContext(t *testing.T) {
	f := newRuntimeHTTPFixture(t, true)
	actor := f.addActor(t, 0, adminperm.RoleReadonly, admin.StatusActive)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/platform/douyin/runtime-status", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	require.False(t, authorizeRuntime(c, f.db, false), "root data must not be authorized without trusted tenant context")
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestRuntimeServiceRejectsMismatchedActorBeforeWriting(t *testing.T) {
	f := newRuntimeHTTPFixture(t, true)
	want := seedRuntimeStatus(t, f.db)
	actor := f.addActor(t, 0, adminperm.RoleAdmin, admin.StatusActive)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/platform/douyin/runtime-status/pause", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, int64(0))
	otherActorID := uuid.New()
	_, err := f.service.Pause(c, &otherActorID, ChangeBody{Reason: "must not persist"})
	require.Error(t, err)
	requireRuntimeStatus(t, f.db, want)
	var count int64
	require.NoError(t, f.db.Model(&operationlog.OperationLog{}).Count(&count).Error)
	require.Zero(t, count)
}

type runtimeHTTPFixture struct {
	db      *gorm.DB
	service *Service
}

func newRuntimeHTTPFixture(t *testing.T, withAudit bool) *runtimeHTTPFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	models := []any{&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shopmodel.Shop{}, &settings.Setting{}}
	if withAudit {
		models = append(models, &operationlog.OperationLog{})
	}
	require.NoError(t, db.AutoMigrate(models...))
	service := &Service{DB: db, Settings: &settings.Service{DB: db}, OpLog: &operationlog.Service{DB: db}}
	return &runtimeHTTPFixture{db: db, service: service}
}

func (f *runtimeHTTPFixture) addActor(t *testing.T, tenantID int64, role, status string) *admin.AdminUser {
	t.Helper()
	actor := &admin.AdminUser{
		Base: model.Base{ID: uuid.New()}, TenantID: tenantID, Username: admin.NewInternalUsername(),
		PasswordHash: "test-only", Role: role, Status: status, TokenVersion: 1,
	}
	require.NoError(t, f.db.Create(actor).Error)
	return actor
}

func (f *runtimeHTTPFixture) request(method, path, body string, actor *admin.AdminUser, withAuth bool) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router := gin.New()
	if withAuth && actor != nil {
		router.Use(func(c *gin.Context) {
			c.Set(ctxkey.AdminID, actor.ID.String())
			c.Set(ctxkey.TenantID, actor.TenantID)
			c.Next()
		})
	}
	router.Use(middleware.RequestID())
	Register(router.Group(""), &Handler{Svc: f.service})
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func seedRuntimeStatus(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()
	items := []settings.PutItem{
		{TenantID: 0, GroupKey: groupKey, ItemKey: "platform_runtime_status", ItemValue: "normal", ValueType: "string"},
		{TenantID: 0, GroupKey: groupKey, ItemKey: "platform_runtime_status_reason", ItemValue: "previous reason", ValueType: "string"},
		{TenantID: 0, GroupKey: groupKey, ItemKey: "platform_runtime_status_changed_at", ItemValue: "2026-01-01T00:00:00Z", ValueType: "string"},
	}
	svc := &settings.Service{DB: db}
	require.NoError(t, svc.PutBulk(t.Context(), items))
	return map[string]string{
		"platform_runtime_status": "normal", "platform_runtime_status_reason": "previous reason",
		"platform_runtime_status_changed_at": "2026-01-01T00:00:00Z",
	}
}

func requireRuntimeStatus(t *testing.T, db *gorm.DB, want map[string]string) {
	t.Helper()
	var rows []settings.Setting
	require.NoError(t, db.Where("tenant_id = 0 AND group_key = ?", groupKey).Find(&rows).Error)
	got := make(map[string]string, len(rows))
	for _, row := range rows {
		got[row.ItemKey] = row.ItemValue
	}
	require.Equal(t, want, got)
}

func settingValue(rows []settings.Setting, key string) string {
	for _, row := range rows {
		if row.ItemKey == key {
			return row.ItemValue
		}
	}
	return fmt.Sprintf("missing setting %s in %s", key, jsonRows(rows))
}

func jsonRows(rows []settings.Setting) string {
	b, _ := json.Marshal(rows)
	return string(b)
}

func TestRuntimeRootBusinessWorkspaceCanReadAndMutatePlatformSettings(t *testing.T) {
	f := newRuntimeHTTPFixture(t, true)
	require.NoError(t, f.db.AutoMigrate(&admin.RootAccount{}, &admin.Tenant{}, &admin.TenantMembership{}))
	root := f.addActor(t, 0, adminperm.RoleAdmin, admin.StatusActive)
	require.NoError(t, f.db.Create(&admin.RootAccount{ID: 1, UserID: root.ID}).Error)
	require.NoError(t, f.db.Create(&admin.Tenant{ID: 33, Code: admin.RootWorkspaceCode, Name: admin.RootWorkspaceName, Status: admin.TenantActive}).Error)
	require.NoError(t, f.db.Create(&admin.TenantMembership{TenantID: 33, UserID: root.ID, Role: adminperm.RoleAdmin, Status: admin.MembershipActive}).Error)
	seedRuntimeStatus(t, f.db)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, root.ID.String())
		c.Set(ctxkey.TenantID, int64(33))
		c.Next()
	})
	router.Use(middleware.RequestID())
	Register(router.Group(""), &Handler{Svc: f.service})
	read := httptest.NewRecorder()
	router.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/platform/douyin/runtime-status", nil))
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	pause := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/platform/douyin/runtime-status/pause", strings.NewReader(`{"reason":"operator test"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(pause, req)
	require.Equal(t, http.StatusOK, pause.Code, pause.Body.String())
	var value settings.Setting
	require.NoError(t, f.db.Where("tenant_id = 0 AND group_key = ? AND item_key = ?", groupKey, "platform_runtime_status").First(&value).Error)
	require.Equal(t, "paused", value.ItemValue)
	require.NoError(t, f.db.Model(&admin.RootAccount{}).Where("id = ?", 1).Update("user_id", uuid.New()).Error)
	denied := httptest.NewRecorder()
	router.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/platform/douyin/runtime-status", nil))
	require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
}
