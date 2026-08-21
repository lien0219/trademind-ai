package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/warehouse"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	pendingAllocationWarehouseCode = "PENDING_ALLOCATION"
	reconciliationMatched          = "matched"
	reconciliationUnmigrated       = "unmigrated"
	reconciliationMismatch         = "mismatch"
)

var (
	ErrInvalidWarehouseAdjustment = errors.New("invalid warehouse adjustment")
	ErrInventoryIdempotency       = errors.New("inventory idempotency conflict")
	ErrWarehouseLedgerMismatch    = errors.New("warehouse ledger mismatch")
)

func inventoryRequestHash(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal inventory request: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func manualAdjustmentEventKey(tenantID int64, key string) string {
	return fmt.Sprintf("manual-adjust:%d:%s", tenantID, strings.TrimSpace(key))
}

func legacyImportEventKey(tenantID int64, skuID uuid.UUID) string {
	return fmt.Sprintf("legacy-stock:%d:%s", tenantID, skuID.String())
}

func isInventoryUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate key") ||
		strings.Contains(message, "unique constraint") ||
		strings.Contains(message, "unique violation") ||
		strings.Contains(message, "constraint failed") ||
		strings.Contains(message, "sqlstate 23505")
}

func stockStatusForSKU(sku product.ProductSKU, stock int) string {
	return product.CalculateSKUStockStatus(stock, sku.WarningStock, sku.SafetyStock)
}

// ensureLegacyBalanceTx creates the opening warehouse fact exactly once for a SKU.
// The caller must hold a row lock on product_skus.
func ensureLegacyBalanceTx(ctx context.Context, tx *gorm.DB, tenantID int64, sku product.ProductSKU, warehouseID uuid.UUID, actor *uuid.UUID) (bool, error) {
	var count int64
	if err := tx.WithContext(ctx).Model(&WarehouseStockBalance{}).
		Where("tenant_id = ? AND product_sku_id = ?", tenantID, sku.ID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count warehouse balances: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	legacyStock := derefStock(sku.Stock)
	if legacyStock < 0 {
		return false, fmt.Errorf("%w: negative legacy stock for sku %s", ErrWarehouseLedgerMismatch, sku.ID)
	}
	balance := WarehouseStockBalance{
		TenantID: tenantID, WarehouseID: warehouseID, ProductSKUID: sku.ID,
		OnHand: legacyStock, Version: 1,
	}
	if err := tx.WithContext(ctx).Create(&balance).Error; err != nil {
		return false, fmt.Errorf("create legacy warehouse balance: %w", err)
	}
	eventKey := legacyImportEventKey(tenantID, sku.ID)
	movement := InventoryMovement{
		TenantID: tenantID, WarehouseID: warehouseID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		MovementType: MovementLegacyImport, Quantity: legacyStock, BeforeOnHand: 0, AfterOnHand: legacyStock,
		SourceType: "legacy_stock", SourceID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(eventKey)),
		BusinessEventKey: eventKey, Reason: "legacy stock migration", CreatedBy: actor,
	}
	if err := tx.WithContext(ctx).Create(&movement).Error; err != nil {
		return false, fmt.Errorf("create legacy inventory movement: %w", err)
	}
	logRow := InventoryChangeLog{
		TenantID: tenantID, ProductID: sku.ProductID, ProductSKUID: sku.ID,
		ChangeType: ChangeImport, BeforeStock: legacyStock, AfterStock: legacyStock, Delta: 0,
		Reason: "legacy stock migration", CreatedBy: actor, BusinessEventKey: eventKey,
	}
	if err := tx.WithContext(ctx).Create(&logRow).Error; err != nil {
		return false, fmt.Errorf("create legacy compatibility log: %w", err)
	}
	return true, nil
}

func ensureLegacyBalanceInMigrationWarehouseTx(ctx context.Context, tx *gorm.DB, tenantID int64, sku product.ProductSKU, actor *uuid.UUID) (bool, error) {
	var count int64
	if err := tx.WithContext(ctx).Model(&WarehouseStockBalance{}).
		Where("tenant_id = ? AND product_sku_id = ?", tenantID, sku.ID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count warehouse balances: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	target, err := resolveLegacyMigrationWarehouse(ctx, tx, tenantID, actor)
	if err != nil {
		return false, err
	}
	return ensureLegacyBalanceTx(ctx, tx, tenantID, sku, target.ID, actor)
}

func (s *Service) warehouseService() *warehouse.Service {
	if s != nil && s.Warehouses != nil {
		return s.Warehouses
	}
	if s == nil {
		return nil
	}
	return &warehouse.Service{DB: s.DB}
}

// AdjustWarehouseStock posts one manual warehouse adjustment and refreshes the
// scalar SKU projection in the same transaction.
func (s *Service) AdjustWarehouseStock(ctx context.Context, tenantID int64, productID, skuID uuid.UUID, body AdjustStockBody, actor *uuid.UUID) (*ManualAdjustmentResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: db unavailable")
	}
	key := strings.TrimSpace(body.IdempotencyKey)
	reason := clampStr(body.Reason, 128)
	remark := clampStr(body.Remark, 520)
	if tenantID < 0 || productID == uuid.Nil || skuID == uuid.Nil || body.WarehouseID == uuid.Nil || body.Stock < 0 || len(key) < 8 || len(key) > 128 || body.Sync {
		return nil, ErrInvalidWarehouseAdjustment
	}
	if reason == "" {
		reason = ChangeManualAdjust
	}
	eventKey := manualAdjustmentEventKey(tenantID, key)
	requestHash, err := inventoryRequestHash(struct {
		ProductID    uuid.UUID `json:"productId"`
		ProductSKUID uuid.UUID `json:"productSkuId"`
		WarehouseID  uuid.UUID `json:"warehouseId"`
		Stock        int       `json:"stock"`
		Reason       string    `json:"reason"`
		Remark       string    `json:"remark"`
	}{productID, skuID, body.WarehouseID, body.Stock, reason, remark})
	if err != nil {
		return nil, err
	}

	result := &ManualAdjustmentResult{}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sku product.ProductSKU
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("product_skus.id = ? AND product_skus.product_id = ? AND products.tenant_id = ?", skuID, productID, tenantID).
			First(&sku).Error; err != nil {
			return fmt.Errorf("load tenant SKU: %w", err)
		}
		var replay InventoryMovement
		if err := tx.Where("business_event_key = ?", eventKey).First(&replay).Error; err == nil {
			if replay.RequestHash != requestHash || replay.ProductSKUID != skuID || replay.WarehouseID != body.WarehouseID {
				return ErrInventoryIdempotency
			}
			var replayLog InventoryChangeLog
			if err := tx.Where("business_event_key = ?", eventKey).First(&replayLog).Error; err != nil {
				return fmt.Errorf("load adjustment replay result: %w", err)
			}
			result = &ManualAdjustmentResult{
				ProductSKUID: skuID, WarehouseID: body.WarehouseID, WarehouseOnHand: replay.AfterOnHand,
				AggregateStock: replayLog.AfterStock, MovementID: replay.ID, IdempotentReplay: true,
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("check adjustment idempotency: %w", err)
		}
		if _, err := s.warehouseService().RequireActive(ctx, tx, tenantID, body.WarehouseID); err != nil {
			return ErrInvalidWarehouseAdjustment
		}

		if _, err := ensureLegacyBalanceInMigrationWarehouseTx(ctx, tx, tenantID, sku, actor); err != nil {
			return err
		}
		var balance WarehouseStockBalance
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND warehouse_id = ? AND product_sku_id = ?", tenantID, body.WarehouseID, skuID).
			First(&balance).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			balance = WarehouseStockBalance{TenantID: tenantID, WarehouseID: body.WarehouseID, ProductSKUID: skuID, Version: 1}
			if err := tx.Create(&balance).Error; err != nil {
				return fmt.Errorf("create adjustment balance: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("load adjustment balance: %w", err)
		}

		beforeAggregate := derefStock(sku.Stock)
		beforeOnHand := balance.OnHand
		if body.Stock < balance.Reserved+balance.Damaged {
			return fmt.Errorf("%w: stock cannot be below reserved and damaged quantity", ErrInvalidWarehouseAdjustment)
		}
		balance.OnHand = body.Stock
		balance.Version++
		update := tx.Model(&WarehouseStockBalance{}).
			Where("id = ? AND version = ?", balance.ID, balance.Version-1).
			Updates(map[string]any{"on_hand": balance.OnHand, "version": balance.Version, "updated_at": time.Now().UTC()})
		if update.Error != nil {
			return fmt.Errorf("update warehouse balance: %w", update.Error)
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("%w: concurrent warehouse adjustment", ErrWarehouseLedgerMismatch)
		}

		movement := InventoryMovement{
			TenantID: tenantID, WarehouseID: body.WarehouseID, ProductID: productID, ProductSKUID: skuID,
			MovementType: MovementManualAdjust, Quantity: body.Stock - beforeOnHand,
			BeforeOnHand: beforeOnHand, AfterOnHand: body.Stock, SourceType: "manual_adjustment",
			SourceID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(eventKey)), BusinessEventKey: eventKey,
			RequestHash: requestHash, Reason: reason, Remark: remark, CreatedBy: actor,
		}
		if err := tx.Create(&movement).Error; err != nil {
			if isInventoryUniqueViolation(err) {
				return ErrInventoryIdempotency
			}
			return fmt.Errorf("create manual inventory movement: %w", err)
		}

		aggregate := beforeAggregate + body.Stock - beforeOnHand
		if err := tx.Model(&product.ProductSKU{}).Where("id = ? AND product_id = ?", skuID, productID).
			Updates(map[string]any{"stock": aggregate, "stock_status": stockStatusForSKU(sku, aggregate), "updated_at": time.Now().UTC()}).Error; err != nil {
			return fmt.Errorf("update SKU stock projection: %w", err)
		}
		logRow := InventoryChangeLog{
			TenantID: tenantID, ProductID: productID, ProductSKUID: skuID,
			ChangeType: ChangeManualAdjust, BeforeStock: beforeAggregate, AfterStock: aggregate, Delta: aggregate - beforeAggregate,
			Reason: reason, Remark: remark, CreatedBy: actor, BusinessEventKey: eventKey,
		}
		if err := tx.Create(&logRow).Error; err != nil {
			return fmt.Errorf("create manual compatibility log: %w", err)
		}
		result = &ManualAdjustmentResult{
			ProductSKUID: skuID, WarehouseID: body.WarehouseID, WarehouseOnHand: body.Stock,
			AggregateStock: aggregate, MovementID: movement.ID,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ListWarehouseBalances(ctx context.Context, tenantID int64, productID, skuID uuid.UUID) ([]WarehouseBalanceDTO, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, fmt.Errorf("inventory: db unavailable")
	}
	var count int64
	if err := s.DB.WithContext(ctx).Table("product_skus").
		Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
		Where("product_skus.id = ? AND product_skus.product_id = ? AND products.tenant_id = ?", skuID, productID, tenantID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, gorm.ErrRecordNotFound
	}
	type row struct {
		WarehouseID   uuid.UUID
		WarehouseCode string
		WarehouseName string
		IsDefault     bool
		OnHand        int
		Reserved      int
		InTransit     int
		Damaged       int
		Version       int
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Table("warehouse_stock_balances AS b").
		Select("b.warehouse_id, w.code AS warehouse_code, w.name AS warehouse_name, w.is_default, b.on_hand, b.reserved, b.in_transit, b.damaged, b.version").
		Joins("JOIN warehouses AS w ON w.id = b.warehouse_id AND w.tenant_id = b.tenant_id").
		Where("b.tenant_id = ? AND b.product_sku_id = ?", tenantID, skuID).
		Order("w.is_default DESC, w.code ASC, w.id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list warehouse balances: %w", err)
	}
	out := make([]WarehouseBalanceDTO, 0, len(rows))
	for _, item := range rows {
		balance := WarehouseStockBalance{OnHand: item.OnHand, Reserved: item.Reserved, Damaged: item.Damaged}
		out = append(out, WarehouseBalanceDTO{
			WarehouseID: item.WarehouseID, WarehouseCode: item.WarehouseCode, WarehouseName: item.WarehouseName,
			IsDefault: item.IsDefault, OnHand: item.OnHand, Reserved: item.Reserved, InTransit: item.InTransit,
			Damaged: item.Damaged, Available: balance.Available(), Version: item.Version,
		})
	}
	return out, nil
}

func resolveLegacyMigrationWarehouse(ctx context.Context, tx *gorm.DB, tenantID int64, actor *uuid.UUID) (*warehouse.Warehouse, error) {
	var row warehouse.Warehouse
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND status = ? AND is_default = ?", tenantID, warehouse.StatusActive, true).
		Order("id ASC").First(&row).Error
	if err == nil {
		return &row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("load default warehouse: %w", err)
	}
	pending := warehouse.Warehouse{
		TenantID: tenantID, Code: pendingAllocationWarehouseCode, Name: "待分配仓",
		Status: warehouse.StatusActive, IsDefault: false, CreatedBy: actor,
	}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "code"}}, DoNothing: true,
	}).Create(&pending).Error; err != nil {
		return nil, fmt.Errorf("create pending allocation warehouse: %w", err)
	}
	if err := tx.WithContext(ctx).Where("tenant_id = ? AND code = ?", tenantID, pendingAllocationWarehouseCode).First(&row).Error; err != nil {
		return nil, fmt.Errorf("load pending allocation warehouse: %w", err)
	}
	if row.Status != warehouse.StatusActive {
		return nil, fmt.Errorf("%w: pending allocation warehouse is inactive", ErrWarehouseLedgerMismatch)
	}
	return &row, nil
}

func countUnmigratedSKUs(ctx context.Context, db *gorm.DB, tenantID int64) (int64, error) {
	var count int64
	err := db.WithContext(ctx).Table("product_skus AS sk").
		Joins("JOIN products AS p ON p.id = sk.product_id AND p.deleted_at IS NULL").
		Where("p.tenant_id = ?", tenantID).
		Where("NOT EXISTS (SELECT 1 FROM warehouse_stock_balances b WHERE b.tenant_id = ? AND b.product_sku_id = sk.id)", tenantID).
		Count(&count).Error
	return count, err
}

// MigrateLegacyStock migrates one bounded batch to the active default warehouse,
// or to a tenant-local pending allocation warehouse when no default exists.
func (s *Service) MigrateLegacyStock(ctx context.Context, tenantID int64, actor *uuid.UUID, limit int) (*LegacyStockMigrationResult, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, fmt.Errorf("inventory: db unavailable")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	result := &LegacyStockMigrationResult{}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var skus []product.ProductSKU
		query := tx.Model(&product.ProductSKU{}).Select("product_skus.*").
			Joins("JOIN products ON products.id = product_skus.product_id AND products.deleted_at IS NULL").
			Where("products.tenant_id = ?", tenantID).
			Where("NOT EXISTS (SELECT 1 FROM warehouse_stock_balances b WHERE b.tenant_id = ? AND b.product_sku_id = product_skus.id)", tenantID).
			Order("product_skus.id ASC").Limit(limit).
			Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "product_skus"}, Options: "SKIP LOCKED"})
		if err := query.Find(&skus).Error; err != nil {
			return fmt.Errorf("select legacy SKU batch: %w", err)
		}
		if len(skus) == 0 {
			return nil
		}
		target, err := resolveLegacyMigrationWarehouse(ctx, tx, tenantID, actor)
		if err != nil {
			return err
		}
		result.WarehouseID = target.ID
		result.WarehouseCode = target.Code
		for _, sku := range skus {
			migrated, err := ensureLegacyBalanceTx(ctx, tx, tenantID, sku, target.ID, actor)
			if err != nil {
				return err
			}
			if migrated {
				result.MigratedCount++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	remaining, err := countUnmigratedSKUs(ctx, s.DB, tenantID)
	if err != nil {
		return nil, fmt.Errorf("count remaining legacy stock: %w", err)
	}
	result.RemainingCount = remaining
	return result, nil
}

func reconciliationStatusFilter(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case reconciliationMatched, reconciliationUnmigrated, reconciliationMismatch:
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return ""
	}
}

// ReconcileWarehouseLedger compares product_skus.stock with the sum of warehouse balances.
func (s *Service) ReconcileWarehouseLedger(ctx context.Context, tenantID int64, page, pageSize int, status string) (*WarehouseLedgerReconciliationResult, error) {
	if s == nil || s.DB == nil || tenantID < 0 {
		return nil, fmt.Errorf("inventory: db unavailable")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	status = reconciliationStatusFilter(status)
	baseSQL := `
		SELECT p.id AS product_id, p.title AS product_title, sk.id AS product_sku_id,
		       sk.sku_code, sk.sku_name, COALESCE(sk.stock, 0) AS aggregate_stock,
		       COALESCE(lb.warehouse_on_hand, 0) AS warehouse_on_hand,
		       COALESCE(lb.balance_count, 0) AS balance_count,
		       CASE
		         WHEN COALESCE(sk.stock, 0) < 0 THEN 'mismatch'
		         WHEN COALESCE(lb.balance_count, 0) = 0 THEN 'unmigrated'
		         WHEN COALESCE(sk.stock, 0) = COALESCE(lb.warehouse_on_hand, 0) THEN 'matched'
		         ELSE 'mismatch'
		       END AS status
		FROM product_skus sk
		JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL
		LEFT JOIN (
			SELECT tenant_id, product_sku_id, SUM(on_hand) AS warehouse_on_hand, COUNT(*) AS balance_count
			FROM warehouse_stock_balances
			GROUP BY tenant_id, product_sku_id
		) lb ON lb.tenant_id = p.tenant_id AND lb.product_sku_id = sk.id
		WHERE p.tenant_id = ?`
	args := []any{tenantID}
	filteredSQL := "SELECT * FROM (" + baseSQL + ") ledger_rows"
	if status != "" {
		filteredSQL += " WHERE status = ?"
		args = append(args, status)
	}
	result := &WarehouseLedgerReconciliationResult{Page: page, PageSize: pageSize, Items: []WarehouseLedgerReconciliationRow{}}
	if err := s.DB.WithContext(ctx).Raw("SELECT COUNT(*) FROM ("+filteredSQL+") filtered", args...).Scan(&result.Total).Error; err != nil {
		slog.Error("reconcile_debug_count", "error", err)
		return nil, fmt.Errorf("count reconciliation rows: %w", err)
	}
	if err := s.DB.WithContext(ctx).Raw(filteredSQL+" ORDER BY product_title ASC, sku_code ASC, product_sku_id ASC LIMIT ? OFFSET ?", append(args, pageSize, (page-1)*pageSize)...).
		Scan(&result.Items).Error; err != nil {
		slog.Error("reconcile_debug_list", "error", err)
		return nil, fmt.Errorf("list reconciliation rows: %w", err)
	}
	for i := range result.Items {
		result.Items[i].Difference = result.Items[i].WarehouseOnHand - result.Items[i].AggregateStock
	}
	type summary struct {
		Matched    int64
		Unmigrated int64
		Mismatch   int64
	}
	var counts summary
	summarySQL := `SELECT
		COALESCE(SUM(CASE WHEN status = 'matched' THEN 1 ELSE 0 END), 0) AS matched,
		COALESCE(SUM(CASE WHEN status = 'unmigrated' THEN 1 ELSE 0 END), 0) AS unmigrated,
		COALESCE(SUM(CASE WHEN status = 'mismatch' THEN 1 ELSE 0 END), 0) AS mismatch
		FROM (` + baseSQL + `) ledger_summary`
	if err := s.DB.WithContext(ctx).Raw(summarySQL, tenantID).Scan(&counts).Error; err != nil {
		slog.Error("reconcile_debug_summary", "error", err)
		return nil, fmt.Errorf("summarize reconciliation rows: %w", err)
	}
	result.Matched, result.Unmigrated, result.Mismatch = counts.Matched, counts.Unmigrated, counts.Mismatch
	result.TotalPages = pagesOf(result.Total, pageSize)
	return result, nil
}
