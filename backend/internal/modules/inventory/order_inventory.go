package inventory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInsufficientSKUStock = errors.New("insufficient stock for sku")

var (
	ErrOrderWarehouseRequired        = errors.New("order warehouse is required")
	ErrOrderDefaultWarehouseRequired = errors.New("active default warehouse is required for platform order inventory")
	ErrOrderWarehouseConflict        = errors.New("order warehouse conflicts with existing inventory effects")
	ErrOrderInventoryState           = errors.New("order inventory state is inconsistent")
)

// StockOrderPolicy mirrors settings.inventory (defaults conservative).
type StockOrderPolicy struct {
	AutoDeductManualOrders               bool
	AutoDeductPlatformOrders             bool
	AutoRestoreCancelledOrders           bool
	AutoSyncPlatformInventoryAfterDeduct bool // effective: auto_sync_inventory_after_order_deduct or legacy key
	AllowNegativeStock                   bool
	AllowManualSKUBindAfterDeduct        bool
	AutoDeductAfterSKUMatch              bool // platform sync: require true with auto_deduct_platform_orders to auto deduct
}

func truthyInventorySetting(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func (s *Service) InventoryPolicy(ctx context.Context) (StockOrderPolicy, error) {
	def := StockOrderPolicy{}
	if s == nil || s.Settings == nil {
		return def, nil
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "inventory")
	if err != nil {
		return def, err
	}
	flagOr := func(k string, defVal bool) bool {
		v, ok := m[k]
		if !ok || strings.TrimSpace(v) == "" {
			return defVal
		}
		return truthyInventorySetting(v)
	}
	syncNew := strings.TrimSpace(m["auto_sync_inventory_after_order_deduct"])
	syncLegacy := strings.TrimSpace(m["auto_sync_platform_inventory_after_deduct"])
	syncVal := syncNew
	if syncVal == "" {
		syncVal = syncLegacy
	}
	syncOn := truthyInventorySetting(syncVal)
	return StockOrderPolicy{
		AutoDeductManualOrders:               truthyInventorySetting(m["auto_deduct_manual_orders"]),
		AutoDeductPlatformOrders:             truthyInventorySetting(m["auto_deduct_platform_orders"]),
		AutoRestoreCancelledOrders:           truthyInventorySetting(m["auto_restore_cancelled_orders"]),
		AutoSyncPlatformInventoryAfterDeduct: syncOn,
		AllowNegativeStock:                   truthyInventorySetting(m["allow_negative_stock"]),
		AllowManualSKUBindAfterDeduct:        flagOr("allow_manual_sku_bind_after_deduct", true),
		AutoDeductAfterSKUMatch:              truthyInventorySetting(m["auto_deduct_after_sku_match"]),
	}, nil
}

// OrderInventoryOptions controls deduction / restore behaviour.
type OrderInventoryOptions struct {
	Reason             string // order_created | order_synced | manual_api | payment_void | ...
	PlatformAuto       bool   // platform sync path respects auto_deduct_platform_orders + eligibility
	SyncPlatforms      bool
	AllowNegativeStock *bool // nil = policy default
	CreatedBy          *uuid.UUID
	WarehouseID        *uuid.UUID // optional explicit binding for a manual order without one
	TenantID           *int64     // optional caller scope for authenticated HTTP writes
	CompensationOnly   bool       // platform sync may process only cancel/refund compensation
}

func allowNegative(policy StockOrderPolicy, opt *bool) bool {
	if opt != nil {
		return *opt
	}
	return policy.AllowNegativeStock
}

type orderInventoryAction string

const (
	orderInventoryNone    orderInventoryAction = ""
	orderInventoryReserve orderInventoryAction = "reserve"
	orderInventoryDeduct  orderInventoryAction = "deduct"
)

func orderInventoryTerminal(o orderMirror) bool {
	st := strings.ToLower(strings.TrimSpace(o.Status))
	if st == "cancelled" || st == "closed" || st == "refunded" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(o.PaymentStatus), "refunded")
}

func orderInventoryActionFor(o orderMirror) orderInventoryAction {
	if orderInventoryTerminal(o) {
		return orderInventoryNone
	}
	st := strings.ToLower(strings.TrimSpace(o.Status))
	fs := strings.ToLower(strings.TrimSpace(o.FulfillmentStatus))
	if st == "shipped" || st == "delivered" || fs == "fulfilled" {
		return orderInventoryDeduct
	}
	ps := strings.ToLower(strings.TrimSpace(o.PaymentStatus))
	if st == "paid" || st == "processing" || ps == "paid" || ps == "partially_refunded" {
		return orderInventoryReserve
	}
	return orderInventoryNone
}

func platformEligibleForDeduction(status, paymentStatus string) bool {
	return orderInventoryActionFor(orderMirror{Status: status, PaymentStatus: paymentStatus}) != orderInventoryNone
}

// DeductionSummary aggregates one deduct pass (HTTP / sync response helper).
type DeductionSummary struct {
	Action       string `json:"action,omitempty"`
	Skipped      bool   `json:"skipped,omitempty"`
	SkipReason   string `json:"skipReason,omitempty"`
	LinesSynced  int    `json:"linesSynced,omitempty"`
	LinesSkipped int    `json:"linesSkipped,omitempty"`
	LinesFailed  int    `json:"linesFailed,omitempty"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
}

type deductLineOutcome struct {
	synced             bool
	skipped            bool
	failedInsufficient bool
	job                *deductAcquire
	completion         *deductCompletion
}

func effectTypeForAction(action orderInventoryAction) string {
	if action == orderInventoryReserve {
		return EffectTypeReserve
	}
	return EffectTypeDeduct
}

// sortOrderInventoryItems gives every order-inventory transaction the same
// SKU lock order. Sorting only by an order's insertion order can deadlock when
// two concurrent orders contain the same SKUs in opposite line order.
func sortOrderInventoryItems(items []orderLineMirror) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := uuid.Nil, uuid.Nil
		if items[i].ProductSKUID != nil {
			left = *items[i].ProductSKUID
		}
		if items[j].ProductSKUID != nil {
			right = *items[j].ProductSKUID
		}
		if left != right {
			return left.String() < right.String()
		}
		return items[i].ID.String() < items[j].ID.String()
	})
}

func orderInventoryEventKey(effectType string, orderID, itemID, skuID uuid.UUID) string {
	if effectType == EffectTypeDeduct {
		return idempotency.InventoryDeduct(orderID.String(), itemID.String(), skuID.String())
	}
	return fmt.Sprintf("order_inventory:%s:%s:%s:%s", effectType, orderID, itemID, skuID)
}

func (s *Service) resolveOrderWarehouseTx(ctx context.Context, tx *gorm.DB, o *orderMirror, opts OrderInventoryOptions) (uuid.UUID, error) {
	if o == nil || o.ID == uuid.Nil || o.TenantID < 0 {
		return uuid.Nil, ErrOrderWarehouseRequired
	}
	requested := uuid.Nil
	if opts.WarehouseID != nil {
		requested = *opts.WarehouseID
	}
	if requested != uuid.Nil {
		if o.WarehouseID != nil && *o.WarehouseID != uuid.Nil && *o.WarehouseID != requested {
			return uuid.Nil, ErrOrderWarehouseConflict
		}
		if _, err := s.warehouseService().RequireActive(ctx, tx, o.TenantID, requested); err != nil {
			return uuid.Nil, fmt.Errorf("%w: %v", ErrOrderWarehouseRequired, err)
		}
		if o.WarehouseID == nil || *o.WarehouseID == uuid.Nil {
			if err := tx.Model(&orderMirror{}).Where("id = ? AND tenant_id = ?", o.ID, o.TenantID).Update("warehouse_id", requested).Error; err != nil {
				return uuid.Nil, err
			}
			o.WarehouseID = ptrUUID(requested)
		}
		return requested, nil
	}
	if o.WarehouseID != nil && *o.WarehouseID != uuid.Nil {
		if _, err := s.warehouseService().RequireActive(ctx, tx, o.TenantID, *o.WarehouseID); err != nil {
			return uuid.Nil, fmt.Errorf("%w: %v", ErrOrderWarehouseRequired, err)
		}
		return *o.WarehouseID, nil
	}
	if strings.EqualFold(strings.TrimSpace(o.Platform), "manual") || strings.TrimSpace(o.Platform) == "" {
		return uuid.Nil, ErrOrderWarehouseRequired
	}
	var row warehouse.Warehouse
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).
		Where("tenant_id = ? AND status = ? AND is_default = ?", o.TenantID, warehouse.StatusActive, true).
		Order("id ASC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, ErrOrderDefaultWarehouseRequired
	}
	if err != nil {
		return uuid.Nil, err
	}
	if err := tx.Model(&orderMirror{}).Where("id = ? AND tenant_id = ?", o.ID, o.TenantID).Update("warehouse_id", row.ID).Error; err != nil {
		return uuid.Nil, err
	}
	o.WarehouseID = ptrUUID(row.ID)
	return row.ID, nil
}

func (s *Service) loadOrderLineStockTx(ctx context.Context, tx *gorm.DB, o orderMirror, it orderLineMirror, warehouseID uuid.UUID, actor *uuid.UUID) (product.ProductSKU, WarehouseStockBalance, error) {
	var sku product.ProductSKU
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("product_skus.id = ? AND products.tenant_id = ?", *it.ProductSKUID, o.TenantID).
		First(&sku).Error
	if err != nil {
		return sku, WarehouseStockBalance{}, err
	}
	if it.ProductID != nil && *it.ProductID != uuid.Nil && sku.ProductID != *it.ProductID {
		return sku, WarehouseStockBalance{}, fmt.Errorf("sku %s does not belong to declared product row", sku.ID)
	}
	if _, err := s.warehouseService().RequireActive(ctx, tx, o.TenantID, warehouseID); err != nil {
		return sku, WarehouseStockBalance{}, fmt.Errorf("warehouse unavailable: %w", err)
	}
	if _, err := ensureLegacyBalanceInMigrationWarehouseTx(ctx, tx, o.TenantID, sku, actor); err != nil {
		return sku, WarehouseStockBalance{}, err
	}
	var balance WarehouseStockBalance
	err = tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", o.TenantID, warehouseID, sku.ID).
		First(&balance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		balance = WarehouseStockBalance{TenantID: o.TenantID, WarehouseID: warehouseID, ProductSKUID: sku.ID, Version: 1}
		if err := tx.WithContext(ctx).Create(&balance).Error; err != nil {
			return sku, balance, err
		}
	} else if err != nil {
		return sku, balance, err
	}
	return sku, balance, nil
}

func updateOrderWarehouseBalanceTx(tx *gorm.DB, balance *WarehouseStockBalance, onHand, reserved int) error {
	previousVersion := balance.Version
	balance.OnHand = onHand
	balance.Reserved = reserved
	balance.Version++
	result := tx.Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", balance.ID, previousVersion).
		Updates(map[string]any{"on_hand": onHand, "reserved": reserved, "version": balance.Version, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: concurrent order inventory update", ErrOrderInventoryState)
	}
	return nil
}

func productIDForEffect(it orderLineMirror, sku product.ProductSKU) *uuid.UUID {
	productID := sku.ProductID
	if it.ProductID != nil && *it.ProductID != uuid.Nil {
		productID = *it.ProductID
	}
	return &productID
}

func (s *Service) upsertFailedOrderEffect(tx *gorm.DB, o orderMirror, it orderLineMirror, warehouseID, skuID uuid.UUID, effectType, reason, message string, actor *uuid.UUID) error {
	payload := map[string]any{
		"tenant_id": o.TenantID, "order_id": o.ID, "warehouse_id": warehouseID,
		"product_id": it.ProductID, "quantity": it.Quantity, "status": InventoryEffectFailed,
		"reason": reason, "error_message": clampStr(message, 1024), "created_by": actor, "updated_at": time.Now().UTC(),
	}
	var row OrderInventoryEffect
	err := tx.Where("tenant_id = ? AND order_item_id = ? AND product_sku_id = ? AND effect_type = ?", o.TenantID, it.ID, skuID, effectType).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&OrderInventoryEffect{
			TenantID: o.TenantID, OrderID: o.ID, OrderItemID: it.ID, WarehouseID: ptrUUID(warehouseID), ProductID: it.ProductID,
			ProductSKUID: skuID, EffectType: effectType, Quantity: it.Quantity, Status: InventoryEffectFailed,
			Reason: reason, ErrorMessage: clampStr(message, 1024), CreatedBy: actor,
		}).Error
	}
	if err != nil || row.Status == InventoryEffectSuccess {
		return err
	}
	return tx.Model(&OrderInventoryEffect{}).Where("id = ? AND tenant_id = ?", row.ID, o.TenantID).Updates(payload).Error
}

func successfulOrderEffectTx(tx *gorm.DB, tenantID int64, itemID, skuID uuid.UUID, effectType string) (*OrderInventoryEffect, error) {
	var row OrderInventoryEffect
	err := tx.Where("tenant_id = ? AND order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ?", tenantID, itemID, skuID, effectType, InventoryEffectSuccess).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// successfulOrderEffectForOrderTx also recognizes legacy tenant_id=0 effects
// while the order itself proves the row belongs to the current tenant. The
// restore path binds such rows to the order tenant and selected warehouse.
func successfulOrderEffectForOrderTx(tx *gorm.DB, tenantID int64, orderID, itemID, skuID uuid.UUID, effectType string) (*OrderInventoryEffect, error) {
	var row OrderInventoryEffect
	err := tx.Where("order_id = ? AND order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ? AND (tenant_id = ? OR tenant_id = 0)", orderID, itemID, skuID, effectType, InventoryEffectSuccess, tenantID).
		Order("CASE WHEN tenant_id = 0 THEN 1 ELSE 0 END, id").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) createOrderInventoryFactTx(tx *gorm.DB, o orderMirror, it orderLineMirror, sku product.ProductSKU, warehouseID uuid.UUID, effectType, movementType, changeType, reason string, quantity, beforeOnHand, afterOnHand, beforeReserved, afterReserved, beforeStock, afterStock int, actor *uuid.UUID) (*InventoryChangeLog, error) {
	eventKey := orderInventoryEventKey(effectType, o.ID, it.ID, sku.ID)
	remark := remarkForOrderStock(o.OrderNo, it.ID.String(), it.ExternalItemID)
	movement := InventoryMovement{
		TenantID: o.TenantID, WarehouseID: warehouseID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		MovementType: movementType, Quantity: quantity, BeforeOnHand: beforeOnHand, AfterOnHand: afterOnHand,
		BeforeReserved: beforeReserved, AfterReserved: afterReserved, SourceType: "order_item", SourceID: it.ID,
		BusinessEventKey: eventKey, Reason: reason, Remark: remark, CreatedBy: actor,
	}
	if err := tx.Create(&movement).Error; err != nil {
		return nil, err
	}
	logRow := InventoryChangeLog{
		TenantID: o.TenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID, ChangeType: changeType,
		BeforeStock: beforeStock, AfterStock: afterStock, Delta: afterStock - beforeStock, Reason: reason, Remark: remark,
		CreatedBy: actor, RefOrderID: &o.ID, RefOrderItemID: &it.ID, BusinessEventKey: eventKey,
	}
	if err := tx.Create(&logRow).Error; err != nil {
		return nil, err
	}
	_ = tx.Where("tenant_id = ? AND order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ?", o.TenantID, it.ID, sku.ID, effectType, InventoryEffectFailed).
		Delete(&OrderInventoryEffect{}).Error
	effect := OrderInventoryEffect{
		TenantID: o.TenantID, OrderID: o.ID, OrderItemID: it.ID, WarehouseID: ptrUUID(warehouseID), ProductID: productIDForEffect(it, sku),
		ProductSKUID: sku.ID, EffectType: effectType, Quantity: it.Quantity, Status: InventoryEffectSuccess,
		BeforeStock: intPtr(beforeStock), AfterStock: intPtr(afterStock), Reason: reason, LogID: &logRow.ID, CreatedBy: actor,
	}
	if err := tx.Create(&effect).Error; err != nil {
		return nil, err
	}
	return &logRow, nil
}

func (s *Service) applyOrderLineTx(ctx context.Context, tx *gorm.DB, o orderMirror, it orderLineMirror, warehouseID uuid.UUID, action orderInventoryAction, reason string, allowNeg bool, opts OrderInventoryOptions) (deductLineOutcome, error) {
	out := deductLineOutcome{}
	var lockedItem orderLineMirror
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND order_id = ?", it.ID, o.ID).First(&lockedItem).Error; err != nil {
		return out, err
	}
	it = lockedItem
	effectType := effectTypeForAction(action)
	if it.ProductSKUID == nil || *it.ProductSKUID == uuid.Nil {
		var count int64
		if err := tx.Model(&OrderInventoryEffect{}).Where("tenant_id = ? AND order_item_id = ? AND product_sku_id = ? AND effect_type = ?", o.TenantID, it.ID, NilInventorySKUUID, effectType).Count(&count).Error; err != nil {
			return out, err
		}
		if count == 0 {
			err := tx.Create(&OrderInventoryEffect{
				TenantID: o.TenantID, OrderID: o.ID, OrderItemID: it.ID, WarehouseID: ptrUUID(warehouseID), ProductID: it.ProductID,
				ProductSKUID: NilInventorySKUUID, EffectType: effectType, Status: InventoryEffectSkipped,
				Reason: "missing_product_sku_id", CreatedBy: opts.CreatedBy,
			}).Error
			if err != nil {
				return out, err
			}
		}
		out.skipped = true
		return out, nil
	}
	if it.Quantity <= 0 {
		out.skipped = true
		return out, nil
	}
	skuID := *it.ProductSKUID
	var deductJob *deductAcquire
	var deductResult *idempotency.AcquireResult
	var err error
	if action == orderInventoryDeduct {
		deductJob, deductResult, err = s.acquireDeductLine(ctx, o.ID, it.ID, skuID, it.Quantity, reason, opts)
		if err != nil {
			return out, err
		}
		out.job = deductJob
		if deductJob == nil && s.Idempotency != nil && (deductResult == nil || !deductResult.Replay) {
			return out, fmt.Errorf("%w: deduct idempotency succeeded without a warehouse effect", ErrOrderInventoryState)
		}
	}
	existing, err := successfulOrderEffectTx(tx, o.TenantID, it.ID, skuID, effectType)
	if err != nil {
		return out, err
	}
	if existing != nil {
		if existing.WarehouseID == nil {
			_ = tx.Model(&OrderInventoryEffect{}).Where("id = ? AND tenant_id = ?", existing.ID, o.TenantID).Updates(map[string]any{"tenant_id": o.TenantID, "warehouse_id": warehouseID}).Error
		}
		if deductJob != nil {
			logID := existing.ID
			if existing.LogID != nil && *existing.LogID != uuid.Nil {
				logID = *existing.LogID
			}
			out.completion = &deductCompletion{Job: deductJob, LogID: logID}
		}
		out.skipped = true
		return out, nil
	}
	if action == orderInventoryDeduct {
		if released, err := successfulOrderEffectTx(tx, o.TenantID, it.ID, skuID, EffectTypeRelease); err != nil {
			return out, err
		} else if released != nil {
			return out, ErrOrderInventoryState
		}
	}

	sku, balance, err := s.loadOrderLineStockTx(ctx, tx, o, it, warehouseID, opts.CreatedBy)
	if err != nil {
		return out, err
	}
	if concurrent, err := successfulOrderEffectTx(tx, o.TenantID, it.ID, skuID, effectType); err != nil {
		return out, err
	} else if concurrent != nil {
		out.skipped = true
		if deductJob != nil {
			out.completion = &deductCompletion{Job: deductJob, LogID: concurrent.ID}
		}
		return out, nil
	}
	beforeStock := derefStock(sku.Stock)
	beforeOnHand, beforeReserved := balance.OnHand, balance.Reserved
	afterOnHand, afterReserved, afterStock := beforeOnHand, beforeReserved, beforeStock
	movementType, changeType, quantity := MovementOrderReserve, ChangeOrderReserve, it.Quantity

	if action == orderInventoryReserve {
		if deducted, err := successfulOrderEffectTx(tx, o.TenantID, it.ID, skuID, EffectTypeDeduct); err != nil {
			return out, err
		} else if deducted != nil {
			out.skipped = true
			if deductJob != nil {
				out.completion = &deductCompletion{Job: deductJob, LogID: deducted.ID}
			}
			return out, nil
		}
		if balance.Available() < it.Quantity && !allowNeg {
			if err := s.upsertFailedOrderEffect(tx, o, it, warehouseID, skuID, effectType, reason, "insufficient warehouse available stock", opts.CreatedBy); err != nil {
				return out, err
			}
			out.failedInsufficient = true
			return out, nil
		}
		afterReserved += it.Quantity
	} else {
		reservation, err := successfulOrderEffectTx(tx, o.TenantID, it.ID, skuID, EffectTypeReserve)
		if err != nil {
			return out, err
		}
		if reservation != nil {
			if reservation.WarehouseID != nil && *reservation.WarehouseID != warehouseID {
				return out, ErrOrderWarehouseConflict
			}
			if beforeReserved < it.Quantity {
				return out, fmt.Errorf("%w: reserved stock is below order quantity", ErrOrderInventoryState)
			}
			afterReserved -= it.Quantity
		}
		if beforeOnHand < it.Quantity && !allowNeg {
			if err := s.upsertFailedOrderEffect(tx, o, it, warehouseID, skuID, effectType, reason, "insufficient warehouse on-hand stock", opts.CreatedBy); err != nil {
				return out, err
			}
			out.failedInsufficient = true
			if deductJob != nil {
				out.completion = &deductCompletion{Job: deductJob}
			}
			return out, nil
		}
		afterOnHand -= it.Quantity
		afterStock -= it.Quantity
		movementType, changeType, quantity = MovementOrderDeduct, ChangeOrderDeduct, -it.Quantity
	}
	if err := updateOrderWarehouseBalanceTx(tx, &balance, afterOnHand, afterReserved); err != nil {
		return out, err
	}
	if action == orderInventoryDeduct {
		if err := tx.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).
			Updates(map[string]any{"stock": afterStock, "stock_status": stockStatusForSKU(sku, afterStock), "updated_at": time.Now().UTC()}).Error; err != nil {
			return out, err
		}
	}
	logRow, err := s.createOrderInventoryFactTx(tx, o, it, sku, warehouseID, effectType, movementType, changeType, reason, quantity, beforeOnHand, afterOnHand, beforeReserved, afterReserved, beforeStock, afterStock, opts.CreatedBy)
	if err != nil {
		return out, err
	}
	if action == orderInventoryDeduct && deductJob != nil {
		out.completion = &deductCompletion{Job: deductJob, LogID: logRow.ID}
	}
	out.synced = true
	return out, nil
}

func (s *Service) syncOrderSKUStocks(ctx context.Context, orderID uuid.UUID, items []orderLineMirror, actor *uuid.UUID, action string) {
	seen := map[uuid.UUID]struct{}{}
	for _, it := range items {
		if it.ProductSKUID == nil || *it.ProductSKUID == uuid.Nil {
			continue
		}
		if _, ok := seen[*it.ProductSKUID]; ok {
			continue
		}
		var sku product.ProductSKU
		if err := s.DB.WithContext(ctx).First(&sku, "id = ?", *it.ProductSKUID).Error; err != nil {
			continue
		}
		seen[sku.ID] = struct{}{}
		if _, err := s.CreateInventorySyncTasksForSKUStock(ctx, sku.ProductID, sku.ID, derefStock(sku.Stock), actor); err != nil && s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: actor, Action: action, Resource: "order", ResourceID: orderID.String(), Status: "failed", Message: clampStr(err.Error(), 480),
			})
		}
	}
}

// DeductInventoryForOrder applies the order lifecycle to warehouse stock. Paid
// orders reserve stock; shipped/fulfilled orders commit the on-hand deduction.
func (s *Service) DeductInventoryForOrder(ctx context.Context, orderID uuid.UUID, opts OrderInventoryOptions) (*DeductionSummary, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	policy, err := s.InventoryPolicy(ctx)
	if err != nil {
		return nil, err
	}
	var o orderMirror
	orderWhere := "id = ? AND deleted_at IS NULL"
	orderArgs := []any{orderID}
	if opts.TenantID != nil {
		orderWhere += " AND tenant_id = ?"
		orderArgs = append(orderArgs, *opts.TenantID)
	}
	findArgs := append([]any{orderWhere}, orderArgs...)
	if err := s.DB.WithContext(ctx).First(&o, findArgs...).Error; err != nil {
		return nil, err
	}
	if opts.PlatformAuto && orderInventoryTerminal(o) {
		if !policy.AutoRestoreCancelledOrders {
			return &DeductionSummary{Skipped: true, SkipReason: "auto_restore_cancelled_orders disabled"}, nil
		}
		opts.SyncPlatforms = opts.SyncPlatforms || policy.AutoSyncPlatformInventoryAfterDeduct
		restored, restoreErr := s.RestoreInventoryForOrder(ctx, orderID, opts)
		summary := &DeductionSummary{Action: restored.Action, LinesSynced: restored.LinesSynced, Message: restored.Message, Error: restored.Error}
		if restored.LinesSynced == 0 {
			summary.Skipped = true
			summary.SkipReason = "no order inventory effect requires compensation"
		}
		return summary, restoreErr
	}
	if opts.CompensationOnly {
		return &DeductionSummary{Skipped: true, SkipReason: "order has no terminal inventory compensation"}, nil
	}
	if opts.PlatformAuto && !policy.AutoDeductPlatformOrders {
		return &DeductionSummary{Skipped: true, SkipReason: "auto_deduct_platform_orders disabled"}, nil
	}
	action := orderInventoryActionFor(o)
	if action == orderInventoryNone {
		return &DeductionSummary{Skipped: true, SkipReason: "order is not eligible for reservation or deduction"}, nil
	}
	reason := clampStr(opts.Reason, 128)
	if reason == "" {
		if opts.PlatformAuto {
			reason = "order_synced"
		} else {
			reason = "manual_order_inventory"
		}
	}
	allowNeg := allowNegative(policy, opts.AllowNegativeStock)
	var warehouseID uuid.UUID
	var items []orderLineMirror
	var outcomes []deductLineOutcome
	var synced, skipped, failed int
	txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize lifecycle changes for the same order. The line and balance
		// locks below then protect cross-order SKU contention deterministically.
		var locked orderMirror
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(orderWhere, orderArgs...).First(&locked).Error; err != nil {
			return err
		}
		o = locked
		action = orderInventoryActionFor(o)
		if action == orderInventoryNone {
			return nil
		}
		var err error
		warehouseID, err = s.resolveOrderWarehouseTx(ctx, tx, &o, opts)
		if err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
			return err
		}
		sortOrderInventoryItems(items)
		for _, it := range items {
			outcome, applyErr := s.applyOrderLineTx(ctx, tx, o, it, warehouseID, action, reason, allowNeg, opts)
			outcomes = append(outcomes, outcome)
			if applyErr != nil {
				return applyErr
			}
			if outcome.synced {
				synced++
			} else if outcome.failedInsufficient {
				failed++
			} else {
				skipped++
			}
		}
		return nil
	})
	if txErr != nil {
		for _, outcome := range outcomes {
			if outcome.job != nil {
				s.failDeductLine(ctx, outcome.job, "INVENTORY_TRANSACTION_FAILED", true)
			}
		}
		return &DeductionSummary{Action: string(action), LinesSynced: synced, LinesSkipped: skipped, LinesFailed: failed, Error: txErr.Error()}, txErr
	}
	if action == orderInventoryNone {
		return &DeductionSummary{Skipped: true, SkipReason: "order is not eligible for reservation or deduction"}, nil
	}
	for _, outcome := range outcomes {
		if outcome.completion == nil || outcome.completion.Job == nil {
			continue
		}
		if outcome.failedInsufficient {
			// Insufficient stock is a recoverable business failure: inventory
			// adjustment or a later manual retry must be able to reacquire the key.
			s.failDeductLine(ctx, outcome.completion.Job, "INSUFFICIENT_STOCK", true)
			continue
		}
		if err := s.completeDeductLine(ctx, outcome.completion.Job, outcome.completion.LogID); err != nil {
			return &DeductionSummary{Action: string(action), LinesSynced: synced, LinesSkipped: skipped, LinesFailed: failed, Error: err.Error()}, err
		}
	}
	if action == orderInventoryDeduct && synced > 0 && (opts.SyncPlatforms || (opts.PlatformAuto && policy.AutoSyncPlatformInventoryAfterDeduct)) {
		s.syncOrderSKUStocks(ctx, orderID, items, opts.CreatedBy, "inventory.order_deduct.sync_enqueue_failed")
	}
	summary := &DeductionSummary{Action: string(action), LinesSynced: synced, LinesSkipped: skipped, LinesFailed: failed, Message: "ok"}
	if failed > 0 {
		summary.Message, summary.Error = ErrInsufficientSKUStock.Error(), ErrInsufficientSKUStock.Error()
		return summary, ErrInsufficientSKUStock
	}
	return summary, nil
}

// RestorationSummary aggregates restore attempts.
type RestorationSummary struct {
	Action      string `json:"action,omitempty"`
	Skipped     bool   `json:"skipped,omitempty"`
	SkipReason  string `json:"skipReason,omitempty"`
	LinesSynced int    `json:"linesSynced,omitempty"`
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
}

func (s *Service) RestoreInventoryForOrder(ctx context.Context, orderID uuid.UUID, opts OrderInventoryOptions) (*RestorationSummary, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	var o orderMirror
	orderWhere := "id = ? AND deleted_at IS NULL"
	orderArgs := []any{orderID}
	if opts.TenantID != nil {
		orderWhere += " AND tenant_id = ?"
		orderArgs = append(orderArgs, *opts.TenantID)
	}
	findArgs := append([]any{orderWhere}, orderArgs...)
	if err := s.DB.WithContext(ctx).First(&o, findArgs...).Error; err != nil {
		return nil, err
	}
	items, err := s.loadOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	reason := clampStr(strings.TrimSpace(opts.Reason), 128)
	if reason == "" {
		reason = "order_cancel_restore"
	}

	var restored, released int
	for _, it := range items {
		txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			// Restore competes with deduct and order-line edits. Lock the order
			// first, then re-read the line so effect checks observe the same
			// lifecycle snapshot as the stock update.
			var lockedOrder orderMirror
			if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
				Where(orderWhere, orderArgs...).First(&lockedOrder).Error; err != nil {
				return err
			}
			o = lockedOrder
			var lockedItem orderLineMirror
			if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("id = ? AND order_id = ?", it.ID, orderID).First(&lockedItem).Error; err != nil {
				return err
			}
			it = lockedItem
			if it.ProductSKUID == nil || *it.ProductSKUID == uuid.Nil {
				return nil
			}
			if it.Quantity <= 0 {
				return nil
			}
			skuID := *it.ProductSKUID
			deductEffect, err := successfulOrderEffectForOrderTx(tx, o.TenantID, o.ID, it.ID, skuID, EffectTypeDeduct)
			if err != nil {
				return err
			}
			reserveEffect, err := successfulOrderEffectForOrderTx(tx, o.TenantID, o.ID, it.ID, skuID, EffectTypeReserve)
			if err != nil {
				return err
			}
			effectType, movementType, changeType := EffectTypeRelease, MovementOrderRelease, ChangeOrderRelease
			sourceEffect := reserveEffect
			if deductEffect != nil {
				effectType, movementType, changeType, sourceEffect = EffectTypeRestore, MovementOrderRestore, ChangeOrderRestore, deductEffect
			}
			if sourceEffect == nil {
				return nil
			}
			if existing, err := successfulOrderEffectForOrderTx(tx, o.TenantID, o.ID, it.ID, skuID, effectType); err != nil || existing != nil {
				return err
			}
			warehouseID := uuid.Nil
			if sourceEffect.WarehouseID != nil {
				warehouseID = *sourceEffect.WarehouseID
			}
			if warehouseID != uuid.Nil {
				if o.WarehouseID != nil && *o.WarehouseID != uuid.Nil && *o.WarehouseID != warehouseID {
					return ErrOrderWarehouseConflict
				}
				if o.WarehouseID == nil || *o.WarehouseID == uuid.Nil {
					if err := tx.Model(&orderMirror{}).Where("id = ? AND tenant_id = ?", o.ID, o.TenantID).Update("warehouse_id", warehouseID).Error; err != nil {
						return err
					}
					o.WarehouseID = ptrUUID(warehouseID)
				}
			} else {
				warehouseID, err = s.resolveOrderWarehouseTx(ctx, tx, &o, opts)
				if err != nil {
					return err
				}
				if err := tx.Model(&OrderInventoryEffect{}).Where("id = ? AND (tenant_id = ? OR tenant_id = 0)", sourceEffect.ID, o.TenantID).
					Updates(map[string]any{"tenant_id": o.TenantID, "warehouse_id": warehouseID}).Error; err != nil {
					return err
				}
			}
			sku, balance, err := s.loadOrderLineStockTx(ctx, tx, o, it, warehouseID, opts.CreatedBy)
			if err != nil {
				return err
			}
			beforeStock := derefStock(sku.Stock)
			beforeOnHand, beforeReserved := balance.OnHand, balance.Reserved
			afterStock, afterOnHand, afterReserved := beforeStock, beforeOnHand, beforeReserved
			quantity := -it.Quantity
			if effectType == EffectTypeRelease {
				if beforeReserved < it.Quantity {
					return fmt.Errorf("%w: reserved stock is below release quantity", ErrOrderInventoryState)
				}
				afterReserved -= it.Quantity
			} else {
				afterOnHand += it.Quantity
				afterStock += it.Quantity
				quantity = it.Quantity
			}
			if err := updateOrderWarehouseBalanceTx(tx, &balance, afterOnHand, afterReserved); err != nil {
				return err
			}
			if effectType == EffectTypeRestore {
				if err := tx.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).
					Updates(map[string]any{"stock": afterStock, "stock_status": stockStatusForSKU(sku, afterStock), "updated_at": time.Now().UTC()}).Error; err != nil {
					return err
				}
			}
			if _, err := s.createOrderInventoryFactTx(tx, o, it, sku, warehouseID, effectType, movementType, changeType, reason, quantity, beforeOnHand, afterOnHand, beforeReserved, afterReserved, beforeStock, afterStock, opts.CreatedBy); err != nil {
				return err
			}
			if effectType == EffectTypeRestore {
				restored++
			} else {
				released++
			}
			return nil
		})
		if txErr != nil {
			return &RestorationSummary{Error: txErr.Error()}, txErr
		}
	}
	if opts.SyncPlatforms && restored > 0 {
		s.syncOrderSKUStocks(ctx, orderID, items, opts.CreatedBy, "inventory.order_restore.sync_enqueue_failed")
	}
	action := ""
	if restored > 0 {
		action = EffectTypeRestore
	} else if released > 0 {
		action = EffectTypeRelease
	}
	return &RestorationSummary{Action: action, LinesSynced: restored + released, Skipped: restored+released == 0, Message: "ok"}, nil
}

func remarkForOrderStock(orderNo string, itemID string, ext *string) string {
	parts := []string{fmt.Sprintf("orderNo=%s", clampStr(orderNo, 96))}
	if itemID != "" {
		parts = append(parts, fmt.Sprintf("orderItemId=%s", clampStr(itemID, 96)))
	}
	if ext != nil && strings.TrimSpace(*ext) != "" {
		parts = append(parts, fmt.Sprintf("externalItem=%s", clampStr(strings.TrimSpace(*ext), 128)))
	}
	return clampStr(strings.Join(parts, " "), 520)
}

func intPtr(v int) *int { return &v }

func (s *Service) loadOrderItems(ctx context.Context, orderID uuid.UUID) ([]orderLineMirror, error) {
	var items []orderLineMirror
	err := s.DB.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at ASC, id ASC").Find(&items).Error
	return items, err
}

// InventorySummary exposes flags for admin order detail drawer.
type OrderInventoryUISummary struct {
	HasReservationSuccess bool `json:"hasReservationSuccess"`
	HasReleaseSuccess     bool `json:"hasReleaseSuccess"`
	HasDeductionSuccess   bool `json:"hasDeductionSuccess"`
	HasRestoreSuccess     bool `json:"hasRestoreSuccess"`
	FullyRestored         bool `json:"fullyRestored"` // heuristic: restore success exists for every deduct-success line with sku
}

func (s *Service) SummarizeOrderInventoryEffects(ctx context.Context, orderID uuid.UUID) (*OrderInventoryUISummary, error) {
	sum := &OrderInventoryUISummary{}
	if s == nil || s.DB == nil {
		return sum, fmt.Errorf("inventory: no db")
	}

	var reserveN, releaseN, deductN, deductSKUN int64
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeReserve, InventoryEffectSuccess).
		Count(&reserveN).Error
	sum.HasReservationSuccess = reserveN > 0
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeRelease, InventoryEffectSuccess).
		Count(&releaseN).Error
	sum.HasReleaseSuccess = releaseN > 0
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeDeduct, InventoryEffectSuccess).
		Count(&deductN).Error
	sum.HasDeductionSuccess = deductN > 0

	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ? AND product_sku_id <> ?", orderID, EffectTypeDeduct, InventoryEffectSuccess, NilInventorySKUUID).
		Count(&deductSKUN).Error

	var restoreSKU int64
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeRestore, InventoryEffectSuccess).
		Count(&restoreSKU).Error
	sum.HasRestoreSuccess = restoreSKU > 0
	if deductSKUN > 0 && restoreSKU >= deductSKUN {
		sum.FullyRestored = true
	}
	return sum, nil
}

// HasSuccessfulOrderDeduction reports whether stock has already been reserved or deducted.
func (s *Service) HasSuccessfulOrderDeduction(ctx context.Context, orderID uuid.UUID) (bool, error) {
	if s == nil || s.DB == nil {
		return false, fmt.Errorf("inventory: no db")
	}
	var n int64
	if err := s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type IN ? AND status = ?", orderID, []string{EffectTypeReserve, EffectTypeDeduct}, InventoryEffectSuccess).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// HasUncompensatedOrderInventory reports stock still reserved or deducted for an order.
func (s *Service) HasUncompensatedOrderInventory(ctx context.Context, orderID uuid.UUID) (bool, error) {
	if s == nil || s.DB == nil {
		return false, fmt.Errorf("inventory: no db")
	}
	var count int64
	err := s.DB.WithContext(ctx).Raw(`
		SELECT COUNT(*)
		FROM order_inventory_effects source
		WHERE source.order_id = ? AND source.status = ? AND (
			(source.effect_type = ? AND NOT EXISTS (
				SELECT 1 FROM order_inventory_effects release_effect
				WHERE release_effect.order_item_id = source.order_item_id
				  AND release_effect.product_sku_id = source.product_sku_id
				  AND release_effect.effect_type IN (?, ?)
				  AND release_effect.status = ?
			)) OR
			(source.effect_type = ? AND NOT EXISTS (
				SELECT 1 FROM order_inventory_effects restore_effect
				WHERE restore_effect.order_item_id = source.order_item_id
				  AND restore_effect.product_sku_id = source.product_sku_id
				  AND restore_effect.effect_type = ?
				  AND restore_effect.status = ?
			))
		)
	`, orderID, InventoryEffectSuccess, EffectTypeReserve, EffectTypeRelease, EffectTypeDeduct, InventoryEffectSuccess,
		EffectTypeDeduct, EffectTypeRestore, InventoryEffectSuccess).Scan(&count).Error
	return count > 0, err
}
