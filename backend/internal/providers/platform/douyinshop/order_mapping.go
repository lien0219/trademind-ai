package douyinshop

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	MethodOrderSearchList = "order.searchList"
)

// MapOrderStatus maps Douyin order_status / main_status to local order status tokens.
func MapOrderStatus(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "unknown"
	}
	switch s {
	case "1", "1010":
		return "pending"
	case "105", "10105":
		return "paid"
	case "2", "102":
		return "processing"
	case "101", "103":
		return "processing"
	case "3", "104":
		return "shipped"
	case "5", "1055":
		return "closed"
	case "4", "21", "22", "39":
		return "cancelled"
	default:
		low := strings.ToLower(s)
		switch {
		case strings.Contains(low, "cancel"):
			return "cancelled"
		case strings.Contains(low, "finish") || strings.Contains(low, "complete"):
			return "closed"
		case strings.Contains(low, "ship"):
			return "shipped"
		case strings.Contains(low, "pay"):
			return "paid"
		case strings.Contains(low, "wait") || strings.Contains(low, "pending"):
			return "pending"
		default:
			return "unknown"
		}
	}
}

func MapPaymentStatus(orderStatus string) string {
	st := MapOrderStatus(orderStatus)
	switch st {
	case "pending", "unknown":
		return "unpaid"
	case "cancelled":
		return "unpaid"
	case "closed", "shipped", "processing", "paid":
		return "paid"
	default:
		return "unpaid"
	}
}

func MapFulfillmentStatus(orderStatus string) string {
	st := MapOrderStatus(orderStatus)
	switch st {
	case "shipped", "closed":
		return "fulfilled"
	case "processing":
		return "partial"
	case "cancelled":
		return "unfulfilled"
	default:
		return "unfulfilled"
	}
}

func MapShipmentStatus(raw string) string {
	st := MapOrderStatus(raw)
	switch st {
	case "shipped", "closed":
		return "shipped"
	case "processing":
		return "pending"
	default:
		return "pending"
	}
}

func parseMoneyCent(v any) float64 {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return n / 100.0
	case float64:
		return t / 100.0
	case int64:
		return float64(t) / 100.0
	case int:
		return float64(t) / 100.0
	default:
		return 0
	}
}

func parseUnixSec(v any) *time.Time {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || n <= 0 {
			return nil
		}
		tt := time.Unix(n, 0).UTC()
		return &tt
	case float64:
		if t <= 0 {
			return nil
		}
		sec := int64(t)
		if sec > 1_000_000_000_000 {
			sec = sec / 1000
		}
		tt := time.Unix(sec, 0).UTC()
		return &tt
	default:
		return nil
	}
}

func pickStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func pickInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil && n > 0 {
					return n
				}
			case float64:
				if int(t) > 0 {
					return int(t)
				}
			}
		}
	}
	return 0
}

func maskCustomerName(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits >= 7 && len(s) <= 20 {
		if len(s) <= 4 {
			return "买家****"
		}
		return s[:2] + "****" + s[len(s)-2:]
	}
	if len(s) > 24 {
		return s[:24] + "…"
	}
	return s
}

func compactOrderRaw(orderStatus, orderID string, warnings []string) map[string]any {
	out := map[string]any{
		"source":              "douyin_shop",
		"platformStatus":      strings.TrimSpace(orderStatus),
		"externalOrderIdHint": strings.TrimSpace(orderID),
	}
	if len(warnings) > 0 {
		out["warnings"] = warnings
	}
	return out
}

func compactItemRaw(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	allowed := []string{"product_id", "sku_id", "code", "out_sku_id", "order_status", "main_status"}
	out := map[string]any{}
	for _, k := range allowed {
		if v, ok := m[k]; ok {
			out[k] = v
		}
	}
	if len(out) == 0 {
		out["truncated"] = true
	}
	return out
}

func skuNameFromSpecs(m map[string]any) string {
	raw, ok := m["sku_specs"].([]any)
	if !ok || len(raw) == 0 {
		return pickStr(m, "spec_desc", "sku_name")
	}
	parts := make([]string, 0, len(raw))
	for _, ent := range raw {
		sm, ok := ent.(map[string]any)
		if !ok {
			continue
		}
		name := pickStr(sm, "name", "spec_name")
		val := pickStr(sm, "value", "spec_value")
		if name != "" && val != "" {
			parts = append(parts, name+":"+val)
		} else if val != "" {
			parts = append(parts, val)
		}
	}
	return strings.Join(parts, " / ")
}

func mapDouyinOrder(m map[string]any) platformp.PlatformOrder {
	ext := pickStr(m, "order_id")
	orderStatus := pickStr(m, "order_status", "main_status")
	warnings := []string{}
	localStatus := MapOrderStatus(orderStatus)
	if localStatus == "unknown" && orderStatus != "" {
		warnings = append(warnings, "unknown_order_status:"+orderStatus)
	}

	total := parseMoneyCent(m["pay_amount"])
	if total <= 0 {
		total = parseMoneyCent(m["order_amount"])
	}
	if total < 0 {
		warnings = append(warnings, "invalid_order_amount")
	}

	customer := maskCustomerName(pickStr(m, "user_nick_name", "buyer_words"))
	if customer == "" {
		customer = "抖店买家"
	}

	o := platformp.PlatformOrder{
		ExternalOrderID:   ext,
		OrderNo:           ext,
		CustomerName:      customer,
		Status:            localStatus,
		PaymentStatus:     MapPaymentStatus(orderStatus),
		FulfillmentStatus: MapFulfillmentStatus(orderStatus),
		Currency:          "CNY",
		TotalAmount:       total,
		OrderedAt:         parseUnixSec(m["create_time"]),
		PaidAt:            parseUnixSec(m["pay_time"]),
		ShippedAt:         parseUnixSec(m["ship_time"]),
		DeliveredAt:       parseUnixSec(m["finish_time"]),
		RawData:           compactOrderRaw(orderStatus, ext, warnings),
	}
	o.Items = mapDouyinLineItems(m)
	o.Shipments = mapDouyinShipments(m, orderStatus)
	return o
}

func mapDouyinLineItems(order map[string]any) []platformp.PlatformOrderItem {
	var blocks []any
	if v, ok := order["sku_order_list"].([]any); ok {
		blocks = v
	}
	out := make([]platformp.PlatformOrderItem, 0, len(blocks))
	for idx, blk := range blocks {
		im, ok := blk.(map[string]any)
		if !ok {
			continue
		}
		extItem := pickStr(im, "order_id", "sku_order_id")
		if extItem == "" {
			extItem = fmt.Sprintf("%s-%d", pickStr(order, "order_id"), idx)
		}
		productID := pickStr(im, "product_id", "product_id_str")
		skuID := pickStr(im, "sku_id")
		qty := pickInt(im, "item_num", "product_count")
		if qty < 1 {
			qty = 1
		}
		unit := parseMoneyCent(im["origin_amount"])
		if unit <= 0 {
			unit = parseMoneyCent(im["pay_amount"])
		}
		total := unit * float64(qty)
		if tp := parseMoneyCent(im["pay_amount"]); tp > 0 {
			total = tp
		}
		code := pickStr(im, "code", "out_sku_id", "outer_sku_id")
		title := pickStr(im, "product_name")
		skuName := skuNameFromSpecs(im)
		attrs := map[string]any{}
		if productID != "" {
			attrs["platformProductId"] = productID
		}
		if skuID != "" {
			attrs["platformSkuId"] = skuID
		}
		out = append(out, platformp.PlatformOrderItem{
			ExternalItemID: extItem,
			ExternalSKUID:  skuID,
			SellerSKU:      code,
			ProductTitle:   title,
			SKUName:        skuName,
			SKUCode:        code,
			Quantity:       qty,
			UnitPrice:      unit,
			TotalPrice:     total,
			Attrs:          attrs,
			RawData:        compactItemRaw(im),
		})
	}
	return out
}

func mapDouyinShipments(order map[string]any, orderStatus string) []platformp.PlatformShipment {
	var blocks []any
	if v, ok := order["logistics_info"].([]any); ok {
		blocks = v
	}
	out := make([]platformp.PlatformShipment, 0, len(blocks))
	for _, blk := range blocks {
		sm, ok := blk.(map[string]any)
		if !ok {
			continue
		}
		carrier := pickStr(sm, "company_name", "company")
		tno := pickStr(sm, "tracking_no", "delivery_id")
		if carrier == "" && tno == "" {
			continue
		}
		out = append(out, platformp.PlatformShipment{
			Carrier:    carrier,
			TrackingNo: tno,
			Status:     MapShipmentStatus(orderStatus),
			ShippedAt:  parseUnixSec(sm["ship_time"]),
			RawData:    sanitizeRawMap(map[string]any{"trackingNo": tno, "carrier": carrier}),
		})
	}
	return out
}

func parsePageCursor(cursor string) int {
	cur := strings.TrimSpace(cursor)
	if cur == "" {
		return 0
	}
	n, err := strconv.Atoi(cur)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func formatPageCursor(page int) string {
	return strconv.Itoa(page)
}
