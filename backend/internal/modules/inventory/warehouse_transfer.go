package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTransferInvalidInput = errors.New("invalid warehouse transfer")
	ErrTransferAbsent       = errors.New("warehouse transfer not found")
	ErrTransferTransition   = errors.New("warehouse transfer transition is not allowed")
	ErrTransferRevision     = errors.New("warehouse transfer revision conflict")
	ErrTransferIdempotency  = errors.New("warehouse transfer idempotency conflict")
)

func transferPayloadHash(value any) (string, error) { return inventoryRequestHash(value) }

func transferNo() string { return "TRF-" + strings.ToUpper(uuid.NewString()[:12]) }

func (s *Service) transferWarehouseService() *warehouse.Service {
	if s != nil && s.Warehouses != nil {
		return s.Warehouses
	}
	return &warehouse.Service{DB: s.DB}
}

func (s *Service) CreateWarehouseTransfer(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateWarehouseTransferBody) (*WarehouseTransfer, error) {
	if s == nil || s.DB == nil || tenantID < 0 || in.SourceWarehouseID == uuid.Nil || in.TargetWarehouseID == uuid.Nil || in.SourceWarehouseID == in.TargetWarehouseID {
		return nil, ErrTransferInvalidInput
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if len(key) < 8 || len(key) > 128 || len(in.Items) == 0 || len(in.Items) > 100 {
		return nil, ErrTransferInvalidInput
	}
	reason, remark := clampStr(in.Reason, 128), clampStr(in.Remark, 520)
	payload := struct {
		Source, Target uuid.UUID
		Reason, Remark string
		Items          []CreateWarehouseTransferItem
	}{in.SourceWarehouseID, in.TargetWarehouseID, reason, remark, in.Items}
	hash, err := transferPayloadHash(payload)
	if err != nil {
		return nil, err
	}
	var out *WarehouseTransfer
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var replay WarehouseTransfer
		if err := tx.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).Preload("Items").First(&replay).Error; err == nil {
			if replay.PayloadHash != hash {
				return ErrTransferIdempotency
			}
			out = &replay
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		ws := s.transferWarehouseService()
		if _, err := ws.RequireActive(ctx, tx, tenantID, in.SourceWarehouseID); err != nil {
			return ErrTransferInvalidInput
		}
		if _, err := ws.RequireActive(ctx, tx, tenantID, in.TargetWarehouseID); err != nil {
			return ErrTransferInvalidInput
		}
		seen := make(map[uuid.UUID]bool, len(in.Items))
		items := make([]WarehouseTransferItem, 0, len(in.Items))
		for _, item := range in.Items {
			if item.ProductSKUID == uuid.Nil || item.Quantity <= 0 || seen[item.ProductSKUID] {
				return ErrTransferInvalidInput
			}
			seen[item.ProductSKUID] = true
			var sku product.ProductSKU
			if err := tx.Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").Where("product_skus.id = ? AND products.tenant_id = ?", item.ProductSKUID, tenantID).First(&sku).Error; err != nil {
				return gorm.ErrRecordNotFound
			}
			items = append(items, WarehouseTransferItem{TenantID: tenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID, Quantity: item.Quantity})
		}
		row := &WarehouseTransfer{TenantID: tenantID, TransferNo: transferNo(), SourceWarehouseID: in.SourceWarehouseID, TargetWarehouseID: in.TargetWarehouseID, Status: TransferDraft, Revision: 1, IdempotencyKey: key, PayloadHash: hash, Reason: reason, Remark: remark, CreatedBy: actor, Items: items}
		if err := tx.Create(row).Error; err != nil {
			if isInventoryUniqueViolation(err) {
				return ErrTransferIdempotency
			}
			return err
		}
		out = row
		return nil
	})
	return out, err
}

func (s *Service) getTransferTx(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID, lock bool) (*WarehouseTransfer, error) {
	var row WarehouseTransfer
	q := tx.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Preload("Items")
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTransferAbsent
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) GetWarehouseTransfer(ctx context.Context, tenantID int64, id uuid.UUID) (*WarehouseTransfer, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: db unavailable")
	}
	return s.getTransferTx(ctx, s.DB, tenantID, id, false)
}

func (s *Service) ListWarehouseTransfers(ctx context.Context, tenantID int64, page, pageSize int, status string) (*WarehouseTransferListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: db unavailable")
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
	q := s.DB.WithContext(ctx).Model(&WarehouseTransfer{}).Where("warehouse_transfers.tenant_id = ?", tenantID)
	if strings.TrimSpace(status) != "" {
		q = q.Where("warehouse_transfers.status = ?", strings.TrimSpace(status))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []WarehouseTransferListRow
	err := q.Select("warehouse_transfers.*, sw.code AS source_warehouse_code, sw.name AS source_warehouse_name, tw.code AS target_warehouse_code, tw.name AS target_warehouse_name, (SELECT COUNT(*) FROM warehouse_transfer_items i WHERE i.transfer_id = warehouse_transfers.id) AS item_count").Joins("JOIN warehouses sw ON sw.id = warehouse_transfers.source_warehouse_id AND sw.tenant_id = warehouse_transfers.tenant_id").Joins("JOIN warehouses tw ON tw.id = warehouse_transfers.target_warehouse_id AND tw.tenant_id = warehouse_transfers.tenant_id").Order("warehouse_transfers.created_at DESC, warehouse_transfers.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return &WarehouseTransferListResult{List: rows, Total: total, Page: page, PageSize: pageSize, TotalPages: pagesOf(total, pageSize)}, nil
}

func (s *Service) transitionWarehouseTransfer(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, action string, in WarehouseTransferActionBody) (*WarehouseTransfer, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil || len(strings.TrimSpace(in.IdempotencyKey)) < 8 || len(strings.TrimSpace(in.IdempotencyKey)) > 128 {
		return nil, ErrTransferInvalidInput
	}
	if in.ExpectedRevision < 1 {
		return nil, ErrTransferRevision
	}
	key, reason := strings.TrimSpace(in.IdempotencyKey), clampStr(in.Reason, 128)
	hash, err := transferPayloadHash(struct {
		Action   string
		Revision int
		Reason   string
	}{action, in.ExpectedRevision, reason})
	if err != nil {
		return nil, err
	}
	var out *WarehouseTransfer
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := s.getTransferTx(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		var done WarehouseTransferAction
		if err := tx.Where("tenant_id = ? AND transfer_id = ? AND action = ?", tenantID, id, action).First(&done).Error; err == nil {
			if done.RequestHash != hash || done.IdempotencyKey != key {
				return ErrTransferIdempotency
			}
			out = row
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Revision != in.ExpectedRevision {
			return ErrTransferRevision
		}
		now := time.Now().UTC()
		switch action {
		case "submit":
			if row.Status != TransferDraft {
				return ErrTransferTransition
			}
			row.Status = TransferPendingApproval
		case "approve":
			if row.Status != TransferPendingApproval {
				return ErrTransferTransition
			}
			row.Status = TransferApproved
			row.ApprovedAt = &now
			row.ApprovedBy = actor
		case "cancel":
			if row.Status != TransferDraft && row.Status != TransferPendingApproval && row.Status != TransferApproved {
				return ErrTransferTransition
			}
			row.Status = TransferCancelled
			row.CancelledAt = &now
		case "dispatch":
			if row.Status != TransferApproved {
				return ErrTransferTransition
			}
			if err := s.dispatchTransferTx(ctx, tx, tenantID, row, actor); err != nil {
				return err
			}
			row.Status = TransferInTransit
			row.DispatchedAt = &now
		case "receive":
			if row.Status != TransferInTransit {
				return ErrTransferTransition
			}
			if err := s.receiveTransferTx(ctx, tx, tenantID, row, actor); err != nil {
				return err
			}
			row.Status = TransferReceived
			row.ReceivedAt = &now
		default:
			return ErrTransferInvalidInput
		}
		row.Revision++
		if err := tx.Model(&WarehouseTransfer{}).Where("id = ? AND tenant_id = ? AND revision = ?", id, tenantID, row.Revision-1).Updates(map[string]any{"status": row.Status, "revision": row.Revision, "approved_by": row.ApprovedBy, "approved_at": row.ApprovedAt, "dispatched_at": row.DispatchedAt, "received_at": row.ReceivedAt, "cancelled_at": row.CancelledAt, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Create(&WarehouseTransferAction{TenantID: tenantID, TransferID: id, Action: action, IdempotencyKey: key, RequestHash: hash}).Error; err != nil {
			if isInventoryUniqueViolation(err) {
				return ErrTransferIdempotency
			}
			return err
		}
		out = row
		return nil
	})
	return out, err
}

func (s *Service) dispatchTransferTx(ctx context.Context, tx *gorm.DB, tenantID int64, row *WarehouseTransfer, actor *uuid.UUID) error {
	for _, item := range row.Items {
		var sku product.ProductSKU
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").Where("product_skus.id = ? AND products.tenant_id = ?", item.ProductSKUID, tenantID).First(&sku).Error; err != nil {
			return gorm.ErrRecordNotFound
		}
		var sourceWarehouse warehouse.Warehouse
		if err := tx.Where("id = ? AND tenant_id = ? AND status = ?", row.SourceWarehouseID, tenantID, warehouse.StatusActive).First(&sourceWarehouse).Error; err != nil {
			return err
		}
		if sourceWarehouse.IsDefault {
			if _, err := ensureLegacyBalanceTx(ctx, tx, tenantID, sku, row.SourceWarehouseID, actor); err != nil {
				return err
			}
		}
		var b WarehouseStockBalance
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, row.SourceWarehouseID, item.ProductSKUID).First(&b).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			b = WarehouseStockBalance{TenantID: tenantID, WarehouseID: row.SourceWarehouseID, ProductSKUID: item.ProductSKUID, Version: 1}
			if err := tx.Create(&b).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if b.Available() < item.Quantity {
			return fmt.Errorf("%w: sku %s available stock is insufficient", ErrTransferInvalidInput, item.ProductSKUID)
		}
		before := b.OnHand
		b.OnHand -= item.Quantity
		b.InTransit += item.Quantity
		prev := b.Version
		b.Version++
		if result := tx.Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", b.ID, prev).Updates(map[string]any{"on_hand": b.OnHand, "in_transit": b.InTransit, "version": b.Version, "updated_at": time.Now().UTC()}); result.Error != nil || result.RowsAffected != 1 {
			return ErrTransferRevision
		}
		movement := InventoryMovement{TenantID: tenantID, WarehouseID: row.SourceWarehouseID, ProductID: sku.ProductID, ProductSKUID: item.ProductSKUID, MovementType: MovementTransferDispatch, Quantity: -item.Quantity, BeforeOnHand: before, AfterOnHand: b.OnHand, SourceType: "warehouse_transfer", SourceID: row.ID, BusinessEventKey: fmt.Sprintf("transfer:%s:dispatch:%s", row.ID, item.ProductSKUID), CreatedBy: actor, Reason: row.Reason, Remark: row.Remark}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) receiveTransferTx(ctx context.Context, tx *gorm.DB, tenantID int64, row *WarehouseTransfer, actor *uuid.UUID) error {
	for _, item := range row.Items {
		var sku product.ProductSKU
		if err := tx.Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").Where("product_skus.id = ? AND products.tenant_id = ?", item.ProductSKUID, tenantID).First(&sku).Error; err != nil {
			return gorm.ErrRecordNotFound
		}
		var source WarehouseStockBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, row.SourceWarehouseID, item.ProductSKUID).First(&source).Error; err != nil {
			return err
		}
		if source.InTransit < item.Quantity {
			return fmt.Errorf("%w: in-transit quantity is insufficient", ErrTransferInvalidInput)
		}
		var target WarehouseStockBalance
		if _, err := s.transferWarehouseService().RequireActive(ctx, tx, tenantID, row.TargetWarehouseID); err != nil {
			return ErrTransferInvalidInput
		}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, row.TargetWarehouseID, item.ProductSKUID).First(&target).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			target = WarehouseStockBalance{TenantID: tenantID, WarehouseID: row.TargetWarehouseID, ProductSKUID: item.ProductSKUID, Version: 1}
			if err := tx.Create(&target).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		sourcePrev := source.Version
		source.InTransit -= item.Quantity
		source.Version++
		if result := tx.Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", source.ID, sourcePrev).Updates(map[string]any{"in_transit": source.InTransit, "version": source.Version, "updated_at": time.Now().UTC()}); result.Error != nil || result.RowsAffected != 1 {
			return ErrTransferRevision
		}
		before := target.OnHand
		target.OnHand += item.Quantity
		target.Version++
		if result := tx.Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", target.ID, target.Version-1).Updates(map[string]any{"on_hand": target.OnHand, "version": target.Version, "updated_at": time.Now().UTC()}); result.Error != nil || result.RowsAffected != 1 {
			return ErrTransferRevision
		}
		movement := InventoryMovement{TenantID: tenantID, WarehouseID: row.TargetWarehouseID, ProductID: sku.ProductID, ProductSKUID: item.ProductSKUID, MovementType: MovementTransferReceive, Quantity: item.Quantity, BeforeOnHand: before, AfterOnHand: target.OnHand, SourceType: "warehouse_transfer", SourceID: row.ID, BusinessEventKey: fmt.Sprintf("transfer:%s:receive:%s", row.ID, item.ProductSKUID), CreatedBy: actor, Reason: row.Reason, Remark: row.Remark}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
		if err := tx.Model(&WarehouseTransferItem{}).Where("id = ? AND transfer_id = ? AND tenant_id = ?", item.ID, row.ID, tenantID).Update("received_quantity", item.Quantity).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SubmitWarehouseTransfer(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in WarehouseTransferActionBody) (*WarehouseTransfer, error) {
	return s.transitionWarehouseTransfer(ctx, tenantID, id, actor, "submit", in)
}
func (s *Service) ApproveWarehouseTransfer(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in WarehouseTransferActionBody) (*WarehouseTransfer, error) {
	return s.transitionWarehouseTransfer(ctx, tenantID, id, actor, "approve", in)
}
func (s *Service) DispatchWarehouseTransfer(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in WarehouseTransferActionBody) (*WarehouseTransfer, error) {
	return s.transitionWarehouseTransfer(ctx, tenantID, id, actor, "dispatch", in)
}
func (s *Service) ReceiveWarehouseTransfer(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in WarehouseTransferActionBody) (*WarehouseTransfer, error) {
	return s.transitionWarehouseTransfer(ctx, tenantID, id, actor, "receive", in)
}
func (s *Service) CancelWarehouseTransfer(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in WarehouseTransferActionBody) (*WarehouseTransfer, error) {
	return s.transitionWarehouseTransfer(ctx, tenantID, id, actor, "cancel", in)
}
