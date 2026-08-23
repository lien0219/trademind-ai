package salesreturn

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	ordermod "github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxItems       = 100
	maxQuantity    = 1_000_000
	maxRefundMinor = int64(1_000_000_000_000_000)
)

var (
	ErrAbsent               = errors.New("sales return not found")
	ErrInvalidInput         = errors.New("invalid sales return input")
	ErrInvalidTransition    = errors.New("invalid sales return transition")
	ErrRevisionConflict     = errors.New("sales return revision conflict")
	ErrIdempotencyConflict  = errors.New("sales return idempotency conflict")
	ErrOverReturn           = errors.New("sales return quantity exceeds deducted quantity")
	ErrDutyConflict         = errors.New("sales return approver cannot receive the return")
	ErrWarehouseUnavailable = errors.New("sales return warehouse unavailable")
)

type Service struct {
	DB         *gorm.DB
	Stock      inventory.WarehouseStockService
	Warehouses *warehouse.Service
}

func (s *Service) ready() error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("sales return: database unavailable")
	}
	return nil
}

func (s *Service) warehouseService() *warehouse.Service {
	if s.Warehouses != nil {
		return s.Warehouses
	}
	return &warehouse.Service{DB: s.DB}
}

func (s *Service) Create(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateInput) (*SalesReturn, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	returnType := strings.ToLower(strings.TrimSpace(in.Type))
	reason := strings.TrimSpace(in.Reason)
	remark := strings.TrimSpace(in.Remark)
	if tenantID < 0 || in.OrderID == uuid.Nil || len(key) < 8 || len(key) > 128 || !validType(returnType) || reason == "" || len([]rune(reason)) > 128 || len([]rune(remark)) > 520 || len(in.Items) == 0 || len(in.Items) > maxItems {
		return nil, ErrInvalidInput
	}
	items := append([]CreateItemInput(nil), in.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].OrderItemID.String() < items[j].OrderItemID.String() })
	var totalRefund int64
	for i := range items {
		items[i].Disposition = strings.ToLower(strings.TrimSpace(items[i].Disposition))
		item := items[i]
		if item.OrderItemID == uuid.Nil || item.Quantity < 1 || item.Quantity > maxQuantity || item.RefundAmountMinor < 0 || item.RefundAmountMinor > maxRefundMinor || (i > 0 && item.OrderItemID == items[i-1].OrderItemID) {
			return nil, ErrInvalidInput
		}
		if returnType == TypeRefundOnly && item.Disposition != "" {
			return nil, ErrInvalidInput
		}
		if returnType == TypeReturnRefund && item.Disposition != inventory.ReturnDispositionSellable && item.Disposition != inventory.ReturnDispositionDamaged {
			return nil, ErrInvalidInput
		}
		if totalRefund > maxRefundMinor-item.RefundAmountMinor {
			return nil, ErrInvalidInput
		}
		totalRefund += item.RefundAmountMinor
	}
	if totalRefund < 1 {
		return nil, ErrInvalidInput
	}
	hash := createPayloadHash(in.OrderID, returnType, reason, remark, items)
	if existing, err := loadByKey(s.DB.WithContext(ctx), tenantID, key); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.PayloadHash != hash {
			return nil, ErrIdempotencyConflict
		}
		return s.Get(ctx, tenantID, existing.ID)
	}

	var returnID uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, err := loadByKey(tx, tenantID, key); err != nil {
			return err
		} else if existing != nil {
			if existing.PayloadHash != hash {
				return ErrIdempotencyConflict
			}
			returnID = existing.ID
			return nil
		}

		var orderRow ordermod.Order
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", in.OrderID, tenantID).First(&orderRow).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidInput
		}
		if err != nil || orderRow.WarehouseID == nil || *orderRow.WarehouseID == uuid.Nil {
			return ErrInvalidInput
		}
		if returnType == TypeReturnRefund {
			if _, err := s.warehouseService().RequireActive(ctx, tx, tenantID, *orderRow.WarehouseID); err != nil {
				return ErrWarehouseUnavailable
			}
		}

		itemIDs := make([]uuid.UUID, 0, len(items))
		for _, item := range items {
			itemIDs = append(itemIDs, item.OrderItemID)
		}
		var orderItems []ordermod.OrderItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("order_id = ? AND id IN ?", orderRow.ID, itemIDs).Find(&orderItems).Error; err != nil {
			return fmt.Errorf("load sales return order items: %w", err)
		}
		if len(orderItems) != len(itemIDs) {
			return ErrInvalidInput
		}
		byID := make(map[uuid.UUID]ordermod.OrderItem, len(orderItems))
		for _, item := range orderItems {
			byID[item.ID] = item
		}
		deducted, restored, err := orderEffectQuantities(tx, tenantID, orderRow.ID, itemIDs)
		if err != nil {
			return err
		}
		allocated, err := allocatedQuantities(tx, tenantID, itemIDs)
		if err != nil {
			return err
		}

		returnItems := make([]SalesReturnItem, 0, len(items))
		for _, item := range items {
			orderItem := byID[item.OrderItemID]
			deduct := deducted[item.OrderItemID]
			remaining := deduct.Quantity - restored[item.OrderItemID] - allocated[item.OrderItemID]
			if orderItem.ID == uuid.Nil || orderItem.ProductSKUID == nil || deduct.ProductSKUID == uuid.Nil || *orderItem.ProductSKUID != deduct.ProductSKUID || deduct.WarehouseID == nil || *deduct.WarehouseID != *orderRow.WarehouseID || item.Quantity > remaining {
				return ErrOverReturn
			}
			returnItems = append(returnItems, SalesReturnItem{
				TenantID: tenantID, OrderItemID: item.OrderItemID, ProductSKUID: deduct.ProductSKUID,
				Quantity: item.Quantity, Disposition: item.Disposition, RefundAmountMinor: item.RefundAmountMinor,
			})
		}

		row := &SalesReturn{
			TenantID: tenantID, ReturnNo: newReturnNumber(), IdempotencyKey: key, PayloadHash: hash,
			OrderID: orderRow.ID, WarehouseID: *orderRow.WarehouseID, Type: returnType, Status: StatusDraft,
			Currency: strings.ToUpper(strings.TrimSpace(orderRow.Currency)), RefundAmountMinor: totalRefund,
			Revision: 1, Reason: reason, Remark: remark, CreatedBy: actor,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row)
		if created.Error != nil {
			return fmt.Errorf("create sales return: %w", created.Error)
		}
		if created.RowsAffected == 0 {
			existing, err := loadByKey(tx, tenantID, key)
			if err != nil {
				return err
			}
			if existing == nil || existing.PayloadHash != hash {
				return ErrIdempotencyConflict
			}
			returnID = existing.ID
			return nil
		}
		for i := range returnItems {
			returnItems[i].SalesReturnID = row.ID
		}
		if err := tx.Create(&returnItems).Error; err != nil {
			return fmt.Errorf("create sales return items: %w", err)
		}
		returnID = row.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, returnID)
}

func (s *Service) Get(ctx context.Context, tenantID int64, id uuid.UUID) (*SalesReturn, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 0 || id == uuid.Nil {
		return nil, ErrAbsent
	}
	var row SalesReturn
	err := s.DB.WithContext(ctx).Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC, id ASC") }).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("get sales return: %w", err)
	}
	if err := enrich(ctx, s.DB, tenantID, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) List(ctx context.Context, tenantID int64, page, pageSize int, status, returnType string, orderID *uuid.UUID) (*ListResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query := s.DB.WithContext(ctx).Model(&SalesReturn{}).Where("sales_returns.tenant_id = ?", tenantID)
	if value := strings.TrimSpace(status); value != "" {
		if !validStatus(value) {
			return nil, ErrInvalidInput
		}
		query = query.Where("sales_returns.status = ?", value)
	}
	if value := strings.TrimSpace(returnType); value != "" {
		if !validType(value) {
			return nil, ErrInvalidInput
		}
		query = query.Where("sales_returns.type = ?", value)
	}
	if orderID != nil && *orderID != uuid.Nil {
		query = query.Where("sales_returns.order_id = ?", *orderID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count sales returns: %w", err)
	}
	var rows []ListRow
	err := query.Select("sales_returns.*, orders.order_no, warehouses.name AS warehouse_name, (SELECT COUNT(*) FROM sales_return_items i WHERE i.sales_return_id = sales_returns.id AND i.tenant_id = sales_returns.tenant_id) AS item_count").
		Joins("JOIN orders ON orders.id = sales_returns.order_id AND orders.tenant_id = sales_returns.tenant_id AND orders.deleted_at IS NULL").
		Joins("JOIN warehouses ON warehouses.id = sales_returns.warehouse_id AND warehouses.tenant_id = sales_returns.tenant_id").
		Order("sales_returns.created_at DESC, sales_returns.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list sales returns: %w", err)
	}
	return &ListResult{List: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}

func (s *Service) ListReturnableItems(ctx context.Context, tenantID int64, orderID uuid.UUID) (*ReturnableItemResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 0 || orderID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var orderRow ordermod.Order
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", orderID, tenantID).First(&orderRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAbsent
		}
		return nil, err
	}
	if orderRow.WarehouseID == nil || *orderRow.WarehouseID == uuid.Nil {
		return &ReturnableItemResult{OrderID: orderRow.ID, OrderNo: orderRow.OrderNo, Currency: orderRow.Currency, List: []ReturnableItem{}}, nil
	}
	var orderItems []ordermod.OrderItem
	if err := s.DB.WithContext(ctx).Where("order_id = ? AND product_sku_id IS NOT NULL", orderRow.ID).Order("created_at ASC, id ASC").Find(&orderItems).Error; err != nil {
		return nil, fmt.Errorf("list sales return order items: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(orderItems))
	for _, item := range orderItems {
		ids = append(ids, item.ID)
	}
	out := &ReturnableItemResult{OrderID: orderRow.ID, OrderNo: orderRow.OrderNo, WarehouseID: *orderRow.WarehouseID, Currency: orderRow.Currency, List: []ReturnableItem{}}
	if len(ids) == 0 {
		return out, nil
	}
	deducted, restored, err := orderEffectQuantities(s.DB.WithContext(ctx), tenantID, orderRow.ID, ids)
	if err != nil {
		return nil, err
	}
	allocated, err := allocatedQuantities(s.DB.WithContext(ctx), tenantID, ids)
	if err != nil {
		return nil, err
	}
	for _, item := range orderItems {
		deduct := deducted[item.ID]
		remaining := deduct.Quantity - restored[item.ID] - allocated[item.ID]
		if remaining <= 0 || item.ProductSKUID == nil || deduct.ProductSKUID == uuid.Nil || deduct.WarehouseID == nil || *deduct.WarehouseID != *orderRow.WarehouseID {
			continue
		}
		out.List = append(out.List, ReturnableItem{
			OrderItemID: item.ID, ProductSKUID: deduct.ProductSKUID, ProductTitle: item.ProductTitle,
			SKUCode: item.SKUCode, SKUName: item.SKUName, UnitPrice: item.UnitPrice,
			DeductedQuantity: deduct.Quantity, RestoredQuantity: restored[item.ID], AllocatedReturnQuantity: allocated[item.ID], RemainingQuantity: remaining,
		})
	}
	return out, nil
}

func (s *Service) Submit(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in ActionInput) (*SalesReturn, error) {
	return s.transition(ctx, tenantID, id, actor, "submit", in)
}

func (s *Service) Approve(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in ActionInput) (*SalesReturn, error) {
	return s.transition(ctx, tenantID, id, actor, "approve", in)
}

func (s *Service) Complete(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in ActionInput) (*SalesReturn, error) {
	return s.transition(ctx, tenantID, id, actor, "complete", in)
}

func (s *Service) Cancel(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in ActionInput) (*SalesReturn, error) {
	return s.transition(ctx, tenantID, id, actor, "cancel", in)
}

func (s *Service) transition(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, action string, in ActionInput) (*SalesReturn, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	if tenantID < 0 || id == uuid.Nil || in.ExpectedRevision < 1 || len(key) < 8 || len(key) > 128 || len([]rune(reason)) > 128 {
		return nil, ErrInvalidInput
	}
	if (action == "approve" || action == "complete") && (actor == nil || *actor == uuid.Nil) {
		return nil, ErrInvalidInput
	}
	hash := actionPayloadHash(action, in.ExpectedRevision, reason)
	var outID uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row SalesReturn
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAbsent
		}
		if err != nil {
			return err
		}
		var done SalesReturnAction
		if err := tx.Where("tenant_id = ? AND sales_return_id = ? AND action = ?", tenantID, id, action).First(&done).Error; err == nil {
			if done.IdempotencyKey != key || done.RequestHash != hash {
				return ErrIdempotencyConflict
			}
			outID = row.ID
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var used SalesReturnAction
		if err := tx.Where("tenant_id = ? AND sales_return_id = ? AND idempotency_key = ?", tenantID, id, key).First(&used).Error; err == nil {
			return ErrIdempotencyConflict
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Revision != in.ExpectedRevision {
			return ErrRevisionConflict
		}

		fromStatus := row.Status
		now := time.Now().UTC()
		switch action {
		case "submit":
			if row.Status != StatusDraft {
				return ErrInvalidTransition
			}
			row.Status, row.SubmittedBy, row.SubmittedAt = StatusPendingApproval, actor, &now
		case "approve":
			if row.Status != StatusPendingApproval {
				return ErrInvalidTransition
			}
			row.Status, row.ApprovedBy, row.ApprovedAt = StatusApproved, actor, &now
		case "complete":
			if row.Status != StatusApproved {
				return ErrInvalidTransition
			}
			if row.ApprovedBy == nil || actor == nil || *row.ApprovedBy == *actor {
				return ErrDutyConflict
			}
			if row.Type == TypeReturnRefund {
				if err := s.receiveTx(ctx, tx, &row, actor, reason); err != nil {
					return err
				}
			}
			row.Status, row.CompletedBy, row.CompletedAt = StatusCompleted, actor, &now
		case "cancel":
			if row.Status != StatusDraft && row.Status != StatusPendingApproval && row.Status != StatusApproved {
				return ErrInvalidTransition
			}
			row.Status, row.CancelledBy, row.CancelledAt = StatusCancelled, actor, &now
		default:
			return ErrInvalidInput
		}

		row.Revision++
		updated := tx.Model(&SalesReturn{}).Where("id = ? AND tenant_id = ? AND revision = ?", row.ID, tenantID, row.Revision-1).Updates(map[string]any{
			"status": row.Status, "revision": row.Revision, "submitted_by": row.SubmittedBy, "submitted_at": row.SubmittedAt,
			"approved_by": row.ApprovedBy, "approved_at": row.ApprovedAt, "completed_by": row.CompletedBy, "completed_at": row.CompletedAt,
			"cancelled_by": row.CancelledBy, "cancelled_at": row.CancelledAt, "updated_at": now,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrRevisionConflict
		}
		actionRow := &SalesReturnAction{
			TenantID: tenantID, SalesReturnID: row.ID, Action: action, IdempotencyKey: key, RequestHash: hash,
			ActorID: actor, FromStatus: fromStatus, ToStatus: row.Status, Reason: reason,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(actionRow)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected != 1 {
			return ErrIdempotencyConflict
		}
		outID = row.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, outID)
}

func (s *Service) receiveTx(ctx context.Context, tx *gorm.DB, row *SalesReturn, actor *uuid.UUID, reason string) error {
	if row == nil || row.WarehouseID == uuid.Nil || len(row.Items) == 0 {
		return ErrInvalidInput
	}
	if _, err := s.warehouseService().RequireActive(ctx, tx, row.TenantID, row.WarehouseID); err != nil {
		return ErrWarehouseUnavailable
	}
	for _, item := range row.Items {
		eventKey := "sales_return:" + row.ID.String() + ":" + item.ID.String()
		stockResult, err := s.Stock.ReceiveSalesReturn(ctx, tx, inventory.SalesReturnStockInput{
			TenantID: row.TenantID, WarehouseID: row.WarehouseID, ProductSKUID: item.ProductSKUID,
			Quantity: item.Quantity, Disposition: item.Disposition, SalesReturnID: row.ID, SalesReturnItemID: item.ID,
			OrderID: row.OrderID, OrderItemID: item.OrderItemID, BusinessEventKey: eventKey, Reason: reason, CreatedBy: actor,
		})
		if err != nil {
			return err
		}
		effect := &SalesReturnInventoryEffect{
			TenantID: row.TenantID, SalesReturnID: row.ID, SalesReturnItemID: item.ID, OrderID: row.OrderID,
			OrderItemID: item.OrderItemID, WarehouseID: row.WarehouseID, ProductSKUID: item.ProductSKUID,
			Disposition: item.Disposition, Quantity: item.Quantity,
			BeforeOnHand: stockResult.Movement.BeforeOnHand, AfterOnHand: stockResult.Movement.AfterOnHand,
			BeforeDamaged: stockResult.Movement.BeforeDamaged, AfterDamaged: stockResult.Movement.AfterDamaged,
			MovementID: stockResult.Movement.ID, ChangeLogID: stockResult.ChangeLog.ID, BusinessEventKey: eventKey,
		}
		if err := tx.Create(effect).Error; err != nil {
			return fmt.Errorf("create sales return inventory effect: %w", err)
		}
	}
	return nil
}

func enrich(ctx context.Context, db *gorm.DB, tenantID int64, row *SalesReturn) error {
	type header struct {
		OrderNo       string
		WarehouseName string
	}
	var labels header
	if err := db.WithContext(ctx).Table("orders").Select("orders.order_no, warehouses.name AS warehouse_name").
		Joins("JOIN warehouses ON warehouses.id = ? AND warehouses.tenant_id = orders.tenant_id", row.WarehouseID).
		Where("orders.id = ? AND orders.tenant_id = ? AND orders.deleted_at IS NULL", row.OrderID, tenantID).Scan(&labels).Error; err != nil {
		return fmt.Errorf("load sales return header labels: %w", err)
	}
	row.OrderNo, row.WarehouseName = labels.OrderNo, labels.WarehouseName
	if len(row.Items) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(row.Items))
	for _, item := range row.Items {
		ids = append(ids, item.OrderItemID)
	}
	var orderItems []ordermod.OrderItem
	if err := db.WithContext(ctx).Where("order_id = ? AND id IN ?", row.OrderID, ids).Find(&orderItems).Error; err != nil {
		return fmt.Errorf("load sales return item labels: %w", err)
	}
	byID := make(map[uuid.UUID]ordermod.OrderItem, len(orderItems))
	for _, item := range orderItems {
		byID[item.ID] = item
	}
	deducted, _, err := orderEffectQuantities(db.WithContext(ctx), tenantID, row.OrderID, ids)
	if err != nil {
		return err
	}
	for i := range row.Items {
		item := byID[row.Items[i].OrderItemID]
		row.Items[i].ProductTitle, row.Items[i].SKUCode, row.Items[i].SKUName = item.ProductTitle, item.SKUCode, item.SKUName
		row.Items[i].DeductedQuantity = deducted[row.Items[i].OrderItemID].Quantity
	}
	return nil
}

func orderEffectQuantities(tx *gorm.DB, tenantID int64, orderID uuid.UUID, itemIDs []uuid.UUID) (map[uuid.UUID]inventory.OrderInventoryEffect, map[uuid.UUID]int, error) {
	var rows []inventory.OrderInventoryEffect
	if err := tx.Where("tenant_id = ? AND order_id = ? AND order_item_id IN ? AND status = ? AND effect_type IN ?", tenantID, orderID, itemIDs, inventory.InventoryEffectSuccess, []string{inventory.EffectTypeDeduct, inventory.EffectTypeRestore}).Find(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("load sales return inventory effects: %w", err)
	}
	deducted := make(map[uuid.UUID]inventory.OrderInventoryEffect, len(itemIDs))
	restored := make(map[uuid.UUID]int, len(itemIDs))
	for _, effect := range rows {
		if effect.EffectType == inventory.EffectTypeDeduct {
			deducted[effect.OrderItemID] = effect
		} else if effect.EffectType == inventory.EffectTypeRestore {
			restored[effect.OrderItemID] += effect.Quantity
		}
	}
	return deducted, restored, nil
}

func allocatedQuantities(tx *gorm.DB, tenantID int64, itemIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	type allocation struct {
		OrderItemID uuid.UUID
		Quantity    int
	}
	var rows []allocation
	err := tx.Table("sales_return_items sri").Select("sri.order_item_id, SUM(sri.quantity) AS quantity").
		Joins("JOIN sales_returns sr ON sr.id = sri.sales_return_id AND sr.tenant_id = sri.tenant_id").
		Where("sri.tenant_id = ? AND sri.order_item_id IN ? AND sr.status <> ?", tenantID, itemIDs, StatusCancelled).
		Group("sri.order_item_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load allocated sales return quantities: %w", err)
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		out[row.OrderItemID] = row.Quantity
	}
	return out, nil
}

func loadByKey(tx *gorm.DB, tenantID int64, key string) (*SalesReturn, error) {
	var row SalesReturn
	err := tx.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load sales return by idempotency key: %w", err)
	}
	return &row, nil
}

func createPayloadHash(orderID uuid.UUID, returnType, reason, remark string, items []CreateItemInput) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%s|%s|%s|", orderID, returnType, reason, remark)
	for _, item := range items {
		_, _ = fmt.Fprintf(hash, "%s:%d:%s:%d|", item.OrderItemID, item.Quantity, item.Disposition, item.RefundAmountMinor)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func actionPayloadHash(action string, revision int, reason string) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%d|%s", action, revision, reason)
	return hex.EncodeToString(hash.Sum(nil))
}

func newReturnNumber() string {
	return fmt.Sprintf("SR-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(uuid.NewString()[:8]))
}

func validType(value string) bool {
	return value == TypeRefundOnly || value == TypeReturnRefund
}

func validStatus(value string) bool {
	switch value {
	case StatusDraft, StatusPendingApproval, StatusApproved, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}
