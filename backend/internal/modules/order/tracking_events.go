package order

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrShipmentEventKeyRequired = errors.New("shipment eventKey is required")
	ErrShipmentEventConflict    = errors.New("shipment event key conflicts with an existing event")
	ErrShipmentEventTransition  = errors.New("invalid shipment status transition")
	ErrShipmentEventImmutable   = errors.New("delivered or returned shipment is immutable")
	ErrShipmentEventInputLong   = errors.New("shipment event input is too long")
)

// ShipmentEventInput is the controlled manual/provider-neutral event payload.
type ShipmentEventInput struct {
	EventKey    string         `json:"eventKey"`
	Status      string         `json:"status"`
	OccurredAt  *time.Time     `json:"occurredAt,omitempty"`
	Location    string         `json:"location,omitempty"`
	Description string         `json:"description,omitempty"`
	Source      string         `json:"source,omitempty"`
	RawData     map[string]any `json:"rawData,omitempty"`
}

type ShipmentEventResult struct {
	Event    *OrderShipmentEvent `json:"event"`
	Shipment *OrderShipment      `json:"shipment"`
	Replay   bool                `json:"replay"`
}

func shipmentTransitionAllowed(from, to string) bool {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == to {
		return true
	}
	if from == ShipmentDelivered || from == ShipmentReturned {
		return false
	}
	switch from {
	case ShipmentPending:
		return to == ShipmentShipped || to == ShipmentException || to == ShipmentReturned
	case ShipmentShipped:
		return to == ShipmentInTransit || to == ShipmentDelivered || to == ShipmentException || to == ShipmentReturned
	case ShipmentInTransit:
		return to == ShipmentDelivered || to == ShipmentException || to == ShipmentReturned
	case ShipmentException:
		return to == ShipmentInTransit || to == ShipmentDelivered || to == ShipmentReturned
	default:
		return false
	}
}

func normalizeShipmentEventInput(in ShipmentEventInput) (ShipmentEventInput, time.Time, []byte, error) {
	in.EventKey = strings.TrimSpace(in.EventKey)
	if in.EventKey == "" {
		return in, time.Time{}, nil, ErrShipmentEventKeyRequired
	}
	in.Status = strings.TrimSpace(in.Status)
	if !validShipmentStatus(in.Status) {
		return in, time.Time{}, nil, fmt.Errorf("invalid shipment status")
	}
	in.Source = strings.TrimSpace(in.Source)
	if in.Source == "" {
		in.Source = "manual"
	}
	in.Location = strings.TrimSpace(in.Location)
	in.Description = strings.TrimSpace(in.Description)
	if len(in.EventKey) > 128 || len(in.Source) > 64 || len(in.Location) > 255 || len(in.Description) > 2000 {
		return in, time.Time{}, nil, ErrShipmentEventInputLong
	}
	when := time.Now().UTC()
	if in.OccurredAt != nil {
		when = in.OccurredAt.UTC()
	}
	if when.After(time.Now().UTC().Add(5 * time.Minute)) {
		return in, time.Time{}, nil, fmt.Errorf("occurredAt cannot be in the future")
	}
	var raw []byte
	if in.RawData != nil {
		in.RawData = sanitizeTrackingRawData(in.RawData)
		var err error
		raw, err = json.Marshal(in.RawData)
		if err != nil {
			return in, time.Time{}, nil, fmt.Errorf("invalid rawData")
		}
		if len(raw) > 32*1024 {
			return in, time.Time{}, nil, ErrShipmentEventInputLong
		}
	}
	return in, when, raw, nil
}

func sanitizeTrackingRawData(input map[string]any) map[string]any {
	out := make(map[string]any)
	for key, value := range input {
		if len(out) >= 32 {
			break
		}
		key = strings.TrimSpace(key)
		lower := strings.ToLower(key)
		if key == "" || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "cookie") || strings.Contains(lower, "authorization") {
			continue
		}
		switch v := value.(type) {
		case string:
			if len(v) > 512 {
				out[key] = v[:512]
			} else {
				out[key] = v
			}
		case bool, float64, int, int64, nil:
			out[key] = value
		}
	}
	return out
}

func sameShipmentEvent(row *OrderShipmentEvent, in ShipmentEventInput, when time.Time, raw []byte) bool {
	if row == nil {
		return false
	}
	return row.Status == in.Status && row.OccurredAt.Equal(when) && row.Location == in.Location &&
		row.Description == in.Description && row.Source == in.Source && bytes.Equal([]byte(row.RawData), raw)
}

func (s *Service) shipmentForOrder(c *gin.Context, orderID, shipmentID uuid.UUID) (*Order, *OrderShipment, error) {
	o, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, nil, err
	}
	var shipment OrderShipment
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ? AND order_id = ?", shipmentID, orderID).First(&shipment).Error; err != nil {
		return nil, nil, err
	}
	return o, &shipment, nil
}

func (s *Service) ListShipments(c *gin.Context, orderID uuid.UUID) ([]OrderShipment, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	if _, err := s.findOrderBare(c, orderID); err != nil {
		return nil, err
	}
	var rows []OrderShipment
	err := s.DB.WithContext(c.Request.Context()).Where("order_id = ?", orderID).Order("created_at ASC, id ASC").Find(&rows).Error
	return rows, err
}

func (s *Service) ListShipmentEvents(c *gin.Context, orderID, shipmentID uuid.UUID) (*OrderShipment, []OrderShipmentEvent, string, error) {
	if s == nil || s.DB == nil {
		return nil, nil, "", fmt.Errorf("order: no db")
	}
	_, shipment, err := s.shipmentForOrder(c, orderID, shipmentID)
	if err != nil {
		return nil, nil, "", err
	}
	ctx, cancel := trackingContext(c.Request.Context())
	defer cancel()
	provider := s.Tracking
	if provider == nil {
		provider = &StoredTrackingProvider{DB: s.DB}
	}
	events, err := provider.ListEvents(ctx, shipmentID)
	if err != nil {
		return nil, nil, provider.Name(), err
	}
	return shipment, events, provider.Name(), nil
}

func (s *Service) AppendShipmentEvent(c *gin.Context, orderID, shipmentID uuid.UUID, body ShipmentEventInput, adminID *uuid.UUID) (*ShipmentEventResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	in, when, raw, err := normalizeShipmentEventInput(body)
	if err != nil {
		return nil, err
	}
	_, shipment, err := s.shipmentForOrder(c, orderID, shipmentID)
	if err != nil {
		return nil, err
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	result := &ShipmentEventResult{}
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var locked OrderShipment
		if err := tx.WithContext(c.Request.Context()).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND order_id = ?", shipmentID, orderID).First(&locked).Error; err != nil {
			return err
		}
		var existing OrderShipmentEvent
		findErr := tx.Where("shipment_id = ? AND event_key = ?", shipmentID, in.EventKey).First(&existing).Error
		if findErr == nil {
			if !sameShipmentEvent(&existing, in, when, raw) {
				return ErrShipmentEventConflict
			}
			result.Event = &existing
			result.Shipment = &locked
			result.Replay = true
			return nil
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return findErr
		}
		if !shipmentTransitionAllowed(locked.Status, in.Status) {
			if locked.Status == ShipmentDelivered || locked.Status == ShipmentReturned {
				return ErrShipmentEventImmutable
			}
			return ErrShipmentEventTransition
		}
		event := OrderShipmentEvent{TenantID: tid, OrderID: orderID, ShipmentID: shipmentID, EventKey: in.EventKey, Status: in.Status, OccurredAt: when, Location: in.Location, Description: in.Description, Source: in.Source, RawData: raw}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		updates := map[string]any{"status": in.Status, "updated_at": time.Now().UTC()}
		if in.Status == ShipmentShipped && locked.ShippedAt == nil {
			updates["shipped_at"] = when
		}
		if in.Status == ShipmentDelivered {
			updates["delivered_at"] = when
		}
		if err := tx.Model(&OrderShipment{}).Where("id = ? AND order_id = ?", shipmentID, orderID).Updates(updates).Error; err != nil {
			return err
		}
		if in.Status == ShipmentDelivered {
			if err := tx.Model(&Order{}).Where("id = ? AND tenant_id = ? AND status NOT IN ?", orderID, tid, []string{StatusCancelled, StatusRefunded, StatusClosed}).Updates(map[string]any{"status": StatusDelivered, "delivered_at": when, "updated_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
		locked.Status = in.Status
		if in.Status == ShipmentShipped && locked.ShippedAt == nil {
			locked.ShippedAt = &when
		}
		if in.Status == ShipmentDelivered {
			locked.DeliveredAt = &when
		}
		result.Event = &event
		result.Shipment = &locked
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !result.Replay && s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{TenantID: tid, AdminUserID: adminID, Action: "order.shipment.event.append", Resource: "order_shipment", ResourceID: shipmentID.String(), Permission: adminperm.PermOrderOperate, Status: "success", Message: fmt.Sprintf("orderId=%s shipmentId=%s eventKey=%s status=%s", orderID, shipmentID, in.EventKey, in.Status)})
	}
	_ = shipment
	return result, nil
}
