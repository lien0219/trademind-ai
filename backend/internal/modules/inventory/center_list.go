package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
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
	TenantID      int64
	Keyword       string
	ProductID     *uuid.UUID
	ProductSKUID  *uuid.UUID
	Platform      string
	ShopID        *uuid.UUID
	StockStatus   string
	AlertStatus   string
	SKUBindStatus string
	SyncStatus    string
	HasException  bool
	Page          int
	PageSize      int
	Cursor        string
	Limit         int
	UseCursor     bool
}

// InventoryCenterEntry is one SKU row in the inventory center list.
type InventoryCenterEntry struct {
	InventoryAlertEntry
	AvailableStock     int        `json:"availableStock"`
	SKUBindStatus      string     `json:"skuBindStatus"`
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
	Limit      int                    `json:"limit"`
	NextCursor string                 `json:"nextCursor,omitempty"`
	HasMore    bool                   `json:"hasMore"`
}

func inventoryCenterCursorScope(q CenterListQuery) (string, string) {
	shopScope := ""
	if q.ShopID != nil && *q.ShopID != uuid.Nil {
		shopScope = q.ShopID.String()
	}
	return pagination.Fingerprint(map[string]any{
		"tenantId":      q.TenantID,
		"shopId":        shopScope,
		"keyword":       q.Keyword,
		"productId":     q.ProductID,
		"productSkuId":  q.ProductSKUID,
		"platform":      q.Platform,
		"stockStatus":   q.StockStatus,
		"alertStatus":   q.AlertStatus,
		"skuBindStatus": q.SKUBindStatus,
		"syncStatus":    q.SyncStatus,
		"hasException":  q.HasException,
		"sort":          "updated_at_desc_id_desc",
	}), shopScope
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
		ext := strings.TrimSpace(p.ExternalSKUID)
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
		t, ok := taskByPub[p.PublicationSKUID]
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

func (s *Service) loadLastDeductBySKU(ctx context.Context, tenantID int64, skuIDs []uuid.UUID) map[uuid.UUID]time.Time {
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
		Where("tenant_id = ? AND product_sku_id IN ? AND effect_type = ?", tenantID, skuIDs, "deduct").
		Group("product_sku_id").
		Scan(&rows).Error
	for _, r := range rows {
		out[r.SK] = r.Tm
	}
	return out
}

func (s *Service) loadExceptionCountsBySKU(ctx context.Context, tenantID int64, skuIDs []uuid.UUID) map[uuid.UUID]int {
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
			Where("tenant_id = ? AND product_sku_id IN ? AND status = ?", tenantID, skuIDs, StatusFailed).
			Group("product_sku_id").
			Scan(&eff).Error
		for _, r := range eff {
			out[r.SK] += r.Cnt
		}
	}
	var syncRows []row
	_ = s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Select("product_sku_id, COUNT(*) AS cnt").
		Where("tenant_id = ? AND product_sku_id IN ? AND status = ?", tenantID, skuIDs, StatusFailed).
		Group("product_sku_id").
		Scan(&syncRows).Error
	for _, r := range syncRows {
		if r.SK != uuid.Nil {
			out[r.SK] += r.Cnt
		}
	}
	return out
}

func (s *Service) loadAffectedOrderCountsBySKU(ctx context.Context, tenantID int64, skuIDs []uuid.UUID) map[uuid.UUID]int {
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
		Where("tenant_id = ? AND product_sku_id IN ?", tenantID, skuIDs).
		Group("product_sku_id").
		Scan(&rows).Error
	for _, r := range rows {
		out[r.SK] = r.Cnt
	}
	return out
}

func (s *Service) applyCenterBindFilter(tx *gorm.DB, bindStatus string, tenantID int64) *gorm.DB {
	bs := strings.TrimSpace(strings.ToLower(bindStatus))
	switch bs {
	case CenterBindAmbiguous:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.tenant_id = ? AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id AND LOWER(pps_b.bind_status) = ?
		)`, tenantID, productpublish.BindStatusAmbiguous)
	case CenterBindUnbound:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.tenant_id = ? AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
			AND (TRIM(COALESCE(pps_b.external_sku_id,'')) = '' OR LOWER(COALESCE(pps_b.bind_status,'')) IN (?,?))
		)`, tenantID, productpublish.BindStatusUnmatched, productpublish.BindStatusFailed)
	case CenterBindBound:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.tenant_id = ? AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
			AND LOWER(COALESCE(pps_b.bind_status,'')) = ? AND TRIM(COALESCE(pps_b.external_sku_id,'')) <> ''
		)`, tenantID, productpublish.BindStatusBound)
	case CenterBindNone:
		return tx.Where(`NOT EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.tenant_id = ? AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
		)`, tenantID)
	default:
		return tx
	}
}

func (s *Service) applyCenterSyncFilter(tx *gorm.DB, syncStatus string, tenantID int64) *gorm.DB {
	st := strings.TrimSpace(strings.ToLower(syncStatus))
	switch st {
	case CenterSyncFailed:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.tenant_id = ? AND t.product_sku_id = sk.id AND t.status = ?
			AND NOT EXISTS (SELECT 1 FROM inventory_sync_tasks t2 WHERE t2.tenant_id = ? AND t2.product_sku_id = sk.id AND t2.created_at > t.created_at)
		)`, tenantID, StatusFailed, tenantID)
	case CenterSyncSuccess:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.tenant_id = ? AND t.product_sku_id = sk.id AND t.status = ?
			AND NOT EXISTS (SELECT 1 FROM inventory_sync_tasks t2 WHERE t2.tenant_id = ? AND t2.product_sku_id = sk.id AND t2.created_at > t.created_at)
		)`, tenantID, StatusSuccess, tenantID)
	case CenterSyncPending:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.tenant_id = ? AND t.product_sku_id = sk.id AND t.status = ?
		)`, tenantID, StatusPending)
	case CenterSyncRunning:
		return tx.Where(`EXISTS (
			SELECT 1 FROM inventory_sync_tasks t
			WHERE t.tenant_id = ? AND t.product_sku_id = sk.id AND t.status = ?
		)`, tenantID, StatusRunning)
	case CenterSyncBlocked:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus pps_b
			INNER JOIN product_publications pp_b ON pp_b.id = pps_b.publication_id AND pp_b.tenant_id = ? AND pp_b.deleted_at IS NULL
			WHERE pps_b.product_sku_id = sk.id
			AND (LOWER(COALESCE(pps_b.bind_status,'')) = ? OR TRIM(COALESCE(pps_b.external_sku_id,'')) = '')
		)`, tenantID, productpublish.BindStatusAmbiguous)
	default:
		return tx
	}
}

func (s *Service) applyCenterHasException(tx *gorm.DB, tenantID int64) *gorm.DB {
	parts := []string{}
	args := []any{}
	if s.DB.Migrator().HasTable(&OrderInventoryEffect{}) {
		parts = append(parts, `EXISTS (
			SELECT 1 FROM order_inventory_effects oie
			WHERE oie.tenant_id = ? AND oie.product_sku_id = sk.id AND oie.status = ?
		)`)
		args = append(args, tenantID, StatusFailed)
	}
	parts = append(parts, `EXISTS (
		SELECT 1 FROM inventory_sync_tasks t
		WHERE t.tenant_id = ? AND t.product_sku_id = sk.id AND t.status = ?
	)`)
	args = append(args, tenantID, StatusFailed)
	return tx.Where("("+strings.Join(parts, " OR ")+")", args...)
}

// ListInventoryCenter pages SKU rows for the inventory center hub.
func (s *Service) ListInventoryCenter(ctx context.Context, q CenterListQuery) (*CenterListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("inventory: no db")
	}
	if q.UseCursor && q.Limit > 0 {
		q.PageSize = q.Limit
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
		TenantID:      q.TenantID,
		Keyword:       q.Keyword,
		ProductID:     q.ProductID,
		ProductSKUID:  q.ProductSKUID,
		Platform:      q.Platform,
		ShopID:        q.ShopID,
		StockStatus:   q.StockStatus,
		OnlyPublished: false,
	})
	base = base.Where("p.tenant_id = ?", q.TenantID)
	if strings.TrimSpace(q.AlertStatus) != "" {
		base = s.applyAlertsSQLAlertType(base, q.AlertStatus, th, q.TenantID)
	}
	if strings.TrimSpace(q.SKUBindStatus) != "" {
		base = s.applyCenterBindFilter(base, q.SKUBindStatus, q.TenantID)
	}
	if strings.TrimSpace(q.SyncStatus) != "" {
		base = s.applyCenterSyncFilter(base, q.SyncStatus, q.TenantID)
	}
	if q.HasException {
		base = s.applyCenterHasException(base, q.TenantID)
	}
	base = base.Group("sk.id, sk.product_id, sk.sku_code, sk.sku_name, sk.stock, sk.warning_stock, sk.safety_stock, sk.updated_at, p.title")
	scopeHash, cursorShopID := inventoryCenterCursorScope(q)
	if q.UseCursor && strings.TrimSpace(q.Cursor) != "" {
		cur, err := pagination.DecodeCursor(q.Cursor, q.TenantID, cursorShopID, scopeHash)
		if err != nil {
			return nil, err
		}
		next, err := pagination.ApplyDescKeyset(base, "sk.updated_at", "sk.id", cur)
		if err != nil {
			return nil, err
		}
		base = next
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}
	var scans []alertSKUScan
	query := base.Order("sk.updated_at DESC, sk.id DESC")
	limit := ps
	if q.UseCursor {
		limit = ps + 1
	} else {
		offset := (page - 1) * ps
		query = query.Offset(offset)
	}
	if err := query.Limit(limit).Scan(&scans).Error; err != nil {
		return nil, err
	}
	hasMore := q.UseCursor && len(scans) > ps
	if hasMore {
		scans = scans[:ps]
	}

	skuIDs := make([]uuid.UUID, 0, len(scans))
	for _, r := range scans {
		skuIDs = append(skuIDs, r.ID)
	}

	pubBySKU := map[uuid.UUID][]pubJoinScan{}
	if len(skuIDs) > 0 {
		var pubs []pubJoinScan
		_ = s.DB.WithContext(ctx).Table("product_publication_skus AS ps").
			Select(`ps.id AS publication_sku_id, ps.product_sku_id, ps.stock AS platform_stock, ps.sku_code,
				ps.external_sku_id, ps.bind_status, pp.shop_id, sh.shop_name, pp.platform, pp.external_product_id, pp.last_synced_at`).
			Joins("INNER JOIN product_publications pp ON pp.id = ps.publication_id AND pp.deleted_at IS NULL").
			Joins("INNER JOIN shops sh ON sh.id = pp.shop_id AND sh.tenant_id = ?", q.TenantID).
			Where("ps.product_sku_id IN ? AND pp.tenant_id = ?", skuIDs, q.TenantID).
			Scan(&pubs).Error
		for _, p := range pubs {
			if p.ProductSKUID == nil {
				continue
			}
			pubBySKU[*p.ProductSKUID] = append(pubBySKU[*p.ProductSKUID], p)
		}
	}

	pubIDs := make([]uuid.UUID, 0, 32)
	for _, r := range scans {
		for _, p := range pubBySKU[r.ID] {
			pubIDs = append(pubIDs, p.PublicationSKUID)
		}
	}
	taskByPub := s.loadLatestTasksByPubSKU(ctx, q.TenantID, pubIDs)
	lastLog := s.loadMaxLogTimeBySKU(ctx, q.TenantID, skuIDs)
	lastDeduct := s.loadLastDeductBySKU(ctx, q.TenantID, skuIDs)
	exCounts := s.loadExceptionCountsBySKU(ctx, q.TenantID, skuIDs)
	orderCounts := s.loadAffectedOrderCountsBySKU(ctx, q.TenantID, skuIDs)

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

		pubs := pubBySKU[row.ID]
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
				PublicationSKUID:    p.PublicationSKUID,
				ShopID:              p.ShopID,
				ShopName:            p.ShopName,
				Platform:            pl,
				ExternalProductID:   p.ExternalProductID,
				ExternalSKUID:       p.ExternalSKUID,
				PlatformStock:       p.PlatformStock,
				PlatformStockStatus: pst,
				LastSyncedAt:        p.LastSyncedAt,
			}
			if t, ok := taskByPub[p.PublicationSKUID]; ok {
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
			ProductSKUID:          row.ID,
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
			SKUBindStatus:       bindSt,
			PlatformSyncStatus:  aggregateSyncStatus(pubs, taskByPub, bindSt),
			LastDeductAt:        ptrTime(lastDeduct[row.ID]),
			ExceptionCount:      exCounts[row.ID],
			AffectedOrderCount:  orderCounts[row.ID],
		})
	}

	nextCursor := ""
	if q.UseCursor && hasMore && len(scans) > 0 {
		last := scans[len(scans)-1]
		nextCursor, err = pagination.BuildNextCursor(true, q.TenantID, cursorShopID, scopeHash, "updated_at", last.UpdatedAt, last.ID.String())
		if err != nil {
			return nil, err
		}
	}

	return &CenterListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
		Limit:      ps,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
