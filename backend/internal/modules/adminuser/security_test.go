package adminuser

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRoleMutationRollsBackWhenAuditFails(t *testing.T) {
	for _, create := range []bool{true, false} {
		t.Run(map[bool]string{true: "create", false: "update"}[create], func(t *testing.T) {
			svc, actor, target := securityFixture(t)
			role := admin.RoleDefinition{TenantID: actor.TenantID, Code: "r_0123456789abcdef01234567", Name: "Test role", PermissionsJSON: "[\"product.view\"]", Enabled: true}
			require.NoError(t, svc.DB.Create(&role).Error)
			require.NoError(t, svc.DB.Model(&target).Update("role", role.Code).Error)
			failAuditInsert(t, svc.DB)
			method, path, body := http.MethodPost, "/api/v1/admin/roles", map[string]any{"name": "New role", "permissions": []string{"product.view"}}
			if !create {
				method, path, body = http.MethodPatch, "/api/v1/admin/roles/"+role.Code, map[string]any{"enabled": false, "permissions": []string{}}
			}
			w := roleRequest(t, svc, actor, method, path, body)
			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), "test audit persistence failure", "database errors must be sanitized")
			var current admin.RoleDefinition
			require.NoError(t, svc.DB.First(&current, "id = ?", role.ID).Error)
			require.True(t, current.Enabled)
			require.Equal(t, role.PermissionsJSON, current.PermissionsJSON)
			var user admin.AdminUser
			require.NoError(t, svc.DB.First(&user, "id = ?", target.ID).Error)
			require.Equal(t, target.TokenVersion, user.TokenVersion)
			var count int64
			require.NoError(t, svc.DB.Model(&admin.RoleDefinition{}).Count(&count).Error)
			require.EqualValues(t, 1, count)
		})
	}
}

func securityFixture(t *testing.T) (*Service, admin.AdminUser, admin.AdminUser) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.AdminMutationLock{}, &admin.RoleDefinition{}, &admin.UserStorePermission{}, &shop.Shop{}, &operationlog.OperationLog{}))
	actor := admin.AdminUser{TenantID: 17, Username: admin.NewInternalUsername(), Email: "admin@example.test", Role: adminperm.RoleAdmin, Status: admin.StatusActive, PasswordHash: "test-only"}
	target := admin.AdminUser{TenantID: 17, Username: admin.NewInternalUsername(), Email: "operator@example.test", Role: adminperm.RoleOperator, Status: admin.StatusActive, PasswordHash: "original-hash"}
	require.NoError(t, db.Create(&actor).Error)
	require.NoError(t, db.Create(&target).Error)
	return &Service{DB: db, OpLog: &operationlog.Service{DB: db}}, actor, target
}

func securityContext(actor admin.AdminUser) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users", nil)
	c.Set(ctxkey.AdminID, actor.ID.String())
	c.Set(ctxkey.TenantID, actor.TenantID)
	c.Set(ctxkey.TraceID, "test-rbac-audit")
	return c
}

func failAuditInsert(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:fail_audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "operation_logs" {
			tx.AddError(errors.New("test audit persistence failure"))
		}
	}))
}

func TestSensitiveUserMutationRollsBackWhenAuditFails(t *testing.T) {
	for _, action := range []string{"create", "update", "stores", "reset"} {
		t.Run(action, func(t *testing.T) {
			svc, actor, target := securityFixture(t)
			failAuditInsert(t, svc.DB)
			c := securityContext(actor)
			var err error
			switch action {
			case "create":
				_, err = svc.Create(c, CreateBody{Email: "new@example.test", Password: "test-password-123", Role: adminperm.RoleReadonly}, &actor.ID)
			case "update":
				role := adminperm.RoleReadonly
				_, err = svc.Update(c, target.ID, UpdateBody{Role: &role}, &actor.ID)
			case "stores":
				_, err = svc.SetStorePermissions(c, target.ID, SetStorePermissionsBody{}, &actor.ID)
			case "reset":
				var secret string
				secret, err = svc.ResetPassword(c, target.ID, &actor.ID)
				require.Empty(t, secret, "failed reset must not disclose an uncommitted credential")
			}
			require.Error(t, err, "sensitive mutation must fail closed if its audit cannot persist")
			var current admin.AdminUser
			require.NoError(t, svc.DB.First(&current, "id = ?", target.ID).Error)
			require.Equal(t, target.Role, current.Role)
			require.Equal(t, target.PasswordHash, current.PasswordHash)
			require.Equal(t, target.TokenVersion, current.TokenVersion)
			var count int64
			require.NoError(t, svc.DB.Model(&admin.AdminUser{}).Count(&count).Error)
			require.EqualValues(t, 2, count)
		})
	}
}

func TestAdminMutationLocksPersistTenantAndGlobalIdentityScopes(t *testing.T) {
	svc, actor, _ := securityFixture(t)
	tx := svc.DB.Begin()
	require.NoError(t, tx.Error)
	require.NoError(t, lockTenantAdminMutations(tx, actor.TenantID))
	require.NoError(t, lockTenantAdminMutations(tx, actor.TenantID))
	require.NoError(t, lockTenantAdminMutations(tx, actor.TenantID+1))
	require.NoError(t, lockAccountIdentity(tx, "email:one@example.test"))
	require.NoError(t, lockAccountIdentity(tx, "phone:+15555550100"))
	require.NoError(t, tx.Commit().Error)

	var count int64
	require.NoError(t, svc.DB.Table("admin_mutation_locks").Count(&count).Error)
	require.EqualValues(t, 3, count, "same-tenant and account-registration writes share stable lock rows")
}

func roleRequest(t *testing.T, svc *Service, actor admin.AdminUser, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Set(ctxkey.TenantID, actor.TenantID)
		c.Set(ctxkey.TraceID, "test-rbac-role-audit")
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: svc})
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCreateDisabledRoleRemainsUnassignable(t *testing.T) {
	svc, actor, _ := securityFixture(t)
	w := roleRequest(t, svc, actor, http.MethodPost, "/api/v1/admin/roles", map[string]any{"name": "Disabled role", "enabled": false, "permissions": []string{"product.view"}})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var result struct {
		Data RoleRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	require.False(t, result.Data.Enabled)
	var stored admin.RoleDefinition
	require.NoError(t, svc.DB.First(&stored, "code = ?", result.Data.Code).Error)
	require.False(t, stored.Enabled)
	_, err := svc.requireAssignableRole(securityContext(actor), actor.TenantID, stored.Code)
	require.Error(t, err)
}

func TestUserMutationsSanitizeDatabaseFailures(t *testing.T) {
	for _, action := range []string{"create", "stores", "reset"} {
		t.Run(action, func(t *testing.T) {
			svc, actor, target := securityFixture(t)
			injected := errors.New("private-database-connection-detail")
			require.NoError(t, svc.DB.Callback().Update().Before("gorm:update").Register("test:fail_update", func(tx *gorm.DB) { tx.AddError(injected) }))
			require.NoError(t, svc.DB.Callback().Create().Before("gorm:create").Register("test:fail_create", func(tx *gorm.DB) { tx.AddError(injected) }))
			method, path, body := http.MethodPost, "/api/v1/admin/users", any(CreateBody{Email: "new@example.test", Password: "test-password-123", Role: "readonly"})
			if action == "stores" {
				method, path, body = http.MethodPut, "/api/v1/admin/users/"+target.ID.String()+"/store-permissions", SetStorePermissionsBody{}
			}
			if action == "reset" {
				path, body = "/api/v1/admin/users/"+target.ID.String()+"/reset-password", map[string]any{}
			}
			w := roleRequest(t, svc, actor, method, path, body)
			require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), injected.Error())
		})
	}
}

func TestCreateUserRejectsMalformedAccountAndWeakPassword(t *testing.T) {
	for _, body := range []CreateBody{
		{Email: "not-an-account", Password: "test-password-123", Role: "operator"},
		{Email: "new@example.test", Password: "short1", Role: "operator"},
	} {
		svc, actor, _ := securityFixture(t)
		w := roleRequest(t, svc, actor, http.MethodPost, "/api/v1/admin/users", body)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		var count int64
		require.NoError(t, svc.DB.Model(&admin.AdminUser{}).Count(&count).Error)
		require.EqualValues(t, 2, count)
	}
}

func TestNormalizeUnknownUserStatusFailsClosed(t *testing.T) {
	for _, status := range []string{"", "suspended", "pending", "ACTIVE?"} {
		require.Equal(t, admin.StatusDisabled, normalizeStatus(status), "unknown status %q must never display as active", status)
	}
	require.Equal(t, admin.StatusActive, normalizeStatus("active"))
	require.Equal(t, admin.StatusDisabled, normalizeStatus("inactive"))
}

func TestRejectedRBACMutationsHaveSanitizedAuditEvidence(t *testing.T) {
	for _, tt := range []struct {
		action, method, suffix string
	}{
		{"user.create", http.MethodPost, "/admin/users"},
		{"user.update", http.MethodPatch, "/admin/users/%s"},
		{"user.store_permissions.set", http.MethodPut, "/admin/users/%s/store-permissions"},
		{"user.password.reset", http.MethodPost, "/admin/users/%s/reset-password"},
		{"role.create", http.MethodPost, "/admin/roles"},
		{"role.update", http.MethodPatch, "/admin/roles/r_0123456789abcdef01234567"},
	} {
		t.Run(tt.action, func(t *testing.T) {
			svc, _, operator := securityFixture(t)
			path := tt.suffix
			if strings.Contains(path, "%s") {
				path = fmt.Sprintf(path, operator.ID)
			}
			w := roleRequest(t, svc, operator, tt.method, "/api/v1"+path, map[string]string{"password": "test-never-log-password", "email": "private@example.test"})
			require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
			var logs []operationlog.OperationLog
			require.NoError(t, svc.DB.Where("action = ?", tt.action).Find(&logs).Error)
			require.Len(t, logs, 1)
			require.Equal(t, "denied", logs[0].Status)
			require.Equal(t, operator.TenantID, logs[0].TenantID)
			require.Equal(t, operator.ID, *logs[0].AdminUserID)
			require.Equal(t, "test-rbac-role-audit", logs[0].RequestID)
			require.NotContains(t, logs[0].Message, "password")
			require.NotContains(t, logs[0].Message, "private@example.test")
		})
	}
}

func TestFailedRBACMutationReturnsErrorIfFailureAuditIsUnavailable(t *testing.T) {
	svc, actor, _ := securityFixture(t)
	failAuditInsert(t, svc.DB)
	w := roleRequest(t, svc, actor, http.MethodPost, "/api/v1/admin/users", map[string]string{"password": "test-never-log-password"})
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "test audit persistence failure")
	var count int64
	require.NoError(t, svc.DB.Model(&admin.AdminUser{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}

func TestUserSecurityAuditRetainsPreviousAndUpdatedState(t *testing.T) {
	svc, actor, target := securityFixture(t)
	nextRole, nextStatus := adminperm.RoleReadonly, admin.StatusDisabled
	_, err := svc.Update(securityContext(actor), target.ID, UpdateBody{Role: &nextRole, Status: &nextStatus}, &actor.ID)
	require.NoError(t, err)
	var log operationlog.OperationLog
	require.NoError(t, svc.DB.Where("action = ?", "user.update").First(&log).Error)
	var states map[string]map[string]string
	require.NoError(t, json.Unmarshal([]byte(log.Message), &states))
	require.Equal(t, target.Role, states["before"]["role"])
	require.Equal(t, target.Status, states["before"]["status"])
	require.Equal(t, nextRole, states["after"]["role"])
	require.Equal(t, nextStatus, states["after"]["status"])
	require.NotContains(t, log.Message, target.PasswordHash)
}

func TestPermissionAuditRetainsCompleteBeforeAndAfter(t *testing.T) {
	svc, actor, target := securityFixture(t)
	grants := adminperm.DelegablePermissions()
	w := roleRequest(t, svc, actor, http.MethodPost, "/api/v1/admin/roles", map[string]any{"name": "All delegable", "permissions": grants})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var roleResult struct {
		Data RoleRow `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &roleResult))
	w = roleRequest(t, svc, actor, http.MethodPatch, "/api/v1/admin/roles/"+roleResult.Data.Code, map[string]any{"permissions": []string{}, "enabled": false})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var log operationlog.OperationLog
	require.NoError(t, svc.DB.Where("action = ?", "role.update").First(&log).Error)
	var change struct {
		Before struct {
			Permissions []string `json:"permissions"`
			Enabled     bool     `json:"enabled"`
		} `json:"before"`
		After struct {
			Permissions []string `json:"permissions"`
			Enabled     bool     `json:"enabled"`
		} `json:"after"`
	}
	require.NoError(t, json.Unmarshal([]byte(log.Message), &change), "audit details must not be truncated")
	require.ElementsMatch(t, grants, change.Before.Permissions)
	require.True(t, change.Before.Enabled)
	require.Empty(t, change.After.Permissions)
	require.False(t, change.After.Enabled)

	store := shop.Shop{TenantID: actor.TenantID, Platform: "shopee", ShopName: "test shop"}
	require.NoError(t, svc.DB.Create(&store).Error)
	_, err := svc.SetStorePermissions(securityContext(actor), target.ID, SetStorePermissionsBody{Items: []StorePermInput{{StoreID: store.ID.String(), PermissionScope: "view"}}}, &actor.ID)
	require.NoError(t, err)
	_, err = svc.SetStorePermissions(securityContext(actor), target.ID, SetStorePermissionsBody{}, &actor.ID)
	require.NoError(t, err)
	var storeLogs []operationlog.OperationLog
	require.NoError(t, svc.DB.Where("action = ?", "user.store_permissions.set").Order("created_at ASC").Find(&storeLogs).Error)
	require.Len(t, storeLogs, 2)
	var storeChange struct {
		Before []StorePermInput `json:"before"`
		After  []StorePermInput `json:"after"`
	}
	require.NoError(t, json.Unmarshal([]byte(storeLogs[1].Message), &storeChange))
	require.Equal(t, []StorePermInput{{StoreID: store.ID.String(), Platform: "shopee", PermissionScope: "view"}}, storeChange.Before)
	require.Empty(t, storeChange.After)
}
