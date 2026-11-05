package taskcenter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestTaskCenterRoutesRejectPrincipalWithoutPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_permission_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}))

	actorID := uuid.New()
	roleCode := "r_0123456789abcdef01234567"
	require.NoError(t, db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: 41, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: roleCode, Status: admin.StatusActive,
	}).Error)
	require.NoError(t, db.Create(&admin.RoleDefinition{
		TenantID: 41, Code: roleCode, Name: "No task center access", PermissionsJSON: `[]`, Enabled: true,
	}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(41))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	id := uuid.NewString()
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/failure-categories"},
		{http.MethodGet, "/alert-notifications"},
		{http.MethodGet, "/alerts"},
		{http.MethodPost, "/alerts/scan"},
		{http.MethodPost, "/alerts/" + id + "/notify"},
		{http.MethodPost, "/alerts/" + id + "/handle"},
		{http.MethodPost, "/alerts/" + id + "/ignore"},
		{http.MethodDelete, "/alerts/" + id + "/mark"},
		{http.MethodGet, "/failures"},
		{http.MethodGet, "/summary"},
		{http.MethodPost, "/failures/batch-retry"},
		{http.MethodPost, "/failures/batch-ignore"},
		{http.MethodPost, "/failures/batch-handle"},
		{http.MethodGet, "/failures/collect/" + id},
		{http.MethodPost, "/failures/collect/" + id + "/generate-alert"},
		{http.MethodPost, "/failures/collect/" + id + "/retry"},
		{http.MethodPost, "/failures/collect/" + id + "/ignore"},
		{http.MethodPost, "/failures/collect/" + id + "/handle"},
		{http.MethodDelete, "/failures/collect/" + id + "/mark"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/api/v1/task-center"+tc.path, nil)
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		})
	}
}

func TestTaskCenterRequiresTaskRetryPermissionBeforeBindingRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_permission_gate_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}))

	for _, tc := range []struct {
		name        string
		role        string
		permissions string
		status      string
		want        int
	}{
		{name: "admin", role: adminperm.RoleAdmin, status: admin.StatusActive, want: http.StatusOK},
		{name: "operator", role: adminperm.RoleOperator, status: admin.StatusActive, want: http.StatusOK},
		{name: "readonly", role: adminperm.RoleReadonly, status: admin.StatusActive, want: http.StatusForbidden},
		{name: "disabled user", role: adminperm.RoleAdmin, status: admin.StatusDisabled, want: http.StatusForbidden},
		{name: "custom task permission", role: "r_1123456789abcdef01234567", permissions: `["task.retry"]`, status: admin.StatusActive, want: http.StatusOK},
		{name: "custom without task permission", role: "r_2123456789abcdef01234567", permissions: `[]`, status: admin.StatusActive, want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actorID := uuid.New()
			require.NoError(t, db.Create(&admin.AdminUser{
				Base: model.Base{ID: actorID}, TenantID: 41, Username: admin.NewInternalUsername(),
				PasswordHash: "test", Role: tc.role, Status: tc.status,
			}).Error)
			if adminperm.IsCustomRole(tc.role) {
				require.NoError(t, db.Create(&admin.RoleDefinition{
					TenantID: 41, Code: tc.role, Name: tc.name, PermissionsJSON: tc.permissions, Enabled: true,
				}).Error)
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(ctxkey.AdminID, actorID.String())
				c.Set(ctxkey.TenantID, int64(41))
				c.Next()
			})
			Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/task-center/failure-categories", nil))
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
		})
	}
}

func TestTaskCenterAlertReadsAcceptAlertsReadWithoutTaskRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:taskcenter_alert_read_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}))

	for _, tc := range []struct {
		name string
		role string
	}{
		{name: "readonly", role: adminperm.RoleReadonly},
		{name: "custom alerts reader", role: "r_3123456789abcdef01234567"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actorID := uuid.New()
			require.NoError(t, db.Create(&admin.AdminUser{
				Base: model.Base{ID: actorID}, TenantID: 41, Username: admin.NewInternalUsername(),
				PasswordHash: "test", Role: tc.role, Status: admin.StatusActive,
			}).Error)
			if adminperm.IsCustomRole(tc.role) {
				require.NoError(t, db.Create(&admin.RoleDefinition{
					TenantID: 41, Code: tc.role, Name: tc.name, PermissionsJSON: `["alerts.read"]`, Enabled: true,
				}).Error)
			}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(ctxkey.AdminID, actorID.String())
				c.Set(ctxkey.TenantID, int64(41))
				c.Next()
			})
			Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/task-center/alerts", nil))
			require.NotEqual(t, http.StatusForbidden, rec.Code, rec.Body.String())
		})
	}
}
