package settlement

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	adminmodel "github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func multipartRequest(t *testing.T, path string, shopID uuid.UUID, data []byte, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("shopId", shopID.String()); err != nil {
		t.Fatalf("write shop: %v", err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	part, err := writer.CreateFormFile("file", "bill.csv")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestSettlementHTTPPreviewAndReadonlyPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openSettlementTestDB(t)
	if err := db.AutoMigrate(&adminmodel.AdminUser{}, &adminmodel.UserStorePermission{}); err != nil {
		t.Fatalf("migrate admin: %v", err)
	}
	shopRow := shop.Shop{TenantID: 41, Platform: "manual", ShopName: "A", Status: "active", AuthStatus: "active"}
	if err := db.Create(&shopRow).Error; err != nil {
		t.Fatalf("create shop: %v", err)
	}
	operator := adminmodel.AdminUser{TenantID: 41, Username: "settlement-operator", PasswordHash: "x", Role: "operator", Status: adminmodel.StatusActive}
	readonly := adminmodel.AdminUser{TenantID: 41, Username: "settlement-readonly", PasswordHash: "x", Role: "readonly", Status: adminmodel.StatusActive}
	if err := db.Create(&operator).Error; err != nil {
		t.Fatalf("create operator: %v", err)
	}
	if err := db.Create(&readonly).Error; err != nil {
		t.Fatalf("create readonly: %v", err)
	}
	if err := db.Create(&adminmodel.UserStorePermission{UserID: operator.ID, StoreID: shopRow.ID, Platform: "manual", PermissionScope: adminmodel.StorePermScopeOperate}).Error; err != nil {
		t.Fatalf("create store permission: %v", err)
	}
	handler := &Handler{Svc: &Service{DB: db}}
	data := settlementCSV("TX-1,SO-1,CNY,100,10,90,2026-09-07T01:00:00Z")

	request := multipartRequest(t, "/settlement-imports/preview", shopRow.ID, data, nil)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, int64(41))
	context.Set(ctxkey.AdminID, operator.ID.String())
	handler.Preview(context)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"valid":true`) {
		t.Fatalf("operator preview status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	request = multipartRequest(t, "/settlement-imports/preview", shopRow.ID, data, nil)
	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, int64(41))
	context.Set(ctxkey.AdminID, readonly.ID.String())
	handler.Preview(context)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("readonly preview status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSettlementRouterRegistersOnlyExpectedWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{}})
	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"POST /api/v1/settlement-imports/preview",
		"POST /api/v1/settlement-imports",
		"GET /api/v1/settlement-reconciliation",
		"GET /api/v1/settlement-reconciliation/:id",
	} {
		if !routes[want] {
			t.Fatalf("route %q is not registered", want)
		}
	}
	for route := range routes {
		if strings.HasPrefix(route, "PUT ") || strings.HasPrefix(route, "PATCH ") || strings.HasPrefix(route, "DELETE ") {
			t.Fatalf("unexpected mutation route %q", route)
		}
	}
}
