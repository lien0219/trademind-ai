package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
)

type localSKUForBinding struct {
	PublicationSKUID uuid.UUID
	ProductSKUID     *uuid.UUID
	ExternalSKUID    string
	SKUCode          string
	SpecName         string
	Attrs            map[string]string
	PriceYuan        float64
}

type skuBindingMatch struct {
	Status     string
	Confidence int
	Message    string
	Platform   *platformdouyin.PlatformProductSKU
}

// DouyinSKUBindingRow is one publication SKU binding projection.
type DouyinSKUBindingRow struct {
	PublicationSKUID uuid.UUID  `json:"publicationSkuId"`
	ProductSKUID     *uuid.UUID `json:"productSkuId,omitempty"`
	SKUCode          string     `json:"skuCode,omitempty"`
	SpecName         string     `json:"specName,omitempty"`
	ExternalSKUID    string     `json:"externalSkuId,omitempty"`
	PlatformSkuName  string     `json:"platformSkuName,omitempty"`
	BindStatus       string     `json:"bindStatus,omitempty"`
	BindConfidence   int        `json:"bindConfidence,omitempty"`
	BindMessage      string     `json:"bindMessage,omitempty"`
	LastSyncedAt     *time.Time `json:"lastSyncedAt,omitempty"`
	Price            *float64   `json:"price,omitempty"`
	Stock            *int       `json:"stock,omitempty"`
}

// DouyinSKUBindingSummary aggregates calibration outcome.
type DouyinSKUBindingSummary struct {
	PublicationID            uuid.UUID                    `json:"publicationId"`
	ExternalProductID        string                       `json:"externalProductId,omitempty"`
	SkuBindingSyncedAt       *time.Time                   `json:"skuBindingSyncedAt,omitempty"`
	Total                    int                          `json:"total"`
	Bound                    int                          `json:"bound"`
	Skipped                  int                          `json:"skipped"`
	Unmatched                int                          `json:"unmatched"`
	Ambiguous                int                          `json:"ambiguous"`
	Failed                   int                          `json:"failed"`
	Rows                     []DouyinSKUBindingRow        `json:"rows"`
	PlatformSkus             []DouyinPlatformSKUCandidate `json:"platformSkus,omitempty"`
	InventorySyncReady       bool                         `json:"inventorySyncReady"`
	InventorySyncBlockReason string                       `json:"inventorySyncBlockReason,omitempty"`
	ErrorCode                string                       `json:"errorCode,omitempty"`
	ErrorMessage             string                       `json:"errorMessage,omitempty"`
}

// GetDouyinSKUBindings returns current binding rows for one publication.
func (s *Service) GetDouyinSKUBindings(ctx context.Context, publicationID uuid.UUID) (*DouyinSKUBindingSummary, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	pub, err := s.loadDouyinPublication(ctx, publicationID)
	if err != nil {
		return nil, err
	}
	rows, err := s.listDouyinBindingRows(ctx, pub)
	if err != nil {
		return nil, err
	}
	sum := summarizeBindingRows(pub, rows)
	sum.PlatformSkus = annotatePlatformSkuCandidates(platformSkusFromPublicationRaw(pub.RawData), sum.Rows)
	sum.InventorySyncReady, sum.InventorySyncBlockReason = DouyinInventorySyncReady(sum.Rows)
	return &sum, nil
}

// SyncDouyinSKUBindings fetches product.detail and calibrates publication SKU bindings.
func (s *Service) SyncDouyinSKUBindings(c *gin.Context, publicationID uuid.UUID, adminID *uuid.UUID) (*DouyinSKUBindingSummary, error) {
	if s == nil || s.DB == nil || s.Shops == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	pub, err := s.loadDouyinPublication(ctx, publicationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(pub.ExternalProductID) == "" {
		return nil, fmt.Errorf("%s: platform product id missing", platformdouyin.CodeDouyinProductNotBound)
	}
	shopRow, err := s.ensureDouyinShopAuthorized(ctx, pub.ShopID)
	if err != nil {
		return nil, err
	}
	_ = shopRow

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "douyin.sku.binding.recheck",
			Resource:    "product_publication",
			ResourceID:  publicationID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("publicationId=%s trigger=sync-sku-bindings", publicationID),
		})
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "douyin.product.detail.sync.start",
			Resource:    "product_publication",
			ResourceID:  publicationID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("publicationId=%s productId=%s shopId=%s", publicationID, pub.ProductID, pub.ShopID),
		})
	}

	client, _, err := s.Shops.DouyinClientForShopContext(ctx, pub.ShopID, adminID)
	if err != nil {
		code := inferDouyinPublishErrorCode(err)
		s.writeDouyinDetailSyncFailed(ctx, adminID, publicationID, code, err.Error())
		return nil, fmt.Errorf("%s: %s", code, err.Error())
	}

	detail, err := client.GetProductDetail(ctx, pub.ShopID.String(), pub.ExternalProductID)
	if err != nil {
		code := inferDouyinPublishErrorCode(err)
		s.writeDouyinDetailSyncFailed(ctx, adminID, publicationID, code, err.Error())
		return nil, err
	}

	localRows, err := s.loadLocalSKUsForBinding(ctx, publicationID, pub.ProductID)
	if err != nil {
		code := platformdouyin.CodeDouyinSKUBindingSyncFailed
		s.writeDouyinDetailSyncFailed(ctx, adminID, publicationID, code, err.Error())
		return nil, fmt.Errorf("%s: %s", code, err.Error())
	}

	now := time.Now().UTC()
	usedPlatform := map[string]struct{}{}
	resultRows := make([]DouyinSKUBindingRow, 0, len(localRows))
	counts := map[string]int{}

	for _, local := range localRows {
		match := matchLocalSKU(local, detail.SKUs, usedPlatform)
		row := DouyinSKUBindingRow{
			PublicationSKUID: local.PublicationSKUID,
			ProductSKUID:     local.ProductSKUID,
			SKUCode:          local.SKUCode,
			SpecName:         local.SpecName,
			ExternalSKUID:    strings.TrimSpace(local.ExternalSKUID),
			BindStatus:       match.Status,
			BindConfidence:   match.Confidence,
			BindMessage:      match.Message,
			Price:            floatPtr(local.PriceYuan),
		}
		updates := map[string]any{
			"bind_status":     match.Status,
			"bind_confidence": match.Confidence,
			"bind_message":    match.Message,
			"last_synced_at":  &now,
			"updated_at":      now,
		}
		if match.Status == BindStatusBound && match.Platform != nil {
			ext := strings.TrimSpace(match.Platform.PlatformSKUID)
			row.ExternalSKUID = ext
			updates["external_sku_id"] = ext
			raw, _ := json.Marshal(match.Platform.Raw)
			updates["raw_data"] = datatypes.JSON(raw)
			usedPlatform[ext] = struct{}{}
			if s.OpLog != nil {
				_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
					AdminUserID: adminID,
					Action:      "douyin.sku.binding.matched",
					Resource:    "product_publication_sku",
					ResourceID:  local.PublicationSKUID.String(),
					Status:      "success",
					Message:     fmt.Sprintf("publicationId=%s platformSkuId=%s confidence=%d", publicationID, ext, match.Confidence),
				})
			}
		} else if match.Status == BindStatusSkipped {
			row.LastSyncedAt = &now
		} else if match.Status == BindStatusAmbiguous {
			if s.OpLog != nil {
				_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
					AdminUserID: adminID,
					Action:      "douyin.sku.binding.ambiguous",
					Resource:    "product_publication_sku",
					ResourceID:  local.PublicationSKUID.String(),
					Status:      "failed",
					Message:     fmt.Sprintf("publicationId=%s reason=%s", publicationID, bindingTruncateMsg(match.Message)),
				})
			}
		} else if match.Status == BindStatusUnmatched {
			if s.OpLog != nil {
				_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
					AdminUserID: adminID,
					Action:      "douyin.sku.binding.unmatched",
					Resource:    "product_publication_sku",
					ResourceID:  local.PublicationSKUID.String(),
					Status:      "failed",
					Message:     fmt.Sprintf("publicationId=%s reason=%s", publicationID, bindingTruncateMsg(match.Message)),
				})
			}
		}
		row.LastSyncedAt = &now
		_ = s.DB.WithContext(ctx).Model(&ProductPublicationSKU{}).Where("id = ?", local.PublicationSKUID).Updates(updates).Error
		resultRows = append(resultRows, row)
		counts[match.Status]++
	}

	_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", publicationID).
		Updates(map[string]any{
			"sku_binding_synced_at": &now,
			"last_synced_at":        &now,
			"raw_data":              mergePublicationRawPlatformSkus(pub.RawData, detail.SKUs),
			"updated_at":            now,
		}).Error

	sum := DouyinSKUBindingSummary{
		PublicationID:      publicationID,
		ExternalProductID:  pub.ExternalProductID,
		SkuBindingSyncedAt: &now,
		Total:              len(resultRows),
		Bound:              counts[BindStatusBound],
		Skipped:            counts[BindStatusSkipped],
		Unmatched:          counts[BindStatusUnmatched],
		Ambiguous:          counts[BindStatusAmbiguous],
		Failed:             counts[BindStatusFailed],
		Rows:               resultRows,
	}
	sum.PlatformSkus = annotatePlatformSkuCandidates(platformSkusToCandidates(detail.SKUs), sum.Rows)
	sum.InventorySyncReady, sum.InventorySyncBlockReason = DouyinInventorySyncReady(sum.Rows)
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "douyin.product.detail.sync.success",
			Resource:    "product_publication",
			ResourceID:  publicationID.String(),
			Status:      "success",
			Message: fmt.Sprintf("publicationId=%s bound=%d skipped=%d unmatched=%d ambiguous=%d",
				publicationID, sum.Bound, sum.Skipped, sum.Unmatched, sum.Ambiguous),
		})
	}
	douyinmetrics.RecordSKUAutoBound(sum.Bound)
	douyinmetrics.RecordSKUUnmatched(sum.Unmatched)
	douyinmetrics.RecordSKUAmbiguous(sum.Ambiguous)
	return &sum, nil
}

func (s *Service) writeDouyinDetailSyncFailed(ctx context.Context, adminID *uuid.UUID, publicationID uuid.UUID, code, msg string) {
	if s.OpLog == nil {
		return
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: adminID,
		Action:      "douyin.product.detail.sync.failed",
		Resource:    "product_publication",
		ResourceID:  publicationID.String(),
		Status:      "failed",
		Message:     fmt.Sprintf("publicationId=%s code=%s err=%s", publicationID, code, bindingTruncateMsg(msg)),
	})
}

func (s *Service) loadDouyinPublication(ctx context.Context, publicationID uuid.UUID) (*ProductPublication, error) {
	var pub ProductPublication
	if err := s.DB.WithContext(ctx).First(&pub, "id = ?", publicationID).Error; err != nil {
		return nil, err
	}
	if strings.TrimSpace(strings.ToLower(pub.Platform)) != "douyin_shop" {
		return nil, fmt.Errorf("publication is not douyin_shop")
	}
	return &pub, nil
}

func (s *Service) ensureDouyinShopAuthorized(ctx context.Context, shopID uuid.UUID) (*shop.Shop, error) {
	shopRow, _, err := s.Shops.PlainAuthForProviderCtx(ctx, shopID)
	if err != nil || shopRow == nil {
		return nil, fmt.Errorf("shop not found")
	}
	if strings.TrimSpace(shopRow.AuthStatus) != shop.AuthAuthorized {
		return nil, fmt.Errorf("%s: shop is not authorized", platformdouyin.CodeDouyinStoreNotAuthorized)
	}
	return shopRow, nil
}

func (s *Service) listDouyinBindingRows(ctx context.Context, pub *ProductPublication) ([]DouyinSKUBindingRow, error) {
	localRows, err := s.loadLocalSKUsForBinding(ctx, pub.ID, pub.ProductID)
	if err != nil {
		return nil, err
	}
	out := make([]DouyinSKUBindingRow, 0, len(localRows))
	for _, lr := range localRows {
		var row ProductPublicationSKU
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", lr.PublicationSKUID).Error; err != nil {
			continue
		}
		out = append(out, DouyinSKUBindingRow{
			PublicationSKUID: row.ID,
			ProductSKUID:     row.ProductSKUID,
			SKUCode:          firstNonEmpty(row.SKUCode, lr.SKUCode),
			SpecName:         lr.SpecName,
			ExternalSKUID:    strings.TrimSpace(row.ExternalSKUID),
			PlatformSkuName:  platformSkuNameFromCache(pub, strings.TrimSpace(row.ExternalSKUID)),
			BindStatus:       strings.TrimSpace(row.BindStatus),
			BindConfidence:   row.BindConfidence,
			BindMessage:      row.BindMessage,
			LastSyncedAt:     row.LastSyncedAt,
			Price:            row.Price,
			Stock:            localStockForBinding(ctx, s.DB, pub.ProductID, row.ProductSKUID),
		})
	}
	return out, nil
}

func summarizeBindingRows(pub *ProductPublication, rows []DouyinSKUBindingRow) DouyinSKUBindingSummary {
	sum := DouyinSKUBindingSummary{
		PublicationID:      pub.ID,
		ExternalProductID:  pub.ExternalProductID,
		SkuBindingSyncedAt: pub.SkuBindingSyncedAt,
		Total:              len(rows),
		Rows:               rows,
	}
	for _, r := range rows {
		switch strings.TrimSpace(r.BindStatus) {
		case BindStatusBound:
			sum.Bound++
		case BindStatusSkipped:
			sum.Skipped++
		case BindStatusAmbiguous:
			sum.Ambiguous++
		case BindStatusFailed:
			sum.Failed++
		case BindStatusUnmatched:
			sum.Unmatched++
		default:
			if strings.TrimSpace(r.ExternalSKUID) == "" {
				sum.Unmatched++
			} else {
				sum.Bound++
			}
		}
	}
	return sum
}

func (s *Service) loadLocalSKUsForBinding(ctx context.Context, publicationID, productID uuid.UUID) ([]localSKUForBinding, error) {
	var pubSkus []ProductPublicationSKU
	if err := s.DB.WithContext(ctx).Where("publication_id = ?", publicationID).Order("created_at ASC").Find(&pubSkus).Error; err != nil {
		return nil, err
	}
	out := make([]localSKUForBinding, 0, len(pubSkus))
	for _, ps := range pubSkus {
		local := localSKUForBinding{
			PublicationSKUID: ps.ID,
			ProductSKUID:     ps.ProductSKUID,
			ExternalSKUID:    strings.TrimSpace(ps.ExternalSKUID),
			SKUCode:          strings.TrimSpace(ps.SKUCode),
			PriceYuan:        derefFloat(ps.Price),
		}
		if ps.ProductSKUID != nil {
			var sku product.ProductSKU
			if err := s.DB.WithContext(ctx).First(&sku, "id = ? AND product_id = ?", *ps.ProductSKUID, productID).Error; err == nil {
				local.Attrs = attrsFromJSON(sku.Attrs)
				if local.SKUCode == "" {
					local.SKUCode = strings.TrimSpace(sku.SKUCode)
				}
				if local.PriceYuan <= 0 && sku.Price != nil {
					local.PriceYuan = *sku.Price
				}
				if local.SpecName == "" {
					local.SpecName = firstNonEmpty(strings.TrimSpace(sku.SKUName), buildSpecLabelFromAttrs(local.Attrs))
				}
			}
		}
		if local.SpecName == "" {
			local.SpecName = firstNonEmpty(local.SKUCode, buildSpecLabelFromAttrs(local.Attrs))
		}
		out = append(out, local)
	}
	return out, nil
}

func matchLocalSKU(local localSKUForBinding, platform []platformdouyin.PlatformProductSKU, used map[string]struct{}) skuBindingMatch {
	if strings.TrimSpace(local.ExternalSKUID) != "" {
		return skuBindingMatch{
			Status:     BindStatusSkipped,
			Confidence: 100,
			Message:    "platform sku id already bound",
		}
	}
	if len(platform) == 0 {
		return skuBindingMatch{
			Status:     BindStatusUnmatched,
			Confidence: 0,
			Message:    "no platform sku candidates",
		}
	}

	exact := make([]platformdouyin.PlatformProductSKU, 0)
	namePrice := make([]platformdouyin.PlatformProductSKU, 0)
	similar := make([]platformdouyin.PlatformProductSKU, 0)

	localAttrsKey := attrsKey(local.Attrs)
	localName := normalizeSpecName(local.SpecName)
	localPrice := local.PriceYuan

	for _, psku := range platform {
		ext := strings.TrimSpace(psku.PlatformSKUID)
		if ext == "" {
			continue
		}
		if _, ok := used[ext]; ok {
			continue
		}
		if localAttrsKey != "" && attrsKey(psku.Attrs) == localAttrsKey {
			exact = append(exact, psku)
			continue
		}
		pName := normalizeSpecName(psku.SpecName)
		if localName != "" && pName == localName && pricesClose(localPrice, psku.PriceYuan) {
			namePrice = append(namePrice, psku)
			continue
		}
		if localName != "" && pName != "" && specNamesSimilar(localName, pName) {
			similar = append(similar, psku)
		}
	}

	switch {
	case len(exact) == 1:
		cp := exact[0]
		return skuBindingMatch{
			Status:     BindStatusBound,
			Confidence: 95,
			Message:    "matched by identical spec attributes",
			Platform:   &cp,
		}
	case len(exact) > 1:
		return skuBindingMatch{
			Status:     BindStatusAmbiguous,
			Confidence: 50,
			Message:    fmt.Sprintf("%d platform skus match identical attributes", len(exact)),
		}
	case len(namePrice) == 1:
		cp := namePrice[0]
		return skuBindingMatch{
			Status:     BindStatusBound,
			Confidence: 85,
			Message:    "matched by spec name and price",
			Platform:   &cp,
		}
	case len(namePrice) > 1:
		return skuBindingMatch{
			Status:     BindStatusAmbiguous,
			Confidence: 50,
			Message:    fmt.Sprintf("%d platform skus match spec name and price", len(namePrice)),
		}
	case len(similar) > 0:
		return skuBindingMatch{
			Status:     BindStatusAmbiguous,
			Confidence: 45,
			Message:    fmt.Sprintf("%d platform skus have similar spec names", len(similar)),
		}
	default:
		return skuBindingMatch{
			Status:     BindStatusUnmatched,
			Confidence: 0,
			Message:    "no platform sku matched local spec",
		}
	}
}

func attrsFromJSON(raw datatypes.JSON) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return out
	}
	for k, v := range m {
		if s := strings.TrimSpace(fmt.Sprint(v)); s != "" && s != "<nil>" {
			out[strings.TrimSpace(k)] = s
		}
	}
	return out
}

func buildSpecLabelFromAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sortStrings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if v := strings.TrimSpace(attrs[k]); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, " / ")
}

func sortStrings(ss []string) {
	for i := 0; i < len(ss); i++ {
		for j := i + 1; j < len(ss); j++ {
			if ss[j] < ss[i] {
				ss[i], ss[j] = ss[j], ss[i]
			}
		}
	}
}

func attrsKey(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sortStrings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+strings.TrimSpace(attrs[k]))
	}
	return strings.Join(parts, "|")
}

func normalizeSpecName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func pricesClose(a, b float64) bool {
	if a <= 0 || b <= 0 {
		return false
	}
	return math.Abs(a-b) < 0.011
}

func specNamesSimilar(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	return levenshteinRatio(a, b) >= 0.82
}

func levenshteinRatio(a, b string) float64 {
	if a == b {
		return 1
	}
	la, lb := len(a), len(b)
	if la == 0 || lb == 0 {
		return 0
	}
	dist := levenshtein(a, b)
	maxLen := la
	if lb > maxLen {
		maxLen = lb
	}
	return 1 - float64(dist)/float64(maxLen)
}

func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func floatPtr(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	x := v
	return &x
}

func bindingTruncateMsg(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) > 240 {
		return msg[:240] + "..."
	}
	return msg
}
