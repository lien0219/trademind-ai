package product

import (
	"bytes"
	"encoding/json"
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
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/gorm"
)

type skuWriteHTTPFixture struct {
	db      *gorm.DB
	product Product
	sku     ProductSKU
}

func newSKUWriteHTTPFixture(t *testing.T) *skuWriteHTTPFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:sku_write_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&admin.AdminUser{}, &Product{}, &ProductSKU{}))

	productRow := Product{TenantID: 1, Source: "manual", Title: "Tenant one product", Status: StatusDraft}
	require.NoError(t, db.Create(&productRow).Error)
	stock := 5
	sku := ProductSKU{ProductID: productRow.ID, SKUCode: "SKU-1", SKUName: "Blue", Stock: &stock}
	require.NoError(t, db.Create(&sku).Error)
	return &skuWriteHTTPFixture{db: db, product: productRow, sku: sku}
}

func (f *skuWriteHTTPFixture) router(t *testing.T, tenantID int64, role string) *gin.Engine {
	t.Helper()
	actorID := uuid.New()
	require.NoError(t, f.db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: role, Status: admin.StatusActive,
	}).Error)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.TraceID, "trace-sku-write-test")
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.AdminID, actorID.String())
		security.SetGin(c, &security.TenantContext{
			TenantID: tenantID, UserID: actorID, RequestID: "trace-sku-write-test", AuthSource: security.AuthSourceAccessToken,
		})
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: f.db}})
	return router
}

func performSKUWriteRequest(t *testing.T, router http.Handler, method, path, body string) (*httptest.ResponseRecorder, response.Envelope) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	var envelope response.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return recorder, envelope
}

func TestSKUWriteRoutesRejectReadonlyPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newSKUWriteHTTPFixture(t)
	router := fixture.router(t, 1, adminperm.RoleReadonly)
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, fmt.Sprintf("/api/v1/products/%s/skus", fixture.product.ID), `{"skuName":"New"}`},
		{http.MethodPut, fmt.Sprintf("/api/v1/products/%s/skus/%s", fixture.product.ID, fixture.sku.ID), `{"skuName":"Updated"}`},
		{http.MethodPut, fmt.Sprintf("/api/v1/products/%s/skus/%s/stock-settings", fixture.product.ID, fixture.sku.ID), `{"warningStock":8,"safetyStock":2}`},
		{http.MethodDelete, fmt.Sprintf("/api/v1/products/%s/skus/%s", fixture.product.ID, fixture.sku.ID), ""},
	}
	for _, test := range tests {
		recorder, envelope := performSKUWriteRequest(t, router, test.method, test.path, test.body)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Equal(t, response.CodeForbidden, envelope.Code)
		require.Equal(t, "trace-sku-write-test", envelope.TraceID)
	}
}

func TestSKUWriteRoutesHideCrossTenantProduct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newSKUWriteHTTPFixture(t)
	router := fixture.router(t, 2, admin.RoleAdmin)
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, fmt.Sprintf("/api/v1/products/%s/skus", fixture.product.ID), `{"skuName":"New"}`},
		{http.MethodPut, fmt.Sprintf("/api/v1/products/%s/skus/%s", fixture.product.ID, fixture.sku.ID), `{"skuName":"Updated"}`},
		{http.MethodPut, fmt.Sprintf("/api/v1/products/%s/skus/%s/stock-settings", fixture.product.ID, fixture.sku.ID), `{"warningStock":8,"safetyStock":2}`},
		{http.MethodDelete, fmt.Sprintf("/api/v1/products/%s/skus/%s", fixture.product.ID, fixture.sku.ID), ""},
	}
	for _, test := range tests {
		recorder, envelope := performSKUWriteRequest(t, router, test.method, test.path, test.body)
		require.Equal(t, http.StatusNotFound, recorder.Code)
		require.Equal(t, response.CodeNotFound, envelope.Code)
	}
	var sku ProductSKU
	require.NoError(t, fixture.db.First(&sku, "id = ?", fixture.sku.ID).Error)
	require.Equal(t, "Blue", sku.SKUName)
	require.Equal(t, 5, *sku.Stock)
}

func TestSKUWriteRoutesRejectStockAndCreateZeroProjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newSKUWriteHTTPFixture(t)
	router := fixture.router(t, 1, admin.RoleAdmin)

	updatePath := fmt.Sprintf("/api/v1/products/%s/skus/%s", fixture.product.ID, fixture.sku.ID)
	recorder, envelope := performSKUWriteRequest(t, router, http.MethodPut, updatePath, `{"stock":9}`)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, response.CodeBadRequest, envelope.Code)
	require.Contains(t, envelope.Message, "warehouse inventory")

	createPath := fmt.Sprintf("/api/v1/products/%s/skus", fixture.product.ID)
	recorder, envelope = performSKUWriteRequest(t, router, http.MethodPost, createPath, `{"skuCode":"REJECTED","skuName":"Rejected","stock":9}`)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, response.CodeBadRequest, envelope.Code)

	recorder, envelope = performSKUWriteRequest(t, router, http.MethodPost, createPath, `{"skuCode":"NEW","skuName":"New SKU"}`)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, response.CodeOK, envelope.Code)
	var created ProductSKU
	require.NoError(t, fixture.db.First(&created, "product_id = ? AND sku_code = ?", fixture.product.ID, "NEW").Error)
	require.NotNil(t, created.Stock)
	require.Zero(t, *created.Stock)

	var original ProductSKU
	require.NoError(t, fixture.db.First(&original, "id = ?", fixture.sku.ID).Error)
	require.Equal(t, 5, *original.Stock)
	var rejectedCount int64
	require.NoError(t, fixture.db.Model(&ProductSKU{}).Where("sku_code = ?", "REJECTED").Count(&rejectedCount).Error)
	require.Zero(t, rejectedCount)
}
