package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	fulfillmentWaveCreateScope   = "fulfillment-wave-create"
	fulfillmentWaveCompleteScope = "fulfillment-wave-complete"
	maxFulfillmentWaveOrders     = 50
)

var (
	ErrFulfillmentWaveInvalidInput      = errors.New("invalid fulfillment wave input")
	ErrFulfillmentWaveNotFound          = errors.New("fulfillment wave not found")
	ErrFulfillmentWaveState             = errors.New("fulfillment wave state conflict")
	ErrFulfillmentWaveRevision          = errors.New("fulfillment wave revision conflict")
	ErrFulfillmentWaveIdempotency       = errors.New("fulfillment wave idempotency conflict")
	ErrFulfillmentWaveOrderUnavailable  = errors.New("order is not eligible for a fulfillment wave")
	ErrFulfillmentWaveOrderAssigned     = errors.New("order already belongs to an active fulfillment wave")
	ErrFulfillmentWaveReservation       = errors.New("order inventory reservation is incomplete")
	ErrFulfillmentWavePickIncomplete    = errors.New("picking result is incomplete")
	ErrFulfillmentWavePackingIncomplete = errors.New("packing review is incomplete")
	ErrFulfillmentWaveScanMismatch      = errors.New("fulfillment wave scan does not match expected sku or location")
	ErrFulfillmentWaveCompleting        = errors.New("fulfillment wave completion is in progress")
	ErrFulfillmentWaveRequired          = errors.New("order must be fulfilled from its active fulfillment wave")
	ErrFulfillmentWaveStorePermission   = errors.New("fulfillment wave store operation permission denied")
)

type CreateFulfillmentWaveInput struct {
	IdempotencyKey string      `json:"idempotencyKey"`
	WarehouseID    uuid.UUID   `json:"warehouseId"`
	OrderIDs       []uuid.UUID `json:"orderIds"`
	Remark         string      `json:"remark,omitempty"`
}

type FulfillmentWaveListQuery struct {
	Page        int
	PageSize    int
	Keyword     string
	Status      string
	WarehouseID *uuid.UUID
}

type FulfillmentWaveListResult struct {
	List       []FulfillmentWave `json:"list"`
	Pagination struct {
		Page       int   `json:"page"`
		PageSize   int   `json:"pageSize"`
		Total      int64 `json:"total"`
		TotalPages int   `json:"totalPages"`
	} `json:"pagination"`
}

type FulfillmentWaveRevisionInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

type FulfillmentWavePickLineInput struct {
	LineID              uuid.UUID `json:"lineId"`
	PickedQty           int       `json:"pickedQuantity"`
	ShortageQty         int       `json:"shortageQuantity"`
	ScannedBarcode      string    `json:"scannedBarcode,omitempty"`
	ScannedLocationCode string    `json:"scannedLocationCode,omitempty"`
}

type RecordFulfillmentWavePickInput struct {
	ExpectedRevision int                            `json:"expectedRevision"`
	IdempotencyKey   string                         `json:"idempotencyKey"`
	Lines            []FulfillmentWavePickLineInput `json:"lines"`
}

type PackFulfillmentWaveOrderInput struct {
	ExpectedRevision int    `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
	Carrier          string `json:"carrier"`
	TrackingNo       string `json:"trackingNo"`
	TrackingURL      string `json:"trackingUrl,omitempty"`
}

type CompleteFulfillmentWaveResult struct {
	Wave      *FulfillmentWave              `json:"wave"`
	Processed int                           `json:"processed"`
	Succeeded int                           `json:"succeeded"`
	Failed    int                           `json:"failed"`
	Items     []CompleteFulfillmentWaveItem `json:"items"`
}

type CompleteFulfillmentWaveItem struct {
	OrderID       uuid.UUID  `json:"orderId"`
	OrderNo       string     `json:"orderNo"`
	Status        string     `json:"status"`
	ShipmentID    *uuid.UUID `json:"shipmentId,omitempty"`
	FailureReason string     `json:"failureReason,omitempty"`
}

func fulfillmentWaveOwner(prefix string, actor *uuid.UUID) string {
	owner := prefix
	if actor != nil && *actor != uuid.Nil {
		owner += ":" + actor.String()
	}
	return owner + ":" + uuid.NewString()
}

func normalizeWaveKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if len(key) < 8 || len(key) > 128 {
		return "", ErrFulfillmentWaveInvalidInput
	}
	return key, nil
}

func clampWaveText(value string, max int) string {
	value = strings.TrimSpace(value)
	chars := []rune(value)
	if len(chars) > max {
		return string(chars[:max])
	}
	return value
}

func fulfillmentWavePayloadHash(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal fulfillment wave payload: %w", err)
	}
	return idempotency.HashRequest(payload), nil
}

func fulfillmentWaveNo(now time.Time) string {
	return fmt.Sprintf("FW%s-%s", now.UTC().Format("20060102"), strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:10]))
}

func isFulfillmentWaveUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") || strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "sqlstate 23505") || strings.Contains(message, "error 1062")
}

func validFulfillmentWaveStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "", FulfillmentWaveDraft, FulfillmentWavePicking, FulfillmentWavePacking,
		FulfillmentWaveCompleting, FulfillmentWavePartial, FulfillmentWaveCompleted, FulfillmentWaveCancelled:
		return true
	default:
		return false
	}
}

func waveScopedQuery(principal *adminperm.Principal, tx *gorm.DB) *gorm.DB {
	if tx == nil || principal == nil || principal.IsAdmin() {
		return tx
	}
	allowed := principal.AllowedStoreIDs()
	if len(allowed) == 0 {
		return tx.Where("1 = 0")
	}
	return tx.Where(`NOT EXISTS (
		SELECT 1 FROM fulfillment_wave_orders fwo
		WHERE fwo.wave_id = fulfillment_waves.id
		AND (fwo.shop_id IS NULL OR fwo.shop_id NOT IN ?)
	)`, allowed)
}

func principalCanViewWaveOrder(principal *adminperm.Principal, shopID *uuid.UUID) bool {
	if principal == nil || principal.IsAdmin() {
		return true
	}
	return shopID != nil && *shopID != uuid.Nil && principal.CanViewStore(*shopID)
}

func principalCanOperateWaveOrder(principal *adminperm.Principal, shopID *uuid.UUID) bool {
	if principal == nil || principal.IsAdmin() {
		return true
	}
	return shopID != nil && *shopID != uuid.Nil && principal.CanOperateStore(*shopID)
}

func requireFulfillmentWaveOperate(principal *adminperm.Principal, wave *FulfillmentWave) error {
	if wave == nil {
		return ErrFulfillmentWaveNotFound
	}
	for _, waveOrder := range wave.Orders {
		if !principalCanOperateWaveOrder(principal, waveOrder.ShopID) {
			return ErrFulfillmentWaveStorePermission
		}
	}
	return nil
}

func (s *Service) loadFulfillmentWave(ctx context.Context, tenantID int64, principal *adminperm.Principal, id uuid.UUID, lock bool) (*FulfillmentWave, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil {
		return nil, ErrFulfillmentWaveNotFound
	}
	q := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var wave FulfillmentWave
	if err := q.First(&wave).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFulfillmentWaveNotFound
		}
		return nil, err
	}
	var orders []FulfillmentWaveOrder
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND wave_id = ?", tenantID, id).Order("created_at ASC, id ASC").Find(&orders).Error; err != nil {
		return nil, err
	}
	for i := range orders {
		if !principalCanViewWaveOrder(principal, orders[i].ShopID) {
			return nil, ErrFulfillmentWaveNotFound
		}
	}
	var lines []FulfillmentWaveLine
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND wave_id = ?", tenantID, id).Order("sku_code ASC, order_id ASC, id ASC").Find(&lines).Error; err != nil {
		return nil, err
	}
	var scans []FulfillmentWavePickScan
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND wave_id = ?", tenantID, id).Order("created_at ASC, id ASC").Find(&scans).Error; err != nil {
		return nil, err
	}
	wave.Orders = orders
	wave.Lines = lines
	wave.PickScans = scans
	var warehouseLabel struct {
		Code string
		Name string
	}
	if err := s.DB.WithContext(ctx).Table("warehouses").Select("code, name").Where("tenant_id = ? AND id = ?", tenantID, wave.WarehouseID).Scan(&warehouseLabel).Error; err != nil {
		return nil, err
	}
	wave.WarehouseCode = warehouseLabel.Code
	wave.WarehouseName = warehouseLabel.Name
	return &wave, nil
}

func (s *Service) GetFulfillmentWave(ctx context.Context, tenantID int64, principal *adminperm.Principal, id uuid.UUID) (*FulfillmentWave, error) {
	return s.loadFulfillmentWave(ctx, tenantID, principal, id, false)
}

func (s *Service) ListFulfillmentWaves(ctx context.Context, tenantID int64, principal *adminperm.Principal, in FulfillmentWaveListQuery) (*FulfillmentWaveListResult, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.PageSize < 1 || in.PageSize > 100 {
		in.PageSize = 20
	}
	status := strings.TrimSpace(in.Status)
	if !validFulfillmentWaveStatus(status) {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	q := s.DB.WithContext(ctx).Model(&FulfillmentWave{}).Where("tenant_id = ?", tenantID)
	q = waveScopedQuery(principal, q)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if in.WarehouseID != nil && *in.WarehouseID != uuid.Nil {
		q = q.Where("warehouse_id = ?", *in.WarehouseID)
	}
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("LOWER(wave_no) LIKE LOWER(?) OR EXISTS (SELECT 1 FROM fulfillment_wave_orders fwo WHERE fwo.wave_id = fulfillment_waves.id AND LOWER(fwo.order_no) LIKE LOWER(?))", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []FulfillmentWave
	if err := q.Order("created_at DESC, id DESC").Offset((in.Page - 1) * in.PageSize).Limit(in.PageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		warehouseIDs := make([]uuid.UUID, 0, len(rows))
		seen := map[uuid.UUID]struct{}{}
		for _, row := range rows {
			if _, ok := seen[row.WarehouseID]; !ok {
				seen[row.WarehouseID] = struct{}{}
				warehouseIDs = append(warehouseIDs, row.WarehouseID)
			}
		}
		var labels []struct {
			ID   uuid.UUID
			Code string
			Name string
		}
		if err := s.DB.WithContext(ctx).Table("warehouses").Select("id, code, name").Where("tenant_id = ? AND id IN ?", tenantID, warehouseIDs).Scan(&labels).Error; err != nil {
			return nil, err
		}
		byID := make(map[uuid.UUID]struct{ Code, Name string }, len(labels))
		for _, label := range labels {
			byID[label.ID] = struct{ Code, Name string }{label.Code, label.Name}
		}
		for i := range rows {
			rows[i].WarehouseCode = byID[rows[i].WarehouseID].Code
			rows[i].WarehouseName = byID[rows[i].WarehouseID].Name
		}
	}
	out := &FulfillmentWaveListResult{List: rows}
	out.Pagination.Page = in.Page
	out.Pagination.PageSize = in.PageSize
	out.Pagination.Total = total
	out.Pagination.TotalPages = int((total + int64(in.PageSize) - 1) / int64(in.PageSize))
	return out, nil
}

func (s *Service) replayCreatedFulfillmentWave(ctx context.Context, tenantID int64, principal *adminperm.Principal, record *idempotency.Record) (*FulfillmentWave, error) {
	if record == nil {
		return nil, ErrFulfillmentWaveIdempotency
	}
	id, err := uuid.Parse(strings.TrimSpace(record.ResourceID))
	if err != nil || id == uuid.Nil {
		return nil, ErrFulfillmentWaveIdempotency
	}
	return s.GetFulfillmentWave(ctx, tenantID, principal, id)
}

func (s *Service) CreateFulfillmentWave(ctx context.Context, tenantID int64, principal *adminperm.Principal, actor *uuid.UUID, in CreateFulfillmentWaveInput) (*FulfillmentWave, error) {
	if s == nil || s.DB == nil || s.Idempotency == nil || s.Idempotency.DB == nil || s.Warehouses == nil || tenantID < 0 || in.WarehouseID == uuid.Nil {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	key, err := normalizeWaveKey(in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	if len(in.OrderIDs) == 0 || len(in.OrderIDs) > maxFulfillmentWaveOrders {
		return nil, ErrFulfillmentWaveInvalidInput
	}
	orderIDs := append([]uuid.UUID(nil), in.OrderIDs...)
	sort.Slice(orderIDs, func(i, j int) bool { return orderIDs[i].String() < orderIDs[j].String() })
	for i, id := range orderIDs {
		if id == uuid.Nil || (i > 0 && id == orderIDs[i-1]) {
			return nil, ErrFulfillmentWaveInvalidInput
		}
	}
	remark := clampWaveText(in.Remark, 500)
	hash, err := fulfillmentWavePayloadHash(struct {
		TenantID    int64       `json:"tenantId"`
		WarehouseID uuid.UUID   `json:"warehouseId"`
		OrderIDs    []uuid.UUID `json:"orderIds"`
		Remark      string      `json:"remark"`
	}{tenantID, in.WarehouseID, orderIDs, remark})
	if err != nil {
		return nil, err
	}
	idemKey := fmt.Sprintf("%d:%s", tenantID, key)
	owner := fulfillmentWaveOwner("fulfillment-wave-create", actor)
	acquired, acquireErr := s.Idempotency.Acquire(ctx, fulfillmentWaveCreateScope, idemKey, hash, owner, idempotency.DefaultLease)
	decision, record, classifyErr := idempotency.Classify(acquired, acquireErr)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return s.replayCreatedFulfillmentWave(ctx, tenantID, principal, record)
	case idempotency.DecisionInProgress:
		return nil, ErrFulfillmentWaveCompleting
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, ErrFulfillmentWaveIdempotency
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if acquired == nil || acquired.Record == nil {
			return nil, ErrFulfillmentWaveIdempotency
		}
	default:
		if classifyErr != nil {
			return nil, classifyErr
		}
		return nil, ErrFulfillmentWaveIdempotency
	}

	var wave FulfillmentWave
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		warehouseRow, requireErr := s.Warehouses.RequireActive(ctx, tx, tenantID, in.WarehouseID)
		if requireErr != nil || warehouseRow == nil {
			return ErrFulfillmentWaveOrderUnavailable
		}
		var orders []Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN ? AND deleted_at IS NULL", tenantID, orderIDs).Order("id ASC").Find(&orders).Error; err != nil {
			return err
		}
		if len(orders) != len(orderIDs) {
			return ErrFulfillmentWaveOrderUnavailable
		}
		var items []OrderItem
		if err := tx.Where("order_id IN ?", orderIDs).Order("order_id ASC, created_at ASC, id ASC").Find(&items).Error; err != nil {
			return err
		}
		skuIDs := make([]uuid.UUID, 0, len(items))
		seenSKU := make(map[uuid.UUID]struct{}, len(items))
		for _, item := range items {
			if item.ProductSKUID != nil && *item.ProductSKUID != uuid.Nil {
				if _, ok := seenSKU[*item.ProductSKUID]; !ok {
					seenSKU[*item.ProductSKUID] = struct{}{}
					skuIDs = append(skuIDs, *item.ProductSKUID)
				}
			}
		}
		placementBySKU, err := inventory.LoadWarehouseSKUPlacementSnapshots(ctx, tx, tenantID, in.WarehouseID, skuIDs)
		if err != nil {
			return err
		}
		itemsByOrder := make(map[uuid.UUID][]OrderItem, len(orders))
		for _, item := range items {
			itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
		}
		var effects []inventory.OrderInventoryEffect
		if err := tx.Where("tenant_id = ? AND order_id IN ? AND status = ?", tenantID, orderIDs, inventory.InventoryEffectSuccess).Find(&effects).Error; err != nil {
			return err
		}
		reserveByItem := make(map[uuid.UUID]inventory.OrderInventoryEffect, len(items))
		terminalEffects := make(map[uuid.UUID]bool, len(orders))
		for _, effect := range effects {
			switch effect.EffectType {
			case inventory.EffectTypeReserve:
				reserveByItem[effect.OrderItemID] = effect
			case inventory.EffectTypeRelease, inventory.EffectTypeDeduct, inventory.EffectTypeRestore:
				terminalEffects[effect.OrderID] = true
			}
		}
		now := time.Now().UTC()
		wave = FulfillmentWave{
			TenantID: tenantID, WaveNo: fulfillmentWaveNo(now), WarehouseID: in.WarehouseID,
			Status: FulfillmentWaveDraft, Revision: 1, Remark: remark, CreatedBy: actor,
		}
		if err := tx.Create(&wave).Error; err != nil {
			return err
		}
		waveOrders := make([]FulfillmentWaveOrder, 0, len(orders))
		waveLines := make([]FulfillmentWaveLine, 0, len(items))
		assignments := make([]FulfillmentWaveAssignment, 0, len(orders))
		for _, orderRow := range orders {
			if !principalCanOperateWaveOrder(principal, orderRow.ShopID) {
				return ErrFulfillmentWaveStorePermission
			}
			if orderRow.PaymentStatus != PaymentPaid ||
				orderRow.Status == StatusCancelled || orderRow.Status == StatusRefunded || orderRow.Status == StatusClosed ||
				orderRow.Status == StatusShipped || orderRow.Status == StatusDelivered || orderRow.FulfillmentStatus == FulfillmentFulfilled ||
				orderRow.WarehouseID == nil || *orderRow.WarehouseID != in.WarehouseID || terminalEffects[orderRow.ID] {
				return ErrFulfillmentWaveOrderUnavailable
			}
			orderItems := itemsByOrder[orderRow.ID]
			if len(orderItems) == 0 {
				return ErrFulfillmentWaveOrderUnavailable
			}
			waveOrder := FulfillmentWaveOrder{TenantID: tenantID, WaveID: wave.ID, OrderID: orderRow.ID, ShopID: orderRow.ShopID, OrderNo: orderRow.OrderNo, Status: FulfillmentWaveOrderPending}
			if err := tx.Create(&waveOrder).Error; err != nil {
				return err
			}
			waveOrders = append(waveOrders, waveOrder)
			for _, item := range orderItems {
				if item.ProductSKUID == nil || *item.ProductSKUID == uuid.Nil || item.Quantity <= 0 {
					return ErrFulfillmentWaveOrderUnavailable
				}
				reserve, ok := reserveByItem[item.ID]
				if !ok || reserve.WarehouseID == nil || *reserve.WarehouseID != in.WarehouseID || reserve.ProductSKUID != *item.ProductSKUID || reserve.Quantity != item.Quantity {
					return ErrFulfillmentWaveReservation
				}
				placement := placementBySKU[*item.ProductSKUID]
				waveLines = append(waveLines, FulfillmentWaveLine{
					TenantID: tenantID, WaveID: wave.ID, WaveOrderID: waveOrder.ID, OrderID: orderRow.ID,
					OrderItemID: item.ID, ProductID: item.ProductID, ProductSKUID: *item.ProductSKUID,
					ProductTitle: clampWaveText(item.ProductTitle, 512), SKUCode: clampWaveText(item.SKUCode, 128), SKUName: clampWaveText(item.SKUName, 512),
					Barcode: placement.Barcode, LocationID: placement.LocationID, LocationCode: placement.LocationCode, LocationName: placement.LocationName,
					RequiredQty: item.Quantity, Status: FulfillmentWaveLinePending,
				})
				wave.RequiredQty += item.Quantity
			}
			assignments = append(assignments, FulfillmentWaveAssignment{TenantID: tenantID, OrderID: orderRow.ID, WaveID: wave.ID})
		}
		if err := tx.Create(&waveLines).Error; err != nil {
			return err
		}
		if err := tx.Create(&assignments).Error; err != nil {
			if isFulfillmentWaveUniqueViolation(err) {
				return ErrFulfillmentWaveOrderAssigned
			}
			return err
		}
		wave.OrderCount = len(waveOrders)
		wave.LineCount = len(waveLines)
		if err := tx.Model(&FulfillmentWave{}).Where("id = ? AND tenant_id = ?", wave.ID, tenantID).Updates(map[string]any{
			"order_count": wave.OrderCount, "line_count": wave.LineCount, "required_qty": wave.RequiredQty,
		}).Error; err != nil {
			return err
		}
		return s.Idempotency.WithDB(tx).Complete(ctx, acquired.Record.ID, owner, idempotency.CompleteResult{
			ResponseCode: "FULFILLMENT_WAVE_CREATED", ResourceType: "fulfillment_wave", ResourceID: wave.ID.String(),
		})
	})
	if err != nil {
		_ = s.Idempotency.Fail(ctx, acquired.Record.ID, owner, clampWaveText(err.Error(), 64), true)
		return nil, err
	}
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{TenantID: tenantID, AdminUserID: actor, Action: "order.fulfillment_wave.create", Resource: "fulfillment_wave", ResourceID: wave.ID.String(), Status: "success", Message: fmt.Sprintf("waveId=%s orderCount=%d", wave.ID, wave.OrderCount)})
	}
	return s.GetFulfillmentWave(ctx, tenantID, principal, wave.ID)
}

func validateWaveTracking(carrier, trackingNo, trackingURL string) error {
	if carrier == "" || trackingNo == "" || len([]rune(carrier)) > 128 || len([]rune(trackingNo)) > 255 || len(trackingURL) > 2048 {
		return ErrFulfillmentWaveInvalidInput
	}
	if trackingURL != "" {
		parsed, err := url.ParseRequestURI(trackingURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return ErrFulfillmentWaveInvalidInput
		}
	}
	return nil
}
