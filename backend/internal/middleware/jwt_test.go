package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func TestBearerAuthProductionTenantRecoveryAndPlatformRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:jwt_tenant_recovery_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}, &admin.PlatformOperator{}, &auth.AuthSession{}))
	user := admin.AdminUser{Username: admin.NewInternalUsername(), PasswordHash: "unused", Role: admin.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&admin.Tenant{ID: 11, Code: "active-tenant", Name: "Active", Status: admin.TenantActive}).Error)
	require.NoError(t, db.Create(&admin.TenantMembership{TenantID: 11, UserID: user.ID, Role: admin.RoleAdmin, Status: admin.MembershipActive}).Error)
	cfg := &config.Config{AppEnv: config.EnvProduction, JWTSecret: "production-recovery-test-signing-secret-32-bytes"}
	for _, tc := range []struct {
		name, method, path string
		tenantID           int64
		operator           bool
		want               int
	}{
		{"recovery profile", "GET", "/api/v1/auth/profile", 0, false, 204},
		{"recovery memberships", "GET", "/api/v1/auth/tenants", 0, false, 204},
		{"recovery switch", "POST", "/api/v1/auth/tenants/switch", 0, false, 204},
		{"recovery invitation", "POST", "/api/v1/tenant/invitations/accept", 0, false, 204},
		{"recovery cannot access business", "GET", "/api/v1/products", 0, false, 403},
		{"recovery must match exact path", "GET", "/api/v1/auth/profile/extra", 0, false, 403},
		{"recovery must match method", "PATCH", "/api/v1/auth/profile", 0, false, 403},
		{"tenant platform provider catalog", "GET", "/api/v1/platform/providers", 11, false, 204},
		{"tenant douyin categories", "GET", "/api/v1/platform/douyin/categories", 11, false, 204},
		{"tenant cannot enter control plane", "GET", "/api/v1/platform/tenants", 11, false, 403},
		{"operator without membership", "GET", "/api/v1/platform/tenants", 0, true, 204},
		{"operator cannot use tenant business without membership", "GET", "/api/v1/platform/providers", 0, true, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, db.Where("user_id = ?", user.ID).Delete(&admin.PlatformOperator{}).Error)
			if tc.operator {
				require.NoError(t, db.Create(&admin.PlatformOperator{UserID: user.ID, Enabled: true}).Error)
			}
			sess := auth.AuthSession{TenantID: tc.tenantID, UserID: user.ID, TokenVersion: user.TokenVersion, Status: auth.SessionStatusActive}
			require.NoError(t, db.Create(&sess).Error)
			claims := auth.AccessClaims{TokenType: "access", TenantID: tc.tenantID, SessionID: sess.ID.String(), TokenVersion: user.TokenVersion, RegisteredClaims: jwt.RegisteredClaims{Subject: user.ID.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
			require.NoError(t, err)
			router := gin.New()
			router.Use(BearerAuthWithDB(cfg, db, &auth.SessionService{DB: db, Cfg: cfg}))
			router.Handle(tc.method, tc.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

func TestBearerAuthRejectsMalformedSessionIDButAcceptsLegacyToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:jwt_malformed_session_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	user := admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     0,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "unused-test-hash",
		Role:         admin.RoleAdmin,
		Status:       admin.StatusActive,
		TokenVersion: 9,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{JWTSecret: "middleware-regression-secret-with-at-least-32-bytes"}
	for _, tc := range []struct {
		name      string
		sessionID string
		wantCode  int
	}{
		{name: "legacy token without session id", sessionID: "", wantCode: http.StatusNoContent},
		{name: "legacy token with nil session id", sessionID: uuid.Nil.String(), wantCode: http.StatusNoContent},
		{name: "malformed session id", sessionID: "not-a-uuid", wantCode: http.StatusUnauthorized},
		{name: "session-bound token without session validator", sessionID: uuid.NewString(), wantCode: http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := auth.AccessClaims{
				Username:     "legacy-admin",
				TokenType:    "access",
				TenantID:     user.TenantID,
				SessionID:    tc.sessionID,
				TokenVersion: user.TokenVersion,
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   user.ID.String(),
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
				},
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.Use(BearerAuthWithDB(cfg, db, nil))
			router.GET("/protected", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantCode, recorder.Body.String())
			}
		})
	}
}

func TestBearerAuthDeniesSuspendedTenantBusinessButAllowsRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:jwt_suspended_tenant_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}, &admin.PlatformOperator{}); err != nil {
		t.Fatal(err)
	}
	user := admin.AdminUser{Base: model.Base{ID: uuid.New()}, TenantID: 11, Username: admin.NewInternalUsername(), PasswordHash: "unused-test-hash", Role: admin.RoleAdmin, Status: admin.StatusActive, TokenVersion: 9}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.Tenant{ID: 11, Code: "suspended", Name: "Suspended", Status: admin.TenantSuspended}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&admin.TenantMembership{TenantID: 11, UserID: user.ID, Role: admin.RoleAdmin, Status: admin.MembershipActive}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{JWTSecret: "middleware-regression-secret-with-at-least-32-bytes"}
	claims := auth.AccessClaims{Username: "suspended-user", TokenType: "access", TenantID: 11, TokenVersion: user.TokenVersion, RegisteredClaims: jwt.RegisteredClaims{Subject: user.ID.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path     string
		wantCode int
	}{{path: "/api/v1/products", wantCode: http.StatusForbidden}, {path: "/api/v1/auth/profile", wantCode: http.StatusNoContent}} {
		t.Run(tc.path, func(t *testing.T) {
			router := gin.New()
			router.Use(BearerAuthWithDB(cfg, db, nil))
			router.Any(tc.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantCode, recorder.Body.String())
			}
		})
	}
}

func TestBearerAuthFailsClosedWhenTenantSchemaIsMissingInProduction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:jwt_missing_tenant_schema_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}); err != nil {
		t.Fatal(err)
	}
	user := admin.AdminUser{Base: model.Base{ID: uuid.New()}, TenantID: 11, Username: admin.NewInternalUsername(), PasswordHash: "unused-test-hash", Role: admin.RoleAdmin, Status: admin.StatusActive, TokenVersion: 9}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{AppEnv: config.EnvProduction, JWTSecret: "middleware-regression-secret-with-at-least-32-bytes"}
	claims := auth.AccessClaims{Username: "tenant-admin", TokenType: "access", TenantID: 11, TokenVersion: user.TokenVersion, RegisteredClaims: jwt.RegisteredClaims{Subject: user.ID.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(BearerAuthWithDB(cfg, db, nil))
	router.GET("/api/v1/products", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/api/v1/auth/profile", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, tc := range []struct {
		path     string
		wantCode int
	}{
		{path: "/api/v1/products", wantCode: http.StatusForbidden},
		{path: "/api/v1/auth/profile", wantCode: http.StatusNoContent},
	} {
		t.Run(tc.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tc.wantCode, recorder.Body.String())
			}
		})
	}
}

func TestBearerAuthRejectsSessionTenantClaimMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:jwt_session_tenant_mismatch_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &auth.AuthSession{}); err != nil {
		t.Fatal(err)
	}
	user := admin.AdminUser{Base: model.Base{ID: uuid.New()}, TenantID: 11, Username: admin.NewInternalUsername(), PasswordHash: "unused-test-hash", Role: admin.RoleAdmin, Status: admin.StatusActive, TokenVersion: 9}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	sessionID := uuid.New()
	if err := db.Create(&auth.AuthSession{ID: sessionID, TenantID: 11, UserID: user.ID, TokenVersion: user.TokenVersion, Status: auth.SessionStatusActive}).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{JWTSecret: "middleware-regression-secret-with-at-least-32-bytes"}
	claims := auth.AccessClaims{Username: "bound-user", TokenType: "access", TenantID: 22, SessionID: sessionID.String(), TokenVersion: user.TokenVersion, RegisteredClaims: jwt.RegisteredClaims{Subject: user.ID.String(), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(BearerAuthWithDB(cfg, db, &auth.SessionService{Cfg: cfg, DB: db}))
	router.GET("/api/v1/auth/profile", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
}
