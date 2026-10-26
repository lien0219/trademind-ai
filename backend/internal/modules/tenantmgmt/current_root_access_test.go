package tenantmgmt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

// Switching an authenticated singleton Root into a newly provisioned active
// enterprise must permit GET /tenant/current without minting an ordinary
// tenant_memberships row for Root.
func TestRootSwitchCanReadAndUpdateNewEnterpriseWithoutMembership(t *testing.T) {
	svc, platformOperator := provisionFixture(t)
	require.NoError(t, svc.DB.AutoMigrate(&admin.RootAccount{}))

	newTenant := admin.Tenant{
		ID: 2, Code: "acme-cn", Name: "测试企业",
		Status: admin.TenantActive, SettingsJSON: "{}", Version: 1,
	}
	anotherTenant := admin.Tenant{
		ID: 3, Code: "another-cn", Name: "另一企业",
		Status: admin.TenantActive, SettingsJSON: "{}", Version: 1,
	}
	require.NoError(t, svc.DB.Create(&newTenant).Error)
	require.NoError(t, svc.DB.Create(&anotherTenant).Error)

	root := admin.AdminUser{
		TenantID: 0, Username: admin.NewInternalUsername(),
		Role: admin.RoleAdmin, Status: admin.StatusActive, PasswordHash: "test-only",
	}
	require.NoError(t, svc.DB.Create(&root).Error)
	require.NoError(t, svc.DB.Create(&admin.RootAccount{ID: 1, UserID: root.ID}).Error)

	member := admin.AdminUser{
		TenantID: newTenant.ID, Username: admin.NewInternalUsername(),
		Role: "readonly", Status: admin.StatusActive, PasswordHash: "test-only",
	}
	require.NoError(t, svc.DB.Create(&member).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{
		TenantID: newTenant.ID, UserID: member.ID, Role: "readonly",
		Status: admin.MembershipActive,
	}).Error)

	request := func(user uuid.UUID, tenantID int64, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set(ctxkey.AdminID, user.String())
			c.Set(ctxkey.TenantID, tenantID)
			c.Next()
		})
		Register(router.Group("/api/v1"), &Handler{Service: svc})
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		return out
	}

	var rootMemberships int64
	require.NoError(t, svc.DB.Model(&admin.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", newTenant.ID, root.ID).Count(&rootMemberships).Error)
	require.Zero(t, rootMemberships)

	// The persisted Root, not an ordinary platform operator, bypasses
	// membership requirements for the selected, active business tenant.
	get := request(root.ID, newTenant.ID, http.MethodGet, "/api/v1/tenant/current", "")
	require.Equal(t, http.StatusOK, get.Code, get.Body.String())
	var payload struct {
		Data TenantSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &payload))
	require.Equal(t, newTenant.ID, payload.Data.Tenant.ID)
	require.Equal(t, "acme-cn", payload.Data.Tenant.Code)

	operatorDenied := request(platformOperator.ID, newTenant.ID, http.MethodGet, "/api/v1/tenant/current", "")
	require.Equal(t, http.StatusForbidden, operatorDenied.Code, operatorDenied.Body.String())
	memberAllowed := request(member.ID, newTenant.ID, http.MethodGet, "/api/v1/tenant/current", "")
	require.Equal(t, http.StatusOK, memberAllowed.Code, memberAllowed.Body.String())
	crossTenantDenied := request(member.ID, anotherTenant.ID, http.MethodGet, "/api/v1/tenant/current", "")
	require.Equal(t, http.StatusForbidden, crossTenantDenied.Code, crossTenantDenied.Body.String())

	updated := request(root.ID, newTenant.ID, http.MethodPatch, "/api/v1/tenant/current",
		`{"name":"新企业资料","version":1}`)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	var saved, unchanged admin.Tenant
	require.NoError(t, svc.DB.First(&saved, "id = ?", newTenant.ID).Error)
	require.NoError(t, svc.DB.First(&unchanged, "id = ?", anotherTenant.ID).Error)
	require.Equal(t, "新企业资料", saved.Name)
	require.Equal(t, "另一企业", unchanged.Name)

	// Tenant suspension still fails closed for Root.
	require.NoError(t, svc.DB.Model(&admin.Tenant{}).
		Where("id = ?", newTenant.ID).Update("status", admin.TenantSuspended).Error)
	suspended := request(root.ID, newTenant.ID, http.MethodGet, "/api/v1/tenant/current", "")
	require.Equal(t, http.StatusForbidden, suspended.Code)
	require.NoError(t, svc.DB.Model(&admin.Tenant{}).
		Where("id = ?", newTenant.ID).Update("status", admin.TenantActive).Error)

	// Revoking the singleton binding immediately restores membership checks.
	require.NoError(t, svc.DB.Where("id = ?", 1).Delete(&admin.RootAccount{}).Error)
	noLongerRoot := request(root.ID, newTenant.ID, http.MethodGet, "/api/v1/tenant/current", "")
	require.Equal(t, http.StatusForbidden, noLongerRoot.Code)

	require.NoError(t, svc.DB.Model(&admin.TenantMembership{}).
		Where("tenant_id = ? AND user_id = ?", newTenant.ID, member.ID).
		Update("status", admin.MembershipRemoved).Error)
	removedMember := request(member.ID, newTenant.ID, http.MethodGet, "/api/v1/tenant/current", "")
	require.Equal(t, http.StatusForbidden, removedMember.Code)
}
