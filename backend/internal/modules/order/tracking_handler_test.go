package order

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func trackingHTTPFixture(t *testing.T) (*gorm.DB, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_tracking_http_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&admin.AdminUser{}, &admin.UserStorePermission{}, &Order{}, &OrderShipment{}, &OrderShipmentEvent{}); err != nil {
		t.Fatal(err)
	}
	operatorID, readonlyID := uuid.New(), uuid.New()
	for _, user := range []admin.AdminUser{
		{Base: model.Base{ID: operatorID}, TenantID: 7, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleOperator, Status: admin.StatusActive},
		{Base: model.Base{ID: readonlyID}, TenantID: 7, Username: admin.NewInternalUsername(), PasswordHash: "test", Role: adminperm.RoleReadonly, Status: admin.StatusActive},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	orderID, shipmentID, otherOrderID := uuid.New(), uuid.New(), uuid.New()
	for _, row := range []Order{
		{Base: model.Base{ID: orderID}, TenantID: 7, Platform: "manual", OrderNo: "TRACKING-HTTP-1", CustomerName: "Buyer", Status: StatusShipped, PaymentStatus: PaymentPaid, FulfillmentStatus: FulfillmentFulfilled, Currency: "CNY"},
		{Base: model.Base{ID: otherOrderID}, TenantID: 8, Platform: "manual", OrderNo: "TRACKING-HTTP-OTHER", CustomerName: "Other", Status: StatusShipped, PaymentStatus: PaymentPaid, FulfillmentStatus: FulfillmentFulfilled, Currency: "CNY"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []OrderShipment{
		{HardDeleteBase: model.HardDeleteBase{ID: shipmentID}, OrderID: orderID, Carrier: "Mock Carrier", TrackingNo: "TRACK-HTTP-1", Status: ShipmentShipped},
		{HardDeleteBase: model.HardDeleteBase{ID: uuid.New()}, OrderID: otherOrderID, Carrier: "Other Carrier", TrackingNo: "TRACK-OTHER", Status: ShipmentShipped},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db, operatorID, readonlyID, orderID, shipmentID, otherOrderID
}

func trackingHTTPRouter(db *gorm.DB, adminID uuid.UUID, tenantID int64) *gin.Engine {
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(ctxkey.AdminID, adminID.String())
		c.Set(ctxkey.TenantID, tenantID)
		c.Next()
	})
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{DB: db}})
	return router
}

func trackingHTTPRequest(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func trackingEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid response envelope: %v body=%s", err, recorder.Body.String())
	}
	return envelope
}

func TestShipmentTrackingHTTPContractsAndTenantScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, operatorID, readonlyID, orderID, shipmentID, otherOrderID := trackingHTTPFixture(t)
	operatorRouter := trackingHTTPRouter(db, operatorID, 7)

	list := trackingHTTPRequest(t, operatorRouter, http.MethodGet, "/api/v1/orders/"+orderID.String()+"/shipments", "")
	if list.Code != http.StatusOK || trackingEnvelope(t, list)["code"] != float64(0) {
		t.Fatalf("shipment list must return success envelope: status=%d body=%s", list.Code, list.Body.String())
	}

	events := trackingHTTPRequest(t, operatorRouter, http.MethodGet, "/api/v1/orders/"+orderID.String()+"/shipments/"+shipmentID.String()+"/events", "")
	if events.Code != http.StatusOK || trackingEnvelope(t, events)["code"] != float64(0) {
		t.Fatalf("shipment events must return success envelope: status=%d body=%s", events.Code, events.Body.String())
	}

	appendPath := "/api/v1/orders/" + orderID.String() + "/shipments/" + shipmentID.String() + "/events"
	created := trackingHTTPRequest(t, operatorRouter, http.MethodPost, appendPath, `{"eventKey":"http-event-1","status":"in_transit","location":"Shenzhen"}`)
	if created.Code != http.StatusOK || trackingEnvelope(t, created)["code"] != float64(0) {
		t.Fatalf("event append must return success envelope: status=%d body=%s", created.Code, created.Body.String())
	}

	conflict := trackingHTTPRequest(t, operatorRouter, http.MethodPost, appendPath, `{"eventKey":"http-event-1","status":"delivered"}`)
	if conflict.Code != http.StatusConflict || trackingEnvelope(t, conflict)["code"] == float64(0) {
		t.Fatalf("event key conflict must return non-success 409 envelope: status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	missingShipment := trackingHTTPRequest(t, operatorRouter, http.MethodGet, "/api/v1/orders/"+orderID.String()+"/shipments/"+uuid.NewString()+"/events", "")
	if missingShipment.Code != http.StatusNotFound {
		t.Fatalf("missing shipment must return 404: status=%d body=%s", missingShipment.Code, missingShipment.Body.String())
	}

	readonlyRouter := trackingHTTPRouter(db, readonlyID, 7)
	readonly := trackingHTTPRequest(t, readonlyRouter, http.MethodPost, appendPath, `{"eventKey":"readonly-event","status":"in_transit"}`)
	if readonly.Code != http.StatusForbidden {
		t.Fatalf("readonly user must not append shipment events: status=%d body=%s", readonly.Code, readonly.Body.String())
	}

	otherTenant := trackingHTTPRequest(t, operatorRouter, http.MethodGet, "/api/v1/orders/"+otherOrderID.String()+"/shipments", "")
	if otherTenant.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant order must return 404: status=%d body=%s", otherTenant.Code, otherTenant.Body.String())
	}
}
