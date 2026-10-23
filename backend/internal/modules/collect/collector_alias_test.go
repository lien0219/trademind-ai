package collect

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
	"gorm.io/gorm"
)

func TestOpenLoginRequiresProductWriteOnCanonicalAndAliasRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &shop.Shop{}))
	user := admin.AdminUser{TenantID: 5, Username: admin.NewInternalUsername(), Role: adminperm.RoleReadonly, Status: admin.StatusActive}
	require.NoError(t, db.Create(&user).Error)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, user.ID.String())
		c.Set(ctxkey.TenantID, user.TenantID)
		c.Next()
	})
	h := &Handler{Svc: &Service{DB: db}}
	Register(r.Group("/api/v1"), h)
	RegisterAliases(r, h, db, func(c *gin.Context) { c.Next() })

	for _, path := range []string{
		"/api/v1/collector/providers/1688/open-login-browser",
		"/api/collector/providers/1688/open-login-browser",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, nil))
		require.Equal(t, http.StatusForbidden, w.Code, path+": "+w.Body.String())
	}
}
