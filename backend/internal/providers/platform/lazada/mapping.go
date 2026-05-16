package lazada

import (
	"fmt"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func mapOrder(in map[string]any) platformp.PlatformOrder {
	if in == nil {
		return platformp.PlatformOrder{Status: "processing", RawData: map[string]any{"source": "lazada", "note": "empty"}}
	}
	ext := firstNonEmpty(
		pickStr(in, "order_id"),
		fmt.Sprint(in["order_id"]),
	)
	onum := firstNonEmpty(pickStr(in, "order_number"), pickStr(in, "order_no"), ext)

	cust := customerName(in)
	stRaw := strings.ToLower(strings.TrimSpace(orderStatusCSV(in)))
	payRaw := strings.ToLower(strings.TrimSpace(pickStr(in, "payment_method", "pay_status")))
	os, ps, fs := mapStatuses(stRaw, payRaw, pickStr(in, "fulfillment_status"))

	curr := strings.ToUpper(firstNonEmpty(pickStr(in, "currency", "currency_code"), "USD"))
	total := toFloatPick(in, "price", "total_amount", "grand_total", "order_amount")

	orderedAt := parseTimePick(in, "created_at", "create_time", "created_time")
	paidAt := parseTimePick(in, "pay_time", "paid_at", "payment_time")
	shipAt := parseTimePick(in, "shipment_date", "shipped_at", "shipping_time", "promise_shipping_time")
	delivAt := parseTimePick(in, "delivered_at", "delivery_time", "order_completed_time", "completed_time")

	items := mapItems(in)
	ships := mapShipments(in)

	raw := trimOrderRaw(in, stRaw)

	return platformp.PlatformOrder{
		ExternalOrderID:   ext,
		OrderNo:           firstNonEmpty(onum, ext),
		CustomerName:      cust,
		Status:            os,
		PaymentStatus:     ps,
		FulfillmentStatus: fs,
		Currency:          curr,
		TotalAmount:       total,
		OrderedAt:         orderedAt,
		PaidAt:            paidAt,
		ShippedAt:         shipAt,
		DeliveredAt:       delivAt,
		Items:             items,
		Shipments:         ships,
		RawData:           raw,
	}
}

func customerName(in map[string]any) string {
	fn := pickStr(in, "customer_first_name", "first_name")
	ln := pickStr(in, "customer_last_name", "last_name")
	if fn != "" || ln != "" {
		return strings.TrimSpace(strings.TrimSpace(fn + " " + ln))
	}
	return pickStr(in, "customer_name", "buyer_name", "buyer", "recipient_name")
}

func toFloatPick(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			f := toFloatAny(v)
			if f != 0 {
				return f
			}
		}
	}
	return 0
}

func toFloatAny(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		_, _ = fmt.Sscanf(strings.TrimSpace(t), "%f", &f)
		return f
	default:
		var f float64
		_, _ = fmt.Sscanf(strings.TrimSpace(fmt.Sprint(t)), "%f", &f)
		return f
	}
}

func parseTimePick(m map[string]any, keys ...string) *time.Time {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if t := parseTimeAny(v); t != nil {
				return t
			}
		}
	}
	return nil
}

func parseTimeAny(v any) *time.Time {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
			if tt, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
				utc := tt.UTC()
				return &utc
			}
		}
		if n, err := parseInt64String(s); err == nil {
			return unixMillisToTime(n)
		}
	case float64:
		tt := unixMillisToTime(int64(t))
		if tt != nil {
			return tt
		}
	}
	return nil
}

func parseInt64String(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}

func unixMillisToTime(ms int64) *time.Time {
	switch {
	case ms <= 0:
		return nil
	case ms < 1e12:
		tt := time.Unix(ms, 0).UTC()
		return &tt
	default:
		tt := time.UnixMilli(ms).UTC()
		return &tt
	}
}

func mapStatuses(orderRaw, payRaw, fulRaw string) (order, pay, fulfill string) {
	o := strings.TrimSpace(orderRaw)
	p := strings.TrimSpace(payRaw)
	f := strings.TrimSpace(fulRaw)

	switch {
	case strings.Contains(o, "cancel"):
		order = "cancelled"
	case strings.Contains(o, "return") || strings.Contains(o, "refund"):
		order = "refunded"
	case strings.Contains(o, "delivered") || strings.Contains(o, "complete") || strings.Contains(o, "finished"):
		order = "delivered"
	case strings.Contains(o, "ship") || strings.Contains(o, "ready_to_ship") || strings.Contains(o, "packed") || strings.Contains(o, "toship") || strings.Contains(o, "rts"):
		order = "shipped"
	case strings.Contains(o, "pending") || strings.Contains(o, "unpaid") || strings.Contains(o, "payment"):
		order = "pending"
	case strings.Contains(o, "paid") || strings.Contains(o, "processing") || o == "":
		order = "processing"
	default:
		order = "processing"
	}

	switch {
	case strings.Contains(p, "refund"):
		pay = "refunded"
	case strings.Contains(p, "partial"):
		pay = "partially_refunded"
	case strings.Contains(p, "paid") || strings.Contains(p, "cod") || strings.Contains(o, "paid"):
		pay = "paid"
	case strings.Contains(p, "unpaid"):
		pay = "unpaid"
	default:
		if strings.Contains(o, "unpaid") || strings.Contains(o, "pending") {
			pay = "unpaid"
		} else {
			pay = "paid"
		}
	}

	switch {
	case strings.Contains(f, "return"):
		fulfill = "returned"
	case strings.Contains(f, "fulfill") || strings.Contains(o, "delivered"):
		fulfill = "fulfilled"
	case strings.Contains(f, "partial"):
		fulfill = "partial"
	case strings.Contains(o, "ship"):
		fulfill = "partial"
	case strings.Contains(o, "cancel"):
		fulfill = "unfulfilled"
	default:
		if strings.Contains(o, "unpaid") || strings.Contains(o, "pending") {
			fulfill = "unfulfilled"
		} else {
			fulfill = "unfulfilled"
		}
	}

	return order, pay, fulfill
}

func mapItems(in map[string]any) []platformp.PlatformOrderItem {
	if ov, ok := in["__items_override"].([]any); ok {
		out := make([]platformp.PlatformOrderItem, 0, len(ov))
		for _, it := range ov {
			im, ok2 := it.(map[string]any)
			if !ok2 {
				continue
			}
			out = append(out, mapOneItem(im))
		}
		if len(out) > 0 {
			return out
		}
	}
	for _, k := range []string{"items", "order_items", "orderItems", "line_items"} {
		if arr, ok := in[k].([]any); ok {
			out := make([]platformp.PlatformOrderItem, 0, len(arr))
			for _, it := range arr {
				im, ok2 := it.(map[string]any)
				if !ok2 {
					continue
				}
				out = append(out, mapOneItem(im))
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func mapOneItem(im map[string]any) platformp.PlatformOrderItem {
	ext := firstNonEmpty(pickStr(im, "order_item_id", "item_id", "sku_id"), fmt.Sprint(im["order_item_id"]))
	title := pickStr(im, "name", "product_name", "sku")
	sku := firstNonEmpty(pickStr(im, "sku", "shop_sku", "seller_sku"), pickStr(im, "Sku"))
	qty := int(toFloatPick(im, "quantity", "qty"))
	unit := toFloatPick(im, "item_price", "paid_price", "price")
	total := toFloatPick(im, "item_total", "total", "paid_price", "price")
	if total == 0 && unit != 0 && qty > 0 {
		total = unit * float64(qty)
	}
	img := pickStr(im, "product_image_url", "image_url", "sku_image")
	return platformp.PlatformOrderItem{
		ExternalItemID: ext,
		ProductTitle:   title,
		SKUName:        pickStr(im, "variation", "sku_name"),
		SKUCode:        sku,
		Quantity:       qty,
		UnitPrice:      unit,
		TotalPrice:     total,
		ImageURL:       img,
		Attrs:          map[string]any{"raw_status": pickStr(im, "status", "item_status")},
		RawData:        trimItemRaw(im),
	}
}

func mapShipments(in map[string]any) []platformp.PlatformShipment {
	var out []platformp.PlatformShipment
	for _, k := range []string{"packages", "tracking_list", "shipment_list", "package_list"} {
		if arr, ok := in[k].([]any); ok {
			for _, ent := range arr {
				pm, ok2 := ent.(map[string]any)
				if !ok2 {
					continue
				}
				out = append(out, platformp.PlatformShipment{
					Carrier:     pickStr(pm, "shipping_provider", "carrier", "provider"),
					TrackingNo:  pickStr(pm, "tracking_number", "tracking_no"),
					TrackingURL: pickStr(pm, "tracking_url"),
					Status:      mapShipmentStatus(pickStr(pm, "status")),
					ShippedAt:   parseTimePick(pm, "shipped_at", "shipping_time"),
					DeliveredAt: parseTimePick(pm, "delivered_at", "delivery_time"),
					RawData:     trimShipmentRaw(pm),
				})
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	if tn := pickStr(in, "tracking_number", "tracking_no"); tn != "" {
		out = append(out, platformp.PlatformShipment{
			Carrier:    pickStr(in, "shipping_provider", "shipment_provider"),
			TrackingNo: tn,
			Status:     mapShipmentStatus(pickStr(in, "shipment_status")),
			RawData:    trimShipmentRaw(in),
		})
	}
	return out
}

func mapShipmentStatus(s string) string {
	ls := strings.ToLower(strings.TrimSpace(s))
	switch {
	case ls == "":
		return "pending"
	case strings.Contains(ls, "deliver"):
		return "delivered"
	case strings.Contains(ls, "return"):
		return "returned"
	case strings.Contains(ls, "transit") || strings.Contains(ls, "shipping"):
		return "in_transit"
	case strings.Contains(ls, "fail") || strings.Contains(ls, "exception"):
		return "exception"
	default:
		return "shipped"
	}
}

func trimOrderRaw(in map[string]any, st string) map[string]any {
	return map[string]any{
		"source":       "lazada",
		"order_id":     pickStr(in, "order_id"),
		"order_number": pickStr(in, "order_number"),
		"status":       st,
		"currency":     pickStr(in, "currency"),
		"total_hint":   toFloatPick(in, "price", "total_amount"),
	}
}

func trimItemRaw(im map[string]any) map[string]any {
	return map[string]any{
		"order_item_id": pickStr(im, "order_item_id"),
		"sku":           pickStr(im, "sku", "shop_sku"),
		"qty":           toFloatPick(im, "quantity"),
	}
}

func trimShipmentRaw(pm map[string]any) map[string]any {
	return map[string]any{
		"tracking": pickStr(pm, "tracking_number", "tracking_no"),
		"carrier":  pickStr(pm, "shipping_provider", "carrier"),
	}
}
