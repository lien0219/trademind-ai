package inventory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
)

func warehouseLedgerHTTPRouter(t *testing.T, fixture *warehouseLedgerFixture, tenantID int64, role string) *gin.Engine {
	t.Helper()
	actorID := uuid.New()
	if err := fixture.db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(),
		PasswordHash: "test", Role: role, Status: admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.TraceID, "trace-warehouse-ledger-test")
		c.Set(ctxkey.TenantID, tenantID)
		c.Set(ctxkey.AdminID, actorID.String())
		security.SetGin(c, &security.TenantContext{
			TenantID: tenantID, UserID: actorID, RequestID: "trace-warehouse-ledger-test", AuthSource: security.AuthSourceAccessToken,
		})
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: fixture.service})
	return router
}

func performWarehouseLedgerRequest(t *testing.T, router http.Handler, method, path, body string) (*httptest.ResponseRecorder, response.Envelope) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return recorder, envelope
}

func adjustmentPath(fixture *warehouseLedgerFixture) string {
	return fmt.Sprintf("/api/v1/products/%s/skus/%s/adjust-stock", fixture.product.ID, fixture.sku.ID)
}

func adjustmentBody(fixture *warehouseLedgerFixture, stock int, key string) string {
	return fmt.Sprintf(`{"warehouseId":%q,"stock":%d,"idempotencyKey":%q,"reason":"count"}`, fixture.main.ID, stock, key)
}

func TestWarehouseAdjustmentHTTPEnforcesPermissionTenantAndDTO(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("readonly", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		router := warehouseLedgerHTTPRouter(t, fixture, 1, adminperm.RoleReadonly)
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, adjustmentPath(fixture), adjustmentBody(fixture, 7, "readonly-adjust-001"))
		if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
			t.Fatalf("unexpected readonly response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})

	t.Run("cross tenant", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		router := warehouseLedgerHTTPRouter(t, fixture, 2, admin.RoleAdmin)
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, adjustmentPath(fixture), adjustmentBody(fixture, 7, "cross-tenant-adjust-001"))
		if recorder.Code != http.StatusNotFound || envelope.Code != response.CodeNotFound {
			t.Fatalf("unexpected cross-tenant response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})

	t.Run("invalid dto", func(t *testing.T) {
		fixture := newWarehouseLedgerFixture(t, true)
		router := warehouseLedgerHTTPRouter(t, fixture, 1, admin.RoleAdmin)
		recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, adjustmentPath(fixture), `{"stock":7}`)
		if recorder.Code != http.StatusBadRequest || envelope.Code != response.CodeBadRequest {
			t.Fatalf("unexpected invalid DTO response: status=%d envelope=%#v", recorder.Code, envelope)
		}
	})
}

func TestWarehouseAdjustmentHTTPReturnsConflictEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newWarehouseLedgerFixture(t, true)
	router := warehouseLedgerHTTPRouter(t, fixture, 1, admin.RoleAdmin)
	path := adjustmentPath(fixture)

	recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodPost, path, adjustmentBody(fixture, 7, "conflict-adjust-001"))
	if recorder.Code != http.StatusOK || envelope.Code != response.CodeOK {
		t.Fatalf("unexpected initial adjustment response: status=%d envelope=%#v", recorder.Code, envelope)
	}
	recorder, envelope = performWarehouseLedgerRequest(t, router, http.MethodPost, path, adjustmentBody(fixture, 8, "conflict-adjust-001"))
	if recorder.Code != http.StatusConflict || envelope.Code == response.CodeOK || envelope.Data != nil {
		t.Fatalf("unexpected conflict response: status=%d envelope=%#v", recorder.Code, envelope)
	}
}

func TestWarehouseBalancesHTTPReturnsNotFoundForCrossTenantSKU(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fixture := newWarehouseLedgerFixture(t, true)
	router := warehouseLedgerHTTPRouter(t, fixture, 2, admin.RoleAdmin)
	path := fmt.Sprintf("/api/v1/products/%s/skus/%s/warehouse-balances", fixture.product.ID, fixture.sku.ID)

	recorder, envelope := performWarehouseLedgerRequest(t, router, http.MethodGet, path, "")
	if recorder.Code != http.StatusNotFound || envelope.Code != response.CodeNotFound || envelope.Data != nil {
		t.Fatalf("unexpected cross-tenant balance response: status=%d envelope=%#v", recorder.Code, envelope)
	}
}
