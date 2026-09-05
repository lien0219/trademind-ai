package profitability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	adminmodel "github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func TestProfitabilityHTTPAppliesStoreScopeAndRestrictsExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:profitability_http?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&adminmodel.AdminUser{}, &adminmodel.UserStorePermission{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	operatorID, readonlyID, storeID := uuid.New(), uuid.New(), uuid.New()
	for _, row := range []adminmodel.AdminUser{
		{TenantID: 41, Username: "profit-operator", PasswordHash: "x", Role: "operator", Status: adminmodel.StatusActive},
		{TenantID: 41, Username: "profit-readonly", PasswordHash: "x", Role: "readonly", Status: adminmodel.StatusActive},
	} {
		copy := row
		if copy.Username == "profit-operator" {
			copy.ID = operatorID
		} else {
			copy.ID = readonlyID
		}
		if err := db.Create(&copy).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	if err := db.Create(&adminmodel.UserStorePermission{UserID: operatorID, StoreID: storeID, Platform: "manual", PermissionScope: adminmodel.StorePermScopeView}).Error; err != nil {
		t.Fatalf("create store permission: %v", err)
	}
	now := time.Now().UTC()
	repo := &fakeRepository{orders: []OrderFact{{ID: uuid.New(), OrderNo: "=FORMULA", Currency: "CNY", TotalAmountText: "1.00", CreatedAt: now, UpdatedAt: now}}}
	handler := &Handler{Svc: &Service{DB: db, Repo: repo, Clock: func() time.Time { return now }}}

	request := httptest.NewRequest(http.MethodGet, "/order-profits", nil)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, int64(41))
	context.Set(ctxkey.AdminID, operatorID.String())
	handler.List(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", recorder.Code, recorder.Body.String())
	}
	if !repo.lastScope.RestrictStoreScope || len(repo.lastScope.AllowedShopIDs) != 1 || repo.lastScope.AllowedShopIDs[0] != storeID {
		t.Fatalf("store scope = %#v", repo.lastScope)
	}

	request = httptest.NewRequest(http.MethodGet, "/order-profits?format=csv", nil)
	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, int64(41))
	context.Set(ctxkey.AdminID, operatorID.String())
	handler.List(context)
	if recorder.Code != http.StatusOK || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/csv") || !strings.Contains(recorder.Body.String(), "'=FORMULA") {
		t.Fatalf("operator export status=%d type=%q body=%s", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "/order-profits?format=csv", nil)
	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, int64(41))
	context.Set(ctxkey.AdminID, readonlyID.String())
	handler.List(context)
	if recorder.Code != http.StatusForbidden || strings.HasPrefix(recorder.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("readonly export status=%d type=%q body=%s", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
}

func TestProfitabilityRouterRegistersReadOnlyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")
	Register(group, &Handler{Svc: &Service{}})
	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{"GET /api/v1/order-profits", "GET /api/v1/order-profits/:orderId"} {
		if !routes[want] {
			t.Fatalf("route %q is not registered", want)
		}
	}
	for route := range routes {
		if strings.HasPrefix(route, "POST ") || strings.HasPrefix(route, "PUT ") || strings.HasPrefix(route, "PATCH ") || strings.HasPrefix(route, "DELETE ") {
			t.Fatalf("unexpected write route %q", route)
		}
	}
}
