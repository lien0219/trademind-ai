package product

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/middleware"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/auth"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type skuSearchRepositorySpy struct {
	calls    int
	tenantID int64
	query    SearchSKUsQuery
	result   []ProductSKUSearchHit
	err      error
}

func (s *skuSearchRepositorySpy) SearchSKUs(_ context.Context, tenantID int64, q SearchSKUsQuery) ([]ProductSKUSearchHit, error) {
	s.calls++
	s.tenantID = tenantID
	s.query = q
	return s.result, s.err
}

func newSKUSearchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &Product{}, &ProductSKU{}))
	return db
}

func seedSKUSearchAdmin(t *testing.T, db *gorm.DB, tenantID int64, status string) uuid.UUID {
	t.Helper()
	actorID := uuid.New()
	require.NoError(t, db.Create(&admin.AdminUser{
		Base:         model.Base{ID: actorID},
		TenantID:     tenantID,
		Username:     admin.NewInternalUsername(),
		PasswordHash: "test",
		Role:         admin.RoleAdmin,
		Status:       status,
	}).Error)
	return actorID
}

func newSKUSearchHandlerContext(method, target string, tenantID int64, actorID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, nil)
	ctx.Set(ctxkey.TraceID, "trace-sku-search-test")
	ctx.Set(ctxkey.TenantID, tenantID)
	ctx.Set(ctxkey.AdminID, actorID.String())
	authSource := security.AuthSourceAccessToken
	if tenantID == 0 {
		authSource = security.AuthSourceLegacyDevZero
	}
	security.SetGin(ctx, &security.TenantContext{TenantID: tenantID, UserID: actorID, RequestID: "trace-sku-search-test", AuthSource: authSource})
	return ctx, recorder
}

func decodeSKUSearchEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) response.Envelope {
	t.Helper()
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope
}

func TestSearchSKUsHandlerUsesTrustedTenantAndPreservesContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newSKUSearchTestDB(t)
	actorID := seedSKUSearchAdmin(t, db, 101, admin.StatusActive)
	spy := &skuSearchRepositorySpy{result: []ProductSKUSearchHit{{ProductID: "product-a", ProductTitle: "Product A", ProductSKUID: "sku-a", SKUCode: "SHARED", SKUName: "Blue"}}}
	handler := &Handler{Svc: &Service{DB: db, skuSearchRepo: spy}}
	ctx, recorder := newSKUSearchHandlerContext(http.MethodGet, "/api/v1/product-skus/search?keyword=shared&productId=product-a&limit=99&tenantId=202&tenant_id=202&status=active&barcode=shared", 101, actorID)

	handler.SearchSKUs(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, int64(101), spy.tenantID)
	require.Equal(t, "shared", spy.query.Keyword)
	require.NotNil(t, spy.query.ProductID)
	require.Equal(t, "product-a", *spy.query.ProductID)
	require.Equal(t, 99, spy.query.Limit)
	require.NotContains(t, recorder.Body.String(), "tenantId")

	envelope := decodeSKUSearchEnvelope(t, recorder)
	require.Equal(t, response.CodeOK, envelope.Code)
	require.Equal(t, "trace-sku-search-test", envelope.TraceID)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	list, ok := data["list"].([]any)
	require.True(t, ok)
	require.Len(t, list, 1)
	row := list[0].(map[string]any)
	require.Equal(t, "product-a", row["productId"])
	require.Equal(t, "Product A", row["productTitle"])
	require.Equal(t, "sku-a", row["productSkuId"])
	require.Equal(t, "SHARED", row["skuCode"])
	require.Equal(t, "Blue", row["skuName"])
}

func TestSearchSKUsHandlerAllowsLegacyTenantZero(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newSKUSearchTestDB(t)
	actorID := seedSKUSearchAdmin(t, db, 0, admin.StatusActive)
	spy := &skuSearchRepositorySpy{result: []ProductSKUSearchHit{{ProductID: "legacy-product"}}}
	handler := &Handler{Svc: &Service{DB: db, skuSearchRepo: spy}}
	ctx, recorder := newSKUSearchHandlerContext(http.MethodGet, "/api/v1/product-skus/search", 0, actorID)

	handler.SearchSKUs(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, int64(0), spy.tenantID)
}

func TestSearchSKUsRouteAllowsLegacyTenantZeroJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newSKUSearchTestDB(t)
	actorID := seedSKUSearchAdmin(t, db, 0, admin.StatusActive)
	spy := &skuSearchRepositorySpy{result: []ProductSKUSearchHit{{ProductID: "legacy-product"}}}
	handler := &Handler{Svc: &Service{DB: db, skuSearchRepo: spy}}
	cfg := &config.Config{AppEnv: config.EnvDevelopment, JWTSecret: "test-jwt-secret-with-enough-length-32"}
	keys, err := auth.BuildKeySet(cfg)
	require.NoError(t, err)
	token, _, err := auth.MintAccessToken(cfg, keys, auth.MintAccessInput{
		UserID:       actorID,
		Username:     "legacy-admin",
		TenantID:     0,
		TokenVersion: 1,
	})
	require.NoError(t, err)

	router := gin.New()
	router.Use(middleware.BearerAuthWithDB(cfg, db, nil))
	router.GET("/api/v1/product-skus/search", handler.SearchSKUs)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/product-skus/search", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, int64(0), spy.tenantID)
}

func TestSearchSKUsHandlerFailsClosedBeforeSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		wantStatus int
		configure  func(*gin.Context, *gorm.DB, uuid.UUID)
	}{
		{name: "missing tenant key", wantStatus: http.StatusUnauthorized, configure: func(c *gin.Context, _ *gorm.DB, _ uuid.UUID) {
			c.Keys[ctxkey.TenantID] = nil
			delete(c.Keys, ctxkey.TenantID)
		}},
		{name: "zero tenant without legacy source", wantStatus: http.StatusUnauthorized, configure: func(c *gin.Context, _ *gorm.DB, actorID uuid.UUID) {
			c.Set(ctxkey.TenantID, int64(0))
			security.SetGin(c, &security.TenantContext{TenantID: 0, UserID: actorID, AuthSource: security.AuthSourceAccessToken})
		}},
		{name: "negative tenant", wantStatus: http.StatusUnauthorized, configure: func(c *gin.Context, _ *gorm.DB, _ uuid.UUID) { c.Set(ctxkey.TenantID, int64(-1)) }},
		{name: "missing security context", wantStatus: http.StatusUnauthorized, configure: func(c *gin.Context, _ *gorm.DB, _ uuid.UUID) { delete(c.Keys, "security.tenant_context") }},
		{name: "tenant context mismatch", wantStatus: http.StatusForbidden, configure: func(c *gin.Context, _ *gorm.DB, actorID uuid.UUID) {
			security.SetGin(c, &security.TenantContext{TenantID: 202, UserID: actorID})
		}},
		{name: "missing actor", wantStatus: http.StatusUnauthorized, configure: func(c *gin.Context, _ *gorm.DB, _ uuid.UUID) { delete(c.Keys, ctxkey.AdminID) }},
		{name: "actor context mismatch", wantStatus: http.StatusForbidden, configure: func(c *gin.Context, _ *gorm.DB, _ uuid.UUID) { c.Set(ctxkey.AdminID, uuid.New().String()) }},
		{name: "missing membership", wantStatus: http.StatusForbidden, configure: func(c *gin.Context, db *gorm.DB, actorID uuid.UUID) {
			require.NoError(t, db.Unscoped().Delete(&admin.AdminUser{}, "id = ?", actorID).Error)
		}},
		{name: "wrong tenant membership", wantStatus: http.StatusForbidden, configure: func(_ *gin.Context, db *gorm.DB, actorID uuid.UUID) {
			require.NoError(t, db.Model(&admin.AdminUser{}).Where("id = ?", actorID).Update("tenant_id", 202).Error)
		}},
		{name: "disabled membership", wantStatus: http.StatusForbidden, configure: func(_ *gin.Context, db *gorm.DB, actorID uuid.UUID) {
			require.NoError(t, db.Model(&admin.AdminUser{}).Where("id = ?", actorID).Update("status", "disabled").Error)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := newSKUSearchTestDB(t)
			actorID := seedSKUSearchAdmin(t, db, 101, admin.StatusActive)
			spy := &skuSearchRepositorySpy{}
			handler := &Handler{Svc: &Service{DB: db, skuSearchRepo: spy}}
			ctx, recorder := newSKUSearchHandlerContext(http.MethodGet, "/api/v1/product-skus/search", 101, actorID)
			test.configure(ctx, db, actorID)

			handler.SearchSKUs(ctx)

			require.Equal(t, test.wantStatus, recorder.Code)
			require.Zero(t, spy.calls)
			require.Contains(t, recorder.Body.String(), "trace-sku-search-test")
			if test.wantStatus == http.StatusUnauthorized {
				require.Contains(t, recorder.Body.String(), "authentication_required")
			} else {
				require.Contains(t, recorder.Body.String(), "permission_denied")
			}
			for _, unsafe := range []string{"SELECT", "SQLSTATE", "admin_users", "product_skus"} {
				require.NotContains(t, recorder.Body.String(), unsafe)
			}
		})
	}
}

func TestSearchSKUsServiceRequiresTenantBeforeRepository(t *testing.T) {
	spy := &skuSearchRepositorySpy{}
	svc := &Service{skuSearchRepo: spy}
	for _, tenantID := range []int64{-1} {
		_, err := svc.SearchSKUs(context.Background(), tenantID, SearchSKUsQuery{})
		require.ErrorIs(t, err, errSKUSearchAuthenticationRequired)
	}
	require.Zero(t, spy.calls)
	_, err := svc.SearchSKUs(context.Background(), 0, SearchSKUsQuery{})
	require.NoError(t, err)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, int64(0), spy.tenantID)

	query := SearchSKUsQuery{Keyword: "shared", Limit: 7}
	_, err = svc.SearchSKUs(context.Background(), 101, query)
	require.NoError(t, err)
	require.Equal(t, 2, spy.calls)
	require.Equal(t, int64(101), spy.tenantID)
	require.Equal(t, query, spy.query)
}

type skuSearchFixture struct {
	productA Product
	productB Product
	skusA    []ProductSKU
	skusB    []ProductSKU
}

func seedSKUSearchFixture(t *testing.T, db *gorm.DB) skuSearchFixture {
	t.Helper()
	productA := Product{TenantID: 101, Source: "manual", Title: "Shared Product Tenant A", Status: StatusDraft}
	productB := Product{TenantID: 202, Source: "manual", Title: "Shared Product Tenant B", Status: StatusDraft}
	require.NoError(t, db.Create(&productA).Error)
	require.NoError(t, db.Create(&productB).Error)
	stock := 5
	skusA := []ProductSKU{
		{ProductID: productA.ID, SKUCode: "SHARED-CODE", SKUName: "Shared Blue A", Stock: &stock, StockStatus: "normal", RawData: datatypes.JSON([]byte(`{"barcode":"SHARED-BARCODE"}`))},
		{ProductID: productA.ID, SKUCode: "A-SECOND", SKUName: "Shared Green A", Stock: &stock, StockStatus: "low_stock", RawData: datatypes.JSON([]byte(`{"barcode":"A-BARCODE"}`))},
	}
	skusB := []ProductSKU{
		{ProductID: productB.ID, SKUCode: "SHARED-CODE", SKUName: "Shared Blue B", Stock: &stock, StockStatus: "normal", RawData: datatypes.JSON([]byte(`{"barcode":"SHARED-BARCODE"}`))},
		{ProductID: productB.ID, SKUCode: "B-SECOND", SKUName: "Shared Green B", Stock: &stock, StockStatus: "low_stock", RawData: datatypes.JSON([]byte(`{"barcode":"B-BARCODE"}`))},
	}
	require.NoError(t, db.Create(&skusA).Error)
	require.NoError(t, db.Create(&skusB).Error)
	require.NoError(t, db.Model(&Product{}).Where("id = ?", productA.ID).Update("updated_at", time.Now().UTC().Add(-time.Hour)).Error)
	require.NoError(t, db.Model(&Product{}).Where("id = ?", productB.ID).Update("updated_at", time.Now().UTC()).Error)
	return skuSearchFixture{productA: productA, productB: productB, skusA: skusA, skusB: skusB}
}

func assertSKUSearchTenant(t *testing.T, rows []ProductSKUSearchHit, tenantProductID uuid.UUID) {
	t.Helper()
	for _, row := range rows {
		require.Equal(t, tenantProductID.String(), row.ProductID)
	}
}

func TestGormSKUSearchRepositoryScopesEveryExistingBranchAndLimit(t *testing.T) {
	db := newSKUSearchTestDB(t)
	fixture := seedSKUSearchFixture(t, db)
	repo := &gormSKUSearchRepository{db: db}
	ctx := context.Background()

	for _, test := range []struct {
		name     string
		tenantID int64
		query    SearchSKUsQuery
		want     int
		product  uuid.UUID
	}{
		{name: "tenant A default", tenantID: 101, query: SearchSKUsQuery{}, want: 2, product: fixture.productA.ID},
		{name: "tenant B default", tenantID: 202, query: SearchSKUsQuery{}, want: 2, product: fixture.productB.ID},
		{name: "shared sku code", tenantID: 101, query: SearchSKUsQuery{Keyword: "SHARED-CODE"}, want: 1, product: fixture.productA.ID},
		{name: "shared sku name", tenantID: 101, query: SearchSKUsQuery{Keyword: "Shared Blue"}, want: 1, product: fixture.productA.ID},
		{name: "shared product title", tenantID: 101, query: SearchSKUsQuery{Keyword: "Shared Product"}, want: 2, product: fixture.productA.ID},
		{name: "tenant A product id", tenantID: 101, query: SearchSKUsQuery{ProductID: ptrString(fixture.productA.ID.String())}, want: 2, product: fixture.productA.ID},
		{name: "tenant B product id", tenantID: 202, query: SearchSKUsQuery{ProductID: ptrString(fixture.productB.ID.String())}, want: 2, product: fixture.productB.ID},
		{name: "product id plus keyword", tenantID: 101, query: SearchSKUsQuery{ProductID: ptrString(fixture.productA.ID.String()), Keyword: "Shared Blue"}, want: 1, product: fixture.productA.ID},
		{name: "limit foreign rows cannot displace", tenantID: 101, query: SearchSKUsQuery{Keyword: "Shared", Limit: 1}, want: 1, product: fixture.productA.ID},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows, err := repo.SearchSKUs(ctx, test.tenantID, test.query)
			require.NoError(t, err)
			require.Len(t, rows, test.want)
			assertSKUSearchTenant(t, rows, test.product)
		})
	}

	for _, test := range []struct {
		name     string
		tenantID int64
		foreign  uuid.UUID
		keyword  string
	}{
		{name: "tenant A foreign product", tenantID: 101, foreign: fixture.productB.ID},
		{name: "tenant B foreign product", tenantID: 202, foreign: fixture.productA.ID},
		{name: "foreign product plus keyword", tenantID: 101, foreign: fixture.productB.ID, keyword: "Shared Blue"},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows, err := repo.SearchSKUs(ctx, test.tenantID, SearchSKUsQuery{ProductID: ptrString(test.foreign.String()), Keyword: test.keyword})
			require.NoError(t, err)
			require.Empty(t, rows)
		})
	}

	for iteration := 0; iteration < 3; iteration++ {
		rows, err := repo.SearchSKUs(ctx, 101, SearchSKUsQuery{Keyword: "Shared", Limit: 1})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assertSKUSearchTenant(t, rows, fixture.productA.ID)
	}

	rows, err := repo.SearchSKUs(ctx, 101, SearchSKUsQuery{Keyword: "SHARED-BARCODE"})
	require.NoError(t, err)
	require.Empty(t, rows, "barcode is stored fixture data but is not part of the existing search contract")
}

func ptrString(value string) *string {
	return &value
}
