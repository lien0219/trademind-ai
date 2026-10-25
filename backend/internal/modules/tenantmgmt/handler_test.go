package tenantmgmt

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func TestCurrentTenantSettingsHTTPRoundTrip(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{Code: "http-settings", Name: "Settings", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)
	require.NoError(t, svc.DB.Create(&admin.TenantMembership{TenantID: tenant.ID, UserID: actor.ID, Role: admin.RoleAdmin, Status: admin.MembershipActive}).Error)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Set(ctxkey.TenantID, tenant.ID)
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Service: svc})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tenant/current", strings.NewReader(`{"name":"Settings","settings":"{\"locale\":\"zh-CN\"}","version":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var first struct {
		Data admin.Tenant `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
	require.EqualValues(t, 2, first.Data.Version)
	require.Equal(t, "Settings", first.Data.Name)
	require.JSONEq(t, `{"locale":"zh-CN"}`, first.Data.SettingsJSON)

	secondBody, err := json.Marshal(map[string]any{
		"name":     "Settings v2",
		"settings": `{"locale":"en-US"}`,
		"version":  first.Data.Version,
	})
	require.NoError(t, err)
	secondReq := httptest.NewRequest(http.MethodPatch, "/api/v1/tenant/current", strings.NewReader(string(secondBody)))
	secondReq.Header.Set("Content-Type", "application/json")
	secondW := httptest.NewRecorder()
	router.ServeHTTP(secondW, secondReq)
	require.Equal(t, http.StatusOK, secondW.Code, secondW.Body.String())
	var second struct {
		Data admin.Tenant `json:"data"`
	}
	require.NoError(t, json.Unmarshal(secondW.Body.Bytes(), &second))
	require.EqualValues(t, 3, second.Data.Version)
	require.Equal(t, "Settings v2", second.Data.Name)
	require.JSONEq(t, `{"locale":"en-US"}`, second.Data.SettingsJSON)

	var stored admin.Tenant
	require.NoError(t, svc.DB.First(&stored, "id = ?", tenant.ID).Error)
	require.JSONEq(t, `{"locale":"en-US"}`, stored.SettingsJSON)
	require.EqualValues(t, 3, stored.Version)
	require.Equal(t, "Settings v2", stored.Name)
}

func TestPlatformTenantUpdateReturnsCommittedTenantForConsecutiveUpdates(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{Code: "platform-settings", Name: "Platform settings", Status: admin.TenantActive, SettingsJSON: "{}", Version: 1}
	require.NoError(t, svc.DB.Create(&tenant).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Service: svc})

	update := func(name, settings string, version int64) *httptest.ResponseRecorder {
		body, err := json.Marshal(map[string]any{"name": name, "settings": settings, "version": version})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/platform/tenants/"+strconv.FormatInt(tenant.ID, 10), strings.NewReader(
			string(body),
		))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}

	first := update("Platform settings v2", `{"region":"us"}`, 1)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	var firstResponse struct {
		Data admin.Tenant `json:"data"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstResponse))
	require.Equal(t, "Platform settings v2", firstResponse.Data.Name)
	require.JSONEq(t, `{"region":"us"}`, firstResponse.Data.SettingsJSON)
	require.EqualValues(t, 2, firstResponse.Data.Version)

	second := update("Platform settings v3", `{"region":"eu"}`, firstResponse.Data.Version)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	var secondResponse struct {
		Data admin.Tenant `json:"data"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondResponse))
	require.Equal(t, "Platform settings v3", secondResponse.Data.Name)
	require.JSONEq(t, `{"region":"eu"}`, secondResponse.Data.SettingsJSON)
	require.EqualValues(t, 3, secondResponse.Data.Version)
}

func TestSetStatusRejectsStaleVersion(t *testing.T) {
	svc, actor := provisionFixture(t)
	tenant := admin.Tenant{
		Code: "stale-status", Name: "Stale status", Status: admin.TenantActive,
		SettingsJSON: "{}", Version: 2,
	}
	require.NoError(t, svc.DB.Create(&tenant).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Service: svc})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/platform/tenants/"+strconv.FormatInt(tenant.ID, 10)+"/status", strings.NewReader(`{"status":"suspended","version":1}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	require.Equal(t, http.StatusConflict, res.Code)
	var stored admin.Tenant
	require.NoError(t, svc.DB.First(&stored, "id = ?", tenant.ID).Error)
	require.Equal(t, admin.TenantActive, stored.Status)
	require.EqualValues(t, 2, stored.Version)
}
