package inventory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrStocktakeInvalidInput = errors.New("invalid inventory stocktake")
	ErrStocktakeAbsent       = errors.New("inventory stocktake not found")
	ErrStocktakeTransition   = errors.New("inventory stocktake transition is not allowed")
	ErrStocktakeRevision     = errors.New("inventory stocktake revision conflict")
	ErrStocktakeIdempotency  = errors.New("inventory stocktake idempotency conflict")
	ErrStocktakeSnapshot     = errors.New("inventory stocktake snapshot is stale")
)

func stocktakeNo() string { return "STK-" + strings.ToUpper(uuid.NewString()[:12]) }

func (s *Service) stocktakeWarehouseService() *warehouse.Service {
	if s != nil && s.Warehouses != nil {
		return s.Warehouses
	}
	if s == nil {
		return nil
	}
	return &warehouse.Service{DB: s.DB}
}

func (s *Service) getStocktakeTx(ctx context.Context, tx *gorm.DB, tenantID int64, id uuid.UUID, lock bool) (*InventoryStocktake, error) {
	var row InventoryStocktake
	q := tx.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Preload("Items")
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStocktakeAbsent
		}
		return nil, err
	}
	if err := s.hydrateStocktakeItems(ctx, tx, tenantID, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) hydrateStocktakeItems(ctx context.Context, tx *gorm.DB, tenantID int64, row *InventoryStocktake) error {
	if row == nil || len(row.Items) == 0 {
		return nil
	}
	type labelRow struct {
		ProductSKUID uuid.UUID
		ProductTitle string
		SKUCode      string
		SKUName      string
	}
	ids := make([]uuid.UUID, 0, len(row.Items))
	for _, item := range row.Items {
		ids = append(ids, item.ProductSKUID)
	}
	var labels []labelRow
	if err := tx.WithContext(ctx).Table("product_skus AS sk").
		Select("sk.id AS product_sku_id, p.title AS product_title, sk.sku_code, sk.sku_name").
		Joins("JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL").
		Where("p.tenant_id = ? AND sk.id IN ?", tenantID, ids).Scan(&labels).Error; err != nil {
		return fmt.Errorf("load stocktake item labels: %w", err)
	}
	bySKU := make(map[uuid.UUID]labelRow, len(labels))
	for _, label := range labels {
		bySKU[label.ProductSKUID] = label
	}
	for i := range row.Items {
		if label, ok := bySKU[row.Items[i].ProductSKUID]; ok {
			row.Items[i].ProductTitle = label.ProductTitle
			row.Items[i].SKUCode = label.SKUCode
			row.Items[i].SKUName = label.SKUName
		}
	}
	return nil
}

func (s *Service) CreateInventoryStocktake(ctx context.Context, tenantID int64, actor *uuid.UUID, in CreateInventoryStocktakeBody) (*InventoryStocktake, error) {
	if s == nil || s.DB == nil || tenantID < 0 || in.WarehouseID == uuid.Nil {
		return nil, ErrStocktakeInvalidInput
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if len(key) < 8 || len(key) > 128 || len(in.Items) == 0 || len(in.Items) > 500 {
		return nil, ErrStocktakeInvalidInput
	}
	reason, remark := clampStr(in.Reason, 128), clampStr(in.Remark, 520)
	hash, err := inventoryRequestHash(struct {
		WarehouseID uuid.UUID
		Reason      string
		Remark      string
		Items       []CreateInventoryStocktakeItem
	}{in.WarehouseID, reason, remark, in.Items})
	if err != nil {
		return nil, err
	}
	var out *InventoryStocktake
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var replay InventoryStocktake
		if err := tx.Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).Preload("Items").First(&replay).Error; err == nil {
			if replay.PayloadHash != hash {
				return ErrStocktakeIdempotency
			}
			out = &replay
			return s.hydrateStocktakeItems(ctx, tx, tenantID, out)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if _, err := s.stocktakeWarehouseService().RequireActive(ctx, tx, tenantID, in.WarehouseID); err != nil {
			return ErrStocktakeInvalidInput
		}
		inputs := append([]CreateInventoryStocktakeItem(nil), in.Items...)
		sort.Slice(inputs, func(i, j int) bool {
			return inputs[i].ProductSKUID.String() < inputs[j].ProductSKUID.String()
		})
		seen := make(map[uuid.UUID]bool, len(inputs))
		items := make([]InventoryStocktakeItem, 0, len(inputs))
		for _, input := range inputs {
			if input.ProductSKUID == uuid.Nil || seen[input.ProductSKUID] {
				return ErrStocktakeInvalidInput
			}
			seen[input.ProductSKUID] = true
			var sku product.ProductSKU
			if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").Where("product_skus.id = ? AND products.tenant_id = ?", input.ProductSKUID, tenantID).First(&sku).Error; err != nil {
				return gorm.ErrRecordNotFound
			}
			if _, err := ensureLegacyBalanceInMigrationWarehouseTx(ctx, tx, tenantID, sku, actor); err != nil {
				return err
			}
			var balance WarehouseStockBalance
			err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, in.WarehouseID, sku.ID).First(&balance).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				balance = WarehouseStockBalance{TenantID: tenantID, WarehouseID: in.WarehouseID, ProductSKUID: sku.ID, Version: 1}
				if err := tx.Create(&balance).Error; err != nil {
					return fmt.Errorf("create stocktake balance: %w", err)
				}
			} else if err != nil {
				return fmt.Errorf("load stocktake balance: %w", err)
			}
			items = append(items, InventoryStocktakeItem{
				TenantID: tenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
				SnapshotOnHand: balance.OnHand, SnapshotReserved: balance.Reserved,
				SnapshotInTransit: balance.InTransit, SnapshotDamaged: balance.Damaged, SnapshotVersion: balance.Version,
			})
		}
		row := &InventoryStocktake{TenantID: tenantID, StocktakeNo: stocktakeNo(), WarehouseID: in.WarehouseID, Status: StocktakeCounting, Revision: 1, IdempotencyKey: key, PayloadHash: hash, Reason: reason, Remark: remark, CreatedBy: actor, Items: items}
		if err := tx.Create(row).Error; err != nil {
			if isInventoryUniqueViolation(err) {
				return ErrStocktakeIdempotency
			}
			return err
		}
		out = row
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) GetInventoryStocktake(ctx context.Context, tenantID int64, id uuid.UUID) (*InventoryStocktake, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: db unavailable")
	}
	return s.getStocktakeTx(ctx, s.DB, tenantID, id, false)
}

func (s *Service) ListInventoryStocktakes(ctx context.Context, tenantID int64, page, pageSize int, status string) (*InventoryStocktakeListResult, error) {
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
	q := s.DB.WithContext(ctx).Model(&InventoryStocktake{}).Where("inventory_stocktakes.tenant_id = ?", tenantID)
	if strings.TrimSpace(status) != "" {
		q = q.Where("inventory_stocktakes.status = ?", strings.TrimSpace(status))
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []InventoryStocktakeListRow
	if err := q.Select("inventory_stocktakes.*, w.code AS warehouse_code, w.name AS warehouse_name, (SELECT COUNT(*) FROM inventory_stocktake_items i WHERE i.stocktake_id = inventory_stocktakes.id) AS item_count").
		Joins("JOIN warehouses w ON w.id = inventory_stocktakes.warehouse_id AND w.tenant_id = inventory_stocktakes.tenant_id").
		Order("inventory_stocktakes.created_at DESC, inventory_stocktakes.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	return &InventoryStocktakeListResult{List: rows, Total: total, Page: page, PageSize: pageSize, TotalPages: pagesOf(total, pageSize)}, nil
}

func (s *Service) updateStocktakeAction(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, action string, in InventoryStocktakeActionBody) (*InventoryStocktake, error) {
	if s == nil || s.DB == nil || tenantID < 0 || id == uuid.Nil || in.ExpectedRevision < 1 || len(strings.TrimSpace(in.IdempotencyKey)) < 8 || len(strings.TrimSpace(in.IdempotencyKey)) > 128 {
		return nil, ErrStocktakeInvalidInput
	}
	key, reason := strings.TrimSpace(in.IdempotencyKey), clampStr(in.Reason, 128)
	hash, err := inventoryRequestHash(struct {
		Action   string
		Revision int
		Reason   string
	}{action, in.ExpectedRevision, reason})
	if err != nil {
		return nil, err
	}
	var out *InventoryStocktake
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := s.getStocktakeTx(ctx, tx, tenantID, id, true)
		if err != nil {
			return err
		}
		var done InventoryStocktakeAction
		if err := tx.Where("tenant_id = ? AND stocktake_id = ? AND action = ?", tenantID, id, action).First(&done).Error; err == nil {
			if done.RequestHash != hash || done.IdempotencyKey != key {
				return ErrStocktakeIdempotency
			}
			out = row
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Revision != in.ExpectedRevision {
			return ErrStocktakeRevision
		}
		now := time.Now().UTC()
		switch action {
		case "submit":
			if row.Status != StocktakeCounting {
				return ErrStocktakeTransition
			}
			for _, item := range row.Items {
				if item.CountedOnHand == nil {
					return fmt.Errorf("%w: all items must be counted", ErrStocktakeInvalidInput)
				}
			}
			row.Status, row.SubmittedAt, row.SubmittedBy = StocktakePendingReview, &now, actor
		case "approve":
			if row.Status != StocktakePendingReview {
				return ErrStocktakeTransition
			}
			row.Status, row.ApprovedAt, row.ApprovedBy = StocktakeApproved, &now, actor
		case "post":
			if row.Status != StocktakeApproved {
				return ErrStocktakeTransition
			}
			if err := s.postStocktakeTx(ctx, tx, tenantID, row, actor); err != nil {
				return err
			}
			row.Status, row.PostedAt, row.PostedBy = StocktakePosted, &now, actor
		case "cancel":
			if row.Status != StocktakeCounting && row.Status != StocktakePendingReview && row.Status != StocktakeApproved {
				return ErrStocktakeTransition
			}
			row.Status, row.CancelledAt = StocktakeCancelled, &now
		default:
			return ErrStocktakeInvalidInput
		}
		row.Revision++
		if result := tx.Model(&InventoryStocktake{}).Where("id = ? AND tenant_id = ? AND revision = ?", id, tenantID, row.Revision-1).Updates(map[string]any{"status": row.Status, "revision": row.Revision, "submitted_at": row.SubmittedAt, "submitted_by": row.SubmittedBy, "approved_at": row.ApprovedAt, "approved_by": row.ApprovedBy, "posted_at": row.PostedAt, "posted_by": row.PostedBy, "cancelled_at": row.CancelledAt, "updated_at": now}); result.Error != nil || result.RowsAffected != 1 {
			return ErrStocktakeRevision
		}
		if err := tx.Create(&InventoryStocktakeAction{TenantID: tenantID, StocktakeID: id, Action: action, IdempotencyKey: key, RequestHash: hash}).Error; err != nil {
			if isInventoryUniqueViolation(err) {
				return ErrStocktakeIdempotency
			}
			return err
		}
		out = row
		return nil
	})
	return out, err
}

func (s *Service) UpdateInventoryStocktakeItem(ctx context.Context, tenantID int64, stocktakeID, itemID uuid.UUID, _ *uuid.UUID, in InventoryStocktakeItemBody) (*InventoryStocktake, error) {
	if s == nil || s.DB == nil || tenantID < 0 || stocktakeID == uuid.Nil || itemID == uuid.Nil || in.ExpectedRevision < 1 || in.CountedOnHand == nil || *in.CountedOnHand < 0 || len(strings.TrimSpace(in.IdempotencyKey)) < 8 || len(strings.TrimSpace(in.IdempotencyKey)) > 128 {
		return nil, ErrStocktakeInvalidInput
	}
	key, remark, counted := strings.TrimSpace(in.IdempotencyKey), clampStr(in.Remark, 520), *in.CountedOnHand
	action := "item_update"
	hash, err := inventoryRequestHash(struct {
		ItemID            uuid.UUID
		Revision, Counted int
		Remark            string
	}{itemID, in.ExpectedRevision, counted, remark})
	if err != nil {
		return nil, err
	}
	var out *InventoryStocktake
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := s.getStocktakeTx(ctx, tx, tenantID, stocktakeID, true)
		if err != nil {
			return err
		}
		var done InventoryStocktakeAction
		if err := tx.Where("tenant_id = ? AND stocktake_id = ? AND action = ? AND idempotency_key = ?", tenantID, stocktakeID, action, key).First(&done).Error; err == nil {
			if done.RequestHash != hash || done.IdempotencyKey != key {
				return ErrStocktakeIdempotency
			}
			out = row
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.Status != StocktakeCounting || row.Revision != in.ExpectedRevision {
			return ErrStocktakeRevision
		}
		var item InventoryStocktakeItem
		if err := tx.Where("tenant_id = ? AND stocktake_id = ? AND id = ?", tenantID, stocktakeID, itemID).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStocktakeAbsent
			}
			return err
		}
		if err := tx.Model(&InventoryStocktakeItem{}).Where("id = ? AND tenant_id = ? AND stocktake_id = ?", itemID, tenantID, stocktakeID).Updates(map[string]any{"counted_on_hand": counted, "remark": remark, "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		row.Revision++
		if result := tx.Model(&InventoryStocktake{}).Where("id = ? AND tenant_id = ? AND revision = ?", stocktakeID, tenantID, row.Revision-1).Updates(map[string]any{"revision": row.Revision, "updated_at": time.Now().UTC()}); result.Error != nil || result.RowsAffected != 1 {
			return ErrStocktakeRevision
		}
		if err := tx.Create(&InventoryStocktakeAction{TenantID: tenantID, StocktakeID: stocktakeID, Action: action, IdempotencyKey: key, RequestHash: hash}).Error; err != nil {
			if isInventoryUniqueViolation(err) {
				return ErrStocktakeIdempotency
			}
			return err
		}
		out, err = s.getStocktakeTx(ctx, tx, tenantID, stocktakeID, false)
		return err
	})
	return out, err
}

func (s *Service) postStocktakeTx(ctx context.Context, tx *gorm.DB, tenantID int64, row *InventoryStocktake, actor *uuid.UUID) error {
	if row == nil {
		return ErrStocktakeInvalidInput
	}
	items := append([]InventoryStocktakeItem(nil), row.Items...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].ProductSKUID.String() < items[j].ProductSKUID.String()
	})
	for _, item := range items {
		if item.CountedOnHand == nil {
			return ErrStocktakeInvalidInput
		}
		var sku product.ProductSKU
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").Where("product_skus.id = ? AND products.tenant_id = ?", item.ProductSKUID, tenantID).First(&sku).Error; err != nil {
			return err
		}
		var balance WarehouseStockBalance
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, row.WarehouseID, item.ProductSKUID).First(&balance).Error; err != nil {
			return err
		}
		if balance.Version != item.SnapshotVersion || balance.OnHand != item.SnapshotOnHand || balance.Reserved != item.SnapshotReserved || balance.InTransit != item.SnapshotInTransit || balance.Damaged != item.SnapshotDamaged {
			return ErrStocktakeSnapshot
		}
		if *item.CountedOnHand < balance.Reserved+balance.Damaged {
			return ErrStocktakeInvalidInput
		}
		beforeAggregate := derefStock(sku.Stock)
		delta := *item.CountedOnHand - balance.OnHand
		beforeOnHand := balance.OnHand
		balance.OnHand = *item.CountedOnHand
		balance.Version++
		if result := tx.Model(&WarehouseStockBalance{}).Where("id = ? AND version = ?", balance.ID, balance.Version-1).Updates(map[string]any{"on_hand": balance.OnHand, "version": balance.Version, "updated_at": time.Now().UTC()}); result.Error != nil || result.RowsAffected != 1 {
			return ErrStocktakeRevision
		}
		eventKey := fmt.Sprintf("stocktake:%s:post:%s", row.ID, item.ProductSKUID)
		movement := InventoryMovement{TenantID: tenantID, WarehouseID: row.WarehouseID, ProductID: sku.ProductID, ProductSKUID: item.ProductSKUID, MovementType: MovementStocktakeAdjust, Quantity: delta, BeforeOnHand: beforeOnHand, AfterOnHand: balance.OnHand, BeforeReserved: balance.Reserved, AfterReserved: balance.Reserved, SourceType: "inventory_stocktake", SourceID: row.ID, BusinessEventKey: eventKey, Reason: row.Reason, Remark: item.Remark, CreatedBy: actor}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
		aggregate := beforeAggregate + delta
		if err := tx.Model(&product.ProductSKU{}).Where("id = ? AND product_id = ?", sku.ID, sku.ProductID).Updates(map[string]any{"stock": aggregate, "stock_status": stockStatusForSKU(sku, aggregate), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err := tx.Create(&InventoryChangeLog{TenantID: tenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID, ChangeType: ChangeStocktakeAdjust, BeforeStock: beforeAggregate, AfterStock: aggregate, Delta: delta, Reason: row.Reason, Remark: item.Remark, CreatedBy: actor, BusinessEventKey: eventKey}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SubmitInventoryStocktake(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in InventoryStocktakeActionBody) (*InventoryStocktake, error) {
	return s.updateStocktakeAction(ctx, tenantID, id, actor, "submit", in)
}
func (s *Service) ApproveInventoryStocktake(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in InventoryStocktakeActionBody) (*InventoryStocktake, error) {
	return s.updateStocktakeAction(ctx, tenantID, id, actor, "approve", in)
}
func (s *Service) PostInventoryStocktake(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in InventoryStocktakeActionBody) (*InventoryStocktake, error) {
	return s.updateStocktakeAction(ctx, tenantID, id, actor, "post", in)
}
func (s *Service) CancelInventoryStocktake(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, in InventoryStocktakeActionBody) (*InventoryStocktake, error) {
	return s.updateStocktakeAction(ctx, tenantID, id, actor, "cancel", in)
}
