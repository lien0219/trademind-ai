package procurement

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type purchaseReturnHTTPEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func purchaseReturnHTTPRouter(t *testing.T, fx *procurementFixture, tenantID int64, role string, actorID uuid.UUID) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if err := fx.DB.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	if err := fx.DB.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(), PasswordHash: "test",
		Role: role, Status: admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: fx.Service})
	return router
}

func performPurchaseReturnRequest(t *testing.T, router *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, purchaseReturnHTTPEnvelope) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	var envelope purchaseReturnHTTPEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response %s: %v body=%s", path, err, recorder.Body.String())
	}
	return recorder, envelope
}

func purchaseReturnActionJSON(revision int, key string) string {
	return fmt.Sprintf(`{"expectedRevision":%d,"idempotencyKey":%q,"reason":"http test"}`, revision, key)
}

func TestPurchaseReturnHTTPSeparatesApprovalAndExecutionPermissions(t *testing.T) {
	fx := newReceivedPurchaseFixture(t, 2)
	creator, reviewer, executor := uuid.New(), uuid.New(), uuid.New()
	row, err := fx.Service.CreatePurchaseReturn(t.Context(), 1, &creator, CreatePurchaseReturnInput{
		IdempotencyKey: "return-http-create-001", PurchaseOrderID: fx.Order.ID, Reason: "quality issue",
		Items: []CreatePurchaseReturnItemInput{{GoodsReceiptItemID: fx.ReceiptItem.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err = fx.Service.SubmitPurchaseReturn(t.Context(), 1, row.ID, &creator, PurchaseReturnActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "return-http-submit-001"})
	if err != nil {
		t.Fatal(err)
	}

	operatorRouter := purchaseReturnHTTPRouter(t, fx.procurementFixture, 1, adminperm.RoleOperator, executor)
	recorder, envelope := performPurchaseReturnRequest(t, operatorRouter, http.MethodPost, "/api/v1/purchase-returns/"+row.ID.String()+"/approve", purchaseReturnActionJSON(row.Revision, "return-http-operator-approve"))
	if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
		t.Fatalf("operator approval should be forbidden: status=%d envelope=%#v", recorder.Code, envelope)
	}

	reviewerRouter := purchaseReturnHTTPRouter(t, fx.procurementFixture, 1, adminperm.RoleReviewer, reviewer)
	recorder, envelope = performPurchaseReturnRequest(t, reviewerRouter, http.MethodPost, "/api/v1/purchase-returns/"+row.ID.String()+"/approve", purchaseReturnActionJSON(row.Revision, "return-http-reviewer-approve"))
	if recorder.Code != http.StatusOK || envelope.Code != response.CodeOK {
		t.Fatalf("reviewer approval should succeed: status=%d envelope=%#v", recorder.Code, envelope)
	}
	row, err = fx.Service.GetPurchaseReturn(t.Context(), 1, row.ID)
	if err != nil {
		t.Fatal(err)
	}

	recorder, envelope = performPurchaseReturnRequest(t, reviewerRouter, http.MethodPost, "/api/v1/purchase-returns/"+row.ID.String()+"/complete", purchaseReturnActionJSON(row.Revision, "return-http-reviewer-complete"))
	if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
		t.Fatalf("reviewer completion should be forbidden: status=%d envelope=%#v", recorder.Code, envelope)
	}
	recorder, envelope = performPurchaseReturnRequest(t, operatorRouter, http.MethodPost, "/api/v1/purchase-returns/"+row.ID.String()+"/complete", purchaseReturnActionJSON(row.Revision, "return-http-operator-complete"))
	if recorder.Code != http.StatusOK || envelope.Code != response.CodeOK {
		t.Fatalf("operator completion should succeed: status=%d envelope=%#v", recorder.Code, envelope)
	}
}

func TestPurchaseReturnHTTPEnforcesReadonlyAndTenantScope(t *testing.T) {
	fx := newReceivedPurchaseFixture(t, 2)
	creator := uuid.New()
	row, err := fx.Service.CreatePurchaseReturn(t.Context(), 1, &creator, CreatePurchaseReturnInput{
		IdempotencyKey: "return-http-scope-create", PurchaseOrderID: fx.Order.ID, Reason: "quality issue",
		Items: []CreatePurchaseReturnItemInput{{GoodsReceiptItemID: fx.ReceiptItem.ID, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	readonlyRouter := purchaseReturnHTTPRouter(t, fx.procurementFixture, 1, adminperm.RoleReadonly, uuid.New())
	recorder, envelope := performPurchaseReturnRequest(t, readonlyRouter, http.MethodPost, "/api/v1/purchase-returns", `{}`)
	if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
		t.Fatalf("readonly create should be forbidden: status=%d envelope=%#v", recorder.Code, envelope)
	}

	otherTenantRouter := purchaseReturnHTTPRouter(t, fx.procurementFixture, 2, adminperm.RoleAdmin, uuid.New())
	recorder, envelope = performPurchaseReturnRequest(t, otherTenantRouter, http.MethodGet, "/api/v1/purchase-returns/"+row.ID.String(), "")
	if recorder.Code != http.StatusNotFound || envelope.Code != response.CodeNotFound || string(envelope.Data) != "null" {
		t.Fatalf("cross-tenant detail should look absent: status=%d envelope=%#v", recorder.Code, envelope)
	}
}
