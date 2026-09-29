package order

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/supplier"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	CostSnapshotSourceSupplierCatalog = "supplier_catalog_at_fulfillment"
	maxSafeCostMinor                  = int64(9007199254740991)

	CostSnapshotResolved         = "resolved"
	CostSnapshotMissing          = "missing"
	CostSnapshotAmbiguous        = "ambiguous"
	CostSnapshotCurrencyMismatch = "currency_mismatch"
	CostSnapshotInvalid          = "invalid"
)

// OrderItemCostSnapshot is an immutable cost fact captured when local
// fulfillment succeeds. Unresolved catalog states are persisted instead of
// blocking shipment or being silently replaced with future prices.
type OrderItemCostSnapshot struct {
	model.HardDeleteBase
	TenantID         int64          `gorm:"not null;uniqueIndex:ux_order_item_cost_snapshot,priority:1;index" json:"tenantId"`
	OrderID          uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:ux_order_item_cost_snapshot,priority:2;index" json:"orderId"`
	OrderItemID      uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:ux_order_item_cost_snapshot,priority:3;index" json:"orderItemId"`
	ShipmentID       uuid.UUID      `gorm:"type:char(36);not null;index" json:"shipmentId"`
	WarehouseID      uuid.UUID      `gorm:"type:char(36);not null;index" json:"warehouseId"`
	ProductSKUID     uuid.UUID      `gorm:"column:product_sku_id;type:char(36);not null;index" json:"productSkuId"`
	Quantity         int            `gorm:"not null" json:"quantity"`
	OrderCurrency    string         `gorm:"size:8;not null" json:"orderCurrency"`
	UnitCostMinor    *int64         `json:"unitCostMinor"`
	LineCostMinor    *int64         `json:"lineCostMinor"`
	CostCurrency     string         `gorm:"size:8" json:"costCurrency,omitempty"`
	ResolutionStatus string         `gorm:"size:32;not null;index" json:"resolutionStatus"`
	ReasonCode       string         `gorm:"size:64;not null" json:"reasonCode"`
	SupplierID       *uuid.UUID     `gorm:"type:char(36);index" json:"supplierId,omitempty"`
	SupplierSKUID    *uuid.UUID     `gorm:"column:supplier_sku_id;type:char(36);index" json:"supplierSkuId,omitempty"`
	SupplierCode     string         `gorm:"size:64" json:"supplierCode,omitempty"`
	SupplierName     string         `gorm:"size:200" json:"supplierName,omitempty"`
	SupplierSKUCode  string         `gorm:"size:128" json:"supplierSkuCode,omitempty"`
	SourceType       string         `gorm:"size:64;not null" json:"sourceType"`
	SourceUpdatedAt  *time.Time     `json:"sourceUpdatedAt,omitempty"`
	CandidateCount   int            `gorm:"not null;default:0" json:"candidateCount"`
	SourceCandidates datatypes.JSON `gorm:"type:jsonb;not null" json:"sourceCandidates"`
	CapturedAt       time.Time      `gorm:"not null;index" json:"capturedAt"`
	CapturedBy       *uuid.UUID     `gorm:"type:char(36);index" json:"capturedBy,omitempty"`
}

func (OrderItemCostSnapshot) TableName() string { return "order_item_cost_snapshots" }

type costSnapshotSource struct {
	SupplierSKUID   uuid.UUID `json:"supplierSkuId"`
	SupplierID      uuid.UUID `json:"supplierId"`
	SupplierCode    string    `json:"supplierCode"`
	SupplierName    string    `json:"supplierName"`
	SupplierSKUCode string    `json:"supplierSkuCode,omitempty"`
	UnitCostMinor   int64     `json:"unitCostMinor"`
	Currency        string    `json:"currency"`
	SourceUpdatedAt time.Time `json:"sourceUpdatedAt"`
}

func (s *Service) captureFulfillmentCostSnapshots(ctx context.Context, tx *gorm.DB, tenantID int64, orderRow Order, shipment OrderShipment, actor *uuid.UUID, capturedAt time.Time) error {
	if tx == nil || tenantID < 0 || orderRow.ID == uuid.Nil || shipment.ID == uuid.Nil {
		return fmt.Errorf("invalid fulfillment cost snapshot context")
	}
	if s == nil || s.Suppliers == nil {
		return fmt.Errorf("supplier cost reader unavailable")
	}
	if orderRow.WarehouseID == nil || *orderRow.WarehouseID == uuid.Nil {
		return fmt.Errorf("fulfilled order warehouse is missing")
	}
	var items []OrderItem
	if err := tx.WithContext(ctx).Where("order_id = ?", orderRow.ID).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return fmt.Errorf("load fulfillment cost items: %w", err)
	}
	if len(items) == 0 {
		return ErrFulfillmentSKURequired
	}
	skuSet := make(map[uuid.UUID]struct{}, len(items))
	for _, item := range items {
		if item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil || item.Quantity < 1 {
			return ErrFulfillmentSKURequired
		}
		skuSet[*item.ProductSKUID] = struct{}{}
	}
	skuIDs := make([]uuid.UUID, 0, len(skuSet))
	for skuID := range skuSet {
		skuIDs = append(skuIDs, skuID)
	}
	sort.Slice(skuIDs, func(i, j int) bool { return skuIDs[i].String() < skuIDs[j].String() })
	costs, err := s.Suppliers.ListActiveCosts(ctx, tx, tenantID, skuIDs)
	if err != nil {
		return err
	}
	costsBySKU := make(map[uuid.UUID][]supplier.ActiveCostFact, len(skuIDs))
	for _, cost := range costs {
		costsBySKU[cost.ProductSKUID] = append(costsBySKU[cost.ProductSKUID], cost)
	}

	orderCurrency := strings.ToUpper(strings.TrimSpace(orderRow.Currency))
	snapshots := make([]OrderItemCostSnapshot, 0, len(items))
	for _, item := range items {
		candidates := costsBySKU[*item.ProductSKUID]
		sourceRows := make([]costSnapshotSource, 0, len(candidates))
		for _, candidate := range candidates {
			sourceRows = append(sourceRows, costSnapshotSource{
				SupplierSKUID: candidate.SupplierSKUID, SupplierID: candidate.SupplierID,
				SupplierCode: candidate.SupplierCode, SupplierName: candidate.SupplierName,
				SupplierSKUCode: candidate.SupplierSKUCode, UnitCostMinor: candidate.UnitCostMinor,
				Currency: strings.ToUpper(strings.TrimSpace(candidate.Currency)), SourceUpdatedAt: candidate.SourceUpdatedAt.UTC(),
			})
		}
		sourceJSON, marshalErr := json.Marshal(sourceRows)
		if marshalErr != nil {
			return fmt.Errorf("encode fulfillment cost sources for order item %s: %w", item.ID, marshalErr)
		}
		snapshot := OrderItemCostSnapshot{
			TenantID: tenantID, OrderID: orderRow.ID, OrderItemID: item.ID, ShipmentID: shipment.ID,
			WarehouseID: *orderRow.WarehouseID, ProductSKUID: *item.ProductSKUID, Quantity: item.Quantity,
			OrderCurrency: orderCurrency, ResolutionStatus: CostSnapshotMissing, ReasonCode: "supplier_cost_missing",
			SourceType: CostSnapshotSourceSupplierCatalog, CandidateCount: len(candidates), SourceCandidates: datatypes.JSON(sourceJSON),
			CapturedAt: capturedAt.UTC(), CapturedBy: actor,
		}
		if len(candidates) == 1 {
			applyCostSnapshotCandidate(&snapshot, candidates[0])
		} else if len(candidates) > 1 {
			snapshot.ResolutionStatus = CostSnapshotAmbiguous
			snapshot.ReasonCode = "multiple_supplier_costs"
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := tx.WithContext(ctx).Create(&snapshots).Error; err != nil {
		return fmt.Errorf("persist fulfillment cost snapshots: %w", err)
	}
	return nil
}

func applyCostSnapshotCandidate(snapshot *OrderItemCostSnapshot, candidate supplier.ActiveCostFact) {
	if snapshot == nil {
		return
	}
	snapshot.SupplierID = uuidPointer(candidate.SupplierID)
	snapshot.SupplierSKUID = uuidPointer(candidate.SupplierSKUID)
	snapshot.SupplierCode = candidate.SupplierCode
	snapshot.SupplierName = candidate.SupplierName
	snapshot.SupplierSKUCode = candidate.SupplierSKUCode
	snapshot.UnitCostMinor = int64Pointer(candidate.UnitCostMinor)
	snapshot.CostCurrency = strings.ToUpper(strings.TrimSpace(candidate.Currency))
	sourceUpdatedAt := candidate.SourceUpdatedAt.UTC()
	snapshot.SourceUpdatedAt = &sourceUpdatedAt
	if candidate.UnitCostMinor < 0 || candidate.UnitCostMinor > maxSafeCostMinor || snapshot.Quantity < 1 || candidate.UnitCostMinor > math.MaxInt64/int64(snapshot.Quantity) {
		snapshot.ResolutionStatus = CostSnapshotInvalid
		snapshot.ReasonCode = "product_cost_overflow"
		return
	}
	lineCost := candidate.UnitCostMinor * int64(snapshot.Quantity)
	if lineCost > maxSafeCostMinor {
		snapshot.ResolutionStatus = CostSnapshotInvalid
		snapshot.ReasonCode = "product_cost_overflow"
		return
	}
	snapshot.LineCostMinor = int64Pointer(lineCost)
	if snapshot.CostCurrency != snapshot.OrderCurrency {
		snapshot.ResolutionStatus = CostSnapshotCurrencyMismatch
		snapshot.ReasonCode = "supplier_cost_currency_mismatch"
		return
	}
	snapshot.ResolutionStatus = CostSnapshotResolved
	snapshot.ReasonCode = ""
}

func uuidPointer(value uuid.UUID) *uuid.UUID { return &value }

func int64Pointer(value int64) *int64 { return &value }
