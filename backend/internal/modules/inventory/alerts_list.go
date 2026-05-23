package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

type alertSKUScan struct {
	ID           uuid.UUID `gorm:"column:id"`
	ProductID    uuid.UUID `gorm:"column:product_id"`
	SKUCode      string    `gorm:"column:sku_code"`
	SKUName      string    `gorm:"column:sku_name"`
	Stock        *int      `gorm:"column:stock"`
	WarningStock int       `gorm:"column:warning_stock"`
	SafetyStock  int       `gorm:"column:safety_stock"`
	ProductTitle string    `gorm:"column:product_title"`
}

type pubJoinScan struct {
	PublicationSkuID  uuid.UUID  `gorm:"column:publication_sku_id"`
	ProductSkuID      *uuid.UUID `gorm:"column:product_sku_id"`
	ShopID            uuid.UUID  `gorm:"column:shop_id"`
	ShopName          string     `gorm:"column:shop_name"`
	Platform          string     `gorm:"column:platform"`
	ExternalProductID string     `gorm:"column:external_product_id"`
	ExternalSkuID     string     `gorm:"column:external_sku_id"`
	SKUCode           string     `gorm:"column:sku_code"`
	PlatformStock     *int       `gorm:"column:platform_stock"`
	LastSyncedAt      *time.Time `gorm:"column:last_synced_at"`
}

type latestTaskScan struct {
	PublicationSkuID uuid.UUID  `gorm:"column:publication_sku_id"`
	TaskID           uuid.UUID  `gorm:"column:id"`
	Status           string     `gorm:"column:status"`
	ErrorMessage     string     `gorm:"column:error_message"`
	FinishedAt       *time.Time `gorm:"column:finished_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
}

func clipErr(msg string, max int) string {
	msg = strings.TrimSpace(msg)
	if max <= 0 || len(msg) <= max {
		return msg
	}
	return msg[:max] + "…"
}

// skuAlertBaseQuery is the shared JOIN/WHERE scope for inventory alert-style SKU listings.
type skuAlertBaseQuery struct {
	Keyword       string
	ProductID     *uuid.UUID
	ProductSkuID  *uuid.UUID
	ProductSkuIDs []uuid.UUID
	Platform      string
	ShopID        *uuid.UUID
	StockStatus   string
	OnlyPublished bool
}

func (s *Service) buildAlertsBaseTX(ctx context.Context, q AlertsListQuery) *gorm.DB {
	return s.buildSKUAlertBaseTX(ctx, skuAlertBaseQuery{
		Keyword:       q.Keyword,
		ProductID:     q.ProductID,
		ProductSkuID:  q.ProductSkuID,
		ProductSkuIDs: nil,
		Platform:      q.Platform,
		ShopID:        q.ShopID,
		StockStatus:   q.StockStatus,
		OnlyPublished: q.OnlyPublished,
	})
}

func (s *Service) buildSKUAlertBaseTX(ctx context.Context, q skuAlertBaseQuery) *gorm.DB {
	tx := s.DB.WithContext(ctx).Table("product_skus AS sk").
		Select(`sk.id, sk.product_id, sk.sku_code, sk.sku_name, sk.stock, sk.warning_stock, sk.safety_stock, sk.updated_at, p.title AS product_title`).
		Joins("INNER JOIN products p ON p.id = sk.product_id AND p.deleted_at IS NULL")
	if pid := q.ProductID; pid != nil && *pid != uuid.Nil {
		tx = tx.Where("sk.product_id = ?", *pid)
	}
	if len(q.ProductSkuIDs) > 0 {
		tx = tx.Where("sk.id IN ?", q.ProductSkuIDs)
	} else if sid := q.ProductSkuID; sid != nil && *sid != uuid.Nil {
		tx = tx.Where("sk.id = ?", *sid)
	}
	kw := strings.TrimSpace(q.Keyword)
	if kw != "" {
		like := "%" + strings.ToLower(kw) + "%"
		tx = tx.Where("LOWER(p.title) LIKE ? OR LOWER(sk.sku_code) LIKE ? OR LOWER(sk.sku_name) LIKE ?", like, like, like)
	}
	pl := strings.TrimSpace(strings.ToLower(q.Platform))
	if pl != "" {
		tx = tx.Joins(`INNER JOIN product_publication_skus pps_pf ON pps_pf.product_sku_id = sk.id`).
			Joins(`INNER JOIN product_publications pp_pf ON pp_pf.id = pps_pf.publication_id AND pp_pf.deleted_at IS NULL`).
			Where("LOWER(pp_pf.platform) = ?", pl)
	}
	if shop := q.ShopID; shop != nil && *shop != uuid.Nil {
		tx = tx.Joins(`INNER JOIN product_publication_skus pps_shop ON pps_shop.product_sku_id = sk.id`).
			Joins(`INNER JOIN product_publications pp_shop ON pp_shop.id = pps_shop.publication_id AND pp_shop.deleted_at IS NULL AND pp_shop.shop_id = ?`, *shop)
	}
	if q.OnlyPublished {
		tx = tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus ppsx
			INNER JOIN product_publications ppx ON ppx.id = ppsx.publication_id AND ppx.deleted_at IS NULL
			WHERE ppsx.product_sku_id = sk.id
		)`)
	}
	switch strings.TrimSpace(q.StockStatus) {
	case product.StockStatusOutOfStock:
		tx = tx.Where("COALESCE(sk.stock,0) <= 0")
	case product.StockStatusBelowSafetyStock:
		tx = tx.Where("sk.safety_stock > 0 AND COALESCE(sk.stock,0) > 0 AND COALESCE(sk.stock,0) <= sk.safety_stock")
	case product.StockStatusLowStock:
		tx = tx.Where(`COALESCE(sk.stock,0) > 0
			AND (sk.safety_stock = 0 OR COALESCE(sk.stock,0) > sk.safety_stock)
			AND COALESCE(sk.stock,0) <= sk.warning_stock`)
	case product.StockStatusNormal:
		tx = tx.Where("COALESCE(sk.stock,0) > sk.warning_stock")
	}
	return tx
}

// applyAlertsSQLAlertTypesOR restricts to SKUs matching any of the given alert dimensions (OR).
func (s *Service) applyAlertsSQLAlertTypesOR(tx *gorm.DB, alertTypes []string, th int) *gorm.DB {
	if len(alertTypes) == 0 {
		return tx
	}
	parts := make([]string, 0, len(alertTypes))
	args := make([]any, 0, 4)
	for _, raw := range alertTypes {
		at := strings.TrimSpace(strings.ToLower(raw))
		if at == "" {
			continue
		}
		switch at {
		case AlertTypeOutOfStock:
			parts = append(parts, "COALESCE(sk.stock,0) <= 0")
		case AlertTypeLowStock:
			parts = append(parts, `(COALESCE(sk.stock,0) > 0
			AND (sk.safety_stock = 0 OR COALESCE(sk.stock,0) > sk.safety_stock)
			AND COALESCE(sk.stock,0) <= sk.warning_stock)`)
		case AlertTypeBelowSafetyStock:
			parts = append(parts, "sk.safety_stock > 0 AND COALESCE(sk.stock,0) > 0 AND COALESCE(sk.stock,0) <= sk.safety_stock")
		case AlertTypePlatformStockUnknown:
			parts = append(parts, `EXISTS (
			SELECT 1 FROM product_publication_skus ppsu
			INNER JOIN product_publications ppu ON ppu.id = ppsu.publication_id AND ppu.deleted_at IS NULL
			WHERE ppsu.product_sku_id = sk.id AND ppsu.stock IS NULL
		)`)
		case AlertTypePlatformStockMismatch:
			parts = append(parts, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM product_publication_skus ppsm
			INNER JOIN product_publications ppm ON ppm.id = ppsm.publication_id AND ppm.deleted_at IS NULL
			WHERE ppsm.product_sku_id = sk.id AND ppsm.stock IS NOT NULL
				AND ABS(COALESCE(sk.stock,0) - ppsm.stock) > %d
		)`, th))
		case AlertTypeInventorySyncFailed:
			parts = append(parts, `EXISTS (
			SELECT 1 FROM product_publication_skus ppsf
			INNER JOIN inventory_sync_tasks tf ON tf.publication_sku_id = ppsf.id
			WHERE ppsf.product_sku_id = sk.id
				AND tf.status = ?
				AND NOT EXISTS (
					SELECT 1 FROM inventory_sync_tasks tf2
					WHERE tf2.publication_sku_id = ppsf.id AND tf2.created_at > tf.created_at
				)
		)`)
			args = append(args, StatusFailed)
		default:
			continue
		}
	}
	if len(parts) == 0 {
		return tx
	}
	return tx.Where("("+strings.Join(parts, " OR ")+")", args...)
}

func (s *Service) applyAlertsSQLAlertType(tx *gorm.DB, alertType string, th int) *gorm.DB {
	at := strings.TrimSpace(strings.ToLower(alertType))
	switch at {
	case AlertTypeOutOfStock:
		return tx.Where("COALESCE(sk.stock,0) <= 0")
	case AlertTypeLowStock:
		return tx.Where(`COALESCE(sk.stock,0) > 0
			AND (sk.safety_stock = 0 OR COALESCE(sk.stock,0) > sk.safety_stock)
			AND COALESCE(sk.stock,0) <= sk.warning_stock`)
	case AlertTypeBelowSafetyStock:
		return tx.Where("sk.safety_stock > 0 AND COALESCE(sk.stock,0) > 0 AND COALESCE(sk.stock,0) <= sk.safety_stock")
	case AlertTypePlatformStockUnknown:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus ppsu
			INNER JOIN product_publications ppu ON ppu.id = ppsu.publication_id AND ppu.deleted_at IS NULL
			WHERE ppsu.product_sku_id = sk.id AND ppsu.stock IS NULL
		)`)
	case AlertTypePlatformStockMismatch:
		return tx.Where(fmt.Sprintf(`EXISTS (
			SELECT 1 FROM product_publication_skus ppsm
			INNER JOIN product_publications ppm ON ppm.id = ppsm.publication_id AND ppm.deleted_at IS NULL
			WHERE ppsm.product_sku_id = sk.id AND ppsm.stock IS NOT NULL
				AND ABS(COALESCE(sk.stock,0) - ppsm.stock) > %d
		)`, th))
	case AlertTypeInventorySyncFailed:
		return tx.Where(`EXISTS (
			SELECT 1 FROM product_publication_skus ppsf
			INNER JOIN inventory_sync_tasks tf ON tf.publication_sku_id = ppsf.id
			WHERE ppsf.product_sku_id = sk.id
				AND tf.status = ?
				AND NOT EXISTS (
					SELECT 1 FROM inventory_sync_tasks tf2
					WHERE tf2.publication_sku_id = ppsf.id AND tf2.created_at > tf.created_at
				)
		)`, StatusFailed)
	default:
		return tx
	}
}

func (s *Service) applyNonNormalAlertScope(tx *gorm.DB, pol inventoryAlertPolicy, mismatchTh int) *gorm.DB {
	local := "(FALSE)"
	if pol.EnableInventoryAlerts {
		local = "COALESCE(sk.stock, 0) <= sk.warning_stock"
	}
	platMismatch := "FALSE"
	if pol.AlertPlatformStockMismatch {
		platMismatch = fmt.Sprintf("(pps0.stock IS NOT NULL AND ABS(COALESCE(sk.stock,0) - pps0.stock) > %d)", mismatchTh)
	}
	q := `
(
  (` + local + `)
  OR EXISTS (
    SELECT 1 FROM product_publication_skus pps0
    INNER JOIN product_publications pp0 ON pp0.id = pps0.publication_id AND pp0.deleted_at IS NULL
    WHERE pps0.product_sku_id = sk.id
    AND (
      pps0.stock IS NULL
      OR (` + platMismatch + `)
      OR EXISTS (
        SELECT 1 FROM inventory_sync_tasks t0
        WHERE t0.publication_sku_id = pps0.id
          AND t0.status = ?
          AND NOT EXISTS (
            SELECT 1 FROM inventory_sync_tasks t1
            WHERE t1.publication_sku_id = pps0.id
              AND t1.created_at > t0.created_at
          )
      )
    )
  )
)`
	return tx.Where(q, StatusFailed)
}

func (s *Service) loadLatestTasksByPubSku(ctx context.Context, pubIDs []uuid.UUID) map[uuid.UUID]latestTaskScan {
	out := map[uuid.UUID]latestTaskScan{}
	if len(pubIDs) == 0 {
		return out
	}
	var rows []latestTaskScan
	_ = s.DB.WithContext(ctx).Raw(`
SELECT DISTINCT ON (publication_sku_id)
  publication_sku_id, id, status, error_message, finished_at, created_at
FROM inventory_sync_tasks
WHERE publication_sku_id IN ?
ORDER BY publication_sku_id, created_at DESC
`, pubIDs).Scan(&rows).Error
	for _, r := range rows {
		out[r.PublicationSkuID] = r
	}
	return out
}

func (s *Service) loadMaxLogTimeBySku(ctx context.Context, skuIDs []uuid.UUID) map[uuid.UUID]time.Time {
	out := map[uuid.UUID]time.Time{}
	if len(skuIDs) == 0 || s == nil || s.DB == nil {
		return out
	}
	if !s.DB.Migrator().HasColumn(&InventoryChangeLog{}, "product_sku_id") {
		return out
	}
	type row struct {
		SK uuid.UUID `gorm:"column:product_sku_id"`
		Tm time.Time `gorm:"column:tm"`
	}
	var rows []row
	_ = s.DB.WithContext(ctx).Model(&InventoryChangeLog{}).
		Select("product_sku_id, MAX(created_at) AS tm").
		Where("product_sku_id IN ?", skuIDs).
		Group("product_sku_id").
		Scan(&rows).Error
	for _, r := range rows {
		out[r.SK] = r.Tm
	}
	return out
}

func platformLineStatus(localStock int, platformStock *int, mismatchTh int, mismatchEnabled bool) string {
	if platformStock == nil {
		return PlatformStockUnknown
	}
	if !mismatchEnabled {
		return PlatformStockSynced
	}
	d := localStock - *platformStock
	if d < 0 {
		d = -d
	}
	if d > mismatchTh {
		return PlatformStockMismatch
	}
	return PlatformStockSynced
}

func appendUnique(slice []string, v string) []string {
	for _, x := range slice {
		if x == v {
			return slice
		}
	}
	return append(slice, v)
}

// ListInventoryAlerts pages SKU rows with stock / platform / sync alert context.
func (s *Service) ListInventoryAlerts(ctx context.Context, q AlertsListQuery) (*AlertsListResult, error) {
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

	base := s.buildAlertsBaseTX(ctx, q)
	if strings.TrimSpace(q.AlertType) != "" {
		base = s.applyAlertsSQLAlertType(base, q.AlertType, th)
	} else if !q.IncludeNormal {
		base = s.applyNonNormalAlertScope(base, pol, th)
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
				ps.external_sku_id, pp.shop_id, sh.shop_name, pp.platform, pp.external_product_id, pp.last_synced_at`).
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

	items := make([]InventoryAlertEntry, 0, len(scans))
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

		stocks := make([]PlatformStockAlertEntry, 0, len(pubBySku[row.ID]))
		var worstFail *latestTaskScan
		for _, p := range pubBySku[row.ID] {
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

		entry := InventoryAlertEntry{
			ProductID:             row.ProductID,
			ProductTitle:          row.ProductTitle,
			ProductSkuID:          row.ID,
			SKUCode:               row.SKUCode,
			SKUName:               row.SKUName,
			Stock:                 derefStock(row.Stock),
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
			entry.LastSyncTaskID = &tid
			entry.LastSyncStatus = worstFail.Status
			entry.LastSyncError = clipErr(worstFail.ErrorMessage, 520)
			ca := worstFail.CreatedAt
			entry.LastSyncAt = &ca
		}
		if q.IncludeNormal || len(entry.AlertTypes) > 0 {
			items = append(items, entry)
		}
	}

	return &AlertsListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pagesOf(total, ps),
	}, nil
}

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
