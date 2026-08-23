package salesreturn

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
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type httpEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

func testRouter(t *testing.T, fx *fixture, tenantID int64, role string, actorID uuid.UUID) *gin.Engine {
	t.Helper()
	if err := fx.db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}); err != nil {
		t.Fatal(err)
	}
	if err := fx.db.Create(&admin.AdminUser{
		Base: model.Base{ID: actorID}, TenantID: tenantID, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: role, Status: admin.StatusActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, actorID.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: fx.service})
	return router
}

func request(t *testing.T, router *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, httpEnvelope) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	var envelope httpEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v body=%s", err, recorder.Body.String())
	}
	return recorder, envelope
}

func actionJSON(revision int, key string) string {
	return fmt.Sprintf(`{"expectedRevision":%d,"idempotencyKey":%q}`, revision, key)
}

func TestSalesReturnHTTPSeparatesApprovalReceiptReadonlyAndTenant(t *testing.T) {
	fx := newFixture(t, 2)
	row := fx.create(t, "sales-return-http-create", TypeReturnRefund, inventoryDispositionSellable, 1)
	row, err := fx.service.Submit(t.Context(), 7, row.ID, uuidPointer(uuid.New()), ActionInput{ExpectedRevision: row.Revision, IdempotencyKey: "sales-return-http-submit"})
	if err != nil {
		t.Fatal(err)
	}

	operatorID, reviewerID := uuid.New(), uuid.New()
	operator := testRouter(t, fx, 7, adminperm.RoleOperator, operatorID)
	reviewer := testRouter(t, fx, 7, adminperm.RoleReviewer, reviewerID)
	recorder, envelope := request(t, operator, http.MethodPost, "/api/v1/sales-returns/"+row.ID.String()+"/approve", actionJSON(row.Revision, "sales-return-http-operator-approve"))
	if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
		t.Fatalf("operator approval should be forbidden: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	recorder, envelope = request(t, reviewer, http.MethodPost, "/api/v1/sales-returns/"+row.ID.String()+"/approve", actionJSON(row.Revision, "sales-return-http-reviewer-approve"))
	if recorder.Code != http.StatusOK || envelope.Code != response.CodeOK {
		t.Fatalf("reviewer approval should succeed: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	row, err = fx.service.Get(t.Context(), 7, row.ID)
	if err != nil {
		t.Fatal(err)
	}
	recorder, envelope = request(t, reviewer, http.MethodPost, "/api/v1/sales-returns/"+row.ID.String()+"/complete", actionJSON(row.Revision, "sales-return-http-reviewer-complete"))
	if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
		t.Fatalf("reviewer receipt should be forbidden: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	recorder, envelope = request(t, operator, http.MethodPost, "/api/v1/sales-returns/"+row.ID.String()+"/complete", actionJSON(row.Revision, "sales-return-http-operator-complete"))
	if recorder.Code != http.StatusOK || envelope.Code != response.CodeOK {
		t.Fatalf("operator receipt should succeed: status=%d body=%s", recorder.Code, recorder.Body.String())
	}

	readonly := testRouter(t, fx, 7, adminperm.RoleReadonly, uuid.New())
	recorder, envelope = request(t, readonly, http.MethodPost, "/api/v1/sales-returns", `{}`)
	if recorder.Code != http.StatusForbidden || envelope.Code != response.CodeForbidden {
		t.Fatalf("readonly create should be forbidden: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	foreign := testRouter(t, fx, 8, adminperm.RoleAdmin, uuid.New())
	recorder, envelope = request(t, foreign, http.MethodGet, "/api/v1/sales-returns/"+row.ID.String(), "")
	if recorder.Code != http.StatusNotFound || envelope.Code != response.CodeNotFound || string(envelope.Data) != "null" {
		t.Fatalf("cross-tenant detail should look absent: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestPlatformAfterSaleHTTPEnforcesTenantAndStoreScope(t *testing.T) {
	fx := newFixture(t, 1)
	externalOrderID := "platform-http-order"
	if err := fx.db.Model(fx.order).Updates(map[string]any{
		"platform": "douyin_shop", "shop_id": fx.shop.ID, "external_order_id": externalOrderID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	local := fx.create(t, "platform-http-local", TypeRefundOnly, "", 1)
	if err := fx.service.UpsertPlatformAfterSale(t.Context(), PlatformAfterSaleInput{
		TenantID: 7, Platform: "douyin_shop", InternalShopID: &fx.shop.ID, PlatformShopID: fx.shop.ExternalShopID,
		EventID: "platform-http-event", EventType: "refund_success", ExternalAfterSaleID: "platform-http-after-sale",
		ExternalOrderID: externalOrderID, PlatformType: TypeRefundOnly, PlatformStatus: "success",
		RefundAmountMinor: local.RefundAmountMinor, Currency: local.Currency, RawPayload: []byte(`{"event":"refund_success"}`),
	}); err != nil {
		t.Fatal(err)
	}
	otherShop := &shop.Shop{TenantID: 7, Platform: "douyin_shop", ExternalShopID: "platform-http-other-shop", ShopName: "Other Shop", Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized}
	if err := fx.db.Create(otherShop).Error; err != nil {
		t.Fatal(err)
	}
	viewerID := uuid.New()
	router := testRouter(t, fx, 7, adminperm.RoleReviewer, viewerID)
	if err := fx.db.Create(&admin.UserStorePermission{UserID: viewerID, StoreID: otherShop.ID, Platform: "douyin_shop", PermissionScope: admin.StorePermScopeView}).Error; err != nil {
		t.Fatal(err)
	}
	recorder, envelope := request(t, router, http.MethodGet, "/api/v1/sales-return-reconciliation", "")
	if recorder.Code != http.StatusOK || envelope.Code != response.CodeOK || strings.Contains(recorder.Body.String(), "platform-http-after-sale") {
		t.Fatalf("store-scoped list leaked inaccessible shop: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	recorder, envelope = request(t, router, http.MethodGet, "/api/v1/sales-return-reconciliation/"+mustPlatformAfterSaleID(t, fx.db, "platform-http-after-sale").String(), "")
	if recorder.Code != http.StatusNotFound || envelope.Code != response.CodeNotFound {
		t.Fatalf("store-scoped detail should be absent: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	foreign := testRouter(t, fx, 8, adminperm.RoleAdmin, uuid.New())
	recorder, envelope = request(t, foreign, http.MethodGet, "/api/v1/sales-return-reconciliation/"+mustPlatformAfterSaleID(t, fx.db, "platform-http-after-sale").String(), "")
	if recorder.Code != http.StatusNotFound || envelope.Code != response.CodeNotFound {
		t.Fatalf("cross-tenant platform detail should be absent: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

const inventoryDispositionSellable = "sellable"
