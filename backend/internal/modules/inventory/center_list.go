package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"gorm.io/gorm"
)

// Center bind / sync summary labels for admin UI (Chinese-facing keys).
const (
	CenterBindBound     = "bound"
	CenterBindUnbound   = "unbound"
	CenterBindAmbiguous = "ambiguous"
	CenterBindNone      = "none"
	CenterSyncSuccess   = "success"
	CenterSyncFailed    = "failed"
	CenterSyncPending   = "pending"
	CenterSyncRunning   = "running"
	CenterSyncBlocked   = "blocked"
	CenterSyncDisabled  = "disabled"
	CenterSyncNone      = "none"
	CenterSyncPartial   = "partial_success"
)

// CenterListQuery filters GET /inventory (inventory center hub).
type CenterListQuery struct {
	Keyword       string
	ProductID     *uuid.UUID
	ProductSkuID  *uuid.UUID
	Platform      string
	ShopID        *uuid.UUID
	StockStatus   string
	AlertStatus   string
	SkuBindStatus string
	SyncStatus    string
	HasException  bool
	Page          int
	PageSize      int
}

// InventoryCenterEntry is one SKU row in the inventory center list.
type InventoryCenterEntry struct {
	InventoryAlertEntry
	AvailableStock     int        `json:"availableStock"`
	SkuBindStatus      string     `json:"skuBindStatus"`
	PlatformSyncStatus string     `json:"platformSyncStatus"`
	LastDeductAt       *time.Time `json:"lastDeductAt,omitempty"`
	ExceptionCount     int        `json:"exceptionCount"`
	AffectedOrderCount int        `json:"affectedOrderCount"`
}

// CenterListResult paginates center rows.
type CenterListResult struct {
	Items      []InventoryCenterEntry `json:"list"`
	Total      int64                  `json:"total"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"pageSize"`
	TotalPages int                    `json:"totalPages"`
}

func aggregateBindStatus(pubs []pubJoinScan) string {
	if len(pubs) == 0 {
		return CenterBindNone
	}
	hasBound := false
	hasAmbiguous := false
	hasUnbound := false
	for _, p := range pubs {
		bs := strings.TrimSpace(strings.ToLower(p.BindStatus))
		ext := strings.TrimSpace(p.ExternalSkuID)
		switch bs {
		case productpublish.BindStatusAmbiguous:
			hasAmbiguous = true
		case productpublish.BindStatusBound:
			if ext != "" {
				hasBound = true
			} else {
				hasUnbound = true
			}
		default:
			if ext == "" {
				hasUnbound = true
			}
		}
	}
	if hasAmbiguous {
		return CenterBindAmbiguous
	}
	if hasUnbound {
		return CenterBindUnbound
	}
	if hasBound {
		return CenterBindBound
	}
	return CenterBindUnbound
}

func aggregateSyncStatus(pubs []pubJoinScan, taskByPub map[uuid.UUID]latestTaskScan, bindSt string) string {
	if bindSt == CenterBindUnbound || bindSt == CenterBindAmbiguous {
		return CenterSyncBlocked
	}
	if len(pubs) == 0 {
		return CenterSyncNone
	}
	hasFailed := false
	hasPending := false
	hasRunning := false
	hasSuccess := false
	for _, p := range pubs {
		t, ok := taskByPub[p.PublicationSkuID]
		if !ok {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(t.Status)) {
		case StatusFailed:
			hasFailed = true
		case StatusPending:
			hasPending = true
		case StatusRunning:
			hasRunning = true
		case StatusSuccess:
			hasSuccess = true
		}
	}
	if hasRunning {
		return CenterSyncRunning
	}
	if hasPending {
		return CenterSyncPending
	}
	if hasFailed && hasSuccess {
		return CenterSyncPartial
	}
	if hasFailed {
		return CenterSyncFailed
	}
	if hasSuccess {
		return CenterSyncSuccess
	}
	return CenterSyncNone
}

func (s *Service) loadLastDeductBySku(ctx context.Context, skuIDs []uuid.UUID) map[uuid.UUID]time.Time {
	out := map[uuid.UUID]time.Time{}
	if len(skuIDs) == 0 || s == nil || s.DB == nil {
		return out
	}
	if !s.DB.Migrator().HasTable(&OrderInventoryEffect{}) {
		return out
	}
	type row struct {
		SK uuid.UUID `gorm:"column:product_sku_id"`
		Tm time.Time `gorm:"column:tm"`
	}
	var rows []row
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Select("product_sku_id, MAX(created_at) AS tm").
		Where("product_sku_id IN ? AND effect_type = ?", skuIDs, "deduct").
		Group("product_sku_id").
		Scan(&rows).Error
	for _, r := range rows {
		out[r.SK] = r.Tm
	}
	return out
}

func (s *Service) loadExceptionCountsBySku(ctx context.Context, skuIDs []uuid.UUID) map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	if len(skuIDs) == 0 || s == nil || s.DB == nil {
		return out
	}
	type row struct {
		SK  uuid.UUID `gorm:"column:product_sku_id"`
		Cnt int       `gorm:"column:cnt"`
	}
	// Failed deduct effects + failed sync tasks tied to SKU.
	if s.DB.Migrator().HasTable(&OrderInventoryEffect{}) {
		var eff []row
		_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
			Select("product_sku_id, COUNT(*) AS cnt").
			Where("product_sku_id IN ? AND status = ?", skuIDs, StatusFailed).
			Group("product_sku_id").
			Scan(&eff).Error
		for _, r := range eff {
			out[r.SK] += r.Cnt
		}
	}
	var syncRows []row
	_ = s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Select("product_sku_id, COUNT(*) AS cnt").
		Where("product_sku_id IN ? AND status = ?", skuIDs, StatusFailed).
		Group("product_sku_id").
		Scan(&syncRows).Error
	for _, r := range syncRows {
		if r.SK != uuid.Nil {
			out[r.SK] += r.Cnt
		}
	}
	return out
}

func (s *Service) loadAffectedOrderCountsBySku(ctx context.Context, skuIDs []uuid.UUID) map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	if len(skuIDs) == 0 || !s.DB.Migrator().HasTable(&OrderInventoryEffect{}) {
		return out
	}
	type row struct {
		SK  uuid.UUID `gorm:"column:product_sku_id"`
		Cnt int       `gorm:"column:cnt"`
	}
	var rows []row
	_ = s.DB.WithContext(ctx).Model(&OrderInventoryEffect{}).
		Select("product_sku_id, COUNT(DISTINCT order_id) AS cnt").
		Where("product_sku_id IN ?", skuIDs).
		Group("product_sku_id").
		Scan(&rows).Error
	for _, r := range rows {
		out[r.SK] = r.Cnt
	}
	return out
}

func (s *Service) applyCenterBindFilter(tx *gorm.DB, bindStatus string) *gorm.DB {
	bs := strings.TrimSpace(strings.ToLower(bindStatus))
	switch bs {
	case CenterBindAmbiguous:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id AND LOWER(pps_b.bind_status) = ?
		)`, productpublish.BindStatusAmbiguous)
	case CenterBindUnbound:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
			AND (TRIM(COALESCE(pps_b.external_sku_id,'')) = '' OR LOWER(COALESCE(pps_b.bind_status,'')) IN (?,?))
		)`, productpublish.BindStatusUnmatched, productpublish.BindStatusFailed)
	case CenterBindBound:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
			AND LOWER(COALESCE(pps_b.bind_status,'')) = ? AND TRIM(COALESCE(pps_b.external_sku_id,'')) <> ''
		)`, productpublish.BindStatusBound)
	case CenterBindNone:
		return tx.Where(`NOT EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
		)`)
	default:
		return tx
	}
}

func (s *Service) applyCenterSyncFilter(tx *gorm.DB, syncStatus string) *gorm.DB {
	st := strings.TrimSpace(strings.ToLower(syncStatus))
	switch st {
	case CenterSyncFailed:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.product_sku_id = sk.id AND t.status = ?
			AND NOT EXISTS (SELECT 1 FROM inventory_sync_tasks t2 WHERE t2.product_sku_id = sk.id AND t2.created_at > t.created_at)
		)`, StatusFailed)
	case CenterSyncSuccess:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.product_sku_id = sk.id AND t.status = ?
			AND NOT EXISTS (SELECT 1 FROM inventory_sync_tasks t2 WHERE t2.product_sku_id = sk.id AND t2.created_at > t.created_at)
		)`, StatusSuccess)
	case CenterSyncPending:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.product_sku_id = sk.id AND t.status = ?
		)`, StatusPending)
	case CenterSyncRunning:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.product_sku_id = sk.id AND t.status = ?
		)`, StatusRunning)
	case CenterSyncBlocked:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
			AND (LOWER(COALESCE(pps_b.bind_status,'')) = ? OR TRIM(COALESCE(pps_b.external_sku_id,'')) = '')
		)`, productpublish.BindStatusAmbiguous)
	default:
		return tx
	}
}

func (s *Service) applyCenterHasException(tx *gorm.DB) *gorm.DB {
	parts := []string{}
	args := []any{}
	if s.DB.Migrator().HasTable(&OrderInventoryEffect{}) {
		parts = append(parts, `EXISTS (
			SELECT 1 FROM order_inventory_effects oie
			WHERE oie.product_sku_id = sk.id AND oie.status = ?
		)`)
		args = append(args, StatusFailed)
	}
	parts = append(parts, `EXISTS (
		SELECT 1 FROM inventory_sync_tasks t
		WHERE t.product_sku_id = sk.id AND t.status = ?
	)`)
	args = append(args, StatusFailed)
	return tx.Where("("+strings.Join(parts, " OR ")+")", args...)
}

// ListInventoryCenter pages SKU rows for the inventory center hub.
func (s *Service) ListInventoryCenter(ctx context.Context, q CenterListQuery) (*CenterListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	page, ps := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if ps < 1 || ps > 100 {
		ps = 20
	}
	pol, err := s.loadInventoryAlertPolicy(ctx)
	if err != nil {
		return nil, err
	}
	th := pol.PlatformStockMismatchThresh
	if th < 0 {
		th = 0
	}

	base := s.buildSKUAlertBaseTX(ctx, skuAlertBaseQuery{
		Keyword:       q.Keyword,
		ProductID:     q.ProductID,
		ProductSkuID:  q.ProductSkuID,
		Platform:      q.Platform,
		ShopID:        q.ShopID,
		StockStatus:   q.StockStatus,
		OnlyPublished: false,
	})
	if strings.TrimSpace(q.AlertStatus) != "" {
		base = s.applyAlertsSQLAlertType(base, q.AlertStatus, th)
	}
	if strings.TrimSpace(q.SkuBindStatus) != "" {
		base = s.applyCenterBindFilter(base, q.SkuBindStatus)
	}
	if strings.TrimSpace(q.SyncStatus) != "" {
		base = s.applyCenterSyncFilter(base, q.SyncStatus)
	}
	if q.HasException {
		base = s.applyCenterHasException(base)
	}
	base = base.Group("sk.id, sk.product_id, sk.sku_code, sk.sku_name, sk.stock, sk.warning_stock, sk.safety_stock, sk.updated_at, p.title")

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var scans []alertSKUScan
	if err := base.Order("sk.updated_at DESC").Offset(offset).Limit(ps).Scan(&scans).Error; err != nil {
		return nil, err
	}

	skuIDs := make([]uuid.UUID, 0, len(scans))
	for _, r := range scans {
		skuIDs = append(skuIDs, r.ID)
	}

	pubBySku := map[uuid.UUID][]pubJoinScan{}
	if len(skuIDs) > 0 {
		var pubs []pubJoinScan
		_ = s.DB.WithContext(ctx).Table("product_publication_skus AS ps").
			Select(`ps.id AS publication_sku_id, ps.product_sku_id, ps.stock AS platform_stock, ps.sku_code,
				ps.external_sku_id, ps.bind_status, pp.shop_id, sh.shop_name, pp.platform, pp.external_product_id, pp.last_synced_at`).
			Joins("INNER JOIN product_publications pp ON pp.id = ps.publication_id AND pp.deleted_at IS NULL").
			Joins("INNER JOIN shops sh ON sh.id = pp.shop_id").
			Where("ps.product_sku_id IN ?", skuIDs).
			Scan(&pubs).Error
		for _, p := range pubs {
			if p.ProductSkuID == nil {
				continue
			}
			pubBySku[*p.ProductSkuID] = append(pubBySku[*p.ProductSkuID], p)
		}
	}

	pubIDs := make([]uuid.UUID, 0, 32)
	for _, r := range scans {
		for _, p := range pubBySku[r.ID] {
			pubIDs = append(pubIDs, p.PublicationSkuID)
		}
	}
	taskByPub := s.loadLatestTasksByPubSku(ctx, pubIDs)
	lastLog := s.loadMaxLogTimeBySku(ctx, skuIDs)
	lastDeduct := s.loadLastDeductBySku(ctx, skuIDs)
	exCounts := s.loadExceptionCountsBySku(ctx, skuIDs)
	orderCounts := s.loadAffectedOrderCountsBySku(ctx, skuIDs)

	items := make([]InventoryCenterEntry, 0, len(scans))
	for _, row := range scans {
		st := product.CalculateSKUStockStatus(derefStock(row.Stock), row.WarningStock, row.SafetyStock)
		alerts := make([]string, 0, 6)
		if pol.EnableInventoryAlerts {
			switch st {
			case product.StockStatusOutOfStock:
				if pol.AlertOutOfStock {
					alerts = append(alerts, AlertTypeOutOfStock)
				}
			case product.StockStatusBelowSafetyStock:
				alerts = append(alerts, AlertTypeBelowSafetyStock)
			case product.StockStatusLowStock:
				alerts = append(alerts, AlertTypeLowStock)
			}
		}

		pubs := pubBySku[row.ID]
		bindSt := aggregateBindStatus(pubs)
		stocks := make([]PlatformStockAlertEntry, 0, len(pubs))
		var worstFail *latestTaskScan
		for _, p := range pubs {
			pl := strings.TrimSpace(strings.ToLower(p.Platform))
			pst := platformLineStatus(derefStock(row.Stock), p.PlatformStock, th, pol.AlertPlatformStockMismatch)
			switch pst {
			case PlatformStockUnknown:
				alerts = appendUnique(alerts, AlertTypePlatformStockUnknown)
			case PlatformStockMismatch:
				if pol.AlertPlatformStockMismatch {
					alerts = appendUnique(alerts, AlertTypePlatformStockMismatch)
				}
			}
			ent := PlatformStockAlertEntry{
				PublicationSkuID:    p.PublicationSkuID,
				ShopID:              p.ShopID,
				ShopName:            p.ShopName,
				Platform:            pl,
				ExternalProductID:   p.ExternalProductID,
				ExternalSkuID:       p.ExternalSkuID,
				PlatformStock:       p.PlatformStock,
				PlatformStockStatus: pst,
				LastSyncedAt:        p.LastSyncedAt,
			}
			if t, ok := taskByPub[p.PublicationSkuID]; ok {
				tidVal := t.TaskID
				ent.LastSyncTaskID = &tidVal
				ent.LastSyncStatus = t.Status
				ent.LastSyncError = clipErr(t.ErrorMessage, 520)
				ca := t.CreatedAt
				ent.LastSyncAt = &ca
				if t.Status == StatusFailed {
					alerts = appendUnique(alerts, AlertTypeInventorySyncFailed)
					cp := t
					if worstFail == nil || cp.CreatedAt.After(worstFail.CreatedAt) {
						worstFail = &cp
					}
				}
			}
			stocks = append(stocks, ent)
		}

		localStock := derefStock(row.Stock)
		alertEntry := InventoryAlertEntry{
			ProductID:             row.ProductID,
			ProductTitle:          row.ProductTitle,
			ProductSkuID:          row.ID,
			SKUCode:               row.SKUCode,
			SKUName:               row.SKUName,
			Stock:                 localStock,
			WarningStock:          row.WarningStock,
			SafetyStock:           row.SafetyStock,
			StockStatus:           st,
			AlertTypes:            alerts,
			PublicationCount:      len(stocks),
			PlatformStocks:        stocks,
			LastInventoryChangeAt: ptrTime(lastLog[row.ID]),
		}
		if worstFail != nil {
			tid := worstFail.TaskID
			alertEntry.LastSyncTaskID = &tid
			alertEntry.LastSyncStatus = worstFail.Status
			alertEntry.LastSyncError = clipErr(worstFail.ErrorMessage, 520)
			ca := worstFail.CreatedAt
			alertEntry.LastSyncAt = &ca
		}

		items = append(items, InventoryCenterEntry{
			InventoryAlertEntry: alertEntry,
			AvailableStock:      localStock,
			SkuBindStatus:       bindSt,
			PlatformSyncStatus:  aggregateSyncStatus(pubs, taskByPub, bindSt),
			LastDeductAt:        ptrTime(lastDeduct[row.ID]),
			ExceptionCount:      exCounts[row.ID],
			AffectedOrderCount:  orderCounts[row.ID],
		})
	}

	return &CenterListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}
