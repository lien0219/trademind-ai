package shopee

import (
	"fmt"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func mapOrder(in map[string]any) platformp.PlatformOrder {
	if in == nil {
		return platformp.PlatformOrder{Status: "processing", RawData: map[string]any{"source": "shopee", "note": "empty payload"}}
	}
	sn := pickStr(in, "order_sn", "ordersn")
	extID := pickStr(in, "order_id", "ordersn", "order_sn")
	if extID == "" {
		extID = sn
	}
	buyer := pickStr(in, "buyer_username", "buyer_user_name", "buyer_name")
	st := strings.TrimSpace(pickStr(in, "order_status"))
	paySt := strings.TrimSpace(pickStr(in, "payment_status", "pay_status"))
	fulSt := inferFulfillment(st, pickStr(in, "fulfillment_status", "fulfillment_flag"))

	os, ps, fs := mapStatuses(st, paySt, fulSt)
	curr := strings.ToUpper(pickStr(in, "currency"))
	if curr == "" {
		curr = pickStr(in, "order_currency")
	}
	total := toFloat(in["total_amount"], in["order_total_price"], in["total_price"])

	var orderedAt, paidAt, shipAt, delivAt *time.Time
	if t := parseUnixAny(in["create_time"]); t != nil {
		orderedAt = t
	}
	if t := parseUnixAny(in["pay_time"]); t != nil {
		paidAt = t
	} else if t := parseUnixAny(in["payment_time"]); t != nil {
		paidAt = t
	}
	shipAt = parseUnixAny(in["ship_time"])
	if shipAt == nil {
		shipAt = parseUnixAny(in["shipping_time"])
	}
	if shipAt == nil {
		shipAt = parseUnixAny(in["pickup_done_time"])
	}
	if shipAt == nil {
		if pl, ok := in["package_list"].([]any); ok && len(pl) > 0 {
			if pm, ok2 := pl[0].(map[string]any); ok2 {
				shipAt = parseUnixAny(pm["shipping_carrier_pickup_time"])
			}
		}
	}
	delivAt = parseUnixAny(in["order_completed_time"])
	if delivAt == nil {
		delivAt = parseUnixAny(in["complete_time"])
	}
	if delivAt == nil {
		delivAt = parseUnixAny(in["delivered_time"])
	}

	items := mapItems(in)
	ships := mapShipments(in)

	raw := trimOrderRaw(in, st)

	return platformp.PlatformOrder{
		ExternalOrderID:   extID,
		OrderNo:           sn,
		CustomerName:      buyer,
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

func inferFulfillment(orderStatus, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	s := strings.ToUpper(orderStatus)
	switch {
	case strings.Contains(s, "READY") || strings.Contains(s, "PROCESSED"):
		return "processing"
	case strings.Contains(s, "SHIPPED") || strings.Contains(s, "SHIPPING"):
		return "shipped"
	case strings.Contains(s, "COMPLETED"):
		return "fulfilled"
	}
	return ""
}

func mapStatuses(orderStatus, payRaw, fulRaw string) (order, pay, fulfill string) {
	o := strings.ToUpper(strings.TrimSpace(orderStatus))
	p := strings.ToUpper(strings.TrimSpace(payRaw))
	f := strings.ToUpper(strings.TrimSpace(fulRaw))

	switch {
	case o == "":
		order = "processing"
	case strings.Contains(o, "CANCEL"):
		order = "cancelled"
	case strings.Contains(o, "REFUND") || strings.Contains(o, "RETURN"):
		order = "refunded"
	case strings.Contains(o, "COMPLETED") || strings.Contains(o, "COMPLETE"):
		order = "closed"
	case strings.Contains(o, "SHIP") || strings.Contains(o, "DELIVER") || strings.Contains(o, "PICKUP"):
		order = "shipped"
	case strings.Contains(o, "READY") || strings.Contains(o, "PROCESSED") || strings.Contains(o, "TO_SHIP"):
		order = "processing"
	case strings.Contains(o, "UNPAID") || o == "UNPAID":
		order = "pending"
	default:
		order = "processing"
	}

	switch {
	case p != "":
		pay = mapPayment(p)
	case strings.Contains(o, "UNPAID"):
		pay = "unpaid"
	case strings.Contains(o, "RETURN") || strings.Contains(o, "REFUND"):
		pay = "refunded"
	default:
		pay = "paid"
	}

	fulfill = "unfulfilled"
	if f != "" {
		fulfill = mapFulfillment(f)
	} else {
		switch {
		case strings.Contains(o, "UNPAID"):
			fulfill = "unfulfilled"
		case strings.Contains(o, "SHIP") || strings.Contains(o, "DELIVER") || strings.Contains(o, "COMPLETED"):
			fulfill = "fulfilled"
		case strings.Contains(o, "READY") || strings.Contains(o, "PROCESSED"):
			fulfill = "partial"
		}
	}
	return order, pay, fulfill
}

func mapPayment(p string) string {
	p = strings.ToUpper(p)
	switch {
	case strings.Contains(p, "REFUND"):
		return "refunded"
	case strings.Contains(p, "PARTIAL"):
		return "partially_refunded"
	case strings.Contains(p, "UNPAID"):
		return "unpaid"
	default:
		return "paid"
	}
}

func mapFulfillment(f string) string {
	switch {
	case strings.Contains(f, "RETURN"):
		return "returned"
	case strings.Contains(f, "PART"):
		return "partial"
	case strings.Contains(f, "FULFILL") || strings.Contains(f, "DONE"):
		return "fulfilled"
	default:
		return "unfulfilled"
	}
}

func mapItems(in map[string]any) []platformp.PlatformOrderItem {
	out := []platformp.PlatformOrderItem{}
	rawList, ok := in["item_list"].([]any)
	if !ok || len(rawList) == 0 {
		return out
	}
	for _, it := range rawList {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		ext := pickStr(m, "order_item_id", "item_id", "itemid")
		title := pickStr(m, "item_name", "item_sku", "model_name")
		skuName := pickStr(m, "model_name", "variation_name")
		skuCode := pickStr(m, "model_sku", "item_sku", "sku")
		qty := int(toFloat(m["model_quantity_purchased"], m["quantity"]))
		price := toFloat(m["model_discounted_price"], m["model_original_price"], m["discounted_price"])
		line := toFloat(m["subtotal"])
		if line <= 0 && qty > 0 && price > 0 {
			line = price * float64(qty)
		}
		img := ""
		if im, ok := m["image_info"].(map[string]any); ok {
			img = pickStr(im, "image_url")
		}
		if img == "" {
			img = pickStr(m, "image_url", "product_image_url")
		}
		attrs := map[string]any{}
		if s := pickStr(m, "variation_name"); s != "" {
			attrs["variation"] = s
		}
		extSku := pickStr(m, "model_id", "item_id", "variation_id")
		sellerSku := pickStr(m, "model_sku", "item_sku", "sku")
		out = append(out, platformp.PlatformOrderItem{
			ExternalItemID: ext,
			ExternalSKUID:  extSku,
			SellerSKU:      sellerSku,
			ProductTitle:   title,
			SKUName:        skuName,
			SKUCode:        skuCode,
			Quantity:       qty,
			UnitPrice:      price,
			TotalPrice:     line,
			ImageURL:       img,
			Attrs:          attrs,
			RawData:        trimItemRaw(m),
		})
	}
	return out
}

func mapShipments(in map[string]any) []platformp.PlatformShipment {
	out := []platformp.PlatformShipment{}
	pl, ok := in["package_list"].([]any)
	if !ok {
		return out
	}
	for _, p := range pl {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		carrier := pickStr(pm, "shipping_carrier", "carrier_name")
		track := pickStr(pm, "tracking_number", "tracking_no")
		stat := pickStr(pm, "package_status", "logistics_status")
		mStat := mapShipmentStatus(stat)
		sAt := parseUnixAny(pm["shipping_carrier_pickup_time"])
		if sAt == nil {
			sAt = parseUnixAny(pm["pickup_time"])
		}
		dAt := parseUnixAny(pm["delivered_time"])
		if dAt == nil {
			dAt = parseUnixAny(pm["delivery_time"])
		}
		url := ""
		out = append(out, platformp.PlatformShipment{
			Carrier:     carrier,
			TrackingNo:  track,
			TrackingURL: url,
			Status:      mStat,
			ShippedAt:   sAt,
			DeliveredAt: dAt,
			RawData:     trimShipRaw(pm),
		})
	}
	if len(out) == 0 {
		return out
	}
	return out
}

func mapShipmentStatus(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case s == "":
		return "pending"
	case strings.Contains(s, "DELIVER"):
		return "delivered"
	case strings.Contains(s, "TRANSIT") || strings.Contains(s, "SHIP"):
		return "in_transit"
	case strings.Contains(s, "RETURN"):
		return "returned"
	case strings.Contains(s, "FAIL") || strings.Contains(s, "EXCEPTION"):
		return "exception"
	default:
		return "shipped"
	}
}

func trimOrderRaw(in map[string]any, origStatus string) map[string]any {
	return map[string]any{
		"source":         "shopee",
		"order_sn":       pickStr(in, "order_sn"),
		"order_status":   origStatus,
		"payment_status": pickStr(in, "payment_status"),
		"currency":       pickStr(in, "currency"),
		"create_time":    in["create_time"],
	}
}

func trimItemRaw(m map[string]any) map[string]any {
	return map[string]any{
		"item_id": pickStr(m, "item_id"),
		"sku":     pickStr(m, "item_sku", "model_sku"),
		"qty":     m["model_quantity_purchased"],
	}
}

func trimShipRaw(m map[string]any) map[string]any {
	return map[string]any{
		"tracking_number": pickStr(m, "tracking_number"),
		"package_status":  pickStr(m, "package_status"),
	}
}

func toFloat(vals ...any) float64 {
	for _, v := range vals {
		switch t := v.(type) {
		case float64:
			return t
		case int64:
			return float64(t)
		case int:
			return float64(t)
		case string:
			var x float64
			_, _ = fmt.Sscanf(strings.TrimSpace(t), "%f", &x)
			if x != 0 {
				return x
			}
		}
	}
	return 0
}

func parseUnixAny(v any) *time.Time {
	switch t := v.(type) {
	case float64:
		if t <= 0 {
			return nil
		}
		x := time.Unix(int64(t), 0).UTC()
		return &x
	case int64:
		if t <= 0 {
			return nil
		}
		x := time.Unix(t, 0).UTC()
		return &x
	case string:
		n := 0.0
		_, _ = fmt.Sscanf(strings.TrimSpace(t), "%f", &n)
		if n <= 0 {
			return nil
		}
		x := time.Unix(int64(n), 0).UTC()
		return &x
	default:
		return nil
	}
}
