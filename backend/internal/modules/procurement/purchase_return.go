package procurement

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
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPurchaseReturnAbsent      = errors.New("purchase return not found")
	ErrReturnInvalidInput        = errors.New("invalid purchase return input")
	ErrReturnInvalidTransition   = errors.New("invalid purchase return transition")
	ErrReturnRevisionConflict    = errors.New("purchase return revision conflict")
	ErrReturnIdempotencyConflict = errors.New("purchase return idempotency conflict")
	ErrOverReturn                = errors.New("return quantity exceeds received quantity")
	ErrReturnInsufficientStock   = errors.New("insufficient warehouse stock for purchase return")
	ErrReturnDutyConflict        = errors.New("purchase return approver cannot complete the return")
)

func (s *Service) CreatePurchaseReturn(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreatePurchaseReturnInput) (*PurchaseReturn, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	remark := strings.TrimSpace(in.Remark)
	if tenantID < 0 || in.PurchaseOrderID == uuid.Nil || len(key) < 8 || len(key) > 128 || reason == "" || len([]rune(reason)) > 128 || len([]rune(remark)) > 520 || len(in.Items) == 0 || len(in.Items) > maxOrderItems {
		return nil, ErrReturnInvalidInput
	}
	items := append([]CreatePurchaseReturnItemInput(nil), in.Items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].GoodsReceiptItemID.String() < items[j].GoodsReceiptItemID.String()
	})
	for i, item := range items {
		if item.GoodsReceiptItemID == uuid.Nil || item.Quantity < 1 || item.Quantity > maxQuantity || (i > 0 && item.GoodsReceiptItemID == items[i-1].GoodsReceiptItemID) {
			return nil, ErrReturnInvalidInput
		}
	}
	hash := purchaseReturnPayloadHash(in.PurchaseOrderID, reason, remark, items)
	if existing, err := loadPurchaseReturnByKey(s.DB.WithContext(ctx), tenantID, key); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.PayloadHash != hash {
			return nil, ErrReturnIdempotencyConflict
		}
		return s.GetPurchaseReturn(ctx, tenantID, existing.ID)
	}

	var returnID uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, err := loadPurchaseReturnByKey(tx, tenantID, key); err != nil {
			return err
		} else if existing != nil {
			if existing.PayloadHash != hash {
				return ErrReturnIdempotencyConflict
			}
			returnID = existing.ID
			return nil
		}

		var po PurchaseOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", in.PurchaseOrderID, tenantID).First(&po).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrReturnInvalidInput
			}
			return err
		}
		if po.Status != StatusPartiallyReceived && po.Status != StatusReceived && po.Status != StatusClosed {
			return ErrReturnInvalidTransition
		}
		receiptItemIDs := make([]uuid.UUID, 0, len(items))
		for _, item := range items {
			receiptItemIDs = append(receiptItemIDs, item.GoodsReceiptItemID)
		}
		var receiptItems []GoodsReceiptItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN ?", tenantID, receiptItemIDs).Find(&receiptItems).Error; err != nil {
			return fmt.Errorf("load return receipt items: %w", err)
		}
		if len(receiptItems) != len(receiptItemIDs) {
			return ErrReturnInvalidInput
		}
		byID := make(map[uuid.UUID]GoodsReceiptItem, len(receiptItems))
		receiptIDs := make([]uuid.UUID, 0, len(receiptItems))
		for _, item := range receiptItems {
			byID[item.ID] = item
			receiptIDs = append(receiptIDs, item.GoodsReceiptID)
		}
		var receipts []GoodsReceipt
		if err := tx.Where("tenant_id = ? AND purchase_order_id = ? AND warehouse_id = ? AND id IN ?", tenantID, po.ID, po.WarehouseID, receiptIDs).Find(&receipts).Error; err != nil {
			return fmt.Errorf("load return receipts: %w", err)
		}
		validReceipts := make(map[uuid.UUID]bool, len(receipts))
		for _, receipt := range receipts {
			validReceipts[receipt.ID] = true
		}
		if len(validReceipts) != len(uniqueUUIDs(receiptIDs)) {
			return ErrReturnInvalidInput
		}

		allocated, err := allocatedReturnQuantities(tx, tenantID, receiptItemIDs)
		if err != nil {
			return err
		}
		returnItems := make([]PurchaseReturnItem, 0, len(items))
		for _, item := range items {
			receiptItem := byID[item.GoodsReceiptItemID]
			if !validReceipts[receiptItem.GoodsReceiptID] || receiptItem.PurchaseOrderItemID == uuid.Nil || receiptItem.ProductSKUID == uuid.Nil || item.Quantity > receiptItem.Quantity-allocated[receiptItem.ID] {
				return ErrOverReturn
			}
			var poItem PurchaseOrderItem
			if err := tx.Where("id = ? AND tenant_id = ? AND purchase_order_id = ? AND product_sku_id = ?", receiptItem.PurchaseOrderItemID, tenantID, po.ID, receiptItem.ProductSKUID).First(&poItem).Error; err != nil {
				return ErrReturnInvalidInput
			}
			returnItems = append(returnItems, PurchaseReturnItem{
				TenantID: tenantID, GoodsReceiptItemID: receiptItem.ID, PurchaseOrderItemID: poItem.ID,
				ProductSKUID: receiptItem.ProductSKUID, Quantity: item.Quantity,
			})
		}

		row := &PurchaseReturn{
			TenantID: tenantID, ReturnNo: newDocumentNumber("PR"), IdempotencyKey: key, PayloadHash: hash,
			PurchaseOrderID: po.ID, SupplierID: po.SupplierID, WarehouseID: po.WarehouseID,
			Status: ReturnStatusDraft, Revision: 1, Reason: reason, Remark: remark, CreatedBy: actor,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(row)
		if result.Error != nil {
			return fmt.Errorf("create purchase return: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			existing, err := loadPurchaseReturnByKey(tx, tenantID, key)
			if err != nil {
				return err
			}
			if existing == nil || existing.PayloadHash != hash {
				return ErrReturnIdempotencyConflict
			}
			returnID = existing.ID
			return nil
		}
		for i := range returnItems {
			returnItems[i].PurchaseReturnID = row.ID
		}
		if err := tx.Create(&returnItems).Error; err != nil {
			return fmt.Errorf("create purchase return items: %w", err)
		}
		returnID = row.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetPurchaseReturn(ctx, tenantID, returnID)
}

func (s *Service) GetPurchaseReturn(ctx context.Context, tenantID int64, id uuid.UUID) (*PurchaseReturn, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 0 || id == uuid.Nil {
		return nil, ErrPurchaseReturnAbsent
	}
	var row PurchaseReturn
	err := s.DB.WithContext(ctx).Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC, id ASC") }).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPurchaseReturnAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("get purchase return: %w", err)
	}
	if err := enrichPurchaseReturn(ctx, s.DB, tenantID, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) ListPurchaseReturns(ctx context.Context, tenantID int64, page, pageSize int, status string, purchaseOrderID *uuid.UUID) (*PurchaseReturnListResult, error) {
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
	query := s.DB.WithContext(ctx).Model(&PurchaseReturn{}).Where("purchase_returns.tenant_id = ?", tenantID)
	if value := strings.TrimSpace(status); value != "" {
		if !validPurchaseReturnStatus(value) {
			return nil, ErrReturnInvalidInput
		}
		query = query.Where("purchase_returns.status = ?", value)
	}
	if purchaseOrderID != nil && *purchaseOrderID != uuid.Nil {
		query = query.Where("purchase_returns.purchase_order_id = ?", *purchaseOrderID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count purchase returns: %w", err)
	}
	var rows []PurchaseReturnListRow
	err := query.Select("purchase_returns.*, po.purchase_order_no, s.name AS supplier_name, w.name AS warehouse_name, (SELECT COUNT(*) FROM purchase_return_items i WHERE i.purchase_return_id = purchase_returns.id AND i.tenant_id = purchase_returns.tenant_id) AS item_count").
		Joins("JOIN purchase_orders po ON po.id = purchase_returns.purchase_order_id AND po.tenant_id = purchase_returns.tenant_id").
		Joins("JOIN suppliers s ON s.id = purchase_returns.supplier_id AND s.tenant_id = purchase_returns.tenant_id").
		Joins("JOIN warehouses w ON w.id = purchase_returns.warehouse_id AND w.tenant_id = purchase_returns.tenant_id").
		Order("purchase_returns.created_at DESC, purchase_returns.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list purchase returns: %w", err)
	}
	return &PurchaseReturnListResult{List: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}

func (s *Service) ListReturnableReceiptItems(ctx context.Context, tenantID int64, purchaseOrderID uuid.UUID) (*ReturnableReceiptItemResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 0 || purchaseOrderID == uuid.Nil {
		return nil, ErrReturnInvalidInput
	}
	var po PurchaseOrder
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", purchaseOrderID, tenantID).First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPurchaseOrderAbsent
		}
		return nil, err
	}
	if po.Status != StatusPartiallyReceived && po.Status != StatusReceived && po.Status != StatusClosed {
		return &ReturnableReceiptItemResult{List: []ReturnableReceiptItem{}}, nil
	}
	var rows []ReturnableReceiptItem
	err := s.DB.WithContext(ctx).Table("goods_receipt_items gri").
		Select("gri.id AS goods_receipt_item_id, gr.id AS goods_receipt_id, gr.receipt_no, gri.purchase_order_item_id, gri.product_sku_id, products.title AS product_title, product_skus.sku_code, product_skus.sku_name, gri.quantity AS received_quantity, COALESCE(returned.allocated_return_quantity, 0) AS allocated_return_quantity, gri.quantity - COALESCE(returned.allocated_return_quantity, 0) AS remaining_quantity").
		Joins("JOIN goods_receipts gr ON gr.id = gri.goods_receipt_id AND gr.tenant_id = gri.tenant_id").
		Joins("JOIN purchase_order_items poi ON poi.id = gri.purchase_order_item_id AND poi.tenant_id = gri.tenant_id AND poi.purchase_order_id = gr.purchase_order_id AND poi.product_sku_id = gri.product_sku_id").
		Joins("JOIN product_skus ON product_skus.id = gri.product_sku_id").
		Joins("JOIN products ON products.id = product_skus.product_id AND products.tenant_id = gri.tenant_id AND products.deleted_at IS NULL").
		Joins("LEFT JOIN (SELECT pri.goods_receipt_item_id, SUM(pri.quantity) AS allocated_return_quantity FROM purchase_return_items pri JOIN purchase_returns pr ON pr.id = pri.purchase_return_id AND pr.tenant_id = pri.tenant_id WHERE pri.tenant_id = ? AND pr.status <> ? GROUP BY pri.goods_receipt_item_id) returned ON returned.goods_receipt_item_id = gri.id", tenantID, ReturnStatusCancelled).
		Where("gri.tenant_id = ? AND gr.purchase_order_id = ? AND gr.warehouse_id = ?", tenantID, purchaseOrderID, po.WarehouseID).
		Where("gri.quantity - COALESCE(returned.allocated_return_quantity, 0) > 0").
		Order("gr.received_at ASC, gr.id ASC, gri.created_at ASC, gri.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list returnable receipt items: %w", err)
	}
	return &ReturnableReceiptItemResult{List: rows}, nil
}

func (s *Service) SubmitPurchaseReturn(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in PurchaseReturnActionInput) (*PurchaseReturn, error) {
	return s.transitionPurchaseReturn(ctx, tenantID, id, actor, "submit", in)
}

func (s *Service) ApprovePurchaseReturn(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in PurchaseReturnActionInput) (*PurchaseReturn, error) {
	return s.transitionPurchaseReturn(ctx, tenantID, id, actor, "approve", in)
}

func (s *Service) CompletePurchaseReturn(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in PurchaseReturnActionInput) (*PurchaseReturn, error) {
	return s.transitionPurchaseReturn(ctx, tenantID, id, actor, "complete", in)
}

func (s *Service) CancelPurchaseReturn(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in PurchaseReturnActionInput) (*PurchaseReturn, error) {
	return s.transitionPurchaseReturn(ctx, tenantID, id, actor, "cancel", in)
}

func (s *Service) transitionPurchaseReturn(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, action string, in PurchaseReturnActionInput) (*PurchaseReturn, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	reason := strings.TrimSpace(in.Reason)
	if tenantID < 0 || id == uuid.Nil || in.ExpectedRevision < 1 || len(key) < 8 || len(key) > 128 || len([]rune(reason)) > 128 {
		return nil, ErrReturnInvalidInput
	}
	if (action == "approve" || action == "complete") && (actor == nil || *actor == uuid.Nil) {
		return nil, ErrReturnInvalidInput
	}
	hash := purchaseReturnActionHash(action, in.ExpectedRevision, reason)
	var outID uuid.UUID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row PurchaseReturn
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Items").Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPurchaseReturnAbsent
		}
		if err != nil {
			return err
		}
		var done PurchaseReturnAction
		if err := tx.Where("tenant_id = ? AND purchase_return_id = ? AND action = ?", tenantID, id, action).First(&done).Error; err == nil {
			if done.IdempotencyKey != key || done.RequestHash != hash {
				return ErrReturnIdempotencyConflict
			}
			outID = row.ID
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var used PurchaseReturnAction
		if err := tx.Where("tenant_id = ? AND purchase_return_id = ? AND idempotency_key = ?", tenantID, id, key).First(&used).Error; err == nil {
			return ErrReturnIdempotencyConflict
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Revision != in.ExpectedRevision {
			return ErrReturnRevisionConflict
		}

		now := time.Now().UTC()
		switch action {
		case "submit":
			if row.Status != ReturnStatusDraft {
				return ErrReturnInvalidTransition
			}
			row.Status = ReturnStatusPendingApproval
			row.SubmittedBy, row.SubmittedAt = actor, &now
		case "approve":
			if row.Status != ReturnStatusPendingApproval {
				return ErrReturnInvalidTransition
			}
			row.Status = ReturnStatusApproved
			row.ApprovedBy, row.ApprovedAt = actor, &now
		case "complete":
			if row.Status != ReturnStatusApproved {
				return ErrReturnInvalidTransition
			}
			if row.ApprovedBy == nil || actor == nil || *row.ApprovedBy == *actor {
				return ErrReturnDutyConflict
			}
			if err := s.completePurchaseReturnTx(ctx, tx, &row, actor, reason); err != nil {
				return err
			}
			row.Status = ReturnStatusCompleted
			row.CompletedBy, row.CompletedAt = actor, &now
		case "cancel":
			if row.Status != ReturnStatusDraft && row.Status != ReturnStatusPendingApproval && row.Status != ReturnStatusApproved {
				return ErrReturnInvalidTransition
			}
			row.Status = ReturnStatusCancelled
			row.CancelledBy, row.CancelledAt = actor, &now
		default:
			return ErrReturnInvalidInput
		}
		row.Revision++
		result := tx.Model(&PurchaseReturn{}).Where("id = ? AND tenant_id = ? AND revision = ?", row.ID, tenantID, row.Revision-1).Updates(map[string]any{
			"status": row.Status, "revision": row.Revision, "submitted_by": row.SubmittedBy, "submitted_at": row.SubmittedAt,
			"approved_by": row.ApprovedBy, "approved_at": row.ApprovedAt, "completed_by": row.CompletedBy, "completed_at": row.CompletedAt,
			"cancelled_by": row.CancelledBy, "cancelled_at": row.CancelledAt, "updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrReturnRevisionConflict
		}
		actionRow := &PurchaseReturnAction{TenantID: tenantID, PurchaseReturnID: row.ID, Action: action, IdempotencyKey: key, RequestHash: hash}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(actionRow)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected != 1 {
			return ErrReturnIdempotencyConflict
		}
		outID = row.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetPurchaseReturn(ctx, tenantID, outID)
}

func (s *Service) completePurchaseReturnTx(ctx context.Context, tx *gorm.DB, row *PurchaseReturn, actor *uuid.UUID, actionReason string) error {
	if row == nil || len(row.Items) == 0 {
		return ErrReturnInvalidInput
	}
	items := append([]PurchaseReturnItem(nil), row.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProductSKUID == items[j].ProductSKUID {
			return items[i].GoodsReceiptItemID.String() < items[j].GoodsReceiptItemID.String()
		}
		return items[i].ProductSKUID.String() < items[j].ProductSKUID.String()
	})
	reason := actionReason
	if reason == "" {
		reason = row.Reason
	}
	for _, item := range items {
		_, err := s.Stock.Return(ctx, tx, inventory.PurchaseReturnStockInput{
			TenantID: row.TenantID, WarehouseID: row.WarehouseID, ProductSKUID: item.ProductSKUID, Quantity: item.Quantity,
			PurchaseReturnID: row.ID, PurchaseReturnItemID: item.ID, GoodsReceiptItemID: item.GoodsReceiptItemID,
			BusinessEventKey: "purchase_return:" + row.ID.String() + ":" + item.ID.String(), Reason: reason, CreatedBy: actor,
		})
		if errors.Is(err, inventory.ErrInsufficientWarehouseAvailable) {
			return ErrReturnInsufficientStock
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func enrichPurchaseReturn(ctx context.Context, db *gorm.DB, tenantID int64, row *PurchaseReturn) error {
	type header struct {
		PurchaseOrderNo string
		SupplierName    string
		WarehouseName   string
	}
	var h header
	if err := db.WithContext(ctx).Table("purchase_orders po").Select("po.purchase_order_no, s.name AS supplier_name, w.name AS warehouse_name").
		Joins("JOIN suppliers s ON s.id = po.supplier_id AND s.tenant_id = po.tenant_id").
		Joins("JOIN warehouses w ON w.id = po.warehouse_id AND w.tenant_id = po.tenant_id").
		Where("po.id = ? AND po.tenant_id = ?", row.PurchaseOrderID, tenantID).Scan(&h).Error; err != nil {
		return fmt.Errorf("load purchase return labels: %w", err)
	}
	row.PurchaseOrderNo, row.SupplierName, row.WarehouseName = h.PurchaseOrderNo, h.SupplierName, h.WarehouseName
	if len(row.Items) == 0 {
		return nil
	}
	type label struct {
		GoodsReceiptItemID uuid.UUID
		ReceiptNo          string
		ReceiptQuantity    int
		ProductTitle       string
		SKUCode            string
		SKUName            string
	}
	ids := make([]uuid.UUID, 0, len(row.Items))
	for _, item := range row.Items {
		ids = append(ids, item.GoodsReceiptItemID)
	}
	var labels []label
	if err := db.WithContext(ctx).Table("goods_receipt_items gri").
		Select("gri.id AS goods_receipt_item_id, gr.receipt_no, gri.quantity AS receipt_quantity, products.title AS product_title, product_skus.sku_code, product_skus.sku_name").
		Joins("JOIN goods_receipts gr ON gr.id = gri.goods_receipt_id AND gr.tenant_id = gri.tenant_id").
		Joins("JOIN product_skus ON product_skus.id = gri.product_sku_id").
		Joins("JOIN products ON products.id = product_skus.product_id AND products.tenant_id = gri.tenant_id AND products.deleted_at IS NULL").
		Where("gri.tenant_id = ? AND gri.id IN ?", tenantID, ids).Scan(&labels).Error; err != nil {
		return fmt.Errorf("load purchase return item labels: %w", err)
	}
	byID := make(map[uuid.UUID]label, len(labels))
	for _, item := range labels {
		byID[item.GoodsReceiptItemID] = item
	}
	for i := range row.Items {
		item := byID[row.Items[i].GoodsReceiptItemID]
		row.Items[i].ReceiptNo, row.Items[i].ReceiptQuantity = item.ReceiptNo, item.ReceiptQuantity
		row.Items[i].ProductTitle, row.Items[i].SKUCode, row.Items[i].SKUName = item.ProductTitle, item.SKUCode, item.SKUName
	}
	return nil
}

func allocatedReturnQuantities(tx *gorm.DB, tenantID int64, receiptItemIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	type allocation struct {
		GoodsReceiptItemID uuid.UUID
		Quantity           int
	}
	var rows []allocation
	err := tx.Table("purchase_return_items pri").Select("pri.goods_receipt_item_id, SUM(pri.quantity) AS quantity").
		Joins("JOIN purchase_returns pr ON pr.id = pri.purchase_return_id AND pr.tenant_id = pri.tenant_id").
		Where("pri.tenant_id = ? AND pri.goods_receipt_item_id IN ? AND pr.status <> ?", tenantID, receiptItemIDs, ReturnStatusCancelled).
		Group("pri.goods_receipt_item_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load allocated return quantities: %w", err)
	}
	out := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		out[row.GoodsReceiptItemID] = row.Quantity
	}
	return out, nil
}

func loadPurchaseReturnByKey(tx *gorm.DB, tenantID int64, key string) (*PurchaseReturn, error) {
	var row PurchaseReturn
	err := tx.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load purchase return by idempotency key: %w", err)
	}
	return &row, nil
}

func purchaseReturnPayloadHash(purchaseOrderID uuid.UUID, reason, remark string, items []CreatePurchaseReturnItemInput) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%s|%s|", purchaseOrderID, reason, remark)
	for _, item := range items {
		_, _ = fmt.Fprintf(hash, "%s:%d|", item.GoodsReceiptItemID, item.Quantity)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func purchaseReturnActionHash(action string, revision int, reason string) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%d|%s", action, revision, reason)
	return hex.EncodeToString(hash.Sum(nil))
}

func uniqueUUIDs(values []uuid.UUID) map[uuid.UUID]struct{} {
	out := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func validPurchaseReturnStatus(value string) bool {
	switch value {
	case ReturnStatusDraft, ReturnStatusPendingApproval, ReturnStatusApproved, ReturnStatusCompleted, ReturnStatusCancelled:
		return true
	default:
		return false
	}
}
