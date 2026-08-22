package procurement

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

func TestReplenishmentHTTPRequiresWarehouseAndExportsCSVWithGET(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fx := newProcurementFixture(t)
	if err := fx.DB.AutoMigrate(&inventory.WarehouseTransfer{}, &inventory.WarehouseTransferItem{}); err != nil {
		t.Fatalf("migrate transfers: %v", err)
	}
	router := purchaseReturnHTTPRouter(t, fx, 1, adminperm.RoleAdmin, uuid.New())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/procurement/replenishment-suggestions", nil)
	router.ServeHTTP(recorder, request)
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode missing warehouse response: %v", err)
	}
	if recorder.Code != http.StatusBadRequest || envelope.Code != response.CodeBadRequest {
		t.Fatalf("missing warehouse should be a bad request: status=%d envelope=%#v", recorder.Code, envelope)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/api/v1/procurement/replenishment-suggestions?warehouseId="+fx.Warehouse.ID.String()+"&format=csv", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("CSV export should be a successful GET download: status=%d contentType=%q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(recorder.Body.String(), "仓库,商品,规格编码") {
		t.Fatalf("CSV header missing: %q", recorder.Body.String())
	}
}

func TestReplenishmentHTTPRejectsUnknownStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fx := newProcurementFixture(t)
	if err := fx.DB.AutoMigrate(&inventory.WarehouseTransfer{}, &inventory.WarehouseTransferItem{}); err != nil {
		t.Fatalf("migrate transfers: %v", err)
	}
	router := purchaseReturnHTTPRouter(t, fx, 1, adminperm.RoleAdmin, uuid.New())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/procurement/replenishment-suggestions?warehouseId="+fx.Warehouse.ID.String()+"&status=blocked_unknown", nil)
	router.ServeHTTP(recorder, request)
	var envelope response.Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode invalid status response: %v", err)
	}
	if recorder.Code != http.StatusBadRequest || envelope.Code != response.CodeBadRequest {
		t.Fatalf("unknown status should be a bad request: status=%d envelope=%#v", recorder.Code, envelope)
	}
}
