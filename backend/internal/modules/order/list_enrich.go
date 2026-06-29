package order

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ListSkuMatchAllMatched = "all_matched"
	ListSkuMatchPartial    = "partial"
	ListSkuMatchUnmatched  = "unmatched"
	ListSkuMatchAmbiguous  = "ambiguous"
	ListSkuMatchNone       = "none"

	ListInvDeductNone    = "none"
	ListInvDeductSuccess = "success"
	ListInvDeductFailed  = "failed"
	ListInvDeductPartial = "partial"
	ListInvDeductBlocked = "blocked"

	ListSyncManual  = "manual"
	ListSyncSynced  = "synced"
	ListSyncUnknown = "unknown"
)

type itemCountRow struct {
	OrderID uuid.UUID `gorm:"column:order_id"`
	Cnt     int       `gorm:"column:cnt"`
}

type skuAggRow struct {
	OrderID   uuid.UUID `gorm:"column:order_id"`
	Total     int       `gorm:"column:total"`
	Matched   int       `gorm:"column:matched"`
	Unmatched int       `gorm:"column:unmatched"`
	Ambiguous int       `gorm:"column:ambiguous"`
}

type invAggRow struct {
	OrderID      uuid.UUID `gorm:"column:order_id"`
	SuccessCnt   int       `gorm:"column:success_cnt"`
	FailedCnt    int       `gorm:"column:failed_cnt"`
	BlockedItems int       `gorm:"column:blocked_items"`
}

func enrichListRows(ctx context.Context, db *gorm.DB, rows []Order, out []ListOrderRow) {
	if db == nil || len(rows) == 0 || len(out) != len(rows) {
		return
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}

	itemCnt := map[uuid.UUID]int{}
	var icRows []itemCountRow
	_ = db.WithContext(ctx).Raw(`
		SELECT order_id, COUNT(*) AS cnt FROM order_items WHERE order_id IN ? GROUP BY order_id
	`, ids).Scan(&icRows).Error
	for _, r := range icRows {
		itemCnt[r.OrderID] = r.Cnt
	}

	skuAgg := map[uuid.UUID]skuAggRow{}
	if db.Migrator().HasTable(&OrderItemSKUMatch{}) {
		var sa []skuAggRow
		_ = db.WithContext(ctx).Raw(`
			SELECT order_id,
				COUNT(*) AS total,
				SUM(CASE WHEN match_status IN ('matched','manual_bound') THEN 1 ELSE 0 END) AS matched,
				SUM(CASE WHEN match_status = 'unmatched' THEN 1 ELSE 0 END) AS unmatched,
				SUM(CASE WHEN match_status = 'ambiguous' THEN 1 ELSE 0 END) AS ambiguous
			FROM order_item_sku_matches
			WHERE order_id IN ?
			GROUP BY order_id
		`, ids).Scan(&sa).Error
		for _, r := range sa {
			skuAgg[r.OrderID] = r
		}
	}

	invAgg := map[uuid.UUID]invAggRow{}
	if db.Migrator().HasTable("order_inventory_effects") {
		var ia []invAggRow
		_ = db.WithContext(ctx).Raw(`
			SELECT oie.order_id,
				SUM(CASE WHEN oie.effect_type = 'deduct' AND oie.status = 'success' THEN 1 ELSE 0 END) AS success_cnt,
				SUM(CASE WHEN oie.effect_type = 'deduct' AND oie.status = 'failed' THEN 1 ELSE 0 END) AS failed_cnt,
				0 AS blocked_items
			FROM order_inventory_effects oie
			WHERE oie.order_id IN ?
			GROUP BY oie.order_id
		`, ids).Scan(&ia).Error
		for _, r := range ia {
			invAgg[r.OrderID] = r
		}
	}

	for i, r := range rows {
		out[i].UpdatedAt = r.UpdatedAt
		out[i].ItemCount = itemCnt[r.ID]
		out[i].DetailURL = "/orders/" + r.ID.String()

		sa := skuAgg[r.ID]
		out[i].SkuMatchedCount = sa.Matched
		out[i].SkuTotalCount = sa.Total
		out[i].SkuMatchStatus = deriveSkuMatchStatus(sa, itemCnt[r.ID])

		ia := invAgg[r.ID]
		out[i].InventoryDeductStatus = deriveInvDeductStatus(ia, out[i].SkuMatchStatus)
		out[i].OpenExceptionCount = countOpenExceptions(sa, ia)
		out[i].SyncStatus = deriveSyncStatus(r)
	}
}

func deriveSkuMatchStatus(sa skuAggRow, itemCount int) string {
	if itemCount == 0 {
		return ListSkuMatchNone
	}
	if sa.Total == 0 {
		return ListSkuMatchNone
	}
	if sa.Ambiguous > 0 {
		return ListSkuMatchAmbiguous
	}
	if sa.Unmatched > 0 {
		return ListSkuMatchUnmatched
	}
	if sa.Matched >= sa.Total {
		return ListSkuMatchAllMatched
	}
	return ListSkuMatchPartial
}

func deriveInvDeductStatus(ia invAggRow, skuStatus string) string {
	if skuStatus == ListSkuMatchUnmatched || skuStatus == ListSkuMatchAmbiguous || skuStatus == ListSkuMatchNone {
		if ia.SuccessCnt == 0 && ia.FailedCnt == 0 {
			return ListInvDeductBlocked
		}
	}
	if ia.SuccessCnt > 0 && ia.FailedCnt > 0 {
		return ListInvDeductPartial
	}
	if ia.FailedCnt > 0 {
		return ListInvDeductFailed
	}
	if ia.SuccessCnt > 0 {
		return ListInvDeductSuccess
	}
	return ListInvDeductNone
}

func countOpenExceptions(sa skuAggRow, ia invAggRow) int {
	n := sa.Unmatched + sa.Ambiguous
	if ia.FailedCnt > 0 {
		n++
	}
	return n
}

func deriveSyncStatus(o Order) string {
	p := strings.TrimSpace(strings.ToLower(o.Platform))
	if p == "" || p == "manual" {
		return ListSyncManual
	}
	if o.ExternalOrderID != nil && strings.TrimSpace(*o.ExternalOrderID) != "" {
		return ListSyncSynced
	}
	return ListSyncUnknown
}

func applyListPostFilters(items []ListOrderRow, q ListQuery) []ListOrderRow {
	if q.SkuMatchStatus == "" && q.InventoryDeductStatus == "" && !q.HasException && q.SyncStatus == "" {
		return items
	}
	out := make([]ListOrderRow, 0, len(items))
	for _, r := range items {
		if q.SkuMatchStatus != "" && r.SkuMatchStatus != q.SkuMatchStatus {
			continue
		}
		if q.InventoryDeductStatus != "" && r.InventoryDeductStatus != q.InventoryDeductStatus {
			continue
		}
		if q.HasException && r.OpenExceptionCount <= 0 {
			continue
		}
		if q.SyncStatus != "" && r.SyncStatus != q.SyncStatus {
			continue
		}
		out = append(out, r)
	}
	return out
}
