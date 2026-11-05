package collectrule

import (
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
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type collectRuleTestShop struct {
	ID       uuid.UUID `gorm:"type:char(36);primaryKey"`
	TenantID int64     `gorm:"not null"`
}

func (collectRuleTestShop) TableName() string { return "shops" }

func TestCollectRuleRoutesRequireSettingsManage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &collectRuleTestShop{}, &CollectRule{}, &operationlog.OperationLog{}))
	user := admin.AdminUser{TenantID: 41, Username: admin.NewInternalUsername(), Role: adminperm.RoleReadonly, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, user.ID.String())
		c.Set(ctxkey.TenantID, user.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db, OpLog: &operationlog.Service{DB: db}}})

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/collect/rules", ""},
		{http.MethodPost, "/api/v1/collect/rules", `{"name":"R","domain":"example.test","rule":{"title":"h1"}}`},
		{http.MethodGet, "/api/v1/collect/rules/not-a-uuid", ""},
		{http.MethodPut, "/api/v1/collect/rules/not-a-uuid", `{}`},
		{http.MethodDelete, "/api/v1/collect/rules/not-a-uuid", ""},
		{http.MethodPost, "/api/v1/collect/rules/not-a-uuid/enable", ""},
		{http.MethodPost, "/api/v1/collect/rules/not-a-uuid/disable", ""},
		{http.MethodPost, "/api/v1/collect/rules/not-a-uuid/test", `{}`},
	}
	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		})
	}
}

func TestCollectRuleGlobalRoutesRejectNonRootAdministrators(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &collectRuleTestShop{}, &CollectRule{}, &operationlog.OperationLog{}))
	root := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	other := admin.AdminUser{TenantID: 42, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&other).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		actor := root
		if c.GetHeader("X-Test-Tenant") == "42" {
			actor = other
		}
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Set(ctxkey.TenantID, actor.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db, OpLog: &operationlog.Service{DB: db}}})

	rootList := httptest.NewRecorder()
	r.ServeHTTP(rootList, httptest.NewRequest(http.MethodGet, "/api/v1/collect/rules", nil))
	require.Equal(t, http.StatusOK, rootList.Code, rootList.Body.String())

	for _, request := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/v1/collect/rules", ""},
		{http.MethodPost, "/api/v1/collect/rules", `{}`},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(request.method, request.path, strings.NewReader(request.body))
		req.Header.Set("X-Test-Tenant", "42")
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	}
}

func TestCollectRuleCreateRollsBackWhenAuditFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &collectRuleTestShop{}, &CollectRule{}, &operationlog.OperationLog{}))
	root := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_operation_log BEFORE INSERT ON operation_logs BEGIN SELECT RAISE(ABORT, 'sqlite rejected operation log insert'); END`).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, root.ID.String())
		c.Set(ctxkey.TenantID, root.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db, OpLog: &operationlog.Service{DB: db}}})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/collect/rules", strings.NewReader(`{"name":"audit-test","domain":"example.test","status":"disabled","rule":{"title":"h1"}}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "审计记录暂时无法保存")
	require.NotContains(t, w.Body.String(), "sqlite rejected operation log insert")
	var count int64
	require.NoError(t, db.Model(&CollectRule{}).Count(&count).Error)
	require.Zero(t, count, "rule must roll back when its audit evidence cannot be persisted")
}

func TestCollectRuleMutationsRollbackWhenAuditFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &collectRuleTestShop{}, &CollectRule{}, &operationlog.OperationLog{}))
	root := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&root).Error)
	rule := CollectRule{
		Name:   "before",
		Source: SourceCustom,
		Domain: "example.test",
		Status: StatusEnabled,
		Rule:   datatypes.JSON(`{"title":"h1","mainImages":"img"}`),
	}
	require.NoError(t, db.Create(&rule).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_operation_log BEFORE INSERT ON operation_logs BEGIN SELECT RAISE(ABORT, 'sqlite rejected operation log insert'); END`).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, root.ID.String())
		c.Set(ctxkey.TenantID, root.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: db, OpLog: &operationlog.Service{DB: db}}})

	requests := []struct {
		name   string
		method string
		suffix string
		body   string
	}{
		{name: "update", method: http.MethodPut, body: `{"name":"after"}`},
		{name: "delete", method: http.MethodDelete},
		{name: "disable", method: http.MethodPost, suffix: "/disable"},
	}
	for _, request := range requests {
		t.Run(request.name, func(t *testing.T) {
			currentRule := CollectRule{
				Name:   "before",
				Source: SourceCustom,
				Domain: "example.test",
				Status: StatusEnabled,
				Rule:   datatypes.JSON(`{"title":"h1","mainImages":"img"}`),
			}
			require.NoError(t, db.Create(&currentRule).Error)
			w := httptest.NewRecorder()
			path := "/api/v1/collect/rules/" + currentRule.ID.String() + request.suffix
			req := httptest.NewRequest(request.method, path, strings.NewReader(request.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), "审计记录暂时无法保存")
			require.NotContains(t, w.Body.String(), "sqlite rejected operation log insert")
			var persisted CollectRule
			require.NoError(t, db.Unscoped().First(&persisted, "id = ?", currentRule.ID).Error)
			require.False(t, persisted.DeletedAt.Valid, "failed audit must roll back a soft delete")
			require.Equal(t, "before", persisted.Name, "failed audit must roll back rule updates")
			require.Equal(t, StatusEnabled, persisted.Status, "failed audit must roll back status changes")
		})
	}
}
