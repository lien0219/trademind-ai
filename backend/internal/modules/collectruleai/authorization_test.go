package collectruleai

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
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestCollectRuleAIRoutesRequireSettingsManage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}))
	user := admin.AdminUser{TenantID: 42, Username: admin.NewInternalUsername(), Role: adminperm.RoleReadonly, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, user.ID.String())
		c.Set(ctxkey.TenantID, user.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{Settings: &settings.Service{DB: db}}})

	for _, path := range []string{"/api/v1/collect/rules/ai-generate", "/api/v1/collect/rules/ai-generate-and-save"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"url":"https://example.test"}`))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		})
	}
}

func TestCollectRuleAIRoutesRejectNonRootAdministrators(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}))
	root := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	other := admin.AdminUser{TenantID: 43, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&other).Error)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		actor := root
		if c.GetHeader("X-Test-Tenant") == "43" {
			actor = other
		}
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Set(ctxkey.TenantID, actor.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{Settings: &settings.Service{DB: db}}})

	for _, path := range []string{"/api/v1/collect/rules/ai-generate", "/api/v1/collect/rules/ai-generate-and-save"} {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Test-Tenant", "43")
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
		})
	}
}
