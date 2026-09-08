package advertisingfee

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	adminmodel "github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
)

func advertisingMultipartRequest(t *testing.T, shopID uuid.UUID, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("shopId", shopID.String()); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "advertising.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/advertising-fee-imports/preview", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func advertisingHandlerContext(request *http.Request, tenantID int64, userID uuid.UUID) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = request
	context.Set(ctxkey.TenantID, tenantID)
	context.Set(ctxkey.AdminID, userID.String())
	return context, recorder
}

func TestAdvertisingFeePreviewRequiresImportPermissionAndStoreOperationScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, shopRow, _ := newAdvertisingFeeService(t)
	if err := svc.DB.AutoMigrate(&adminmodel.AdminUser{}, &adminmodel.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	operator := adminmodel.AdminUser{TenantID: 7, Username: "advertising-operator", PasswordHash: "x", Role: "operator", Status: adminmodel.StatusActive}
	viewOnly := adminmodel.AdminUser{TenantID: 7, Username: "advertising-view-only", PasswordHash: "x", Role: "operator", Status: adminmodel.StatusActive}
	readonly := adminmodel.AdminUser{TenantID: 7, Username: "advertising-readonly", PasswordHash: "x", Role: "readonly", Status: adminmodel.StatusActive}
	for _, row := range []*adminmodel.AdminUser{&operator, &viewOnly, &readonly} {
		if err := svc.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, grant := range []adminmodel.UserStorePermission{
		{UserID: operator.ID, StoreID: shopRow.ID, Platform: "manual", PermissionScope: adminmodel.StorePermScopeOperate},
		{UserID: viewOnly.ID, StoreID: shopRow.ID, Platform: "manual", PermissionScope: adminmodel.StorePermScopeView},
	} {
		if err := svc.DB.Create(&grant).Error; err != nil {
			t.Fatal(err)
		}
	}
	handler := &Handler{Svc: svc}
	data := advertisingCSV(CoverageExcluded, 100)

	context, recorder := advertisingHandlerContext(advertisingMultipartRequest(t, shopRow.ID, data), 7, operator.ID)
	handler.Preview(context)
	if recorder.Code != http.StatusOK {
		t.Fatalf("operator preview status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	context, recorder = advertisingHandlerContext(advertisingMultipartRequest(t, shopRow.ID, data), 7, readonly.ID)
	handler.Preview(context)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("readonly preview status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	context, recorder = advertisingHandlerContext(advertisingMultipartRequest(t, shopRow.ID, data), 7, viewOnly.ID)
	handler.Preview(context)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("view-only preview status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	context, recorder = advertisingHandlerContext(advertisingMultipartRequest(t, uuid.New(), data), 7, operator.ID)
	handler.Preview(context)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("out-of-scope preview status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
