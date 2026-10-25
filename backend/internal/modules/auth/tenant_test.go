package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestResolveLoginTenantRestoresLastActiveMembershipThenFallsBack(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth_tenant_login_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}))
	u := admin.AdminUser{Base: model.Base{ID: uuid.New()}, Username: "multi-tenant-user", PasswordHash: "test-only", Role: admin.RoleAdmin, Status: admin.StatusActive, TenantID: 1, LastTenantID: 22, TokenVersion: 1}
	require.NoError(t, db.Create(&u).Error)
	for _, tenant := range []admin.Tenant{{ID: 11, Code: "tenant-a", Name: "A", Status: admin.TenantActive}, {ID: 22, Code: "tenant-b", Name: "B", Status: admin.TenantActive}} {
		require.NoError(t, db.Create(&tenant).Error)
	}
	for _, membership := range []admin.TenantMembership{{TenantID: 11, UserID: u.ID, Role: "operator", Status: admin.MembershipActive}, {TenantID: 22, UserID: u.ID, Role: "readonly", Status: admin.MembershipActive}} {
		require.NoError(t, db.Create(&membership).Error)
	}
	tenantID, err := resolveLoginTenant(context.Background(), db, &u)
	require.NoError(t, err)
	require.EqualValues(t, 22, tenantID)
	require.NoError(t, db.Model(&admin.Tenant{}).Where("id = ?", 22).Update("status", admin.TenantSuspended).Error)
	tenantID, err = resolveLoginTenant(context.Background(), db, &u)
	require.NoError(t, err)
	require.EqualValues(t, 11, tenantID)
}

func TestSwitchTenantRevokesOldSessionAndIssuesTenantBoundPair(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth_tenant_switch_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}, &admin.AdminMutationLock{}, &AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &operationlog.OperationLog{}))
	uid := uuid.New()
	email := "switch-" + uuid.NewString() + "@example.test"
	password := "valid-password-123"
	passwordHashBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	passwordHash := string(passwordHashBytes)
	u := admin.AdminUser{Base: model.Base{ID: uid}, TenantID: 11, LastTenantID: 11, Username: "switch-" + uuid.NewString(), Email: email, PasswordHash: passwordHash, Role: "admin", Status: "active", TokenVersion: 3}
	require.NoError(t, db.Create(&u).Error)
	for _, tenant := range []admin.Tenant{{ID: 11, Code: "switch-a", Name: "A", Status: admin.TenantActive}, {ID: 22, Code: "switch-b", Name: "B", Status: admin.TenantActive}} {
		require.NoError(t, db.Create(&tenant).Error)
	}
	require.NoError(t, db.Create(&admin.TenantMembership{TenantID: 11, UserID: uid, Role: "admin", Status: admin.MembershipActive}).Error)
	require.NoError(t, db.Create(&admin.TenantMembership{TenantID: 22, UserID: uid, Role: "readonly", Status: admin.MembershipActive}).Error)
	cfg := &config.Config{JWTSecret: "switch-tenant-test-secret-at-least-thirty-two-bytes", Auth: config.AuthConfig{SessionMode: config.AuthSessionModeSecure, AccessTokenTTLMinutes: 15, RefreshTokenTTLDays: 7}}
	sessions := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}, OpLog: &operationlog.Service{DB: db}}
	old, err := sessions.CreateSession(context.Background(), email, password, "127.0.0.1", "test")
	require.NoError(t, err)
	oldClaims, err := ParseAccessToken(cfg, nil, old.AccessToken)
	require.NoError(t, err)
	result, err := sessions.SwitchTenant(context.Background(), uid, old.SessionID, oldClaims.TokenVersion, 22, "127.0.0.1", "test")
	require.NoError(t, err)
	newClaims, err := ParseAccessToken(cfg, nil, result.AccessToken)
	require.NoError(t, err)
	require.EqualValues(t, 22, newClaims.TenantID)
	require.NotEqual(t, old.SessionID, result.SessionID)
	require.Error(t, sessions.ValidateSessionAccess(context.Background(), old.SessionID, uid, oldClaims.TokenVersion))
	var oldSession AuthSession
	require.NoError(t, db.First(&oldSession, "id = ?", old.SessionID).Error)
	require.Equal(t, SessionStatusRevoked, oldSession.Status)
	var newRefresh AuthRefreshToken
	require.NoError(t, db.Where("session_id = ? AND status = ?", result.SessionID, RefreshStatusActive).First(&newRefresh).Error)
	require.EqualValues(t, 22, newRefresh.TenantID)
	var auditCount int64
	require.NoError(t, db.Model(&operationlog.OperationLog{}).Where("action = ? AND resource_id = ?", "auth.tenant.switch", result.SessionID.String()).Count(&auditCount).Error)
	require.EqualValues(t, 1, auditCount)
}

func TestSwitchTenantRejectsMissingMembershipWithoutRevokingCurrentSession(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth_tenant_switch_denied_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}, &admin.AdminMutationLock{}, &AuthSession{}, &AuthRefreshToken{}, &AuthLoginAttempt{}, &operationlog.OperationLog{}))
	uid := uuid.New()
	email := fmt.Sprintf("denied-%s@example.test", uuid.NewString())
	hashBytes, err := bcrypt.GenerateFromPassword([]byte("valid-password-123"), bcrypt.MinCost)
	require.NoError(t, err)
	hash := string(hashBytes)
	u := admin.AdminUser{Base: model.Base{ID: uid}, TenantID: 11, Username: "denied-" + uuid.NewString(), Email: email, PasswordHash: hash, Role: "admin", Status: "active", TokenVersion: 3}
	require.NoError(t, db.Create(&u).Error)
	for _, tenant := range []admin.Tenant{{ID: 11, Code: "denied-a", Name: "A", Status: admin.TenantActive}, {ID: 22, Code: "denied-b", Name: "B", Status: admin.TenantActive}} {
		require.NoError(t, db.Create(&tenant).Error)
	}
	require.NoError(t, db.Create(&admin.TenantMembership{TenantID: 11, UserID: uid, Role: "admin", Status: admin.MembershipActive}).Error)
	cfg := &config.Config{JWTSecret: "switch-tenant-test-secret-at-least-thirty-two-bytes", Auth: config.AuthConfig{SessionMode: config.AuthSessionModeSecure, AccessTokenTTLMinutes: 15, RefreshTokenTTLDays: 7}}
	sessions := &SessionService{Cfg: cfg, DB: db, Admins: &admin.Store{DB: db}, OpLog: &operationlog.Service{DB: db}}
	old, err := sessions.CreateSession(context.Background(), email, "valid-password-123", "127.0.0.1", "test")
	require.NoError(t, err)
	claims, err := ParseAccessToken(cfg, nil, old.AccessToken)
	require.NoError(t, err)
	_, err = sessions.SwitchTenant(context.Background(), uid, old.SessionID, claims.TokenVersion, 22, "127.0.0.1", "test")
	require.Error(t, err)
	require.NoError(t, sessions.ValidateSessionAccess(context.Background(), old.SessionID, uid, claims.TokenVersion))
}

func TestResolveLoginTenantDoesNotFallbackToLegacyTenantWhenMembershipsExist(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth_tenant_orphan_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}))
	u := admin.AdminUser{Base: model.Base{ID: uuid.New()}, Username: "orphaned-identity", PasswordHash: "test-only", Role: admin.RoleAdmin, Status: admin.StatusActive, TenantID: 11, TokenVersion: 1}
	require.NoError(t, db.Create(&u).Error)
	tenantID, err := resolveLoginTenant(context.Background(), db, &u)
	require.NoError(t, err)
	require.Zero(t, tenantID)
}

func TestTenantsListsOnlySwitchableActiveMemberships(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:auth_tenant_list_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}))

	uid := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{Base: model.Base{ID: uid}, TenantID: 11, Username: admin.NewInternalUsername(), Role: "readonly", Status: admin.StatusActive, PasswordHash: "test-only"}).Error)
	for _, tenant := range []admin.Tenant{
		{ID: 11, Code: "active-member", Name: "Active Member", Status: admin.TenantActive},
		{ID: 22, Code: "disabled-member", Name: "Disabled Member", Status: admin.TenantActive},
		{ID: 33, Code: "suspended-tenant", Name: "Suspended Tenant", Status: admin.TenantSuspended},
	} {
		require.NoError(t, db.Create(&tenant).Error)
	}
	for _, membership := range []admin.TenantMembership{
		{TenantID: 11, UserID: uid, Role: "admin", Status: admin.MembershipActive},
		{TenantID: 22, UserID: uid, Role: "operator", Status: admin.MembershipDisabled},
		{TenantID: 33, UserID: uid, Role: "admin", Status: admin.MembershipActive},
	} {
		require.NoError(t, db.Create(&membership).Error)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, uid.String())
		c.Next()
	})
	engine.GET("/auth/tenants", (&Handler{DB: db}).Tenants)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/tenants", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Data struct {
			Items []struct {
				ID int64 `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	var tenantIDs []int64
	for _, item := range envelope.Data.Items {
		tenantIDs = append(tenantIDs, item.ID)
	}
	require.Equal(t, []int64{11}, tenantIDs)
}
