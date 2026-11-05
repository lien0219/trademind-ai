package idor_test

import (
	"bytes"
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
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productcheck"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const readinessIDORTenant int64 = 7101

type readinessIDORFixture struct {
	db        *gorm.DB
	cfg       *config.Config
	product   uuid.UUID
	shop      uuid.UUID
	otherShop uuid.UUID
	token     string
}

func newReadinessIDORFixture(t *testing.T, role string, grant bool) *readinessIDORFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:readiness_idor_%s?mode=memory&cache=shared", uuid.NewString())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	conn, err := db.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	require.NoError(t, db.AutoMigrate(
		&admin.AdminUser{}, &admin.Tenant{}, &admin.TenantMembership{}, &admin.UserStorePermission{},
		&product.Product{}, &product.ProductImage{}, &product.ProductSKU{}, &product.ProductPlatformPublishConfig{},
		&productpublish.ProductPublication{}, &shop.Shop{},
	))

	require.NoError(t, db.Create(&admin.Tenant{
		ID: readinessIDORTenant, Code: "readiness-idor", Name: "Readiness IDOR", Status: admin.TenantActive,
	}).Error)
	actor := admin.AdminUser{
		Base:         model.Base{ID: uuid.New()},
		TenantID:     readinessIDORTenant,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "unused",
		Role:         admin.RoleAdmin,
		Status:       admin.StatusActive,
		TokenVersion: 1,
	}
	require.NoError(t, db.Create(&actor).Error)
	require.NoError(t, db.Create(&admin.TenantMembership{
		TenantID: readinessIDORTenant, UserID: actor.ID, Role: role, Status: admin.MembershipActive,
	}).Error)

	store := shop.Shop{TenantID: readinessIDORTenant, Platform: "manual", ShopName: "restricted-shop", Status: shop.StatusActive}
	require.NoError(t, db.Create(&store).Error)
	otherStore := shop.Shop{TenantID: readinessIDORTenant, Platform: "manual", ShopName: "other-shop", Status: shop.StatusActive}
	require.NoError(t, db.Create(&otherStore).Error)
	if grant {
		require.NoError(t, db.Create(&admin.UserStorePermission{
			UserID: actor.ID, StoreID: store.ID, Platform: store.Platform, PermissionScope: admin.StorePermScopeView,
		}).Error)
	}
	draft := product.Product{
		TenantID: readinessIDORTenant, Source: "test", OriginalTitle: "restricted draft",
		Title: "restricted draft", Status: product.StatusDraft, Currency: "CNY",
	}
	require.NoError(t, db.Create(&draft).Error)
	require.NoError(t, db.Create(&product.ProductPlatformPublishConfig{
		ProductID: draft.ID, Platform: "manual", ShopID: &store.ID,
	}).Error)

	cfg := &config.Config{AppEnv: config.EnvTest, JWTSecret: "readiness-idor-test-signing-secret-32-bytes"}
	keys, err := auth.BuildKeySet(cfg)
	require.NoError(t, err)
	token, _, err := auth.MintAccessToken(cfg, keys, auth.MintAccessInput{
		UserID: actor.ID, Username: actor.Username, TenantID: readinessIDORTenant, TokenVersion: actor.TokenVersion,
	})
	require.NoError(t, err)
	return &readinessIDORFixture{db: db, cfg: cfg, product: draft.ID, shop: store.ID, otherShop: otherStore.ID, token: token}
}

func (f *readinessIDORFixture) router() *gin.Engine {
	router := gin.New()
	router.Use(middleware.BearerAuthWithDB(f.cfg, f.db, nil))
	productcheck.Register(router.Group("/api/v1"), &productcheck.Handler{
		Svc: &productcheck.Service{DB: f.db},
	})
	return router
}

func (f *readinessIDORFixture) request(t *testing.T, method, path, body string) (*httptest.ResponseRecorder, response.Envelope) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+f.token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	f.router().ServeHTTP(recorder, request)
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope), recorder.Body.String())
	return recorder, envelope
}

func TestIDOR_ReadinessSingleRequiresProductViewAndStoreScope(t *testing.T) {
	t.Run("role without product view is denied", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleReviewer, false)
		recorder, envelope := f.request(t, http.MethodGet, fmt.Sprintf("/api/v1/products/%s/readiness", f.product), "")
		require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		require.NotEqual(t, 0, envelope.Code)
	})

	t.Run("operator without store grant cannot omit shop scope", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleOperator, false)
		recorder, _ := f.request(t, http.MethodGet, fmt.Sprintf("/api/v1/products/%s/readiness", f.product), "")
		require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	})

	t.Run("operator without store grant cannot name an ungranted shop", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleOperator, false)
		recorder, _ := f.request(t, http.MethodGet, fmt.Sprintf("/api/v1/products/%s/readiness?shopId=%s", f.product, f.shop), "")
		require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	})

	t.Run("operator with view grant can read", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleOperator, true)
		recorder, envelope := f.request(t, http.MethodGet, fmt.Sprintf("/api/v1/products/%s/readiness?shopId=%s", f.product, f.shop), "")
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, 0, envelope.Code)
	})

	t.Run("operator cannot select an ungranted shop for a visible product", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleOperator, true)
		recorder, _ := f.request(t, http.MethodGet, fmt.Sprintf("/api/v1/products/%s/readiness?shopId=%s", f.product, f.otherShop), "")
		require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	})
}

func TestIDOR_ReadinessBatchRequiresProductViewAndStoreScope(t *testing.T) {
	body := func(productID, shopID uuid.UUID) string {
		return fmt.Sprintf(`{"productIds":["%s"],"platform":"manual","shopId":"%s"}`, productID, shopID)
	}

	t.Run("role without product view is denied", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleReviewer, false)
		recorder, envelope := f.request(t, http.MethodPost, "/api/v1/products/readiness/batch", body(f.product, f.shop))
		require.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
		require.NotEqual(t, 0, envelope.Code)
	})

	t.Run("operator without store grant cannot read", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleOperator, false)
		recorder, _ := f.request(t, http.MethodPost, "/api/v1/products/readiness/batch", body(f.product, f.shop))
		require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	})

	t.Run("operator with view grant can read", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleOperator, true)
		recorder, envelope := f.request(t, http.MethodPost, "/api/v1/products/readiness/batch", body(f.product, f.shop))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, 0, envelope.Code)
	})

	t.Run("operator cannot select an ungranted shop for a visible product", func(t *testing.T) {
		f := newReadinessIDORFixture(t, adminperm.RoleOperator, true)
		recorder, _ := f.request(t, http.MethodPost, "/api/v1/products/readiness/batch", body(f.product, f.otherShop))
		require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	})
}
