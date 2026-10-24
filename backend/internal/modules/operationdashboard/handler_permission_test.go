package operationdashboard

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
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestDashboardHandlersRequireProductView(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:operation_dashboard_permission_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}))
	actorID := uuid.New()
	roleCode := "r_0123456789abcdef01234567"
	require.NoError(t, db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: 41, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: roleCode, Status: admin.StatusActive,
	}).Error)
	require.NoError(t, db.Create(&admin.RoleDefinition{
		TenantID: 41, Code: roleCode, Name: "No dashboard access", PermissionsJSON: `[]`, Enabled: true,
	}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, int64(41))
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})

	for _, path := range []string{
		"/api/v1/dashboard/product-operations",
		"/api/v1/dashboard/overview",
		"/api/v1/dashboard/todos",
		"/api/v1/dashboard/health",
	} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		})
	}
}
