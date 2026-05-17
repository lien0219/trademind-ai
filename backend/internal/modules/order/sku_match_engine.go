package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

type publicationSKUHit struct {
	ProductSKUID uuid.UUID
	ProductID    uuid.UUID
	SKUCode      string
}

// MatchOrderItemToSKU applies deterministic precedence; never writes DB (use persist helpers).
func (s *Service) MatchOrderItemToSKU(ctx context.Context, o *Order, it *OrderItem) (*MatchOrderItemResult, error) {
	if s == nil || s.DB == nil || o == nil || it == nil {
		return nil, fmt.Errorf("order: invalid match args")
	}
	plat := strings.TrimSpace(o.Platform)
	if plat == "" || plat == "manual" {
		return &MatchOrderItemResult{
			MatchType:   MatchTypeNone,
			MatchStatus: MatchStatusSkipped,
			Confidence:  0,
			Reason:      "manual_or_empty_platform",
			RawData:     map[string]any{"hint": "skipped_non_platform_order"},
		}, nil
	}

	extItem := ""
	if it.ExternalItemID != nil {
		extItem = strings.TrimSpace(*it.ExternalItemID)
	}
	extSku := ""
	if it.ExternalSKUID != nil {
		extSku = strings.TrimSpace(*it.ExternalSKUID)
	}
	seller := strings.TrimSpace(it.SellerSKU)
	code := strings.TrimSpace(it.SKUCode)
	codeOrSeller := code
	if codeOrSeller == "" {
		codeOrSeller = seller
	}

	baseRaw := map[string]any{
		"externalItemId": extItem,
		"externalSkuId":  extSku,
		"sellerSku":      seller,
		"skuCode":        code,
	}

	// Priority 1: publication external_sku_id
	if extSku != "" && o.ShopID != nil && *o.ShopID != uuid.Nil {
		hits, err := s.findPublicationSKUsByExternalSKUID(ctx, plat, *o.ShopID, extSku)
		if err != nil {
			return nil, err
		}
		if len(hits) > 1 {
			raw := cloneRaw(baseRaw)
			raw["candidates"] = publicationHitsToCandidates(hits)
			return &MatchOrderItemResult{
				MatchType:   MatchTypePublicationSKUExternalID,
				MatchStatus: MatchStatusAmbiguous,
				Confidence:  50,
				Reason:      "multiple_publication_hits_for_external_sku_id",
				RawData:     raw,
			}, nil
		}
		if len(hits) == 1 {
			h := hits[0]
			pid := h.ProductID
			sid := h.ProductSKUID
			raw := cloneRaw(baseRaw)
			raw["publicationSkuCode"] = h.SKUCode
			return &MatchOrderItemResult{
				MatchType:        MatchTypePublicationSKUExternalID,
				MatchStatus:      MatchStatusMatched,
				Confidence:       100,
				Reason:           "publication_external_sku_id",
				ProductID:        &pid,
				ProductSKUID:     &sid,
				RawData:          raw,
				UpdateOrderLines: true,
			}, nil
		}
	}

	// Priority 2: publication sku_code
	if codeOrSeller != "" && o.ShopID != nil && *o.ShopID != uuid.Nil {
		hits, err := s.findPublicationSKUsBySKUCode(ctx, plat, *o.ShopID, codeOrSeller)
		if err != nil {
			return nil, err
		}
		if len(hits) > 0 {
			uniq := uniqueProductSKUIDs(hits)
			if len(uniq) > 1 {
				raw := cloneRaw(baseRaw)
				raw["candidates"] = publicationHitsToCandidates(hits)
				return &MatchOrderItemResult{
					MatchType:   MatchTypePublicationSKUCode,
					MatchStatus: MatchStatusAmbiguous,
					Confidence:  50,
					Reason:      "multiple_publication_hits_for_sku_code",
					RawData:     raw,
				}, nil
			}
			h := hits[0]
			pid := h.ProductID
			sid := h.ProductSKUID
			raw := cloneRaw(baseRaw)
			raw["matchedPublicationSkuCode"] = h.SKUCode
			reason := "publication_sku_code"
			if len(hits) > 1 {
				reason = "publication_sku_code_multi_row_same_sku"
			}
			return &MatchOrderItemResult{
				MatchType:        MatchTypePublicationSKUCode,
				MatchStatus:      MatchStatusMatched,
				Confidence:       90,
				Reason:           reason,
				ProductID:        &pid,
				ProductSKUID:     &sid,
				RawData:          raw,
				UpdateOrderLines: true,
			}, nil
		}
	}

	// Priority 3: local sku_code (product_skus)
	if codeOrSeller != "" {
		skus, err := s.findLocalSKUsByCode(ctx, codeOrSeller)
		if err != nil {
			return nil, err
		}
		if len(skus) > 1 {
			raw := cloneRaw(baseRaw)
			raw["candidates"] = localSKUsToCandidates(skus)
			return &MatchOrderItemResult{
				MatchType:   MatchTypeLocalSKUCode,
				MatchStatus: MatchStatusAmbiguous,
				Confidence:  50,
				Reason:      "multiple_local_skus_for_code",
				RawData:     raw,
			}, nil
		}
		if len(skus) == 1 {
			sku := skus[0]
			pid := sku.ProductID
			sid := sku.ID
			raw := cloneRaw(baseRaw)
			raw["localSkuCode"] = strings.TrimSpace(sku.SKUCode)
			return &MatchOrderItemResult{
				MatchType:        MatchTypeLocalSKUCode,
				MatchStatus:      MatchStatusMatched,
				Confidence:       70,
				Reason:           "unique_local_sku_code",
				ProductID:        &pid,
				ProductSKUID:     &sid,
				RawData:          raw,
				UpdateOrderLines: true,
			}, nil
		}
	}

	if extSku == "" && codeOrSeller == "" {
		return &MatchOrderItemResult{
			MatchType:   MatchTypeNone,
			MatchStatus: MatchStatusSkipped,
			Confidence:  0,
			Reason:      "no_identifiers",
			RawData:     baseRaw,
		}, nil
	}

	return &MatchOrderItemResult{
		MatchType:   MatchTypeNone,
		MatchStatus: MatchStatusUnmatched,
		Confidence:  0,
		Reason:      "no_match",
		RawData:     baseRaw,
	}, nil
}

func cloneRaw(m map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

func uniqueProductSKUIDs(hits []publicationSKUHit) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	var out []uuid.UUID
	for _, h := range hits {
		if _, ok := seen[h.ProductSKUID]; ok {
			continue
		}
		seen[h.ProductSKUID] = struct{}{}
		out = append(out, h.ProductSKUID)
	}
	return out
}

func publicationHitsToCandidates(hits []publicationSKUHit) []map[string]any {
	var out []map[string]any
	seen := map[uuid.UUID]struct{}{}
	for _, h := range hits {
		if _, ok := seen[h.ProductSKUID]; ok {
			continue
		}
		seen[h.ProductSKUID] = struct{}{}
		out = append(out, map[string]any{
			"productSkuId": h.ProductSKUID.String(),
			"productId":    h.ProductID.String(),
			"skuCode":      h.SKUCode,
		})
	}
	return out
}

func localSKUsToCandidates(rows []product.ProductSKU) []map[string]any {
	var out []map[string]any
	for _, sku := range rows {
		sc := strings.TrimSpace(sku.SKUCode)
		out = append(out, map[string]any{
			"productSkuId": sku.ID.String(),
			"productId":    sku.ProductID.String(),
			"skuCode":      sc,
			"skuName":      strings.TrimSpace(sku.SKUName),
		})
	}
	return out
}

func (s *Service) findPublicationSKUsByExternalSKUID(ctx context.Context, platform string, shopID uuid.UUID, ext string) ([]publicationSKUHit, error) {
	var rows []struct {
		PSKU  *uuid.UUID `gorm:"column:product_sku_id"`
		PID   uuid.UUID  `gorm:"column:product_id"`
		PCode string     `gorm:"column:pub_code"`
	}
	err := s.DB.WithContext(ctx).Table("product_publication_skus AS pps").
		Select("pps.product_sku_id AS product_sku_id, skus.product_id AS product_id, pps.sku_code AS pub_code").
		Joins("JOIN product_publications pp ON pp.id = pps.publication_id AND pp.deleted_at IS NULL").
		Joins("JOIN product_skus skus ON skus.id = pps.product_sku_id AND skus.deleted_at IS NULL").
		Where("pp.platform = ? AND pp.shop_id = ? AND pps.external_sku_id = ? AND pps.product_sku_id IS NOT NULL", platform, shopID, ext).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	var hits []publicationSKUHit
	for _, r := range rows {
		if r.PSKU == nil || *r.PSKU == uuid.Nil {
			continue
		}
		hits = append(hits, publicationSKUHit{ProductSKUID: *r.PSKU, ProductID: r.PID, SKUCode: strings.TrimSpace(r.PCode)})
	}
	return hits, nil
}

func (s *Service) findPublicationSKUsBySKUCode(ctx context.Context, platform string, shopID uuid.UUID, code string) ([]publicationSKUHit, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	var rows []struct {
		PSKU  *uuid.UUID `gorm:"column:product_sku_id"`
		PID   uuid.UUID  `gorm:"column:product_id"`
		PCode string     `gorm:"column:pub_code"`
	}
	err := s.DB.WithContext(ctx).Table("product_publication_skus AS pps").
		Select("pps.product_sku_id AS product_sku_id, skus.product_id AS product_id, pps.sku_code AS pub_code").
		Joins("JOIN product_publications pp ON pp.id = pps.publication_id AND pp.deleted_at IS NULL").
		Joins("JOIN product_skus skus ON skus.id = pps.product_sku_id AND skus.deleted_at IS NULL").
		Where("pp.platform = ? AND pp.shop_id = ? AND LOWER(TRIM(pps.sku_code)) = LOWER(?)", platform, shopID, code).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	var hits []publicationSKUHit
	for _, r := range rows {
		if r.PSKU == nil || *r.PSKU == uuid.Nil {
			continue
		}
		hits = append(hits, publicationSKUHit{ProductSKUID: *r.PSKU, ProductID: r.PID, SKUCode: strings.TrimSpace(r.PCode)})
	}
	return hits, nil
}

func (s *Service) findLocalSKUsByCode(ctx context.Context, code string) ([]product.ProductSKU, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	var skus []product.ProductSKU
	err := s.DB.WithContext(ctx).
		Where("deleted_at IS NULL AND LOWER(TRIM(sku_code)) = LOWER(?)", code).
		Order("created_at ASC, id ASC").
		Find(&skus).Error
	return skus, err
}

// LoadSKUForBind returns the SKU row after ownership checks (no soft-deleted).
func (s *Service) LoadSKUForBind(ctx context.Context, skuID uuid.UUID) (*product.ProductSKU, error) {
	var sku product.ProductSKU
	if err := s.DB.WithContext(ctx).First(&sku, "id = ? AND deleted_at IS NULL", skuID).Error; err != nil {
		return nil, err
	}
	return &sku, nil
}

func matchRowFromResult(o *Order, it *OrderItem, r *MatchOrderItemResult, admin *uuid.UUID) (*OrderItemSKUMatch, error) {
	if o == nil || it == nil || r == nil {
		return nil, fmt.Errorf("invalid row build")
	}
	var extOrder *string
	if o.ExternalOrderID != nil && strings.TrimSpace(*o.ExternalOrderID) != "" {
		v := strings.TrimSpace(*o.ExternalOrderID)
		extOrder = &v
	}
	var extItem *string
	if it.ExternalItemID != nil && strings.TrimSpace(*it.ExternalItemID) != "" {
		v := strings.TrimSpace(*it.ExternalItemID)
		extItem = &v
	}
	var extSku *string
	if it.ExternalSKUID != nil && strings.TrimSpace(*it.ExternalSKUID) != "" {
		v := strings.TrimSpace(*it.ExternalSKUID)
		extSku = &v
	}
	rawJSON, err := json.Marshal(trimRawDataMap(r.RawData))
	if err != nil {
		return nil, err
	}
	return &OrderItemSKUMatch{
		OrderID:         o.ID,
		OrderItemID:     it.ID,
		Platform:        strings.TrimSpace(o.Platform),
		ExternalOrderID: extOrder,
		ExternalItemID:  extItem,
		ExternalSKUID:   extSku,
		SellerSKU:       strings.TrimSpace(it.SellerSKU),
		SKUCode:         strings.TrimSpace(it.SKUCode),
		ProductID:       r.ProductID,
		ProductSKUID:    r.ProductSKUID,
		MatchType:       r.MatchType,
		MatchStatus:     r.MatchStatus,
		Confidence:      r.Confidence,
		Reason:          r.Reason,
		RawData:         rawJSON,
		CreatedBy:       admin,
	}, nil
}

func trimRawDataMap(m map[string]any) map[string]any {
	if len(m) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	for k, v := range m {
		switch k {
		case "candidates":
			if arr, ok := v.([]map[string]any); ok {
				const maxCand = 20
				if len(arr) > maxCand {
					arr = arr[:maxCand]
				}
				out[k] = arr
			}
		default:
			out[k] = v
		}
	}
	return out
}

// upsertSKUMatchRow saves one match row per order_item_id.
func upsertSKUMatchRowTx(tx *gorm.DB, row *OrderItemSKUMatch) error {
	if tx == nil || row == nil {
		return fmt.Errorf("invalid tx")
	}
	var ex OrderItemSKUMatch
	err := tx.Where("order_item_id = ?", row.OrderItemID).First(&ex).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(row).Error
	}
	if err != nil {
		return err
	}
	row.ID = ex.ID
	row.CreatedAt = ex.CreatedAt
	return tx.Model(&OrderItemSKUMatch{}).Where("id = ?", ex.ID).Updates(map[string]any{
		"order_id":          row.OrderID,
		"platform":          row.Platform,
		"external_order_id": row.ExternalOrderID,
		"external_item_id":  row.ExternalItemID,
		"external_sku_id":   row.ExternalSKUID,
		"seller_sku":        row.SellerSKU,
		"sku_code":          row.SKUCode,
		"product_id":        row.ProductID,
		"product_sku_id":    row.ProductSKUID,
		"match_type":        row.MatchType,
		"match_status":      row.MatchStatus,
		"confidence":        row.Confidence,
		"reason":            row.Reason,
		"raw_data":          row.RawData,
		"updated_at":        gorm.Expr("CURRENT_TIMESTAMP"),
	}).Error
}

func (s *Service) upsertSKUMatchRow(ctx context.Context, row *OrderItemSKUMatch) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("order: no db")
	}
	return upsertSKUMatchRowTx(s.DB.WithContext(ctx), row)
}

// readOrderSKUSettings from settings.inventory.
func (s *Service) readOrderSKUSettings(ctx context.Context) (OrderSKUSettings, error) {
	def := OrderSKUSettings{
		AutoMatchOrderSKUs:                true,
		AutoDeductAfterSKUMatch:           false,
		AutoSyncInventoryAfterOrderDeduct: false,
		AllowManualSkuBindAfterDeduct:     true,
	}
	if s == nil || s.Settings == nil {
		return def, nil
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "inventory")
	if err != nil {
		return def, err
	}
	truthy := func(k string) bool {
		v := strings.TrimSpace(strings.ToLower(m[k]))
		return v == "1" || v == "true" || v == "yes" || v == "on"
	}
	flagOr := func(k string, def bool) bool {
		v, ok := m[k]
		if !ok || strings.TrimSpace(v) == "" {
			return def
		}
		vv := strings.TrimSpace(strings.ToLower(v))
		return vv == "1" || vv == "true" || vv == "yes" || vv == "on"
	}
	syncNew := strings.TrimSpace(m["auto_sync_inventory_after_order_deduct"])
	syncLegacy := strings.TrimSpace(m["auto_sync_platform_inventory_after_deduct"])
	syncVal := syncNew
	if syncVal == "" {
		syncVal = syncLegacy
	}
	sv := strings.TrimSpace(strings.ToLower(syncVal))
	syncEffective := sv == "1" || sv == "true" || sv == "yes" || sv == "on"
	return OrderSKUSettings{
		AutoMatchOrderSKUs:                flagOr("auto_match_order_skus", true),
		AutoDeductAfterSKUMatch:           truthy("auto_deduct_after_sku_match"),
		AutoSyncInventoryAfterOrderDeduct: syncEffective,
		AllowManualSkuBindAfterDeduct:     flagOr("allow_manual_sku_bind_after_deduct", true),
	}, nil
}

// MatchOrderItemsOptions controls batch matching.
type MatchOrderItemsOptions struct {
	Overwrite bool
	Force     bool
	CreatedBy *uuid.UUID
	Source    string
}

// MatchOrderItemsForOrder runs auto-match for each line; failures on single lines do not abort unless DB error.
func (s *Service) MatchOrderItemsForOrder(ctx context.Context, orderID uuid.UUID, opts MatchOrderItemsOptions) (*MatchOrderSummary, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("order: no db")
	}
	sum := &MatchOrderSummary{OrderID: orderID}
	st, err := s.readOrderSKUSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !st.AutoMatchOrderSKUs {
		src := strings.TrimSpace(opts.Source)
		if src == "order_sync" {
			return sum, nil
		}
	}
	var o Order
	if err := s.DB.WithContext(ctx).First(&o, "id = ? AND deleted_at IS NULL", orderID).Error; err != nil {
		return nil, err
	}
	var items []OrderItem
	if err := s.DB.WithContext(ctx).Where("order_id = ?", orderID).Order("created_at ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	sum.ItemsTotal = len(items)
	for _, it := range items {
		item := it
		var prev OrderItemSKUMatch
		_ = s.DB.WithContext(ctx).Where("order_item_id = ?", item.ID).First(&prev).Error

		if prev.MatchStatus == MatchStatusManualBound && !opts.Force {
			sum.ManualBound++
			continue
		}
		if !opts.Overwrite && !opts.Force && (prev.MatchStatus == MatchStatusMatched || prev.MatchStatus == MatchStatusManualBound) {
			sum.Preserved++
			continue
		}

		res, mErr := s.MatchOrderItemToSKU(ctx, &o, &item)
		if mErr != nil {
			sum.Errors = append(sum.Errors, fmt.Sprintf("item %s: %v", item.ID.String(), mErr))
			continue
		}
		row, rErr := matchRowFromResult(&o, &item, res, opts.CreatedBy)
		if rErr != nil {
			sum.Errors = append(sum.Errors, rErr.Error())
			continue
		}
		if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if res.UpdateOrderLines && res.ProductSKUID != nil && *res.ProductSKUID != uuid.Nil {
				up := map[string]any{
					"product_id":     res.ProductID,
					"product_sku_id": res.ProductSKUID,
					"updated_at":     gorm.Expr("CURRENT_TIMESTAMP"),
				}
				if err := tx.Model(&OrderItem{}).Where("id = ?", item.ID).Updates(up).Error; err != nil {
					return err
				}
			}
			return upsertSKUMatchRowTx(tx, row)
		}); err != nil {
			sum.Errors = append(sum.Errors, err.Error())
			continue
		}
		switch res.MatchStatus {
		case MatchStatusMatched:
			sum.Matched++
		case MatchStatusUnmatched:
			sum.Unmatched++
		case MatchStatusAmbiguous:
			sum.Ambiguous++
		case MatchStatusSkipped:
			sum.Skipped++
		default:
			sum.Unmatched++
		}
		if s.OpLog != nil && shouldLogAutoMatch(res.MatchStatus, opts.Source) {
			s.writeSKUMatchOp(ctx, opts.CreatedBy, res.MatchStatus, o.ID, item.ID, res)
		}
	}
	return sum, nil
}

func shouldLogAutoMatch(status string, source string) bool {
	if status == MatchStatusSkipped {
		return false
	}
	if strings.TrimSpace(source) == "order_sync" {
		return status == MatchStatusUnmatched || status == MatchStatusAmbiguous
	}
	return status != MatchStatusSkipped
}

func (s *Service) writeSKUMatchOp(ctx context.Context, admin *uuid.UUID, status string, orderID, itemID uuid.UUID, res *MatchOrderItemResult) {
	if s.OpLog == nil || res == nil {
		return
	}
	act := "order.sku_match.auto"
	switch status {
	case MatchStatusUnmatched:
		act = "order.sku_match.unmatched"
	case MatchStatusAmbiguous:
		act = "order.sku_match.ambiguous"
	case MatchStatusMatched:
		act = "order.sku_match.auto"
	case MatchStatusSkipped:
		return
	default:
		act = "order.sku_match.auto"
	}
	msg := fmt.Sprintf("orderId=%s orderItemId=%s matchType=%s status=%s conf=%d reason=%s",
		orderID.String(), itemID.String(), res.MatchType, res.MatchStatus, res.Confidence, clampReason(res.Reason))
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: admin,
		Action:      act,
		Resource:    "order_item",
		ResourceID:  itemID.String(),
		Status:      "success",
		Message:     msg,
	})
}

func clampReason(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 160 {
		return s[:160]
	}
	return s
}
