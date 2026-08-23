package order

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func trackingFixture(t *testing.T) (*Service, *gin.Context, uuid.UUID, uuid.UUID) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:order_tracking_%s?mode=memory&cache=shared", uuid.NewString())), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Order{}, &OrderShipment{}, &OrderShipmentEvent{}); err != nil {
		t.Fatal(err)
	}
	orderID, shipmentID := uuid.New(), uuid.New()
	if err := db.Create(&Order{Base: model.Base{ID: orderID}, TenantID: 7, Platform: "manual", OrderNo: "TRACKING-1", CustomerName: "Buyer", Status: StatusShipped, PaymentStatus: PaymentPaid, FulfillmentStatus: FulfillmentFulfilled, Currency: "CNY"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&OrderShipment{HardDeleteBase: model.HardDeleteBase{ID: shipmentID}, OrderID: orderID, Carrier: "Local", TrackingNo: "TRACK-1", Status: ShipmentShipped}).Error; err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/orders/"+orderID.String()+"/shipments/"+shipmentID.String()+"/events", nil)
	c.Set(ctxkey.TenantID, int64(7))
	return &Service{DB: db}, c, orderID, shipmentID
}

func TestAppendShipmentEventIsIdempotentAndAdvancesOrder(t *testing.T) {
	svc, c, orderID, shipmentID := trackingFixture(t)
	when := time.Now().UTC().Add(-time.Minute)
	first, err := svc.AppendShipmentEvent(c, orderID, shipmentID, ShipmentEventInput{EventKey: "carrier-1", Status: ShipmentInTransit, OccurredAt: &when, Source: "manual", RawData: map[string]any{"trackingStatus": "moving", "token": "must-not-store"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.Replay || first.Shipment.Status != ShipmentInTransit {
		t.Fatalf("unexpected first result: %#v", first)
	}
	replay, err := svc.AppendShipmentEvent(c, orderID, shipmentID, ShipmentEventInput{EventKey: "carrier-1", Status: ShipmentInTransit, OccurredAt: &when, Source: "manual", RawData: map[string]any{"trackingStatus": "moving", "token": "must-not-store"}}, nil)
	if err != nil || !replay.Replay || replay.Event.ID != first.Event.ID {
		t.Fatalf("event replay should be idempotent: result=%#v err=%v", replay, err)
	}
	var stored OrderShipmentEvent
	if err := svc.DB.First(&stored, "id = ?", first.Event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if string(stored.RawData) != `{"trackingStatus":"moving"}` {
		t.Fatalf("sensitive raw data must be omitted: %s", stored.RawData)
	}
}

func TestAppendShipmentEventRejectsConflictAndTerminalRegression(t *testing.T) {
	svc, c, orderID, shipmentID := trackingFixture(t)
	when := time.Now().UTC().Add(-time.Minute)
	if _, err := svc.AppendShipmentEvent(c, orderID, shipmentID, ShipmentEventInput{EventKey: "same-key", Status: ShipmentInTransit, OccurredAt: &when}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendShipmentEvent(c, orderID, shipmentID, ShipmentEventInput{EventKey: "same-key", Status: ShipmentDelivered, OccurredAt: &when}, nil); err != ErrShipmentEventConflict {
		t.Fatalf("expected event key conflict, got %v", err)
	}
	if _, err := svc.AppendShipmentEvent(c, orderID, shipmentID, ShipmentEventInput{EventKey: "delivered", Status: ShipmentDelivered, OccurredAt: &when}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AppendShipmentEvent(c, orderID, shipmentID, ShipmentEventInput{EventKey: "regression", Status: ShipmentInTransit, OccurredAt: &when}, nil); err != ErrShipmentEventImmutable {
		t.Fatalf("expected terminal immutability, got %v", err)
	}
	var orderRow Order
	if err := svc.DB.First(&orderRow, "id = ?", orderID).Error; err != nil {
		t.Fatal(err)
	}
	if orderRow.Status != StatusDelivered || orderRow.DeliveredAt == nil {
		t.Fatalf("delivered event must advance order lifecycle: %#v", orderRow)
	}
}
