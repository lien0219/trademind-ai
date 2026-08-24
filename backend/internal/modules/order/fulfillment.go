package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const orderFulfillmentScope = "order-fulfillment"

var (
	ErrFulfillmentIdempotencyRequired = errors.New("fulfillment idempotencyKey is required")
	ErrFulfillmentNotPaid             = errors.New("order must be paid before fulfillment")
	ErrFulfillmentAlreadyCompleted    = errors.New("order is already fulfilled")
	ErrFulfillmentSKURequired         = errors.New("all order items must be bound to a local SKU before fulfillment")
	ErrFulfillmentShipmentRequired    = errors.New("carrier and trackingNo are required")
	ErrFulfillmentInputTooLong        = errors.New("fulfillment input is too long")
	ErrFulfillmentTrackingURLInvalid  = errors.New("trackingUrl must use http or https")
	ErrFulfillmentInProgress          = errors.New("FULFILLMENT_IN_PROGRESS")
)

func fulfillmentOwner(actor *uuid.UUID) string {
	owner := "order-fulfillment"
	if actor != nil && *actor != uuid.Nil {
		owner += ":" + actor.String()
	}
	// Every HTTP attempt gets a distinct owner. Reusing an administrator ID
	// would let a concurrent request take over the same processing lease.
	return owner + ":" + uuid.NewString()
}

// FulfillOrderInput confirms one single-warehouse shipment. It deliberately
// does not contain platform or carrier credentials; the first version records
// a manually supplied package and keeps external integrations disabled.
type FulfillOrderInput struct {
	IdempotencyKey string     `json:"idempotencyKey"`
	WarehouseID    *uuid.UUID `json:"warehouseId,omitempty"`
	Carrier        string     `json:"carrier"`
	TrackingNo     string     `json:"trackingNo"`
	TrackingURL    string     `json:"trackingUrl,omitempty"`
}

type FulfillOrderResult struct {
	Order              *DetailDTO                  `json:"order"`
	Shipment           *OrderShipment              `json:"shipment"`
	InventoryDeduction *inventory.DeductionSummary `json:"inventoryDeduction"`
}

func fulfillmentHash(orderID uuid.UUID, in FulfillOrderInput) string {
	payload, _ := json.Marshal(struct {
		OrderID     string     `json:"orderId"`
		WarehouseID *uuid.UUID `json:"warehouseId,omitempty"`
		Carrier     string     `json:"carrier"`
		TrackingNo  string     `json:"trackingNo"`
		TrackingURL string     `json:"trackingUrl,omitempty"`
	}{
		OrderID: orderID.String(), WarehouseID: in.WarehouseID,
		Carrier: strings.TrimSpace(in.Carrier), TrackingNo: strings.TrimSpace(in.TrackingNo),
		TrackingURL: strings.TrimSpace(in.TrackingURL),
	})
	return idempotency.HashRequest(payload)
}

func findShipmentInDetail(detail *DetailDTO, shipmentID uuid.UUID) (*OrderShipment, bool) {
	if detail == nil {
		return nil, false
	}
	for i := range detail.Shipments {
		if detail.Shipments[i].ID == shipmentID {
			row := detail.Shipments[i]
			return &row, true
		}
	}
	return nil, false
}

func (s *Service) replayFulfillmentResult(c *gin.Context, orderID uuid.UUID, record *idempotency.Record) (*FulfillOrderResult, error) {
	if record == nil || strings.TrimSpace(record.ResourceID) == "" {
		return nil, fmt.Errorf("fulfillment idempotency record has no shipment")
	}
	shipmentID, err := uuid.Parse(record.ResourceID)
	if err != nil {
		return nil, fmt.Errorf("invalid fulfillment resource")
	}
	detail, err := s.Get(c, orderID)
	if err != nil {
		return nil, err
	}
	shipment, ok := findShipmentInDetail(detail, shipmentID)
	if !ok {
		return nil, fmt.Errorf("fulfilled shipment is missing")
	}
	return &FulfillOrderResult{Order: detail, Shipment: shipment}, nil
}

// FulfillOrder atomically confirms a package, updates the order lifecycle and
// deducts warehouse stock. The caller-generated idempotency record is completed
// inside the same database transaction as the ledger and shipment facts.
func (s *Service) FulfillOrder(c *gin.Context, inv *inventory.Service, orderID uuid.UUID, in FulfillOrderInput, actor *uuid.UUID) (*FulfillOrderResult, error) {
	return s.fulfillOrder(c, inv, orderID, in, actor, nil)
}

func (s *Service) fulfillOrderForWave(c *gin.Context, inv *inventory.Service, orderID uuid.UUID, in FulfillOrderInput, actor *uuid.UUID, waveID uuid.UUID) (*FulfillOrderResult, error) {
	return s.fulfillOrder(c, inv, orderID, in, actor, &waveID)
}

func (s *Service) fulfillOrder(c *gin.Context, inv *inventory.Service, orderID uuid.UUID, in FulfillOrderInput, actor *uuid.UUID, waveID *uuid.UUID) (*FulfillOrderResult, error) {
	if s == nil || s.DB == nil || inv == nil {
		return nil, fmt.Errorf("fulfillment unavailable")
	}
	if s.Idempotency == nil || s.Idempotency.DB == nil {
		return nil, fmt.Errorf("fulfillment idempotency unavailable")
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return nil, ErrFulfillmentIdempotencyRequired
	}
	// The persisted idempotency key also contains the order scope and UUID.
	// Keep the caller portion bounded so the composite stays within the
	// idempotency_records.idempotency_key VARCHAR(255) column.
	if len(key) > 128 {
		return nil, ErrFulfillmentInputTooLong
	}
	carrier := strings.TrimSpace(in.Carrier)
	trackingNo := strings.TrimSpace(in.TrackingNo)
	trackingURL := strings.TrimSpace(in.TrackingURL)
	if carrier == "" || trackingNo == "" {
		return nil, ErrFulfillmentShipmentRequired
	}
	if len(carrier) > 128 || len(trackingNo) > 255 {
		return nil, ErrFulfillmentInputTooLong
	}
	if len(trackingURL) > 2048 {
		return nil, ErrFulfillmentInputTooLong
	}
	if trackingURL != "" {
		parsed, parseErr := url.ParseRequestURI(trackingURL)
		if parseErr != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return nil, ErrFulfillmentTrackingURLInvalid
		}
	}

	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	if err := s.requireFulfillmentWaveAssignment(c.Request.Context(), s.DB, tenantID, orderID, waveID); err != nil {
		return nil, err
	}
	owner := fulfillmentOwner(actor)
	idemKey := idempotency.OrderFulfillment(orderID.String(), key)
	requestHash := fulfillmentHash(orderID, in)
	orderRow, err := s.findOrderBare(c, orderID)
	if err != nil {
		return nil, err
	}
	if existing, getErr := s.Idempotency.Get(c.Request.Context(), orderFulfillmentScope, idemKey); getErr == nil {
		if existing.RequestHash != requestHash {
			return nil, idempotency.ErrKeyConflict
		}
		switch existing.Status {
		case idempotency.StatusSucceeded:
			return s.replayFulfillmentResult(c, orderID, existing)
		case idempotency.StatusProcessing:
			return nil, ErrFulfillmentInProgress
		}
	} else if !errors.Is(getErr, gorm.ErrRecordNotFound) {
		return nil, getErr
	}
	if orderRow.PaymentStatus != PaymentPaid {
		return nil, ErrFulfillmentNotPaid
	}
	if orderRow.Status == StatusCancelled || orderRow.Status == StatusRefunded || orderRow.Status == StatusClosed || orderRow.FulfillmentStatus == FulfillmentFulfilled {
		return nil, ErrFulfillmentAlreadyCompleted
	}
	var items []OrderItem
	if err := s.DB.WithContext(c.Request.Context()).Where("order_id = ?", orderID).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrFulfillmentSKURequired
	}
	for _, item := range items {
		if item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil || item.Quantity <= 0 {
			return nil, ErrFulfillmentSKURequired
		}
	}
	var deducted int64
	if err := s.DB.WithContext(c.Request.Context()).Model(&inventory.OrderInventoryEffect{}).
		Where("tenant_id = ? AND order_id = ? AND effect_type = ? AND status = ?", tenantID, orderID, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess).
		Count(&deducted).Error; err != nil {
		return nil, err
	}
	if deducted > 0 {
		return nil, ErrFulfillmentAlreadyCompleted
	}
	if in.WarehouseID != nil && *in.WarehouseID != uuid.Nil && orderRow.WarehouseID != nil && *orderRow.WarehouseID != uuid.Nil && *in.WarehouseID != *orderRow.WarehouseID {
		return nil, inventory.ErrOrderWarehouseConflict
	}

	acquired, acquireErr := s.Idempotency.Acquire(c.Request.Context(), orderFulfillmentScope, idemKey, requestHash, owner, idempotency.DefaultLease)
	decision, record, classifyErr := idempotency.Classify(acquired, acquireErr)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return s.replayFulfillmentResult(c, orderID, record)
	case idempotency.DecisionInProgress:
		return nil, ErrFulfillmentInProgress
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		if classifyErr != nil {
			return nil, classifyErr
		}
		return nil, idempotency.ErrKeyConflict
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if acquired == nil || acquired.Record == nil {
			return nil, fmt.Errorf("fulfillment idempotency record missing")
		}
	default:
		if classifyErr != nil {
			return nil, classifyErr
		}
		return nil, fmt.Errorf("fulfillment idempotency unavailable")
	}

	var shipment OrderShipment
	var deduction *inventory.DeductionSummary
	now := time.Now().UTC()
	deduction, err = inv.DeductInventoryForOrder(c.Request.Context(), orderID, inventory.OrderInventoryOptions{
		Reason:                 "fulfillment_ship_confirm",
		CreatedBy:              actor,
		WarehouseID:            in.WarehouseID,
		TenantID:               &tenantID,
		ForceDeduct:            true,
		RequireAllLinesApplied: true,
		AfterApply: func(tx *gorm.DB, action string) error {
			if action != inventory.EffectTypeDeduct {
				return ErrFulfillmentAlreadyCompleted
			}
			if err := s.requireFulfillmentWaveAssignment(c.Request.Context(), tx, tenantID, orderID, waveID); err != nil {
				return err
			}
			var locked Order
			if err := tx.WithContext(c.Request.Context()).Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", orderID, tenantID).First(&locked).Error; err != nil {
				return err
			}
			if locked.PaymentStatus != PaymentPaid {
				return ErrFulfillmentNotPaid
			}
			if locked.Status == StatusShipped || locked.Status == StatusDelivered ||
				locked.Status == StatusCancelled || locked.Status == StatusRefunded || locked.Status == StatusClosed ||
				locked.FulfillmentStatus == FulfillmentFulfilled {
				return ErrFulfillmentAlreadyCompleted
			}
			shipment = OrderShipment{
				OrderID: orderID, Carrier: carrier, TrackingNo: trackingNo, TrackingURL: trackingURL,
				Status: ShipmentShipped, ShippedAt: &now,
			}
			if err := tx.WithContext(c.Request.Context()).Create(&shipment).Error; err != nil {
				return err
			}
			if err := tx.Model(&Order{}).Where("id = ? AND tenant_id = ?", orderID, tenantID).Updates(map[string]any{
				"status": StatusShipped, "fulfillment_status": FulfillmentFulfilled, "shipped_at": now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			summary, _ := json.Marshal(map[string]string{"orderId": orderID.String(), "shipmentId": shipment.ID.String()})
			return s.Idempotency.WithDB(tx).Complete(c.Request.Context(), acquired.Record.ID, owner, idempotency.CompleteResult{
				ResponseCode: "ORDER_FULFILLMENT_SUCCESS", ResponseSummary: string(summary), ResourceType: "order_shipment", ResourceID: shipment.ID.String(),
			})
		},
	})
	if err != nil && shipment.ID == uuid.Nil {
		_ = s.Idempotency.Fail(c.Request.Context(), acquired.Record.ID, owner, err.Error(), true)
		return nil, err
	}
	if shipment.ID == uuid.Nil {
		// ForceDeduct can legitimately observe a terminal/unpaid order after
		// the initial preflight and return a skipped inventory action. Convert
		// that race into a stable business conflict instead of a misleading 500.
		fallbackErr := fmt.Errorf("fulfillment completed without shipment")
		if deduction != nil && deduction.Skipped {
			current, currentErr := s.findOrderBare(c, orderID)
			if currentErr != nil {
				fallbackErr = currentErr
			} else if current.PaymentStatus != PaymentPaid {
				fallbackErr = ErrFulfillmentNotPaid
			} else if current.Status == StatusCancelled || current.Status == StatusRefunded ||
				current.Status == StatusClosed || current.Status == StatusShipped ||
				current.Status == StatusDelivered || current.FulfillmentStatus == FulfillmentFulfilled {
				fallbackErr = ErrFulfillmentAlreadyCompleted
			} else {
				fallbackErr = inventory.ErrOrderInventoryState
			}
		}
		_ = s.Idempotency.Fail(c.Request.Context(), acquired.Record.ID, owner, fallbackErr.Error(), true)
		return nil, fallbackErr
	}
	detail, err := s.Get(c, orderID)
	if err != nil {
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationLogForFulfillment(actor, orderID, shipment.ID))
	}
	return &FulfillOrderResult{Order: detail, Shipment: &shipment, InventoryDeduction: deduction}, nil
}

func (s *Service) requireFulfillmentWaveAssignment(ctx context.Context, tx *gorm.DB, tenantID int64, orderID uuid.UUID, waveID *uuid.UUID) error {
	if tx == nil {
		return fmt.Errorf("fulfillment assignment unavailable")
	}
	var assignment FulfillmentWaveAssignment
	err := tx.WithContext(ctx).Where("tenant_id = ? AND order_id = ?", tenantID, orderID).First(&assignment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if waveID != nil {
			return ErrFulfillmentWaveRequired
		}
		return nil
	}
	if err != nil {
		return err
	}
	if waveID == nil || *waveID == uuid.Nil || assignment.WaveID != *waveID {
		return ErrFulfillmentWaveRequired
	}
	return nil
}

func operationLogForFulfillment(actor *uuid.UUID, orderID, shipmentID uuid.UUID) operationlog.WriteOpts {
	return operationlog.WriteOpts{
		AdminUserID: actor, Action: "order.fulfillment.complete", Resource: "order", ResourceID: orderID.String(), Status: "success",
		Message: fmt.Sprintf("orderId=%s shipmentId=%s", orderID.String(), shipmentID.String()),
	}
}
