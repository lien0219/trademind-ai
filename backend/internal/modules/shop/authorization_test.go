package shop

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestShopRoutesRejectWritesWithoutStoreOperateOrGlobalAdmin(t *testing.T) {
	fx := newShopAuthorizationFixture(t)

	t.Run("readonly cannot update credentials", func(t *testing.T) {
		r := newShopAuthorizationRouter(fx, fx.readonly)
		body := mustJSON(t, UpdateAuthBody{AuthType: "oauth2", AppKey: "ignored"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/shops/"+fx.shopA.ID.String()+"/auth", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("scoped operator cannot update an ungranted shop", func(t *testing.T) {
		r := newShopAuthorizationRouter(fx, fx.operator)
		body := mustJSON(t, UpdateAuthBody{AuthType: "oauth2", AppKey: "ignored"})
		req := httptest.NewRequest(http.MethodPut, "/api/v1/shops/"+fx.shopB.ID.String()+"/auth", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("scoped operator cannot create a shop without global admin scope", func(t *testing.T) {
		r := newShopAuthorizationRouter(fx, fx.operator)
		body := mustJSON(t, CreateBody{Platform: "manual", ShopName: "blocked"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/shops", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("tenant admin cannot view a shop owned by another tenant", func(t *testing.T) {
		r := newShopAuthorizationRouter(fx, fx.admin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/shops/"+fx.foreign.ID.String(), nil))
		require.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("scoped operator cannot delete a shop", func(t *testing.T) {
		r := newShopAuthorizationRouter(fx, fx.operator)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/shops/"+fx.shopA.ID.String(), nil))
		require.Equal(t, http.StatusForbidden, w.Code)
		var count int64
		require.NoError(t, fx.db.Model(&Shop{}).Where("id = ?", fx.shopA.ID).Count(&count).Error)
		require.EqualValues(t, 1, count)
	})

	t.Run("scoped operator cannot start OAuth without a shop", func(t *testing.T) {
		r := newShopAuthorizationRouter(fx, fx.operator)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/shops/oauth/douyin/start", nil))
		require.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestPlatformSettingsRoutesRequireSettingsManage(t *testing.T) {
	fx := newShopAuthorizationFixture(t)
	r := newShopAuthorizationRouter(fx, fx.operator)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/platform/settings/manual", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestShopDeniedMutationIsAuditedWithoutRequestSecrets(t *testing.T) {
	fx := newShopAuthorizationFixture(t)
	r := newShopAuthorizationRouter(fx, fx.readonly)
	body := mustJSON(t, UpdateAuthBody{AuthType: "oauth2", AppSecret: "synthetic-secret-must-not-enter-audit"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/shops/"+fx.shopA.ID.String()+"/auth", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	var rows []operationlog.OperationLog
	require.NoError(t, fx.db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "denied", rows[0].Status)
	require.Equal(t, fx.readonly.TenantID, rows[0].TenantID)
	require.Equal(t, fx.shopA.ID.String(), rows[0].ResourceID)
	require.NotContains(t, rows[0].Message, "synthetic-secret")
}

func TestShopAuthorizationMiddlewareStopsRejectedRequests(t *testing.T) {
	fx := newShopAuthorizationFixture(t)
	h := &Handler{Svc: &Service{DB: fx.db, OpLog: &operationlog.Service{DB: fx.db}}}
	for _, tt := range []struct {
		name  string
		guard gin.HandlerFunc
	}{
		{"shop service unavailable", requireShopPermission(nil, adminperm.PermStoreView, "view")},
		{"shop permission denied", requireShopPermission(h, adminperm.PermSettingsManage, "")},
		{"global admin unavailable", requireGlobalAdmin(nil)},
		{"global admin denied", requireGlobalAdmin(h)},
		{"platform settings unavailable", requirePlatformSettingsManage(nil)},
		{"platform settings tenant denied", requirePlatformSettingsManage(h)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(ctxkey.AdminID, fx.operator.ID.String())
				c.Set(ctxkey.TenantID, fx.operator.TenantID)
			})
			called := false
			r.GET("/probe", tt.guard, func(c *gin.Context) {
				called = true
				c.JSON(http.StatusOK, gin.H{"sensitive": "must not be disclosed"})
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
			require.False(t, called, "a rejected request must never reach its handler")
			require.GreaterOrEqual(t, w.Code, http.StatusBadRequest)
			require.NotContains(t, w.Body.String(), "sensitive")
			require.True(t, json.Valid(w.Body.Bytes()), "response must contain exactly one JSON envelope")
		})
	}
}

func TestDouyinCategoryRoutesRequireStoreScope(t *testing.T) {
	fx := newShopAuthorizationFixture(t)
	t.Run("operator cannot sync categories for an ungranted shop", func(t *testing.T) {
		r := newShopAuthorizationRouter(fx, fx.operator)
		body := mustJSON(t, map[string]string{"shopId": fx.shopB.ID.String()})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/platform/douyin/categories/sync", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusNotFound, w.Code)
	})
	t.Run("reviewer cannot read store-scoped category metadata", func(t *testing.T) {
		reviewer := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(), Email: "reviewer@example.test", PasswordHash: "test-only", Role: adminperm.RoleReviewer, Status: admin.StatusActive, TokenVersion: 1}
		require.NoError(t, fx.db.Create(&reviewer).Error)
		r := newShopAuthorizationRouter(fx, reviewer)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/platform/douyin/categories/stats", nil))
		require.Equal(t, http.StatusForbidden, w.Code)
	})
}

func newShopAuthorizationRouter(fx shopAuthorizationFixture, actor admin.AdminUser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actor.ID.String())
		c.Set(ctxkey.TenantID, actor.TenantID)
		c.Next()
	})
	Register(r.Group("/api/v1"), &Handler{Svc: &Service{DB: fx.db, OpLog: &operationlog.Service{DB: fx.db}}})
	return r
}

type shopAuthorizationFixture struct {
	db       *gorm.DB
	admin    admin.AdminUser
	operator admin.AdminUser
	readonly admin.AdminUser
	shopA    Shop
	shopB    Shop
	foreign  Shop
}

func newShopAuthorizationFixture(t *testing.T) shopAuthorizationFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:shop_authz_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &Shop{}, &ShopAuthToken{}, &operationlog.OperationLog{}))
	adminUser := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(), Email: "admin@example.test", PasswordHash: "test-only", Role: adminperm.RoleAdmin, Status: admin.StatusActive, TokenVersion: 1}
	operator := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(), Email: "operator@example.test", PasswordHash: "test-only", Role: adminperm.RoleOperator, Status: admin.StatusActive, TokenVersion: 1}
	readonly := admin.AdminUser{TenantID: 7, Username: admin.NewInternalUsername(), Email: "readonly@example.test", PasswordHash: "test-only", Role: adminperm.RoleReadonly, Status: admin.StatusActive, TokenVersion: 1}
	require.NoError(t, db.Create(&adminUser).Error)
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&readonly).Error)
	shopA := Shop{TenantID: 7, Platform: "manual", ShopName: "A", Status: StatusActive, AuthStatus: AuthAuthorized}
	shopB := Shop{TenantID: 7, Platform: "manual", ShopName: "B", Status: StatusActive, AuthStatus: AuthAuthorized}
	foreign := Shop{TenantID: 8, Platform: "manual", ShopName: "Foreign", Status: StatusActive, AuthStatus: AuthAuthorized}
	require.NoError(t, db.Create(&shopA).Error)
	require.NoError(t, db.Create(&shopB).Error)
	require.NoError(t, db.Create(&foreign).Error)
	require.NoError(t, db.Create(&admin.UserStorePermission{UserID: operator.ID, StoreID: shopA.ID, Platform: "manual", PermissionScope: admin.StorePermScopeOperate}).Error)
	return shopAuthorizationFixture{db: db, admin: adminUser, operator: operator, readonly: readonly, shopA: shopA, shopB: shopB, foreign: foreign}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	require.NoError(t, err)
	return b
}
