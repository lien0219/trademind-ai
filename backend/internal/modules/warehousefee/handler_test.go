package warehousefee

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	adminmodel "github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestWarehouseFeePreviewRequiresManageAndStoreOperationScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:warehouse-fee-handler-%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&adminmodel.AdminUser{}, &adminmodel.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	operator := adminmodel.AdminUser{TenantID: 7, Username: "fee-operator", PasswordHash: "x", Role: "operator", Status: adminmodel.StatusActive}
	readonly := adminmodel.AdminUser{TenantID: 7, Username: "fee-readonly", PasswordHash: "x", Role: "readonly", Status: adminmodel.StatusActive}
	if err := db.Create(&operator).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&readonly).Error; err != nil {
		t.Fatal(err)
	}
	shopID := uuid.New()
	if err := db.Create(&adminmodel.UserStorePermission{UserID: operator.ID, StoreID: shopID, Platform: "manual", PermissionScope: adminmodel.StorePermScopeOperate}).Error; err != nil {
		t.Fatal(err)
	}
	reader := &scopeCapturingReader{}
	handler := &Handler{Svc: &Service{DB: db, Fulfillment: reader}}

	request := httptest.NewRequest(http.MethodPost, "/warehouse-operation-fees/preview", bytes.NewBufferString(`{"orderId":"`+uuid.NewString()+`","rateCardId":"`+uuid.NewString()+`","rateCardRevision":1}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, int64(7))
	context.Set(ctxkey.AdminID, readonly.ID.String())
	handler.Preview(context)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("readonly preview status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/warehouse-operation-fees/candidates", nil)
	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, int64(7))
	context.Set(ctxkey.AdminID, operator.ID.String())
	handler.ListCandidates(context)
	if recorder.Code != http.StatusOK || len(reader.scope.AllowedShopIDs) != 1 || reader.scope.AllowedShopIDs[0] != shopID {
		t.Fatalf("operator candidate scope status=%d scope=%#v body=%s", recorder.Code, reader.scope, recorder.Body.String())
	}
}

type scopeCapturingReader struct {
	scope order.WarehouseFeeScope
}

func (r *scopeCapturingReader) ListWarehouseFeeFacts(_ context.Context, _ *gorm.DB, _ int64, scope order.WarehouseFeeScope, query order.WarehouseFeeFactQuery) (*order.WarehouseFeeFactList, error) {
	r.scope = scope
	return &order.WarehouseFeeFactList{List: []order.WarehouseFeeFact{}, Page: query.Page, PageSize: query.PageSize}, nil
}

func (r *scopeCapturingReader) GetWarehouseFeeFact(context.Context, *gorm.DB, int64, order.WarehouseFeeScope, uuid.UUID, bool) (*order.WarehouseFeeFact, error) {
	return nil, order.ErrFulfillmentWaveNotFound
}

func TestWarehouseFeeRouterRegistersOnlyAppendOnlyLedgerWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{}})
	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/warehouse-fee-rate-cards",
		"GET /api/v1/warehouse-fee-rate-cards/:id",
		"POST /api/v1/warehouse-fee-rate-cards",
		"PUT /api/v1/warehouse-fee-rate-cards/:id",
		"GET /api/v1/warehouse-operation-fees/candidates",
		"POST /api/v1/warehouse-operation-fees/preview",
		"POST /api/v1/warehouse-operation-fees",
		"GET /api/v1/warehouse-operation-fees",
		"GET /api/v1/warehouse-operation-fees/:id",
		"POST /api/v1/warehouse-operation-fees/:id/adjustments",
		"POST /api/v1/warehouse-operation-fees/:id/adjustments/:adjustmentId/reverse",
	} {
		if !routes[want] {
			t.Fatalf("route %q is not registered", want)
		}
	}
	for route := range routes {
		if strings.HasPrefix(route, "PATCH ") || strings.HasPrefix(route, "DELETE ") {
			t.Fatalf("unexpected ledger mutation route %q", route)
		}
	}
}
