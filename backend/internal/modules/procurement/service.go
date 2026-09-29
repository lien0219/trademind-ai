package procurement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidInput        = errors.New("invalid procurement input")
	ErrPurchaseOrderAbsent = errors.New("purchase order not found")
	ErrInvalidTransition   = errors.New("invalid purchase order transition")
	ErrRevisionConflict    = errors.New("purchase order revision conflict")
	ErrOverReceipt         = errors.New("receipt quantity exceeds remaining quantity")
	ErrIdempotencyConflict = errors.New("idempotency key was already used with another payload")
)

const (
	maxOrderItems = 100
	maxQuantity   = 1_000_000
	maxUnitCost   = int64(1_000_000_000_000)
)

type WarehouseValidator interface {
	ValidateActive(context.Context, *gorm.DB, int64, uuid.UUID) error
}

type SupplierValidator interface {
	ValidateActive(context.Context, *gorm.DB, int64, uuid.UUID) error
	ValidateBinding(context.Context, *gorm.DB, int64, uuid.UUID, uuid.UUID, *uuid.UUID) error
}

type WarehouseStockWriter interface {
	Receive(context.Context, *gorm.DB, inventory.ReceiptStockInput) (*inventory.WarehouseStockBalance, error)
	Return(context.Context, *gorm.DB, inventory.PurchaseReturnStockInput) (*inventory.WarehouseStockBalance, error)
}

type Service struct {
	DB         *gorm.DB
	Warehouses WarehouseValidator
	Suppliers  SupplierValidator
	Stock      WarehouseStockWriter
}

func (s *Service) ready() error {
	if s == nil || s.DB == nil || s.Warehouses == nil || s.Suppliers == nil || s.Stock == nil {
		return fmt.Errorf("procurement: dependencies unavailable")
	}
	return nil
}

func (s *Service) Create(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreatePurchaseOrderInput) (*PurchaseOrder, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	key := strings.TrimSpace(in.IdempotencyKey)
	remark := strings.TrimSpace(in.Remark)
	if tenantID < 0 || in.SupplierID == uuid.Nil || in.WarehouseID == uuid.Nil || len(key) < 8 || len(key) > 128 || len(currency) != 3 || len(in.Items) == 0 || len(in.Items) > maxOrderItems || len([]rune(remark)) > 1000 {
		return nil, ErrInvalidInput
	}
	seen := make(map[uuid.UUID]struct{}, len(in.Items))
	skuIDs := make([]uuid.UUID, 0, len(in.Items))
	orderItems := make([]PurchaseOrderItem, 0, len(in.Items))
	totalAmount := int64(0)
	for _, item := range in.Items {
		if item.ProductSKUID == uuid.Nil || item.Quantity < 1 || item.Quantity > maxQuantity || item.UnitCostMinor < 0 || item.UnitCostMinor > maxUnitCost {
			return nil, ErrInvalidInput
		}
		if _, ok := seen[item.ProductSKUID]; ok {
			return nil, ErrInvalidInput
		}
		seen[item.ProductSKUID] = struct{}{}
		skuIDs = append(skuIDs, item.ProductSKUID)
		if item.Quantity > 0 && item.UnitCostMinor > math.MaxInt64/int64(item.Quantity) {
			return nil, ErrInvalidInput
		}
		lineAmount := int64(item.Quantity) * item.UnitCostMinor
		if lineAmount > math.MaxInt64-totalAmount {
			return nil, ErrInvalidInput
		}
		totalAmount += lineAmount
		orderItems = append(orderItems, PurchaseOrderItem{
			TenantID: tenantID, ProductSKUID: item.ProductSKUID, SupplierSKUID: item.SupplierSKUID,
			Quantity: item.Quantity, UnitCostMinor: item.UnitCostMinor, LineAmountMinor: lineAmount,
		})
	}
	payloadHash := createPayloadHash(in.SupplierID, in.WarehouseID, currency, remark, in.Items)
	if existing, err := loadPurchaseOrderByKey(s.DB.WithContext(ctx), tenantID, key); err != nil {
		return nil, err
	} else if existing != nil {
		if existing.PayloadHash != payloadHash {
			return nil, ErrIdempotencyConflict
		}
		return existing, nil
	}

	po := &PurchaseOrder{
		TenantID:         tenantID,
		PurchaseOrderNo:  newDocumentNumber("PO"),
		IdempotencyKey:   key,
		PayloadHash:      payloadHash,
		SupplierID:       in.SupplierID,
		WarehouseID:      in.WarehouseID,
		Status:           StatusDraft,
		Currency:         currency,
		TotalAmountMinor: totalAmount,
		Revision:         1,
		Remark:           remark,
		CreatedBy:        actor,
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.Suppliers.ValidateActive(ctx, tx, tenantID, in.SupplierID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidInput
			}
			return err
		}
		if err := s.Warehouses.ValidateActive(ctx, tx, tenantID, in.WarehouseID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidInput
			}
			return err
		}
		var skus []product.ProductSKU
		if err := tx.Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("product_skus.id IN ? AND products.tenant_id = ?", skuIDs, tenantID).Find(&skus).Error; err != nil {
			return fmt.Errorf("load purchase SKUs: %w", err)
		}
		if len(skus) != len(skuIDs) {
			return ErrInvalidInput
		}
		for _, item := range in.Items {
			if err := s.Suppliers.ValidateBinding(ctx, tx, tenantID, in.SupplierID, item.ProductSKUID, item.SupplierSKUID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrInvalidInput
				}
				return err
			}
		}
		result := tx.Omit("Items").Clauses(clause.OnConflict{DoNothing: true}).Create(po)
		if result.Error != nil {
			return fmt.Errorf("create purchase order: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			existing, err := loadPurchaseOrderByKey(tx, tenantID, key)
			if err != nil {
				return err
			}
			if existing == nil || existing.PayloadHash != payloadHash {
				return ErrIdempotencyConflict
			}
			po = existing
			return nil
		}
		for i := range orderItems {
			orderItems[i].PurchaseOrderID = po.ID
		}
		if err := tx.Create(&orderItems).Error; err != nil {
			return fmt.Errorf("create purchase order items: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, po.ID)
}

func (s *Service) Get(ctx context.Context, tenantID int64, id uuid.UUID) (*PurchaseOrder, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 0 || id == uuid.Nil {
		return nil, ErrPurchaseOrderAbsent
	}
	var row PurchaseOrder
	err := s.DB.WithContext(ctx).Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC, id ASC") }).
		Where("id = ? AND tenant_id = ?", id, tenantID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPurchaseOrderAbsent
	}
	if err != nil {
		return nil, fmt.Errorf("get purchase order: %w", err)
	}
	if err := enrichPurchaseOrderItems(ctx, s.DB, tenantID, row.Items); err != nil {
		return nil, err
	}
	return &row, nil
}

func enrichPurchaseOrderItems(ctx context.Context, db *gorm.DB, tenantID int64, items []PurchaseOrderItem) error {
	if len(items) == 0 {
		return nil
	}
	type skuLabel struct {
		ID           uuid.UUID
		ProductTitle string
		SKUCode      string
		SKUName      string
	}
	ids := make([]uuid.UUID, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ProductSKUID)
	}
	var labels []skuLabel
	err := db.WithContext(ctx).Table("product_skus").
		Select("product_skus.id, products.title AS product_title, product_skus.sku_code, product_skus.sku_name").
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("product_skus.id IN ? AND products.tenant_id = ?", ids, tenantID).
		Scan(&labels).Error
	if err != nil {
		return fmt.Errorf("load purchase SKU labels: %w", err)
	}
	byID := make(map[uuid.UUID]skuLabel, len(labels))
	for _, label := range labels {
		byID[label.ID] = label
	}
	for i := range items {
		label := byID[items[i].ProductSKUID]
		items[i].ProductTitle = label.ProductTitle
		items[i].SKUCode = label.SKUCode
		items[i].SKUName = label.SKUName
	}
	return nil
}

func (s *Service) List(ctx context.Context, tenantID int64, page, pageSize int) (*ListResult, error) {
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
	query := s.DB.WithContext(ctx).Model(&PurchaseOrder{}).Where("tenant_id = ?", tenantID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count purchase orders: %w", err)
	}
	var rows []PurchaseOrder
	if err := query.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list purchase orders: %w", err)
	}
	return &ListResult{List: rows, Page: page, PageSize: pageSize, Total: total, TotalPages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}

func (s *Service) Submit(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int) (*PurchaseOrder, error) {
	return s.transition(ctx, tenantID, id, expectedRevision, nil, "submit")
}

func (s *Service) Approve(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int, actor *uuid.UUID) (*PurchaseOrder, error) {
	return s.transition(ctx, tenantID, id, expectedRevision, actor, "approve")
}

func (s *Service) Cancel(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int) (*PurchaseOrder, error) {
	return s.transition(ctx, tenantID, id, expectedRevision, nil, "cancel")
}

func (s *Service) Close(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int) (*PurchaseOrder, error) {
	return s.transition(ctx, tenantID, id, expectedRevision, nil, "close")
}

func (s *Service) transition(ctx context.Context, tenantID int64, id uuid.UUID, expectedRevision int, actor *uuid.UUID, action string) (*PurchaseOrder, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if tenantID < 0 || id == uuid.Nil || expectedRevision < 1 {
		return nil, ErrInvalidInput
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var po PurchaseOrder
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", id, tenantID).First(&po).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPurchaseOrderAbsent
		}
		if err != nil {
			return err
		}
		if po.Revision != expectedRevision {
			return ErrRevisionConflict
		}
		now := time.Now().UTC()
		updates := map[string]any{"revision": po.Revision + 1}
		switch action {
		case "submit":
			if po.Status != StatusDraft {
				return ErrInvalidTransition
			}
			updates["status"] = StatusPendingApproval
		case "approve":
			if po.Status != StatusPendingApproval {
				return ErrInvalidTransition
			}
			updates["status"] = StatusApproved
			updates["approved_by"] = actor
			updates["approved_at"] = now
		case "cancel":
			if po.Status != StatusDraft && po.Status != StatusPendingApproval && po.Status != StatusApproved {
				return ErrInvalidTransition
			}
			var received int64
			if err := tx.Model(&PurchaseOrderItem{}).Where("tenant_id = ? AND purchase_order_id = ? AND received_quantity > 0", tenantID, id).Count(&received).Error; err != nil {
				return err
			}
			if received > 0 {
				return ErrInvalidTransition
			}
			updates["status"] = StatusCancelled
			updates["cancelled_at"] = now
		case "close":
			if po.Status != StatusApproved && po.Status != StatusPartiallyReceived {
				return ErrInvalidTransition
			}
			updates["status"] = StatusClosed
			updates["closed_at"] = now
		default:
			return ErrInvalidTransition
		}
		result := tx.Model(&PurchaseOrder{}).Where("id = ? AND tenant_id = ? AND revision = ?", id, tenantID, expectedRevision).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRevisionConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, tenantID, id)
}

func (s *Service) Receive(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in ReceivePurchaseOrderInput) (*ReceiptResult, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if tenantID < 0 || id == uuid.Nil || in.ExpectedRevision < 1 || len(key) < 8 || len(key) > 128 || len(in.Items) == 0 || len(in.Items) > maxOrderItems {
		return nil, ErrInvalidInput
	}
	sorted := append([]ReceivePurchaseOrderItemInput(nil), in.Items...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PurchaseOrderItemID.String() < sorted[j].PurchaseOrderItemID.String()
	})
	for i, item := range sorted {
		if item.PurchaseOrderItemID == uuid.Nil || item.Quantity < 1 || item.Quantity > maxQuantity {
			return nil, ErrInvalidInput
		}
		if i > 0 && item.PurchaseOrderItemID == sorted[i-1].PurchaseOrderItemID {
			return nil, ErrInvalidInput
		}
	}
	payloadHash := receiptPayloadHash(id, sorted)
	receipt := &GoodsReceipt{}
	replayed := false

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		found, err := loadReceiptByKey(tx, tenantID, id, key)
		if err != nil {
			return err
		}
		if found != nil {
			if found.PayloadHash != payloadHash {
				return ErrIdempotencyConflict
			}
			receipt = found
			replayed = true
			return nil
		}

		var po PurchaseOrder
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", id, tenantID).First(&po).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPurchaseOrderAbsent
		}
		if err != nil {
			return err
		}
		if po.Revision != in.ExpectedRevision {
			return ErrRevisionConflict
		}
		if po.Status != StatusApproved && po.Status != StatusPartiallyReceived {
			return ErrInvalidTransition
		}
		if err := s.Warehouses.ValidateActive(ctx, tx, tenantID, po.WarehouseID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvalidInput
			}
			return err
		}

		receipt = &GoodsReceipt{
			TenantID: tenantID, ReceiptNo: newDocumentNumber("GR"), PurchaseOrderID: po.ID, WarehouseID: po.WarehouseID,
			IdempotencyKey: key, PayloadHash: payloadHash, CreatedBy: actor, ReceivedAt: time.Now().UTC(),
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(receipt)
		if result.Error != nil {
			return fmt.Errorf("create goods receipt: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			found, err = loadReceiptByKey(tx, tenantID, id, key)
			if err != nil {
				return err
			}
			if found == nil || found.PayloadHash != payloadHash {
				return ErrIdempotencyConflict
			}
			receipt = found
			replayed = true
			return nil
		}

		itemIDs := make([]uuid.UUID, 0, len(sorted))
		for _, item := range sorted {
			itemIDs = append(itemIDs, item.PurchaseOrderItemID)
		}
		var poItems []PurchaseOrderItem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND purchase_order_id = ? AND id IN ?", tenantID, po.ID, itemIDs).Find(&poItems).Error; err != nil {
			return fmt.Errorf("load purchase order items: %w", err)
		}
		if len(poItems) != len(itemIDs) {
			return ErrInvalidInput
		}
		byID := make(map[uuid.UUID]*PurchaseOrderItem, len(poItems))
		for i := range poItems {
			byID[poItems[i].ID] = &poItems[i]
		}
		for _, item := range sorted {
			poItem := byID[item.PurchaseOrderItemID]
			if poItem == nil || item.Quantity > poItem.Quantity-poItem.ReceivedQuantity {
				return ErrOverReceipt
			}
			if _, err := s.Stock.Receive(ctx, tx, inventory.ReceiptStockInput{
				TenantID: tenantID, WarehouseID: po.WarehouseID, ProductSKUID: poItem.ProductSKUID, Quantity: item.Quantity,
				ReceiptID: receipt.ID, PurchaseOrderID: po.ID, PurchaseOrderItemID: poItem.ID,
				BusinessEventKey: "purchase_receipt:" + receipt.ID.String() + ":" + poItem.ID.String(), CreatedBy: actor,
			}); err != nil {
				return err
			}
			newReceived := poItem.ReceivedQuantity + item.Quantity
			update := tx.Model(&PurchaseOrderItem{}).Where("id = ? AND tenant_id = ? AND received_quantity = ?", poItem.ID, tenantID, poItem.ReceivedQuantity).
				Update("received_quantity", newReceived)
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return ErrRevisionConflict
			}
			receipt.Items = append(receipt.Items, GoodsReceiptItem{
				TenantID: tenantID, GoodsReceiptID: receipt.ID, PurchaseOrderItemID: poItem.ID, ProductSKUID: poItem.ProductSKUID, Quantity: item.Quantity,
			})
		}
		if err := tx.Create(&receipt.Items).Error; err != nil {
			return fmt.Errorf("create receipt items: %w", err)
		}
		var incomplete int64
		if err := tx.Model(&PurchaseOrderItem{}).Where("tenant_id = ? AND purchase_order_id = ? AND received_quantity < quantity", tenantID, po.ID).Count(&incomplete).Error; err != nil {
			return err
		}
		status := StatusPartiallyReceived
		if incomplete == 0 {
			status = StatusReceived
		}
		updatePO := tx.Model(&PurchaseOrder{}).Where("id = ? AND tenant_id = ? AND revision = ?", po.ID, tenantID, po.Revision).
			Updates(map[string]any{"status": status, "revision": po.Revision + 1})
		if updatePO.Error != nil {
			return updatePO.Error
		}
		if updatePO.RowsAffected != 1 {
			return ErrRevisionConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	po, err := s.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if !replayed {
		if err := s.DB.WithContext(ctx).Preload("Items").First(receipt, "id = ? AND tenant_id = ?", receipt.ID, tenantID).Error; err != nil {
			return nil, fmt.Errorf("load goods receipt: %w", err)
		}
	}
	return &ReceiptResult{PurchaseOrder: po, Receipt: receipt}, nil
}

func loadReceiptByKey(tx *gorm.DB, tenantID int64, purchaseOrderID uuid.UUID, key string) (*GoodsReceipt, error) {
	var row GoodsReceipt
	err := tx.Preload("Items").Where("tenant_id = ? AND purchase_order_id = ? AND idempotency_key = ?", tenantID, purchaseOrderID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load goods receipt by idempotency key: %w", err)
	}
	return &row, nil
}

func loadPurchaseOrderByKey(tx *gorm.DB, tenantID int64, key string) (*PurchaseOrder, error) {
	var row PurchaseOrder
	err := tx.Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC, id ASC") }).
		Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load purchase order by idempotency key: %w", err)
	}
	return &row, nil
}

func createPayloadHash(supplierID, warehouseID uuid.UUID, currency, remark string, items []CreatePurchaseOrderItemInput) string {
	sorted := append([]CreatePurchaseOrderItemInput(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ProductSKUID.String() < sorted[j].ProductSKUID.String() })
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%s|%s|%s|", supplierID.String(), warehouseID.String(), currency, remark)
	for _, item := range sorted {
		supplierSKUID := ""
		if item.SupplierSKUID != nil {
			supplierSKUID = item.SupplierSKUID.String()
		}
		_, _ = fmt.Fprintf(hash, "%s:%s:%d:%d|", item.ProductSKUID.String(), supplierSKUID, item.Quantity, item.UnitCostMinor)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func receiptPayloadHash(purchaseOrderID uuid.UUID, items []ReceivePurchaseOrderItemInput) string {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|", purchaseOrderID.String())
	for _, item := range items {
		_, _ = fmt.Fprintf(hash, "%s:%d|", item.PurchaseOrderItemID.String(), item.Quantity)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func newDocumentNumber(prefix string) string {
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().UTC().Format("20060102"), strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", "")))
}
