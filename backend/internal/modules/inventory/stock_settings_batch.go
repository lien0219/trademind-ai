package inventory

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

type stockSettingsFilter struct {
	ProductID     *uuid.UUID
	ProductSkuIDs []uuid.UUID
	Platform      string
	ShopID        *uuid.UUID
	Keyword       string
	StockStatus   string
	AlertTypes    []string
	OnlyPublished bool
	IncludeNormal bool
}

func parseUUIDPtr(s string) (*uuid.UUID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid uuid")
	}
	if u == uuid.Nil {
		return nil, nil
	}
	return &u, nil
}

func parseStockSettingsFilter(body StockSettingsBatchPreviewBody) (stockSettingsFilter, error) {
	var f stockSettingsFilter
	pid, err := parseUUIDPtr(body.ProductID)
	if err != nil {
		return f, fmt.Errorf("invalid productId")
	}
	f.ProductID = pid
	sid, err := parseUUIDPtr(body.ShopID)
	if err != nil {
		return f, fmt.Errorf("invalid shopId")
	}
	f.ShopID = sid
	f.Platform = strings.TrimSpace(strings.ToLower(body.Platform))
	f.Keyword = strings.TrimSpace(body.Keyword)
	f.StockStatus = strings.TrimSpace(body.StockStatus)
	f.OnlyPublished = body.OnlyPublished
	f.IncludeNormal = body.IncludeNormal
	for _, raw := range body.AlertTypes {
		t := strings.TrimSpace(strings.ToLower(raw))
		if t != "" {
			f.AlertTypes = append(f.AlertTypes, t)
		}
	}
	for _, raw := range body.ProductSkuIDs {
		u, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return f, fmt.Errorf("invalid productSkuIds entry")
		}
		if u != uuid.Nil {
			f.ProductSkuIDs = append(f.ProductSkuIDs, u)
		}
	}
	return f, nil
}

func stockSettingsNeedsConfirmAll(f stockSettingsFilter) bool {
	if len(f.ProductSkuIDs) > 0 {
		return false
	}
	if f.ProductID != nil && *f.ProductID != uuid.Nil {
		return false
	}
	if strings.TrimSpace(f.Platform) != "" {
		return false
	}
	if f.ShopID != nil && *f.ShopID != uuid.Nil {
		return false
	}
	if strings.TrimSpace(f.Keyword) != "" {
		return false
	}
	if strings.TrimSpace(f.StockStatus) != "" {
		return false
	}
	if f.OnlyPublished {
		return false
	}
	if len(f.AlertTypes) > 0 {
		return false
	}
	if !f.IncludeNormal {
		return false
	}
	return true
}

func (s *Service) stockSettingsBatchMax(ctx context.Context) int {
	max := 500
	if s == nil || s.Settings == nil {
		return max
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "inventory")
	if err != nil {
		return max
	}
	if v := strings.TrimSpace(m["inventory_stock_settings_batch_max_size"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			return n
		}
	}
	return max
}

func (s *Service) applyStockSettingsAlertFilters(tx *gorm.DB, pol inventoryAlertPolicy, th int, alertTypes []string, includeNormal bool) *gorm.DB {
	if len(alertTypes) > 0 {
		return s.applyAlertsSQLAlertTypesOR(tx, alertTypes, th)
	}
	if !includeNormal {
		return s.applyNonNormalAlertScope(tx, pol, th)
	}
	return tx
}

func (s *Service) buildStockSettingsGroupedTX(ctx context.Context, f stockSettingsFilter) (*gorm.DB, inventoryAlertPolicy, int, error) {
	var zero inventoryAlertPolicy
	if s == nil || s.DB == nil {
		return nil, zero, 0, fmt.Errorf("inventory: no db")
	}
	pol, err := s.loadInventoryAlertPolicy(ctx)
	if err != nil {
		return nil, zero, 0, err
	}
	th := pol.PlatformStockMismatchThresh
	if th < 0 {
		th = 0
	}
	base := s.buildSKUAlertBaseTX(ctx, skuAlertBaseQuery{
		Keyword:       f.Keyword,
		ProductID:     f.ProductID,
		ProductSkuIDs: f.ProductSkuIDs,
		Platform:      f.Platform,
		ShopID:        f.ShopID,
		StockStatus:   f.StockStatus,
		OnlyPublished: f.OnlyPublished,
	})
	base = s.applyStockSettingsAlertFilters(base, pol, th, f.AlertTypes, f.IncludeNormal)
	base = base.Group(`sk.id, sk.product_id, sk.sku_code, sk.sku_name, sk.stock, sk.warning_stock, sk.safety_stock, sk.updated_at, p.title`)
	return base, pol, th, nil
}

// PreviewStockSettingsBatch returns matched SKU count and a page of sample rows.
func (s *Service) PreviewStockSettingsBatch(ctx context.Context, body StockSettingsBatchPreviewBody) (*StockSettingsBatchPreviewResult, error) {
	f, err := parseStockSettingsFilter(body)
	if err != nil {
		return nil, err
	}
	base, _, _, err := s.buildStockSettingsGroupedTX(ctx, f)
	if err != nil {
		return nil, err
	}
	page, ps := body.Page, body.PageSize
	if page < 1 {
		page = 1
	}
	if ps < 1 || ps > 100 {
		ps = 20
	}
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var scans []alertSKUScan
	if err := base.Order("sk.updated_at DESC").Offset(offset).Limit(ps).Scan(&scans).Error; err != nil {
		return nil, err
	}
	out := make([]StockSettingsSampleSKU, 0, len(scans))
	for _, r := range scans {
		out = append(out, StockSettingsSampleSKU{
			ProductID:    r.ProductID,
			ProductSkuID: r.ID,
			SKUCode:      r.SKUCode,
			ProductTitle: r.ProductTitle,
		})
	}
	return &StockSettingsBatchPreviewResult{
		MatchedCount: total,
		SampleSkus:   out,
		Page:         page,
		PageSize:     ps,
		TotalPages:   pagesOf(total, ps),
	}, nil
}

func truncateUUIDList(ids []uuid.UUID, max int) string {
	if max <= 0 {
		max = 5
	}
	if len(ids) == 0 {
		return ""
	}
	n := len(ids)
	if n > max {
		n = max
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		s := ids[i].String()
		if len(s) > 12 {
			s = s[:8] + "…"
		}
		parts = append(parts, s)
	}
	out := strings.Join(parts, ",")
	if len(ids) > max {
		out += fmt.Sprintf("…+%d", len(ids)-max)
	}
	return out
}

func formatStockSettingsOpSummary(matched, updated int64, w, s int, f stockSettingsFilter, alertTypes []string) string {
	var pid string
	if f.ProductID != nil {
		pid = f.ProductID.String()
	}
	var shop string
	if f.ShopID != nil {
		shop = f.ShopID.String()
	}
	return fmt.Sprintf(
		"matchedCount=%d updatedCount=%d warningStock=%d safetyStock=%d productId=%s platform=%s shopId=%s stockStatus=%s alertTypes=%s",
		matched, updated, w, s, pid, f.Platform, shop, f.StockStatus, strings.Join(alertTypes, ","),
	)
}

// BatchUpdateStockSettings updates warning_stock / safety_stock / stock_status only for matched SKUs.
func (s *Service) BatchUpdateStockSettings(ctx context.Context, body StockSettingsBatchUpdateBody, admin *uuid.UUID) (*StockSettingsBatchUpdateResult, error) {
	if err := product.ValidateSKUStockThresholds(body.WarningStock, body.SafetyStock); err != nil {
		return nil, err
	}
	if !body.Confirm {
		return nil, fmt.Errorf("需要确认：将 confirm 设为 true 后重试")
	}
	f, err := parseStockSettingsFilter(body.StockSettingsBatchPreviewBody)
	if err != nil {
		return nil, err
	}
	if stockSettingsNeedsConfirmAll(f) && !body.ConfirmAll {
		return nil, fmt.Errorf("缺少筛选条件且包含全部 SKU：将 confirmAll=true 后方可执行")
	}
	base, _, _, err := s.buildStockSettingsGroupedTX(ctx, f)
	if err != nil {
		return nil, err
	}
	var matched int64
	if err := base.Session(&gorm.Session{}).Count(&matched).Error; err != nil {
		return nil, err
	}
	if matched == 0 {
		return nil, fmt.Errorf("没有匹配的 SKU，请调整筛选条件")
	}
	maxN := s.stockSettingsBatchMax(ctx)
	if int64(maxN) < matched && !body.ConfirmLarge {
		return nil, fmt.Errorf("匹配 SKU 数（%d）超过单次上限（%d），请确认后设置 confirmLarge=true 后重试", matched, maxN)
	}
	var idScans []struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	q := base.Session(&gorm.Session{}).Select("sk.id")
	if err := q.Scan(&idScans).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(idScans))
	for _, r := range idScans {
		ids = append(ids, r.ID)
	}
	if int64(len(ids)) != matched {
		return nil, fmt.Errorf("internal: matched count / id list mismatch")
	}

	sql := fmt.Sprintf(`UPDATE product_skus AS sk SET
  warning_stock = ?,
  safety_stock = ?,
  stock_status = CASE
    WHEN COALESCE(sk.stock,0) <= 0 THEN '%s'
    WHEN ? > 0 AND COALESCE(sk.stock,0) > 0 AND COALESCE(sk.stock,0) <= ? THEN '%s'
    WHEN COALESCE(sk.stock,0) > 0 AND (? = 0 OR COALESCE(sk.stock,0) > ?) AND COALESCE(sk.stock,0) <= ? THEN '%s'
    ELSE '%s'
  END,
  updated_at = NOW()
WHERE sk.id IN ?`,
		product.StockStatusOutOfStock,
		product.StockStatusBelowSafetyStock,
		product.StockStatusLowStock,
		product.StockStatusNormal,
	)
	args := []any{
		body.WarningStock, body.SafetyStock,
		body.SafetyStock, body.SafetyStock,
		body.SafetyStock, body.SafetyStock, body.WarningStock,
		ids,
	}
	var updatedCnt int64
	txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := tx.Exec(sql, args...)
		if r.Error != nil {
			return r.Error
		}
		updatedCnt = r.RowsAffected
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	summary := formatStockSettingsOpSummary(matched, updatedCnt, body.WarningStock, body.SafetyStock, f, f.AlertTypes)
	sample := truncateUUIDList(ids, 5)
	opMsg := summary + " sampleSkuIds=" + sample
	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.stock_alert.batch_update",
			Resource:    "product_sku",
			ResourceID:  "batch",
			Status:      "success",
			Message:     clampStr(opMsg, 2000),
		})
	}
	return &StockSettingsBatchUpdateResult{
		MatchedCount: matched,
		UpdatedCount: updatedCnt,
		Summary:      summary,
	}, nil
}
