package securitymod_test

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/securitymod"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func TestSingletonRootCanReadRotationFromProtectedBusinessWorkspace(t *testing.T) {
	db, svc, _, rootID := rotationHTTPFixture(t, 0, "admin", admin.StatusActive, true, true)
	require.NoError(t, db.AutoMigrate(&admin.RootAccount{}, &admin.Tenant{}, &admin.TenantMembership{}))
	require.NoError(t, db.Create(&admin.RootAccount{ID: 1, UserID: rootID}).Error)
	require.NoError(t, db.Create(&admin.Tenant{
		ID: 57, Code: admin.RootWorkspaceCode, Name: admin.RootWorkspaceName,
		Status: admin.TenantActive, Version: 1,
	}).Error)
	require.NoError(t, db.Create(&admin.TenantMembership{
		TenantID: 57, UserID: rootID, Role: "admin", Status: admin.MembershipActive,
	}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, rootID.String())
		c.Set(ctxkey.TenantID, int64(57))
		c.Next()
	})
	securitymod.RegisterRoutes(router.Group("/api/v1"), &securitymod.Handler{DB: db, Svc: svc})

	ok := rotationRequest(router, http.MethodGet, "/keys/rotation/status", "")
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
	require.NoError(t, db.Model(&admin.RootAccount{}).Where("id = ?", 1).Update("user_id", uuid.New()).Error)
	denied := rotationRequest(router, http.MethodGet, "/keys/rotation/status", "")
	require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
}
