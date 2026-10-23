package settings

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestPlatformSettingsAreRootTenantScoped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:settings_security?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Setting{}, &admin.AdminUser{}))
	root := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	other := admin.AdminUser{TenantID: 12, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, db.Create(&Setting{TenantID: 0, GroupKey: "test", ItemKey: "root", ItemValue: "root-value"}).Error)
	require.NoError(t, db.Create(&Setting{TenantID: 12, GroupKey: "test", ItemKey: "other", ItemValue: "other-value"}).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		id := root.ID.String()
		tenantID := int64(0)
		if c.GetHeader("X-Test-Tenant") == "12" {
			id = other.ID.String()
			tenantID = 12
		}
		c.Set(ctxkey.AdminID, id)
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	h := &Handler{Svc: &Service{DB: db}, DB: db}
	r.GET("/settings", h.List)
	r.PUT("/settings", h.Put)

	rootList := httptest.NewRecorder()
	r.ServeHTTP(rootList, httptest.NewRequest(http.MethodGet, "/settings", nil))
	require.Equal(t, http.StatusOK, rootList.Code, rootList.Body.String())
	require.Contains(t, rootList.Body.String(), "root")
	require.NotContains(t, rootList.Body.String(), "other")

	spoof := httptest.NewRecorder()
	body, err := json.Marshal(map[string]any{"items": []map[string]any{{
		"tenantId": 12, "groupKey": "test", "itemKey": "root", "itemValue": "spoofed",
	}}})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(spoof, req)
	require.Equal(t, http.StatusForbidden, spoof.Code, spoof.Body.String())

	otherList := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/settings", nil)
	req.Header.Set("X-Test-Tenant", "12")
	r.ServeHTTP(otherList, req)
	require.Equal(t, http.StatusForbidden, otherList.Code, otherList.Body.String())

	var row Setting
	require.NoError(t, db.First(&row, "tenant_id = ? AND group_key = ? AND item_key = ?", 0, "test", "root").Error)
	require.Equal(t, "root-value", row.ItemValue)
}

func TestSettingsMutationRollsBackWhenAuditInsertFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:settings_audit_atomic?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Setting{}, &admin.AdminUser{}, &operationlog.OperationLog{}))
	root := admin.AdminUser{TenantID: 0, Username: admin.NewInternalUsername(), Role: adminperm.RoleAdmin, Status: admin.StatusActive}
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, db.Create(&Setting{TenantID: 0, GroupKey: "test", ItemKey: "atomic", ItemValue: "before"}).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_operation_log BEFORE INSERT ON operation_logs BEGIN SELECT RAISE(ABORT, 'audit unavailable'); END`).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, root.ID.String())
		c.Set(ctxkey.TenantID, int64(0))
		c.Next()
	})
	h := &Handler{
		Svc:   &Service{DB: db},
		OpLog: &operationlog.Service{DB: db},
		DB:    db,
	}
	r.PUT("/settings", h.Put)

	body, err := json.Marshal(map[string]any{"items": []map[string]any{{
		"groupKey": "test", "itemKey": "atomic", "itemValue": "after",
	}}})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	require.Equal(t, http.StatusInternalServerError, res.Code, res.Body.String())
	var setting Setting
	require.NoError(t, db.First(&setting, "tenant_id = ? AND group_key = ? AND item_key = ?", 0, "test", "atomic").Error)
	require.Equal(t, "before", setting.ItemValue)
	var auditCount int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Count(&auditCount).Error)
	require.Zero(t, auditCount)
}
