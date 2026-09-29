package order

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

const maxBatchFulfillmentItems = 100

var (
	ErrBatchFulfillmentInputInvalid = errors.New("batch fulfillment payload is invalid")
	ErrBatchFulfillmentNotAllocated = errors.New("order must be allocated before batch fulfillment")
)

const (
	BatchFulfillmentSucceeded  = "succeeded"
	BatchFulfillmentBlocked    = "blocked"
	BatchFulfillmentInProgress = "in_progress"
	BatchFulfillmentFailed     = "failed"
)

// BatchFulfillOrdersInput is intentionally a transport-level batch. Each item
// is still executed through FulfillOrder, so inventory, shipment and
// per-order idempotency remain atomic and no background worker is introduced.
type BatchFulfillOrdersInput struct {
	BatchIdempotencyKey string                  `json:"batchIdempotencyKey"`
	Items               []BatchFulfillOrderItem `json:"items"`
}

type BatchFulfillOrderItem struct {
	OrderID     uuid.UUID  `json:"orderId"`
	WarehouseID *uuid.UUID `json:"warehouseId,omitempty"`
	Carrier     string     `json:"carrier"`
	TrackingNo  string     `json:"trackingNo"`
	TrackingURL string     `json:"trackingUrl,omitempty"`
}

type BatchFulfillmentItemResult struct {
	OrderID     uuid.UUID      `json:"orderId"`
	OrderNo     string         `json:"orderNo"`
	WarehouseID *uuid.UUID     `json:"warehouseId,omitempty"`
	Status      string         `json:"status"`
	Shipment    *OrderShipment `json:"shipment,omitempty"`
	Error       string         `json:"error,omitempty"`
}

type BatchFulfillmentSummary struct {
	Requested  int `json:"requested"`
	Succeeded  int `json:"succeeded"`
	Blocked    int `json:"blocked"`
	InProgress int `json:"inProgress"`
	Failed     int `json:"failed"`
}

type BatchPickListLine struct {
	WarehouseID   uuid.UUID `json:"warehouseId"`
	WarehouseCode string    `json:"warehouseCode,omitempty"`
	WarehouseName string    `json:"warehouseName,omitempty"`
	ProductSKUID  uuid.UUID `json:"productSkuId"`
	SKUCode       string    `json:"skuCode,omitempty"`
	SKUName       string    `json:"skuName,omitempty"`
	ProductTitle  string    `json:"productTitle,omitempty"`
	Quantity      int       `json:"quantity"`
	OrderCount    int       `json:"orderCount"`
}

type BatchFulfillmentResult struct {
	BatchIdempotencyKey string                       `json:"batchIdempotencyKey"`
	Summary             BatchFulfillmentSummary      `json:"summary"`
	Items               []BatchFulfillmentItemResult `json:"items"`
	PickList            []BatchPickListLine          `json:"pickList"`
}

type batchPickListAggregate struct {
	line     BatchPickListLine
	orderIDs map[uuid.UUID]struct{}
}

func batchFulfillmentItemKey(batchKey string, orderID uuid.UUID) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(batchKey) + "|" + orderID.String()))
	return "batch-fulfillment-" + hex.EncodeToString(sum[:])
}

func validateBatchFulfillmentInput(in BatchFulfillOrdersInput) (string, error) {
	key := strings.TrimSpace(in.BatchIdempotencyKey)
	if key == "" || len(key) > 128 || len(in.Items) == 0 || len(in.Items) > maxBatchFulfillmentItems {
		return "", ErrBatchFulfillmentInputInvalid
	}
	seen := make(map[uuid.UUID]struct{}, len(in.Items))
	for _, item := range in.Items {
		if item.OrderID == uuid.Nil {
			return "", ErrBatchFulfillmentInputInvalid
		}
		if _, ok := seen[item.OrderID]; ok {
			return "", ErrBatchFulfillmentInputInvalid
		}
		seen[item.OrderID] = struct{}{}
	}
	return key, nil
}

func batchFulfillmentErrorStatus(err error) string {
	if errors.Is(err, ErrFulfillmentInProgress) {
		return BatchFulfillmentInProgress
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return BatchFulfillmentBlocked
	}
	switch {
	case errors.Is(err, ErrBatchFulfillmentNotAllocated),
		errors.Is(err, ErrFulfillmentNotPaid),
		errors.Is(err, ErrFulfillmentAlreadyCompleted),
		errors.Is(err, ErrFulfillmentSKURequired),
		errors.Is(err, ErrFulfillmentShipmentRequired),
		errors.Is(err, ErrFulfillmentInputTooLong),
		errors.Is(err, ErrFulfillmentTrackingURLInvalid),
		errors.Is(err, inventory.ErrInsufficientSKUStock),
		errors.Is(err, inventory.ErrOrderInventoryState),
		errors.Is(err, inventory.ErrOrderWarehouseConflict),
		errors.Is(err, idempotency.ErrKeyConflict):
		return BatchFulfillmentBlocked
	default:
		return BatchFulfillmentFailed
	}
}

func batchFulfillmentErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "order not found"
	}
	if batchFulfillmentErrorStatus(err) == BatchFulfillmentFailed {
		return "fulfillment failed"
	}
	return err.Error()
}

func (s *Service) buildBatchPickList(c *gin.Context, tenantID int64, results []BatchFulfillmentItemResult) []BatchPickListLine {
	warehouses := make(map[uuid.UUID]warehouse.Warehouse)
	warehouseIDs := make([]uuid.UUID, 0)
	seenWarehouses := make(map[uuid.UUID]struct{})
	warehouseByOrder := make(map[uuid.UUID]uuid.UUID)
	orderIDs := make([]uuid.UUID, 0)
	for _, result := range results {
		if result.Status != BatchFulfillmentSucceeded || result.WarehouseID == nil || *result.WarehouseID == uuid.Nil {
			continue
		}
		warehouseByOrder[result.OrderID] = *result.WarehouseID
		orderIDs = append(orderIDs, result.OrderID)
		if _, ok := seenWarehouses[*result.WarehouseID]; !ok {
			seenWarehouses[*result.WarehouseID] = struct{}{}
			warehouseIDs = append(warehouseIDs, *result.WarehouseID)
		}
	}
	if len(warehouseIDs) > 0 {
		var rows []warehouse.Warehouse
		if err := s.DB.WithContext(c.Request.Context()).Where("tenant_id = ? AND id IN ?", tenantID, warehouseIDs).Find(&rows).Error; err == nil {
			for _, row := range rows {
				warehouses[row.ID] = row
			}
		}
	}

	aggregates := make(map[string]*batchPickListAggregate)
	if len(orderIDs) > 0 {
		var items []OrderItem
		if err := s.DB.WithContext(c.Request.Context()).Where("order_id IN ?", orderIDs).Order("order_id ASC, created_at ASC, id ASC").Find(&items).Error; err == nil {
			for _, item := range items {
				warehouseID, ok := warehouseByOrder[item.OrderID]
				if !ok || item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil || item.Quantity <= 0 {
					continue
				}
				wh := warehouses[warehouseID]
				key := warehouseID.String() + ":" + item.ProductSKUID.String()
				agg := aggregates[key]
				if agg == nil {
					agg = &batchPickListAggregate{
						line: BatchPickListLine{
							WarehouseID: warehouseID, WarehouseCode: wh.Code, WarehouseName: wh.Name,
							ProductSKUID: *item.ProductSKUID, SKUCode: strings.TrimSpace(item.SKUCode),
							SKUName: strings.TrimSpace(item.SKUName), ProductTitle: strings.TrimSpace(item.ProductTitle),
						},
						orderIDs: make(map[uuid.UUID]struct{}),
					}
					aggregates[key] = agg
				}
				agg.line.Quantity += item.Quantity
				agg.orderIDs[item.OrderID] = struct{}{}
				agg.line.OrderCount = len(agg.orderIDs)
			}
		}
	}

	lines := make([]BatchPickListLine, 0, len(aggregates))
	for _, agg := range aggregates {
		lines = append(lines, agg.line)
	}
	sort.Slice(lines, func(i, j int) bool {
		if lines[i].WarehouseCode != lines[j].WarehouseCode {
			return lines[i].WarehouseCode < lines[j].WarehouseCode
		}
		if lines[i].SKUCode != lines[j].SKUCode {
			return lines[i].SKUCode < lines[j].SKUCode
		}
		return lines[i].ProductSKUID.String() < lines[j].ProductSKUID.String()
	})
	return lines
}

// FulfillOrders executes each already-allocated order sequentially. The
// result intentionally reports item-level blocks so an operator can correct
// exceptions without losing successful shipments from the same batch.
func (s *Service) FulfillOrders(c *gin.Context, inv *inventory.Service, in BatchFulfillOrdersInput, actor *uuid.UUID) (*BatchFulfillmentResult, error) {
	if s == nil || s.DB == nil || inv == nil {
		return nil, fmt.Errorf("batch fulfillment unavailable")
	}
	key, err := validateBatchFulfillmentInput(in)
	if err != nil {
		return nil, err
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	result := &BatchFulfillmentResult{
		BatchIdempotencyKey: key,
		Summary:             BatchFulfillmentSummary{Requested: len(in.Items)},
		Items:               make([]BatchFulfillmentItemResult, 0, len(in.Items)),
	}
	for _, item := range in.Items {
		row, findErr := s.findOrderBare(c, item.OrderID)
		if findErr != nil {
			status := batchFulfillmentErrorStatus(findErr)
			result.Items = append(result.Items, BatchFulfillmentItemResult{OrderID: item.OrderID, Status: status, Error: batchFulfillmentErrorMessage(findErr)})
			continue
		}
		if row.WarehouseID == nil || *row.WarehouseID == uuid.Nil {
			result.Items = append(result.Items, BatchFulfillmentItemResult{OrderID: item.OrderID, OrderNo: row.OrderNo, Status: BatchFulfillmentBlocked, Error: ErrBatchFulfillmentNotAllocated.Error()})
			continue
		}
		if item.WarehouseID != nil && *item.WarehouseID != uuid.Nil && *item.WarehouseID != *row.WarehouseID {
			result.Items = append(result.Items, BatchFulfillmentItemResult{OrderID: item.OrderID, OrderNo: row.OrderNo, WarehouseID: row.WarehouseID, Status: BatchFulfillmentBlocked, Error: inventory.ErrOrderWarehouseConflict.Error()})
			continue
		}
		fulfilled, fulfillErr := s.FulfillOrder(c, inv, item.OrderID, FulfillOrderInput{
			IdempotencyKey: batchFulfillmentItemKey(key, item.OrderID), WarehouseID: row.WarehouseID,
			Carrier: strings.TrimSpace(item.Carrier), TrackingNo: strings.TrimSpace(item.TrackingNo), TrackingURL: strings.TrimSpace(item.TrackingURL),
		}, actor)
		if fulfillErr != nil {
			result.Items = append(result.Items, BatchFulfillmentItemResult{
				OrderID: item.OrderID, OrderNo: row.OrderNo, WarehouseID: row.WarehouseID,
				Status: batchFulfillmentErrorStatus(fulfillErr), Error: batchFulfillmentErrorMessage(fulfillErr),
			})
			continue
		}
		result.Items = append(result.Items, BatchFulfillmentItemResult{
			OrderID: item.OrderID, OrderNo: row.OrderNo, WarehouseID: row.WarehouseID,
			Status: BatchFulfillmentSucceeded, Shipment: fulfilled.Shipment,
		})
	}
	for _, item := range result.Items {
		switch item.Status {
		case BatchFulfillmentSucceeded:
			result.Summary.Succeeded++
		case BatchFulfillmentBlocked:
			result.Summary.Blocked++
		case BatchFulfillmentInProgress:
			result.Summary.InProgress++
		case BatchFulfillmentFailed:
			result.Summary.Failed++
		}
	}
	result.PickList = s.buildBatchPickList(c, tenantID, result.Items)
	return result, nil
}
