package settlement

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type repository struct {
	db *gorm.DB
}

func (r repository) shop(ctx context.Context, tenantID int64, shopID uuid.UUID) (*shopFact, error) {
	if r.db == nil {
		return nil, fmt.Errorf("settlement repository unavailable")
	}
	var row shopFact
	err := r.db.WithContext(ctx).Table("shops").
		Select("id, shop_name, platform").
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, shopID).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r repository) lockShop(ctx context.Context, tenantID int64, shopID uuid.UUID) error {
	var row struct {
		ID uuid.UUID
	}
	if err := r.db.WithContext(ctx).Table("shops").
		Select("id").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND id = ? AND deleted_at IS NULL", tenantID, shopID).
		Take(&row).Error; err != nil {
		return fmt.Errorf("lock settlement import shop: %w", err)
	}
	return nil
}

func (r repository) existingTransactions(ctx context.Context, tenantID int64, shopID uuid.UUID, platform string, externalIDs []string, lock bool) ([]Transaction, error) {
	rows := make([]Transaction, 0)
	if len(externalIDs) == 0 {
		return rows, nil
	}
	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND shop_id = ? AND platform = ? AND external_transaction_id IN ?", tenantID, shopID, platform, externalIDs)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list existing settlement transactions: %w", err)
	}
	return rows, nil
}

func (r repository) findImportByIdempotency(ctx context.Context, tenantID int64, key string) (*Import, error) {
	var row Import
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND idempotency_key = ?", tenantID, key).Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r repository) findImportByFile(ctx context.Context, tenantID int64, shopID uuid.UUID, hash string) (*Import, error) {
	var row Import
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND shop_id = ? AND file_hash = ?", tenantID, shopID, hash).Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r repository) groupQuery(ctx context.Context, tenantID int64, scope Scope, q ListQuery) *gorm.DB {
	tx := r.db.WithContext(ctx).Table("settlement_transactions AS settlement").
		Where("settlement.tenant_id = ?", tenantID)
	if scope.RestrictStoreScope {
		if len(scope.AllowedShopIDs) == 0 {
			tx = tx.Where("1 = 0")
		} else {
			tx = tx.Where("settlement.shop_id IN ?", scope.AllowedShopIDs)
		}
	}
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		tx = tx.Where("settlement.shop_id = ?", *q.ShopID)
	}
	if value := strings.TrimSpace(q.OrderNo); value != "" {
		tx = tx.Where("LOWER(settlement.order_no) LIKE ?", "%"+strings.ToLower(value)+"%")
	}
	if value := strings.TrimSpace(q.Platform); value != "" {
		tx = tx.Where("LOWER(settlement.platform) = ?", strings.ToLower(value))
	}
	if value := strings.TrimSpace(q.Currency); value != "" {
		tx = tx.Where(`EXISTS (
			SELECT 1 FROM settlement_transactions currency_fact
			WHERE currency_fact.tenant_id = settlement.tenant_id
				AND currency_fact.reconciliation_id = settlement.reconciliation_id
				AND UPPER(currency_fact.currency) = ?
		)`, strings.ToUpper(value))
	}
	if q.Start != nil {
		tx = tx.Where("settlement.settled_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("settlement.settled_at <= ?", *q.End)
	}
	return tx
}

func (r repository) listGroups(ctx context.Context, tenantID int64, scope Scope, q ListQuery, offset, limit int) ([]groupFact, int64, error) {
	if r.db == nil {
		return nil, 0, fmt.Errorf("settlement repository unavailable")
	}
	var total int64
	if err := r.groupQuery(ctx, tenantID, scope, q).
		Distinct("settlement.reconciliation_id").
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count settlement reconciliation groups: %w", err)
	}
	rows := make([]groupFact, 0)
	if total == 0 || limit < 1 {
		return rows, total, nil
	}
	err := r.groupQuery(ctx, tenantID, scope, q).
		Select(`settlement.reconciliation_id, settlement.shop_id,
			MAX(COALESCE(shops.shop_name, '')) AS shop_name,
			MAX(settlement.platform) AS platform, MAX(settlement.order_no) AS order_no`).
		Joins("LEFT JOIN shops ON shops.id = settlement.shop_id AND shops.tenant_id = settlement.tenant_id AND shops.deleted_at IS NULL").
		Group("settlement.reconciliation_id, settlement.shop_id").
		Order("MAX(settlement.settled_at) DESC, settlement.reconciliation_id DESC").
		Offset(offset).Limit(limit).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list settlement reconciliation groups: %w", err)
	}
	return rows, total, nil
}

func (r repository) getGroup(ctx context.Context, tenantID int64, scope Scope, id uuid.UUID) (*groupFact, error) {
	q := ListQuery{}
	var row groupFact
	err := r.groupQuery(ctx, tenantID, scope, q).
		Where("settlement.reconciliation_id = ?", id).
		Select(`settlement.reconciliation_id, settlement.shop_id,
			MAX(COALESCE(shops.shop_name, '')) AS shop_name,
			MAX(settlement.platform) AS platform, MAX(settlement.order_no) AS order_no`).
		Joins("LEFT JOIN shops ON shops.id = settlement.shop_id AND shops.tenant_id = settlement.tenant_id AND shops.deleted_at IS NULL").
		Group("settlement.reconciliation_id, settlement.shop_id").Take(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r repository) groupsByIDs(ctx context.Context, tenantID int64, ids []uuid.UUID) ([]groupFact, error) {
	rows := make([]groupFact, 0)
	if len(ids) == 0 {
		return rows, nil
	}
	err := r.db.WithContext(ctx).Table("settlement_transactions AS settlement").
		Select(`settlement.reconciliation_id, settlement.shop_id,
			MAX(COALESCE(shops.shop_name, '')) AS shop_name,
			MAX(settlement.platform) AS platform, MAX(settlement.order_no) AS order_no`).
		Joins("LEFT JOIN shops ON shops.id = settlement.shop_id AND shops.tenant_id = settlement.tenant_id AND shops.deleted_at IS NULL").
		Where("settlement.tenant_id = ? AND settlement.reconciliation_id IN ?", tenantID, ids).
		Group("settlement.reconciliation_id, settlement.shop_id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list settlement reconciliation groups by id: %w", err)
	}
	return rows, nil
}

func (r repository) listTransactions(ctx context.Context, tenantID int64, groupIDs []uuid.UUID) ([]Transaction, error) {
	rows := make([]Transaction, 0)
	if len(groupIDs) == 0 {
		return rows, nil
	}
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND reconciliation_id IN ?", tenantID, groupIDs).
		Order("settled_at ASC, created_at ASC, id ASC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list settlement facts: %w", err)
	}
	return rows, nil
}

func (r repository) listOrders(ctx context.Context, tenantID int64, groupIDs []uuid.UUID) ([]orderFact, error) {
	rows := make([]orderFact, 0)
	if len(groupIDs) == 0 {
		return rows, nil
	}
	err := r.db.WithContext(ctx).Table("orders").Distinct().
		Select(`orders.id, settlement.reconciliation_id, orders.platform, orders.currency,
			orders.total_amount AS total_amount_text`).
		Joins(`JOIN settlement_transactions AS settlement
			ON settlement.tenant_id = orders.tenant_id
			AND settlement.shop_id = orders.shop_id
			AND settlement.order_no = orders.order_no`).
		Where("orders.tenant_id = ? AND orders.deleted_at IS NULL AND settlement.reconciliation_id IN ?", tenantID, groupIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list settlement local orders: %w", err)
	}
	return rows, nil
}

func (r repository) groupIDsForOrders(ctx context.Context, tenantID int64, orderIDs []uuid.UUID) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0)
	if len(orderIDs) == 0 {
		return ids, nil
	}
	err := r.db.WithContext(ctx).Table("settlement_transactions AS settlement").Distinct().
		Joins(`JOIN orders ON orders.tenant_id = settlement.tenant_id
			AND orders.shop_id = settlement.shop_id
			AND orders.order_no = settlement.order_no
			AND orders.deleted_at IS NULL`).
		Where("settlement.tenant_id = ? AND orders.id IN ?", tenantID, orderIDs).
		Pluck("settlement.reconciliation_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list settlement groups for orders: %w", err)
	}
	return ids, nil
}
