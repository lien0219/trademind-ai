package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInsufficientSKUStock = errors.New("insufficient stock for sku")

// StockOrderPolicy mirrors settings.inventory (defaults conservative).
type StockOrderPolicy struct {
	AutoDeductManualOrders               bool
	AutoDeductPlatformOrders             bool
	AutoRestoreCancelledOrders           bool
	AutoSyncPlatformInventoryAfterDeduct bool
	AllowNegativeStock                   bool
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
	return StockOrderPolicy{
		AutoDeductManualOrders:               truthyInventorySetting(m["auto_deduct_manual_orders"]),
		AutoDeductPlatformOrders:             truthyInventorySetting(m["auto_deduct_platform_orders"]),
		AutoRestoreCancelledOrders:           truthyInventorySetting(m["auto_restore_cancelled_orders"]),
		AutoSyncPlatformInventoryAfterDeduct: truthyInventorySetting(m["auto_sync_platform_inventory_after_deduct"]),
		AllowNegativeStock:                   truthyInventorySetting(m["allow_negative_stock"]),
	}, nil
}

// OrderInventoryOptions controls deduction / restore behaviour.
type OrderInventoryOptions struct {
	Reason             string // order_created | order_synced | manual_api | payment_void | ...
	PlatformAuto       bool   // platform sync path respects auto_deduct_platform_orders + eligibility
	SyncPlatforms      bool
	AllowNegativeStock *bool // nil = policy default
	CreatedBy          *uuid.UUID
}

func allowNegative(policy StockOrderPolicy, opt *bool) bool {
	if opt != nil {
		return *opt
	}
	return policy.AllowNegativeStock
}

func platformEligibleForDeduction(status, paymentStatus string) bool {
	st := strings.TrimSpace(status)
	switch st {
	case "cancelled", "closed", "refunded", "pending":
		return false
	}
	ps := strings.TrimSpace(paymentStatus)
	if ps == "unpaid" || ps == "refunded" {
		return false
	}
	switch st {
	case "paid", "processing", "shipped", "delivered":
		return true
	default:
		return false
	}
}

// DeductionSummary aggregates one deduct pass (HTTP / sync response helper).
type DeductionSummary struct {
	Skipped      bool   `json:"skipped,omitempty"`
	SkipReason   string `json:"skipReason,omitempty"`
	LinesSynced  int    `json:"linesSynced,omitempty"`
	LinesSkipped int    `json:"linesSkipped,omitempty"`
	LinesFailed  int    `json:"linesFailed,omitempty"`
	Message      string `json:"message,omitempty"`
	Error        string `json:"error,omitempty"`
}

// DeductInventoryForOrder applies SKU stock decreases per line (transactional).
func (s *Service) DeductInventoryForOrder(ctx context.Context, orderID uuid.UUID, opts OrderInventoryOptions) (*DeductionSummary, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	policy, polErr := s.InventoryPolicy(ctx)
	if polErr != nil {
		return nil, polErr
	}
	var o orderMirror
	if err := s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", orderID).Error; err != nil {
		return nil, err
	}
	if opts.PlatformAuto {
		if !policy.AutoDeductPlatformOrders {
			return &DeductionSummary{Skipped: true, SkipReason: "auto_deduct_platform_orders disabled"}, nil
		}
		if !platformEligibleForDeduction(o.Status, o.PaymentStatus) {
			return &DeductionSummary{Skipped: true, SkipReason: "order not eligible for platform stock deduct"}, nil
		}
	}

	items, err := s.loadOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	syncAfter := opts.SyncPlatforms
	if opts.PlatformAuto && policy.AutoSyncPlatformInventoryAfterDeduct {
		syncAfter = true
	}

	allowNeg := allowNegative(policy, opts.AllowNegativeStock)

	var synced, skippedCount int

	txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		reasonBase := clampStr(strings.TrimSpace(opts.Reason), 128)
		if reasonBase == "" {
			if opts.PlatformAuto {
				reasonBase = "order_synced"
			} else {
				reasonBase = "order_created"
			}
		}

		itemCopy := append([]orderLineMirror(nil), items...)
		for _, it := range itemCopy {
			if it.ProductSKUID == nil || *it.ProductSKUID == uuid.Nil {
				sk := NilInventorySKUUID
				count := int64(0)
				_ = tx.Model(&OrderInventoryEffect{}).
					Where("order_item_id = ? AND product_sku_id = ? AND effect_type = ?", it.ID, sk, EffectTypeDeduct).
					Count(&count).Error
				if count == 0 {
					e := OrderInventoryEffect{
						OrderID:      orderID,
						OrderItemID:  it.ID,
						ProductID:    it.ProductID,
						ProductSKUID: sk,
						EffectType:   EffectTypeDeduct,
						Quantity:     0,
						Status:       InventoryEffectSkipped,
						Reason:       "missing_product_sku_id",
						CreatedBy:    opts.CreatedBy,
					}
					if err := tx.Create(&e).Error; err != nil {
						return err
					}
				}
				skippedCount++
				continue
			}
			qty := it.Quantity
			if qty <= 0 {
				skippedCount++
				continue
			}

			var hit int64
			if err := tx.Model(&OrderInventoryEffect{}).
				Where("order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ?", it.ID, *it.ProductSKUID, EffectTypeDeduct, InventoryEffectSuccess).
				Count(&hit).Error; err != nil {
				return err
			}
			if hit > 0 {
				continue
			}

			var sku product.ProductSKU
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&sku, "id = ? AND deleted_at IS NULL", it.ProductSKUID).Error; err != nil {
				return err
			}
			if it.ProductID != nil && *it.ProductID != uuid.Nil && sku.ProductID != *it.ProductID {
				return fmt.Errorf("sku %s does not belong to declared product row", sku.ID.String())
			}
			before := derefStock(sku.Stock)
			if before < qty && !allowNeg {
				return ErrInsufficientSKUStock
			}
			after := before - qty
			if after < 0 && !allowNeg {
				return ErrInsufficientSKUStock
			}

			rm := remarkForOrderStock(o.OrderNo, it.ID.String(), it.ExternalItemID)
			chg := InventoryChangeLog{
				ProductID:      sku.ProductID,
				ProductSKUID:   sku.ID,
				ChangeType:     ChangeOrderDeduct,
				BeforeStock:    before,
				AfterStock:     after,
				Delta:          -qty,
				Reason:         reasonBase,
				Remark:         rm,
				CreatedBy:      opts.CreatedBy,
				RefOrderID:     &orderID,
				RefOrderItemID: &it.ID,
			}
			if err := tx.Create(&chg).Error; err != nil {
				return err
			}
			if err := tx.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).
				Updates(map[string]any{"stock": after, "updated_at": now}).Error; err != nil {
				return err
			}

			var prodForEff *uuid.UUID
			if it.ProductID != nil && *it.ProductID != uuid.Nil {
				pp := *it.ProductID
				prodForEff = &pp
			}

			eff := OrderInventoryEffect{
				OrderID:      orderID,
				OrderItemID:  it.ID,
				ProductID:    prodForEff,
				ProductSKUID: sku.ID,
				EffectType:   EffectTypeDeduct,
				Quantity:     qty,
				Status:       InventoryEffectSuccess,
				BeforeStock:  intPtr(before),
				AfterStock:   intPtr(after),
				Reason:       reasonBase,
				LogID:        &chg.ID,
				CreatedBy:    opts.CreatedBy,
			}
			if err := tx.Create(&eff).Error; err != nil {
				return err
			}
			synced++
		}
		return nil
	})

	if txErr != nil {
		sum := &DeductionSummary{
			Error: txErr.Error(),
		}
		if errors.Is(txErr, ErrInsufficientSKUStock) {
			sum.Message = ErrInsufficientSKUStock.Error()
			return sum, txErr
		}
		return sum, txErr
	}

	if syncAfter && synced > 0 {
		syncedSKU := map[uuid.UUID]struct{}{}
		for _, it := range items {
			if it.ProductSKUID == nil {
				continue
			}
			if _, ok := syncedSKU[*it.ProductSKUID]; ok {
				continue
			}
			var sku product.ProductSKU
			if err := s.DB.WithContext(ctx).First(&sku, "id = ?", *it.ProductSKUID).Error; err != nil {
				continue
			}
			syncedSKU[*it.ProductSKUID] = struct{}{}
			if _, err := s.CreateInventorySyncTasksForSKUStock(ctx, sku.ProductID, sku.ID, derefStock(sku.Stock), opts.CreatedBy); err != nil {
				// platform sync failures do not rollback local deduction
				if s.OpLog != nil {
					_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
						AdminUserID: opts.CreatedBy,
						Action:      "inventory.order_deduct.sync_enqueue_failed",
						Resource:    "order",
						ResourceID:  orderID.String(),
						Status:      "failed",
						Message:     clampStr(err.Error(), 480),
					})
				}
			}
		}
	}

	return &DeductionSummary{
		LinesSynced:  synced,
		LinesSkipped: skippedCount,
		Message:      "ok",
	}, nil
}

// RestorationSummary aggregates restore attempts.
type RestorationSummary struct {
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
	if err := s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", orderID).Error; err != nil {
		return nil, err
	}
	items, err := s.loadOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	syncAfter := opts.SyncPlatforms
	reason := clampStr(strings.TrimSpace(opts.Reason), 128)
	if reason == "" {
		reason = "order_cancel_restore"
	}

	var restored int

	txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		rmBase := remarkForOrderStock(o.OrderNo, "", nil)

		for _, it := range items {
			if it.ProductSKUID == nil || *it.ProductSKUID == uuid.Nil {
				continue
			}
			qty := it.Quantity
			if qty <= 0 {
				continue
			}

			var dHit int64
			if err := tx.Model(&OrderInventoryEffect{}).
				Where("order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ?", it.ID, *it.ProductSKUID, EffectTypeDeduct, InventoryEffectSuccess).
				Count(&dHit).Error; err != nil {
				return err
			}
			if dHit == 0 {
				continue
			}

			var rHit int64
			if err := tx.Model(&OrderInventoryEffect{}).
				Where("order_item_id = ? AND product_sku_id = ? AND effect_type = ? AND status = ?", it.ID, *it.ProductSKUID, EffectTypeRestore, InventoryEffectSuccess).
				Count(&rHit).Error; err != nil {
				return err
			}
			if rHit > 0 {
				continue
			}

			var sku product.ProductSKU
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				First(&sku, "id = ? AND deleted_at IS NULL", it.ProductSKUID).Error; err != nil {
				return err
			}
			before := derefStock(sku.Stock)
			after := before + qty

			chg := InventoryChangeLog{
				ProductID:      sku.ProductID,
				ProductSKUID:   sku.ID,
				ChangeType:     ChangeOrderCancel,
				BeforeStock:    before,
				AfterStock:     after,
				Delta:          qty,
				Reason:         reason,
				Remark:         clampStr(strings.TrimSpace(rmBase)+" orderItem="+it.ID.String(), 520),
				CreatedBy:      opts.CreatedBy,
				RefOrderID:     &orderID,
				RefOrderItemID: &it.ID,
			}
			if err := tx.Create(&chg).Error; err != nil {
				return err
			}
			if err := tx.Model(&product.ProductSKU{}).Where("id = ?", sku.ID).
				Updates(map[string]any{"stock": after, "updated_at": now}).Error; err != nil {
				return err
			}

			var prodForEff *uuid.UUID
			if it.ProductID != nil && *it.ProductID != uuid.Nil {
				pp := *it.ProductID
				prodForEff = &pp
			}

			eff := OrderInventoryEffect{
				OrderID:      orderID,
				OrderItemID:  it.ID,
				ProductID:    prodForEff,
				ProductSKUID: sku.ID,
				EffectType:   EffectTypeRestore,
				Quantity:     qty,
				Status:       InventoryEffectSuccess,
				BeforeStock:  intPtr(before),
				AfterStock:   intPtr(after),
				Reason:       reason,
				LogID:        &chg.ID,
				CreatedBy:    opts.CreatedBy,
			}
			if err := tx.Create(&eff).Error; err != nil {
				return err
			}
			restored++
		}
		return nil
	})

	if txErr != nil {
		return &RestorationSummary{Error: txErr.Error()}, txErr
	}

	if syncAfter && restored > 0 {
		skuSeen := map[uuid.UUID]struct{}{}
		for _, it := range items {
			if it.ProductSKUID == nil {
				continue
			}
			if _, ok := skuSeen[*it.ProductSKUID]; ok {
				continue
			}
			var sku product.ProductSKU
			if err := s.DB.WithContext(ctx).First(&sku, "id = ?", *it.ProductSKUID).Error; err != nil {
				continue
			}
			skuSeen[*it.ProductSKUID] = struct{}{}
			if _, err := s.CreateInventorySyncTasksForSKUStock(ctx, sku.ProductID, sku.ID, derefStock(sku.Stock), opts.CreatedBy); err != nil && s.OpLog != nil {
				_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
					AdminUserID: opts.CreatedBy,
					Action:      "inventory.order_restore.sync_enqueue_failed",
					Resource:    "order",
					ResourceID:  orderID.String(),
					Status:      "failed",
					Message:     clampStr(err.Error(), 480),
				})
			}
		}
	}

	return &RestorationSummary{
		LinesSynced: restored,
		Message:     "ok",
	}, nil
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
	HasDeductionSuccess bool `json:"hasDeductionSuccess"`
	HasRestoreSuccess   bool `json:"hasRestoreSuccess"`
	FullyRestored       bool `json:"fullyRestored"` // heuristic: restore success exists for every deduct-success line with sku
}

func (s *Service) SummarizeOrderInventoryEffects(ctx context.Context, orderID uuid.UUID) (*OrderInventoryUISummary, error) {
	sum := &OrderInventoryUISummary{}
	if s == nil || s.DB == nil {
		return sum, fmt.Errorf("inventory: no db")
	}

	var deductN, deductSkuN int64
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeDeduct, InventoryEffectSuccess).
		Count(&deductN).Error
	sum.HasDeductionSuccess = deductN > 0

	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ? AND product_sku_id <> ?", orderID, EffectTypeDeduct, InventoryEffectSuccess, NilInventorySKUUID).
		Count(&deductSkuN).Error

	var restoreSKU int64
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Where("order_id = ? AND effect_type = ? AND status = ?", orderID, EffectTypeRestore, InventoryEffectSuccess).
		Count(&restoreSKU).Error
	sum.HasRestoreSuccess = restoreSKU > 0
	if deductSkuN > 0 && restoreSKU >= deductSkuN {
		sum.FullyRestored = true
	}
	return sum, nil
}
