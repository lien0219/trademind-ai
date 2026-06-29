package customerchat

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

// ContextSummary is a safe, user-visible AI context digest (no raw platform data).
type ContextSummary struct {
	OrderStatus       string `json:"orderStatus,omitempty"`
	SkuMatchStatus    string `json:"skuMatchStatus,omitempty"`
	InventoryStatus   string `json:"inventoryStatus,omitempty"`
	ProductTitle      string `json:"productTitle,omitempty"`
	CustomerQuestion  string `json:"customerQuestion,omitempty"`
	IncompleteWarning string `json:"incompleteWarning,omitempty"`
}

// ProductContextItem is one linked product/SKU row for conversation detail.
type ProductContextItem struct {
	ProductID     uuid.UUID `json:"productId,omitempty"`
	ProductTitle  string    `json:"productTitle,omitempty"`
	SKUCode       string    `json:"skuCode,omitempty"`
	SKUName       string    `json:"skuName,omitempty"`
	StockStatus   string    `json:"stockStatus,omitempty"`
	PublishStatus string    `json:"publishStatus,omitempty"`
	AIOpsStatus   string    `json:"aiOpsStatus,omitempty"`
}

// InventoryContextItem is stock snapshot for order-linked SKU.
type InventoryContextItem struct {
	SKUCode     string `json:"skuCode,omitempty"`
	SKUName     string `json:"skuName,omitempty"`
	Stock       *int   `json:"stock,omitempty"`
	StockStatus string `json:"stockStatus,omitempty"`
	BindStatus  string `json:"bindStatus,omitempty"`
}

func (s *Service) buildContextSummary(c *gin.Context, conv *CustomerConversation, customerMsg string) ContextSummary {
	out := ContextSummary{CustomerQuestion: truncateRunes(strings.TrimSpace(customerMsg), 120)}
	if conv == nil {
		out.IncompleteWarning = "缺少会话信息，AI 建议可能不完整"
		return out
	}
	if conv.OrderID == nil || s.Orders == nil {
		out.IncompleteWarning = "未关联订单，AI 建议可能不完整"
		return out
	}
	sum, err := s.Orders.ConversationSummary(c, *conv.OrderID)
	if err != nil || sum == nil {
		out.IncompleteWarning = "订单上下文不可用，AI 建议可能不完整"
		return out
	}
	out.OrderStatus = humanOrderStatus(sum.Status)
	out.SkuMatchStatus = humanSkuMatchStatus(sum.SkuMatchStatus)
	out.InventoryStatus = humanInvDeductStatus(sum.InventoryDeductStatus)
	if sum.ItemCount > 0 {
		out.ProductTitle = s.firstOrderProductTitle(c, *conv.OrderID)
	}
	if out.ProductTitle == "" {
		out.IncompleteWarning = "缺少商品上下文，AI 建议可能不完整"
	}
	return out
}

func (s *Service) buildProductContexts(c *gin.Context, orderID uuid.UUID) []ProductContextItem {
	if s == nil || s.DB == nil || orderID == uuid.Nil {
		return nil
	}
	var items []order.OrderItem
	if err := s.DB.WithContext(c.Request.Context()).Where("order_id = ?", orderID).Find(&items).Error; err != nil || len(items) == 0 {
		return nil
	}
	out := make([]ProductContextItem, 0, len(items))
	for _, it := range items {
		row := ProductContextItem{
			ProductTitle: it.ProductTitle,
			SKUCode:      it.SKUCode,
			SKUName:      it.SKUName,
		}
		if it.ProductID != nil {
			row.ProductID = *it.ProductID
			row.PublishStatus, row.AIOpsStatus = s.productOpsLabels(c, *it.ProductID)
		}
		row.StockStatus = s.skuStockStatus(c, it.SKUCode)
		out = append(out, row)
	}
	return out
}

func (s *Service) buildInventoryContexts(c *gin.Context, orderID uuid.UUID) []InventoryContextItem {
	if s == nil || s.DB == nil || orderID == uuid.Nil {
		return nil
	}
	type row struct {
		SKUCode     string `gorm:"column:sku_code"`
		SKUName     string `gorm:"column:sku_name"`
		Stock       *int   `gorm:"column:stock"`
		MatchStatus string `gorm:"column:match_status"`
	}
	var rows []row
	_ = s.DB.WithContext(c.Request.Context()).Raw(`
SELECT oi.sku_code, oi.sku_name, ps.stock, COALESCE(m.match_status,'') AS match_status
FROM order_items oi
LEFT JOIN order_item_sku_matches m ON m.order_item_id = oi.id
LEFT JOIN product_skus ps ON ps.id = m.product_sku_id
WHERE oi.order_id = ?
ORDER BY oi.created_at ASC
`, orderID).Scan(&rows).Error
	if len(rows) == 0 {
		return nil
	}
	out := make([]InventoryContextItem, 0, len(rows))
	for _, r := range rows {
		st := product.CalculateSKUStockStatus(derefInt(r.Stock), 0, 0)
		out = append(out, InventoryContextItem{
			SKUCode:     r.SKUCode,
			SKUName:     r.SKUName,
			Stock:       r.Stock,
			StockStatus: humanStockStatus(st),
			BindStatus:  humanSkuMatchStatus(r.MatchStatus),
		})
	}
	return out
}

func (s *Service) firstOrderProductTitle(c *gin.Context, orderID uuid.UUID) string {
	var it order.OrderItem
	if err := s.DB.WithContext(c.Request.Context()).Where("order_id = ?", orderID).Order("created_at ASC").First(&it).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(it.ProductTitle)
}

func (s *Service) productOpsLabels(c *gin.Context, productID uuid.UUID) (publish, aiOps string) {
	var p product.Product
	if err := s.DB.WithContext(c.Request.Context()).Select("status", "ai_title").First(&p, "id = ?", productID).Error; err != nil {
		return "—", "—"
	}
	publish = strings.TrimSpace(p.Status)
	if publish == "" {
		publish = "draft"
	}
	if strings.TrimSpace(p.AITitle) != "" {
		aiOps = "ai_title_ready"
	} else {
		aiOps = "pending_ai"
	}
	return publish, aiOps
}

func (s *Service) skuStockStatus(c *gin.Context, skuCode string) string {
	code := strings.TrimSpace(skuCode)
	if code == "" || s.DB == nil {
		return "—"
	}
	var ps product.ProductSKU
	if err := s.DB.WithContext(c.Request.Context()).Where("sku_code = ?", code).First(&ps).Error; err != nil {
		return "—"
	}
	return humanStockStatus(product.CalculateSKUStockStatus(derefInt(ps.Stock), ps.WarningStock, ps.SafetyStock))
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func humanOrderStatus(st string) string {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case order.StatusPaid:
		return "已付款"
	case order.StatusProcessing:
		return "处理中"
	case order.StatusShipped:
		return "已发货"
	case order.StatusDelivered:
		return "已送达"
	case order.StatusCancelled:
		return "已取消"
	case order.StatusRefunded:
		return "已退款"
	default:
		if st == "" {
			return "—"
		}
		return st
	}
}

func humanSkuMatchStatus(st string) string {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case order.ListSkuMatchAllMatched, "matched", "manual_bound":
		return "已匹配"
	case order.ListSkuMatchPartial:
		return "部分匹配"
	case order.ListSkuMatchUnmatched:
		return "未匹配"
	case order.ListSkuMatchAmbiguous:
		return "匹配歧义"
	case order.ListSkuMatchNone:
		return "无 SKU 行"
	default:
		if st == "" {
			return "—"
		}
		return st
	}
}

func humanInvDeductStatus(st string) string {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case order.ListInvDeductSuccess:
		return "已扣减"
	case order.ListInvDeductFailed:
		return "扣减失败"
	case order.ListInvDeductPartial:
		return "部分扣减"
	case order.ListInvDeductBlocked:
		return "已阻断"
	case order.ListInvDeductNone:
		return "未扣减"
	default:
		if st == "" {
			return "—"
		}
		return st
	}
}

func humanStockStatus(st string) string {
	switch strings.TrimSpace(st) {
	case product.StockStatusNormal:
		return "库存充足"
	case product.StockStatusLowStock:
		return "库存偏低"
	case product.StockStatusBelowSafetyStock:
		return "低于安全库存"
	case product.StockStatusOutOfStock:
		return "缺货"
	default:
		if st == "" {
			return "—"
		}
		return st
	}
}
