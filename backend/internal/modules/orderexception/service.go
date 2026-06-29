package orderexception

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

// Service aggregates cross-table order exceptions for the admin workbench.
type Service struct {
	DB     *gorm.DB
	Orders *order.Service
	Inv    *inventory.Service
}

type aggRow struct {
	exceptionType   string
	severity        string
	sourceType      string
	sourceID        uuid.UUID
	orderID         uuid.UUID
	orderItemID     *uuid.UUID
	platform        string
	shopID          *uuid.UUID
	orderNo         string
	externalOrderID string
	externalItemID  string
	externalSkuID   string
	skuCode         string
	skuName         string
	productID       string
	productSkuID    string
	productTitle    string
	localSkuCode    string
	quantity        int
	errorMessage    string
	suggestedAction string
	createdAt       time.Time
	updatedAt       time.Time
}

type markPair struct {
	handled bool
	ignored bool
}

func buildMarkIndex(rows []OrderExceptionMark) map[string]markPair {
	out := map[string]markPair{}
	for _, r := range rows {
		k := markKey(r.ExceptionType, r.SourceType, r.SourceID)
		mp := out[k]
		if r.MarkType == MarkHandled {
			mp.handled = true
		}
		if r.MarkType == MarkIgnored {
			mp.ignored = true
		}
		out[k] = mp
	}
	return out
}

func markKey(exceptionType, sourceType, sourceID string) string {
	return strings.TrimSpace(exceptionType) + "|" + strings.TrimSpace(sourceType) + "|" + strings.TrimSpace(sourceID)
}

func appendUniqueAgg(dst *[]aggRow, r aggRow) {
	key := r.exceptionType + "|" + r.sourceType + "|" + r.sourceID.String()
	for _, x := range *dst {
		if x.exceptionType+"|"+x.sourceType+"|"+x.sourceID.String() == key {
			return
		}
	}
	*dst = append(*dst, r)
}

func (s *Service) shopName(ctx context.Context, sid *uuid.UUID) string {
	if s == nil || s.DB == nil || sid == nil || *sid == uuid.Nil {
		return ""
	}
	var row shop.Shop
	if err := s.DB.WithContext(ctx).Select("shop_name").First(&row, "id = ?", *sid).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(row.ShopName)
}

// ListOrderExceptions aggregates exceptions from local tables (no platform HTTP).
func (s *Service) ListOrderExceptions(ctx context.Context, req ListOrderExceptionsRequest) (*ListOrderExceptionsResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("orderexception: unavailable")
	}
	page := req.Page
	if page < 1 {
		page = 1
	}
	ps := req.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}

	var markRows []OrderExceptionMark
	if err := s.DB.WithContext(ctx).Find(&markRows).Error; err != nil {
		return nil, err
	}
	marks := buildMarkIndex(markRows)

	var rows []aggRow
	if req.ExceptionType == "" || req.ExceptionType == TypeSKUUnmatched {
		if xs, err := s.collectSKUUnmatched(ctx, req); err == nil {
			for _, x := range xs {
				appendUniqueAgg(&rows, x)
			}
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeSKUAmbiguous {
		if xs, err := s.collectSKUAmbiguous(ctx, req); err == nil {
			for _, x := range xs {
				appendUniqueAgg(&rows, x)
			}
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeInsufficientStock || req.ExceptionType == TypeInventoryDeductFailed {
		if xs, err := s.collectInventoryEffects(ctx, req, inventory.EffectTypeDeduct); err == nil {
			for _, x := range xs {
				appendUniqueAgg(&rows, x)
			}
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeInventoryRestoreFailed {
		if xs, err := s.collectInventoryEffects(ctx, req, inventory.EffectTypeRestore); err == nil {
			for _, x := range xs {
				appendUniqueAgg(&rows, x)
			}
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeInventorySyncFailed {
		if xs, err := s.collectInventorySyncFailed(ctx, req); err == nil {
			for _, x := range xs {
				appendUniqueAgg(&rows, x)
			}
		}
	}
	if req.ExceptionType == "" || req.ExceptionType == TypeOrderSyncPartialFailed {
		if xs, err := s.collectOrderSyncPartialFailed(ctx, req); err == nil {
			for _, x := range xs {
				appendUniqueAgg(&rows, x)
			}
		}
	}

	sum := ExceptionSummaryDTO{}
	for _, r := range rows {
		mp := marks[markKey(r.exceptionType, r.sourceType, r.sourceID.String())]
		if mp.handled || mp.ignored {
			continue
		}
		sum.TotalOpen++
		switch r.exceptionType {
		case TypeSKUUnmatched:
			sum.SKUUnmatched++
		case TypeSKUAmbiguous:
			sum.SKUAmbiguous++
		case TypeInsufficientStock:
			sum.InsufficientStock++
		case TypeInventoryDeductFailed:
			sum.InventoryDeductFailed++
		case TypeInventoryRestoreFailed:
			sum.InventoryRestoreFailed++
		case TypeInventorySyncFailed:
			sum.InventorySyncFailed++
		case TypeOrderSyncPartialFailed:
			sum.OrderSyncPartial++
		}
	}

	filtered := filterAggRows(rows, marks, req)
	sortAggRows(filtered)

	total := int64(len(filtered))
	start := (page - 1) * ps
	end := start + ps
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	pageRows := filtered[start:end]

	out := make([]OrderExceptionDTO, 0, len(pageRows))
	for _, r := range pageRows {
		dto := exceptionToDTO(ctx, s, r)
		mp := marks[markKey(r.exceptionType, r.sourceType, r.sourceID.String())]
		dto.Handled = mp.handled
		dto.Ignored = mp.ignored
		switch {
		case mp.handled:
			dto.Status = StatusHandled
		case mp.ignored:
			dto.Status = StatusIgnored
		default:
			dto.Status = StatusOpen
		}
		out = append(out, dto)
	}

	return &ListOrderExceptionsResult{
		List:    out,
		Total:   total,
		Summary: sum,
	}, nil
}

func filterAggRows(rows []aggRow, marks map[string]markPair, req ListOrderExceptionsRequest) []aggRow {
	var out []aggRow
	kw := strings.ToLower(strings.TrimSpace(req.Keyword))

	showHandled := req.Handled != nil && *req.Handled
	showIgnored := req.Ignored != nil && *req.Ignored
	defaultOpen := !(showHandled || showIgnored)

	for _, r := range rows {
		mp := marks[markKey(r.exceptionType, r.sourceType, r.sourceID.String())]
		if defaultOpen && (mp.handled || mp.ignored) {
			continue
		}
		if showHandled && !mp.handled {
			continue
		}
		if showIgnored && !mp.ignored {
			continue
		}

		if req.Severity != "" && !strings.EqualFold(req.Severity, r.severity) {
			continue
		}
		if req.Platform != "" && !strings.EqualFold(req.Platform, r.platform) {
			continue
		}
		if req.ShopID != "" {
			sid, err := uuid.Parse(strings.TrimSpace(req.ShopID))
			if err != nil || r.shopID == nil || *r.shopID != sid {
				continue
			}
		}
		if req.OrderID != "" {
			oid, err := uuid.Parse(strings.TrimSpace(req.OrderID))
			if err != nil || r.orderID != oid {
				continue
			}
		}
		if req.ExceptionType != "" && !strings.EqualFold(req.ExceptionType, r.exceptionType) {
			continue
		}
		if kw != "" {
			hay := strings.ToLower(strings.Join([]string{
				r.orderNo, r.externalOrderID, r.externalItemID, r.externalSkuID,
				r.skuCode, r.productTitle, r.localSkuCode, r.errorMessage,
			}, " "))
			if !strings.Contains(hay, kw) {
				continue
			}
		}
		if req.Start != nil && r.createdAt.Before(*req.Start) {
			continue
		}
		if req.End != nil && r.createdAt.After(*req.End) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func sortAggRows(rows []aggRow) {
	for i := 1; i < len(rows); i++ {
		j := i
		for j > 0 && rows[j-1].updatedAt.Before(rows[j].updatedAt) {
			rows[j-1], rows[j] = rows[j], rows[j-1]
			j--
		}
	}
}

func exceptionToDTO(ctx context.Context, s *Service, r aggRow) OrderExceptionDTO {
	d := OrderExceptionDTO{
		ID:              r.sourceID.String(),
		ExceptionType:   r.exceptionType,
		Severity:        r.severity,
		SourceType:      r.sourceType,
		SourceID:        r.sourceID.String(),
		OrderID:         "",
		OrderNo:         r.orderNo,
		ExternalOrderID: r.externalOrderID,
		Platform:        r.platform,
		ShopID:          "",
		ShopName:        "",
		OrderItemID:     "",
		ExternalItemID:  r.externalItemID,
		ExternalSkuID:   r.externalSkuID,
		SKUCode:         r.skuCode,
		SKUName:         r.skuName,
		ProductID:       r.productID,
		ProductSkuID:    r.productSkuID,
		ProductTitle:    r.productTitle,
		LocalSkuCode:    r.localSkuCode,
		Quantity:        r.quantity,
		ErrorMessage:    r.errorMessage,
		SuggestedAction: r.suggestedAction,
		CreatedAt:       r.createdAt,
		UpdatedAt:       r.updatedAt,
		DetailURL:       fmt.Sprintf("/orders/exceptions/%s/%s", r.sourceType, r.sourceID.String()),
	}
	if r.orderID != uuid.Nil {
		d.OrderID = r.orderID.String()
		d.OrderURL = "/orders/" + r.orderID.String()
	}
	if r.sourceType == SourceOrderSyncTask {
		d.SyncTaskID = r.sourceID.String()
		d.TaskCenterURL = "/ops/task-center/failures?taskType=order_sync&keyword=" + r.sourceID.String()
		d.DetailURL = "/orders/sync-tasks?id=" + r.sourceID.String()
	} else if r.sourceType == SourceInventorySyncTask {
		d.SyncTaskID = r.sourceID.String()
		d.TaskCenterURL = "/ops/task-center/failures?taskType=inventory_sync&jumpId=" + r.sourceID.String()
		d.DetailURL = "/inventory/sync-tasks?id=" + r.sourceID.String()
		if r.productSkuID != "" {
			d.InventoryURL = "/inventory?productSkuId=" + r.productSkuID
		}
	} else if r.sourceType == SourceOrderInventoryEffect {
		d.DetailURL = "/inventory/deductions?orderId=" + r.orderID.String()
		d.TaskCenterURL = "/ops/task-center/failures?taskType=inventory_sync"
		if r.productSkuID != "" {
			d.InventoryURL = "/inventory?productSkuId=" + r.productSkuID
		}
	} else if r.orderID != uuid.Nil {
		d.DetailURL = "/orders/" + r.orderID.String()
		if r.orderItemID != nil {
			d.DetailURL += "?itemId=" + r.orderItemID.String()
		}
	}
	if r.shopID != nil && *r.shopID != uuid.Nil {
		d.ShopID = r.shopID.String()
		d.ShopName = s.shopName(ctx, r.shopID)
	}
	if r.orderItemID != nil {
		d.OrderItemID = r.orderItemID.String()
	}
	return d
}

func (s *Service) collectSKUUnmatched(ctx context.Context, req ListOrderExceptionsRequest) ([]aggRow, error) {
	if s == nil || s.DB == nil {
		return nil, nil
	}
	orderItem := &order.OrderItem{}
	if !s.DB.Migrator().HasTable(orderItem) {
		return nil, nil
	}
	if !s.DB.Migrator().HasColumn(orderItem, "product_sku_id") || !s.DB.Migrator().HasColumn(orderItem, "external_sku_id") {
		return nil, nil
	}
	type hit struct {
		MatchID      *uuid.UUID `gorm:"column:match_id"`
		OrderItemID  uuid.UUID  `gorm:"column:order_item_id"`
		OrderID      uuid.UUID  `gorm:"column:order_id"`
		Platform     string     `gorm:"column:platform"`
		ShopID       *uuid.UUID `gorm:"column:shop_id"`
		OrderNo      string     `gorm:"column:order_no"`
		ExternalOID  *string    `gorm:"column:external_order_id"`
		ExternalItem *string    `gorm:"column:external_item_id"`
		ExternalSku  *string    `gorm:"column:external_sku_id"`
		SKUCodeCol   string     `gorm:"column:sku_code"`
		SellerSKU    string     `gorm:"column:seller_sku"`
		ProductTitle string     `gorm:"column:product_title"`
		Quantity     int        `gorm:"column:quantity"`
		MCreated     time.Time  `gorm:"column:m_created"`
		MUpdated     time.Time  `gorm:"column:m_updated"`
	}
	q := `
SELECT
  m.id AS match_id,
  oi.id AS order_item_id,
  o.id AS order_id,
  o.platform AS platform,
  o.shop_id AS shop_id,
  o.order_no AS order_no,
  o.external_order_id AS external_order_id,
  oi.external_item_id AS external_item_id,
  oi.external_sku_id AS external_sku_id,
  oi.sku_code AS sku_code,
  oi.seller_sku AS seller_sku,
  oi.product_title AS product_title,
  oi.quantity AS quantity,
  COALESCE(m.created_at, oi.created_at) AS m_created,
  COALESCE(m.updated_at, oi.updated_at) AS m_updated
FROM order_items oi
JOIN orders o ON o.id = oi.order_id AND o.deleted_at IS NULL
LEFT JOIN order_item_sku_matches m ON m.order_item_id = oi.id
WHERE LOWER(TRIM(o.platform)) NOT IN ('', 'manual')
  AND (
    (oi.product_sku_id IS NULL OR oi.product_sku_id = '00000000-0000-0000-0000-000000000000')
    OR m.match_status IN ('unmatched','skipped')
  )
  AND (m.id IS NULL OR m.match_status <> 'ambiguous')
`
	args := []any{}
	if req.Platform != "" {
		q += ` AND LOWER(o.platform) = ?`
		args = append(args, strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			q += ` AND o.shop_id = ?`
			args = append(args, sid)
		}
	}
	if req.OrderID != "" {
		if oid, err := uuid.Parse(strings.TrimSpace(req.OrderID)); err == nil {
			q += ` AND o.id = ?`
			args = append(args, oid)
		}
	}

	var hits []hit
	if err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&hits).Error; err != nil {
		return nil, err
	}
	out := make([]aggRow, 0, len(hits))
	for _, h := range hits {
		srcType := SourceOrderItemSKUMatch
		var srcID uuid.UUID
		if h.MatchID != nil && *h.MatchID != uuid.Nil {
			srcID = *h.MatchID
		} else {
			srcType = SourceOrderItem
			srcID = h.OrderItemID
		}
		extOid := ""
		if h.ExternalOID != nil {
			extOid = strings.TrimSpace(*h.ExternalOID)
		}
		sc := strings.TrimSpace(h.SKUCodeCol)
		if sc == "" {
			sc = strings.TrimSpace(h.SellerSKU)
		}
		out = append(out, aggRow{
			exceptionType:   TypeSKUUnmatched,
			severity:        SeverityHigh,
			sourceType:      srcType,
			sourceID:        srcID,
			orderID:         h.OrderID,
			orderItemID:     &h.OrderItemID,
			platform:        strings.TrimSpace(h.Platform),
			shopID:          h.ShopID,
			orderNo:         strings.TrimSpace(h.OrderNo),
			externalOrderID: extOid,
			externalItemID:  derefStr(h.ExternalItem),
			externalSkuID:   derefStr(h.ExternalSku),
			skuCode:         sc,
			productTitle:    strings.TrimSpace(h.ProductTitle),
			quantity:        h.Quantity,
			suggestedAction: "请人工绑定本地 SKU，绑定后可重新扣减库存。",
			createdAt:       h.MCreated,
			updatedAt:       h.MUpdated,
		})
	}
	return out, nil
}

func (s *Service) collectSKUAmbiguous(ctx context.Context, req ListOrderExceptionsRequest) ([]aggRow, error) {
	var matches []order.OrderItemSKUMatch
	tx := s.DB.WithContext(ctx).Model(&order.OrderItemSKUMatch{}).
		Joins("JOIN orders o ON o.id = order_item_sku_matches.order_id AND o.deleted_at IS NULL").
		Where("order_item_sku_matches.match_status = ?", order.MatchStatusAmbiguous)
	if req.Platform != "" {
		tx = tx.Where("LOWER(order_item_sku_matches.platform) = ?", strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			tx = tx.Where("o.shop_id = ?", sid)
		}
	}
	if req.OrderID != "" {
		if oid, err := uuid.Parse(strings.TrimSpace(req.OrderID)); err == nil {
			tx = tx.Where("order_item_sku_matches.order_id = ?", oid)
		}
	}
	if err := tx.Find(&matches).Error; err != nil {
		return nil, err
	}
	out := make([]aggRow, 0, len(matches))
	for _, m := range matches {
		var o order.Order
		_ = s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", m.OrderID).Error
		var oi order.OrderItem
		_ = s.DB.WithContext(ctx).First(&oi, "id = ?", m.OrderItemID).Error
		extOid := ""
		if o.ExternalOrderID != nil {
			extOid = strings.TrimSpace(*o.ExternalOrderID)
		}
		sc := strings.TrimSpace(m.SKUCode)
		if sc == "" {
			sc = strings.TrimSpace(m.SellerSKU)
		}
		oiid := m.OrderItemID
		out = append(out, aggRow{
			exceptionType:   TypeSKUAmbiguous,
			severity:        SeverityMedium,
			sourceType:      SourceOrderItemSKUMatch,
			sourceID:        m.ID,
			orderID:         m.OrderID,
			orderItemID:     &oiid,
			platform:        strings.TrimSpace(m.Platform),
			shopID:          o.ShopID,
			orderNo:         strings.TrimSpace(o.OrderNo),
			externalOrderID: extOid,
			externalItemID:  derefStr(m.ExternalItemID),
			externalSkuID:   derefStr(m.ExternalSKUID),
			skuCode:         sc,
			productTitle:    strings.TrimSpace(oi.ProductTitle),
			quantity:        oi.Quantity,
			suggestedAction: "请确认候选 SKU 后人工绑定。",
			createdAt:       m.CreatedAt,
			updatedAt:       m.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) collectInventoryEffects(ctx context.Context, req ListOrderExceptionsRequest, effectType string) ([]aggRow, error) {
	var effects []inventory.OrderInventoryEffect
	tx := s.DB.WithContext(ctx).Model(&inventory.OrderInventoryEffect{}).
		Where("effect_type = ? AND status = ?", effectType, inventory.InventoryEffectFailed)
	if req.OrderID != "" {
		if oid, err := uuid.Parse(strings.TrimSpace(req.OrderID)); err == nil {
			tx = tx.Where("order_id = ?", oid)
		}
	}
	if err := tx.Find(&effects).Error; err != nil {
		return nil, err
	}
	out := make([]aggRow, 0, len(effects))
	for _, e := range effects {
		var o order.Order
		if err := s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", e.OrderID).Error; err != nil {
			continue
		}
		if req.Platform != "" && !strings.EqualFold(req.Platform, o.Platform) {
			continue
		}
		if req.ShopID != "" {
			sid, err := uuid.Parse(strings.TrimSpace(req.ShopID))
			if err != nil || o.ShopID == nil || *o.ShopID != sid {
				continue
			}
		}
		var oi order.OrderItem
		_ = s.DB.WithContext(ctx).First(&oi, "id = ?", e.OrderItemID).Error
		extOid := ""
		if o.ExternalOrderID != nil {
			extOid = strings.TrimSpace(*o.ExternalOrderID)
		}
		exType := TypeInventoryDeductFailed
		sev := SeverityHigh
		suggest := "请检查本地库存，调整库存后重新扣减。"
		msg := strings.TrimSpace(e.ErrorMessage)
		if effectType == inventory.EffectTypeRestore {
			exType = TypeInventoryRestoreFailed
			sev = SeverityMedium
			suggest = "请核对库存流水与订单状态，必要时人工恢复或重试。"
		} else if strings.Contains(strings.ToLower(msg), "insufficient") {
			exType = TypeInsufficientStock
		}
		psku := ""
		ar := aggRow{
			exceptionType:   exType,
			severity:        sev,
			sourceType:      SourceOrderInventoryEffect,
			sourceID:        e.ID,
			orderID:         e.OrderID,
			orderItemID:     &e.OrderItemID,
			platform:        strings.TrimSpace(o.Platform),
			shopID:          o.ShopID,
			orderNo:         strings.TrimSpace(o.OrderNo),
			externalOrderID: extOid,
			externalItemID:  derefStr(oi.ExternalItemID),
			externalSkuID:   derefStr(oi.ExternalSKUID),
			skuCode:         strings.TrimSpace(oi.SKUCode),
			productTitle:    strings.TrimSpace(oi.ProductTitle),
			quantity:        e.Quantity,
			errorMessage:    clampExcMsg(msg),
			suggestedAction: suggest,
			createdAt:       e.CreatedAt,
			updatedAt:       e.UpdatedAt,
		}
		if e.ProductSKUID != uuid.Nil && e.ProductSKUID != inventory.NilInventorySKUUID {
			psku = e.ProductSKUID.String()
			ar.productSkuID = psku
			var loc product.ProductSKU
			if err := s.DB.WithContext(ctx).First(&loc, "id = ? AND deleted_at IS NULL", e.ProductSKUID).Error; err == nil {
				ar.localSkuCode = strings.TrimSpace(loc.SKUCode)
				ar.productID = loc.ProductID.String()
			}
		}
		out = append(out, ar)
	}
	return out, nil
}

func (s *Service) collectInventorySyncFailed(ctx context.Context, req ListOrderExceptionsRequest) ([]aggRow, error) {
	if s == nil || s.DB == nil {
		return nil, nil
	}
	dst := &inventory.InventorySyncTask{}
	if !s.DB.Migrator().HasTable(dst) {
		return nil, nil
	}

	var tasks []inventory.InventorySyncTask
	tx := s.DB.WithContext(ctx).Model(dst).Where("status = ?", inventory.StatusFailed)

	// When SKU linkage columns exist, only surface failures tied to successful order deducts.
	hasTaskSKU := s.DB.Migrator().HasColumn(dst, "product_sku_id")
	hasEffectSKU := s.DB.Migrator().HasTable(&inventory.OrderInventoryEffect{}) &&
		s.DB.Migrator().HasColumn(&inventory.OrderInventoryEffect{}, "product_sku_id")
	if hasTaskSKU && hasEffectSKU {
		tx = tx.Where("product_sku_id IS NOT NULL").Where(`EXISTS (
			SELECT 1 FROM order_inventory_effects oie
			WHERE oie.product_sku_id = inventory_sync_tasks.product_sku_id
			  AND oie.effect_type = ?
			  AND oie.status = ?
		)`, inventory.EffectTypeDeduct, inventory.InventoryEffectSuccess)
	}

	if req.Platform != "" {
		tx = tx.Where("LOWER(platform) = ?", strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			tx = tx.Where("shop_id = ?", sid)
		}
	}
	if err := tx.Find(&tasks).Error; err != nil {
		return nil, err
	}
	out := make([]aggRow, 0, len(tasks))
	for _, t := range tasks {
		pid := t.ProductID.String()
		psku := ""
		if t.ProductSKUID != nil {
			psku = t.ProductSKUID.String()
		}
		ptitle := ""
		lcode := ""
		if t.ProductSKUID != nil {
			var loc product.ProductSKU
			if err := s.DB.WithContext(ctx).First(&loc, "id = ? AND deleted_at IS NULL", *t.ProductSKUID).Error; err == nil {
				lcode = strings.TrimSpace(loc.SKUCode)
				var pr product.Product
				if err := s.DB.WithContext(ctx).First(&pr, "id = ? AND deleted_at IS NULL", loc.ProductID).Error; err == nil {
					ptitle = strings.TrimSpace(pr.Title)
				}
			}
		}
		out = append(out, aggRow{
			exceptionType:   TypeInventorySyncFailed,
			severity:        SeverityMedium,
			sourceType:      SourceInventorySyncTask,
			sourceID:        t.ID,
			orderID:         uuid.Nil,
			orderItemID:     nil,
			platform:        strings.TrimSpace(t.Platform),
			shopID:          &t.ShopID,
			productID:       pid,
			productSkuID:    psku,
			productTitle:    ptitle,
			localSkuCode:    lcode,
			errorMessage:    clampExcMsg(t.ErrorMessage),
			suggestedAction: "请检查平台库存同步配置或在库存同步任务页重试。",
			createdAt:       t.CreatedAt,
			updatedAt:       t.UpdatedAt,
		})
	}
	return out, nil
}

func clampExcMsg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 520 {
		return s[:520] + "…"
	}
	return s
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// GetOrderExceptionDetail loads one exception row by source.
func (s *Service) GetOrderExceptionDetail(ctx context.Context, sourceType, sourceID string) (*OrderExceptionDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("orderexception: unavailable")
	}
	sid, err := uuid.Parse(strings.TrimSpace(sourceID))
	if err != nil {
		return nil, fmt.Errorf("invalid sourceId")
	}
	st := strings.TrimSpace(sourceType)

	var markRows []OrderExceptionMark
	_ = s.DB.WithContext(ctx).Find(&markRows).Error
	marks := buildMarkIndex(markRows)

	switch st {
	case SourceOrderItemSKUMatch:
		var m order.OrderItemSKUMatch
		if err := s.DB.WithContext(ctx).First(&m, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		r, err := s.rowFromSKUMatch(ctx, m)
		if err != nil {
			return nil, err
		}
		d := exceptionToDTO(ctx, s, r)
		applyMarkDTO(&d, marks)
		return &d, nil
	case SourceOrderItem:
		var oi order.OrderItem
		if err := s.DB.WithContext(ctx).First(&oi, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		r, err := s.rowFromOrderItem(ctx, oi)
		if err != nil {
			return nil, err
		}
		d := exceptionToDTO(ctx, s, r)
		applyMarkDTO(&d, marks)
		return &d, nil
	case SourceOrderInventoryEffect:
		var e inventory.OrderInventoryEffect
		if err := s.DB.WithContext(ctx).First(&e, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		req := ListOrderExceptionsRequest{}
		xs, err := s.collectInventoryEffects(ctx, req, e.EffectType)
		if err != nil {
			return nil, err
		}
		for _, r := range xs {
			if r.sourceID == e.ID {
				d := exceptionToDTO(ctx, s, r)
				applyMarkDTO(&d, marks)
				return &d, nil
			}
		}
		return nil, gorm.ErrRecordNotFound
	case SourceInventorySyncTask:
		var t inventory.InventorySyncTask
		if err := s.DB.WithContext(ctx).First(&t, "id = ?", sid).Error; err != nil {
			return nil, err
		}
		if t.Status != inventory.StatusFailed || t.ProductSKUID == nil {
			return nil, gorm.ErrRecordNotFound
		}
		req := ListOrderExceptionsRequest{}
		xs, err := s.collectInventorySyncFailed(ctx, req)
		if err != nil {
			return nil, err
		}
		for _, r := range xs {
			if r.sourceID == t.ID {
				d := exceptionToDTO(ctx, s, r)
				applyMarkDTO(&d, marks)
				return &d, nil
			}
		}
		return nil, gorm.ErrRecordNotFound
	default:
		return nil, fmt.Errorf("unsupported sourceType")
	}
}

func applyMarkDTO(d *OrderExceptionDTO, marks map[string]markPair) {
	mp := marks[markKey(d.ExceptionType, d.SourceType, d.SourceID)]
	d.Handled = mp.handled
	d.Ignored = mp.ignored
	switch {
	case mp.handled:
		d.Status = StatusHandled
	case mp.ignored:
		d.Status = StatusIgnored
	default:
		d.Status = StatusOpen
	}
}

func (s *Service) rowFromSKUMatch(ctx context.Context, m order.OrderItemSKUMatch) (aggRow, error) {
	var o order.Order
	if err := s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", m.OrderID).Error; err != nil {
		return aggRow{}, err
	}
	var oi order.OrderItem
	if err := s.DB.WithContext(ctx).First(&oi, "id = ?", m.OrderItemID).Error; err != nil {
		return aggRow{}, err
	}
	switch m.MatchStatus {
	case order.MatchStatusMatched, order.MatchStatusManualBound:
		return aggRow{}, gorm.ErrRecordNotFound
	default:
		break
	}
	extOid := ""
	if o.ExternalOrderID != nil {
		extOid = strings.TrimSpace(*o.ExternalOrderID)
	}
	sc := strings.TrimSpace(m.SKUCode)
	if sc == "" {
		sc = strings.TrimSpace(m.SellerSKU)
	}
	oiid := m.OrderItemID
	switch m.MatchStatus {
	case order.MatchStatusAmbiguous:
		return aggRow{
			exceptionType:   TypeSKUAmbiguous,
			severity:        SeverityMedium,
			sourceType:      SourceOrderItemSKUMatch,
			sourceID:        m.ID,
			orderID:         m.OrderID,
			orderItemID:     &oiid,
			platform:        strings.TrimSpace(m.Platform),
			shopID:          o.ShopID,
			orderNo:         strings.TrimSpace(o.OrderNo),
			externalOrderID: extOid,
			externalItemID:  derefStr(m.ExternalItemID),
			externalSkuID:   derefStr(m.ExternalSKUID),
			skuCode:         sc,
			productTitle:    strings.TrimSpace(oi.ProductTitle),
			quantity:        oi.Quantity,
			suggestedAction: "请确认候选 SKU 后人工绑定。",
			createdAt:       m.CreatedAt,
			updatedAt:       m.UpdatedAt,
		}, nil
	default:
		return aggRow{
			exceptionType:   TypeSKUUnmatched,
			severity:        SeverityHigh,
			sourceType:      SourceOrderItemSKUMatch,
			sourceID:        m.ID,
			orderID:         m.OrderID,
			orderItemID:     &oiid,
			platform:        strings.TrimSpace(m.Platform),
			shopID:          o.ShopID,
			orderNo:         strings.TrimSpace(o.OrderNo),
			externalOrderID: extOid,
			externalItemID:  derefStr(m.ExternalItemID),
			externalSkuID:   derefStr(m.ExternalSKUID),
			skuCode:         sc,
			productTitle:    strings.TrimSpace(oi.ProductTitle),
			quantity:        oi.Quantity,
			suggestedAction: "请人工绑定本地 SKU，绑定后可重新扣减库存。",
			createdAt:       m.CreatedAt,
			updatedAt:       m.UpdatedAt,
		}, nil
	}
}

func (s *Service) rowFromOrderItem(ctx context.Context, oi order.OrderItem) (aggRow, error) {
	var o order.Order
	if err := s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", oi.OrderID).Error; err != nil {
		return aggRow{}, err
	}
	extOid := ""
	if o.ExternalOrderID != nil {
		extOid = strings.TrimSpace(*o.ExternalOrderID)
	}
	sc := strings.TrimSpace(oi.SKUCode)
	if sc == "" {
		sc = strings.TrimSpace(oi.SellerSKU)
	}
	oiid := oi.ID
	return aggRow{
		exceptionType:   TypeSKUUnmatched,
		severity:        SeverityHigh,
		sourceType:      SourceOrderItem,
		sourceID:        oi.ID,
		orderID:         oi.OrderID,
		orderItemID:     &oiid,
		platform:        strings.TrimSpace(o.Platform),
		shopID:          o.ShopID,
		orderNo:         strings.TrimSpace(o.OrderNo),
		externalOrderID: extOid,
		externalItemID:  derefStr(oi.ExternalItemID),
		externalSkuID:   derefStr(oi.ExternalSKUID),
		skuCode:         sc,
		productTitle:    strings.TrimSpace(oi.ProductTitle),
		quantity:        oi.Quantity,
		suggestedAction: "请人工绑定本地 SKU，绑定后可重新扣减库存。",
		createdAt:       oi.CreatedAt,
		updatedAt:       oi.UpdatedAt,
	}, nil
}

func (s *Service) UpsertMark(ctx context.Context, exceptionType, sourceType, sourceID, markType, remark string, admin *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("orderexception: unavailable")
	}
	oid, oiid, err := s.resolveOrderPointers(ctx, sourceType, sourceID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	row := OrderExceptionMark{
		ExceptionType: strings.TrimSpace(exceptionType),
		SourceType:    strings.TrimSpace(sourceType),
		SourceID:      strings.TrimSpace(sourceID),
		MarkType:      strings.TrimSpace(markType),
		OrderID:       oid,
		OrderItemID:   oiid,
		Remark:        strings.TrimSpace(remark),
		CreatedBy:     admin,
	}
	row.UpdatedAt = now
	row.CreatedAt = now

	opposite := MarkIgnored
	if markType == MarkIgnored {
		opposite = MarkHandled
	}
	_ = s.DB.WithContext(ctx).
		Where("exception_type = ? AND source_type = ? AND source_id = ? AND mark_type = ?", row.ExceptionType, row.SourceType, row.SourceID, opposite).
		Delete(&OrderExceptionMark{}).Error

	var existing OrderExceptionMark
	err = s.DB.WithContext(ctx).
		Where("exception_type = ? AND source_type = ? AND source_id = ? AND mark_type = ?", row.ExceptionType, row.SourceType, row.SourceID, row.MarkType).
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.DB.WithContext(ctx).Create(&row).Error
	}
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Model(&existing).Updates(map[string]any{
		"remark":        row.Remark,
		"updated_at":    now,
		"created_by":    admin,
		"order_id":      oid,
		"order_item_id": oiid,
	}).Error
}

func (s *Service) DeleteMarks(ctx context.Context, sourceType, sourceID string) error {
	return s.DB.WithContext(ctx).
		Where("source_type = ? AND source_id = ?", strings.TrimSpace(sourceType), strings.TrimSpace(sourceID)).
		Delete(&OrderExceptionMark{}).Error
}

func (s *Service) resolveOrderPointers(ctx context.Context, sourceType, sourceID string) (*uuid.UUID, *uuid.UUID, error) {
	sid, err := uuid.Parse(strings.TrimSpace(sourceID))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid sourceId")
	}
	st := strings.TrimSpace(sourceType)
	switch st {
	case SourceOrderItemSKUMatch:
		var m order.OrderItemSKUMatch
		if err := s.DB.WithContext(ctx).First(&m, "id = ?", sid).Error; err != nil {
			return nil, nil, err
		}
		oiid := m.OrderItemID
		return &m.OrderID, &oiid, nil
	case SourceOrderItem:
		var oi order.OrderItem
		if err := s.DB.WithContext(ctx).First(&oi, "id = ?", sid).Error; err != nil {
			return nil, nil, err
		}
		oid := oi.OrderID
		iid := oi.ID
		return &oid, &iid, nil
	case SourceOrderInventoryEffect:
		var e inventory.OrderInventoryEffect
		if err := s.DB.WithContext(ctx).First(&e, "id = ?", sid).Error; err != nil {
			return nil, nil, err
		}
		oiid := e.OrderItemID
		return &e.OrderID, &oiid, nil
	case SourceInventorySyncTask:
		return nil, nil, nil
	default:
		return nil, nil, fmt.Errorf("unsupported sourceType")
	}
}

func (s *Service) collectOrderSyncPartialFailed(ctx context.Context, req ListOrderExceptionsRequest) ([]aggRow, error) {
	if s == nil || s.DB == nil {
		return nil, nil
	}
	dst := &ordersync.OrderSyncTask{}
	if !s.DB.Migrator().HasTable(dst) {
		return nil, nil
	}
	var tasks []ordersync.OrderSyncTask
	tx := s.DB.WithContext(ctx).Model(dst).Where("status = ?", ordersync.StatusPartialSuccess)
	if req.Platform != "" {
		tx = tx.Where("LOWER(platform) = ?", strings.ToLower(strings.TrimSpace(req.Platform)))
	}
	if req.ShopID != "" {
		if sid, err := uuid.Parse(strings.TrimSpace(req.ShopID)); err == nil {
			tx = tx.Where("shop_id = ?", sid)
		}
	}
	if err := tx.Find(&tasks).Error; err != nil {
		return nil, err
	}
	out := make([]aggRow, 0, len(tasks))
	for _, t := range tasks {
		msg := strings.TrimSpace(t.ErrorMessage)
		if msg == "" {
			msg = "订单同步部分成功：存在失败页，请查看同步任务详情并重试失败页"
		}
		out = append(out, aggRow{
			exceptionType:   TypeOrderSyncPartialFailed,
			severity:        SeverityHigh,
			sourceType:      SourceOrderSyncTask,
			sourceID:        t.ID,
			orderID:         uuid.Nil,
			platform:        t.Platform,
			shopID:          &t.ShopID,
			orderNo:         "",
			errorMessage:    msg,
			suggestedAction: "打开同步任务详情，查看失败页列表并重试失败页；或在失败任务中心处理",
			createdAt:       t.CreatedAt,
			updatedAt:       t.UpdatedAt,
		})
	}
	return out, nil
}

// ResolveOrderItemForBind maps an exception source to an order line id when bind-sku applies.
func (s *Service) ResolveOrderItemForBind(ctx context.Context, sourceType, sourceID string) (uuid.UUID, error) {
	_, oiid, err := s.resolveOrderPointers(ctx, sourceType, sourceID)
	if err != nil {
		return uuid.Nil, err
	}
	if oiid == nil {
		return uuid.Nil, fmt.Errorf("bind-sku not applicable for this exception source")
	}
	return *oiid, nil
}

// DashboardSummary returns open-exception counts for the board (read-only).
func (s *Service) DashboardSummary(ctx context.Context, platform, shopID string, start, end *time.Time) (ExceptionSummaryDTO, error) {
	req := ListOrderExceptionsRequest{
		Platform: strings.TrimSpace(platform),
		ShopID:   strings.TrimSpace(shopID),
		Start:    start,
		End:      end,
		Page:     1,
		PageSize: 1,
	}
	res, err := s.ListOrderExceptions(ctx, req)
	if err != nil || res == nil {
		return ExceptionSummaryDTO{}, err
	}
	return res.Summary, nil
}
