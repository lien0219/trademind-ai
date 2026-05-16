package tiktok

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// OrdersSearch pulls one page of orders and maps them to PlatformOrder (best-effort for varied API shapes).
func OrdersSearch(ctx context.Context, auth platformp.TestConnectionRequest, access string, cursor string, limit int, start, end *time.Time) ([]platformp.PlatformOrder, string, bool, map[string]interface{}, error) {
	cfg, err := ResolveRuntime(auth)
	if err != nil {
		return nil, "", false, nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, "", false, nil, fmt.Errorf("missing access_token")
	}
	if limit <= 0 {
		limit = 50
	}
	client := http.Client{Timeout: cfg.HTTPTimeout}

	body := map[string]interface{}{
		"page_size": limit,
	}
	if strings.TrimSpace(cursor) != "" {
		tok := strings.TrimSpace(cursor)
		body["cursor"] = tok
		body["page_token"] = tok
	}
	if start != nil {
		body["create_time_from"] = start.Unix()
	}
	if end != nil {
		body["create_time_to"] = end.Unix()
	}

	blob, err := signedPOSTJSON(ctx, client, cfg, cfg.APIOrderSearchPath, access, body)
	if err != nil {
		return nil, "", false, nil, err
	}
	root, err := firstJSONMap(blob)
	if err != nil {
		return nil, "", false, nil, err
	}
	sum := compactTopLevelSummary(root)

	if code, ok := root["code"].(float64); ok && int(code) != 0 {
		msg, _ := root["message"].(string)
		return nil, "", false, sum, fmt.Errorf("%s", strings.TrimSpace(msg))
	}

	rawData := extractOrdersPayload(root)

	var orderObjs []interface{}
	for _, k := range []string{"order_list", "orders", "data_list"} {
		if v, ok := rawData[k].([]interface{}); ok {
			orderObjs = v
			break
		}
	}
	next := strField(rawData, "next_cursor", "cursor", "page_token")
	more := false
	if b, ok := rawData["has_more"].(bool); ok {
		more = b
	} else if next != "" {
		more = true
	}

	out := make([]platformp.PlatformOrder, 0, len(orderObjs))
	for _, o := range orderObjs {
		m, ok := o.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, mapOrder(m))
	}
	return out, next, more, sum, nil
}

func extractOrdersPayload(root map[string]interface{}) map[string]interface{} {
	if d, ok := root["data"].(map[string]interface{}); ok {
		return d
	}
	return root
}

func compactTopLevelSummary(root map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	if c, ok := root["code"]; ok {
		out["code"] = c
	}
	if m, ok := root["message"]; ok && fmt.Sprint(m) != "" {
		out["message"] = m
	}
	if r, ok := root["request_id"]; ok && fmt.Sprint(r) != "" {
		out["request_id"] = r
	}
	return out
}

func mapOrder(in map[string]interface{}) platformp.PlatformOrder {
	ext := strField(in, "order_id", "id")
	if ext == "" {
		return platformp.PlatformOrder{
			Status:            "processing",
			PaymentStatus:     "unpaid",
			FulfillmentStatus: "unfulfilled",
			RawData:           compactSummary(in),
		}
	}

	orderNo := coalesce(ext, strField(in, "order_sn", "order_sn_fuzzy"))
	tiktokStatus := strField(in, "order_status", "status")
	payment := strField(in, "payment_method")
	fulfillRaw := strField(in, "fulfillment_status", "delivery_status")
	currency := strings.ToUpper(strField(in, "currency"))

	pm := paymentMap(in)
	total := parseMoney(in["payment"])
	if total == 0 {
		total = parseMoney(pm["total_amount"])
	}
	if currency == "" {
		currency = "USD"
	}

	buyer := firstNestedMap(in, "buyer", "recipient_address")
	customer := strField(buyer, "name", "full_name")

	o := platformp.PlatformOrder{
		ExternalOrderID:   ext,
		OrderNo:           orderNo,
		CustomerName:      customer,
		Status:            MapOrderStatus(tiktokStatus),
		PaymentStatus:     MapPaymentStatus(coalesce(strField(pm, "status"), payment, tiktokStatus)),
		FulfillmentStatus: MapFulfillmentStatus(coalesce(fulfillRaw, tiktokStatus)),
		Currency:          currency,
		TotalAmount:       total,
		OrderedAt:         parseUnixFlexible(in["create_time"]),
		PaidAt:            parseUnixFlexible(in["paid_time"]),
		ShippedAt:         parseUnixFlexible(firstIface(in, "ship_time", "shipped_at")),
		DeliveredAt:       parseUnixFlexible(firstIface(in, "delivered_at", "delivered_time")),
		RawData:           compactOrderRaw(in),
	}
	o.Items = mapLineItems(in)
	o.Shipments = mapShipments(in)
	return o
}

func paymentMap(in map[string]interface{}) map[string]interface{} {
	pm, ok := in["payment"].(map[string]interface{})
	if ok && pm != nil {
		return pm
	}
	return map[string]interface{}{}
}

func firstIface(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func firstNestedMap(in map[string]interface{}, keys ...string) map[string]interface{} {
	for _, k := range keys {
		cm, ok := in[k].(map[string]interface{})
		if ok && cm != nil {
			return cm
		}
	}
	return map[string]interface{}{}
}

func mapLineItems(in map[string]interface{}) []platformp.PlatformOrderItem {
	var blocks []interface{}
	for _, key := range []string{"items", "line_items", "order_items", "skus"} {
		if v, ok := in[key].([]interface{}); ok {
			blocks = v
			break
		}
	}
	out := make([]platformp.PlatformOrderItem, 0, len(blocks))
	for idx, blk := range blocks {
		im, ok := blk.(map[string]interface{})
		if !ok {
			continue
		}
		pid := coalesce(strField(im, "product_id"), strField(im, "sku_id"))
		ext := strField(im, "id", "sku_item_id")
		if ext == "" && pid != "" {
			ext = fmt.Sprintf("%s-%d", pid, idx)
		}
		title := strField(im, "product_name", "title")
		qty := qtyInt(im["quantity"])
		if qty < 1 {
			qty = 1
		}
		unit := parseMoney(coalesceIface(im["price"], im["original_price"], im["sale_price"]))
		total := parseMoney(coalesceIface(im["subtotal_amount"], im["total_price"]))
		if total <= 0 {
			total = unit * float64(qty)
		}
		skuCode := strField(im, "seller_sku", "sku_id", "sku_code")
		skuName := strField(im, "sku_name", "variant")
		img := strField(im, "image_url", "sku_image")
		raw := compactSummary(im)
		out = append(out, platformp.PlatformOrderItem{
			ExternalItemID: ext,
			ProductTitle:   title,
			SKUName:        skuName,
			SKUCode:        skuCode,
			Quantity:       qty,
			UnitPrice:      unit,
			TotalPrice:     total,
			ImageURL:       img,
			Attrs:          map[string]any{"tiktokProductId": pid},
			RawData:        raw,
		})
	}
	return out
}

func mapShipments(in map[string]interface{}) []platformp.PlatformShipment {
	var blocks []interface{}
	for _, key := range []string{"tracking", "shipments", "packages", "shipping_list"} {
		if v, ok := in[key].([]interface{}); ok {
			blocks = v
			break
		}
	}
	if len(blocks) == 0 {
		if m, ok := in["tracking_number"].(string); ok && strings.TrimSpace(m) != "" {
			blocks = []interface{}{map[string]interface{}{"tracking_number": m}}
		}
	}
	out := make([]platformp.PlatformShipment, 0, len(blocks))
	for _, blk := range blocks {
		sm, ok := blk.(map[string]interface{})
		if !ok {
			continue
		}
		carrier := strField(sm, "shipping_provider", "carrier")
		tno := strField(sm, "tracking_number", "tracking_no")
		st := strField(sm, "status", "shipping_status")
		raw := compactSummary(sm)
		out = append(out, platformp.PlatformShipment{
			Carrier:     carrier,
			TrackingNo:  tno,
			Status:      MapShipmentStatus(st),
			ShippedAt:   parseUnixFlexible(sm["ship_time"]),
			DeliveredAt: parseUnixFlexible(sm["delivered_time"]),
			RawData:     raw,
		})
	}
	return out
}

func parseMoney(v interface{}) float64 {
	switch t := v.(type) {
	case string:
		t = strings.TrimSpace(t)
		if t == "" {
			return 0
		}
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0
		}
		return f
	case float64:
		return t
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}

func qtyInt(v interface{}) int {
	n := int(parseMoney(v))
	if n <= 0 {
		return 0
	}
	return n
}

func parseUnixFlexible(v interface{}) *time.Time {
	switch t := v.(type) {
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
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil || n <= 0 {
			return nil
		}
		sec := n
		if sec > 1_000_000_000_000 {
			sec = sec / 1000
		}
		tt := time.Unix(sec, 0).UTC()
		return &tt
	default:
	}
	return nil
}

func compactSummary(m map[string]interface{}) map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	allowed := []string{"id", "status", "order_status", "order_id", "order_sn", "product_id", "sku_id"}
	out := map[string]interface{}{}
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

func compactOrderRaw(in map[string]interface{}) map[string]interface{} {
	pm := paymentMap(in)
	return map[string]interface{}{
		"source":                "tiktok",
		"orderStatusOriginal":   strField(in, "order_status", "status"),
		"paymentStatusOriginal": strField(pm, "status"),
		"fulfillmentHint":       strField(in, "fulfillment_status"),
	}
}

func coalesce(s ...string) string {
	for _, x := range s {
		x = strings.TrimSpace(x)
		if x != "" {
			return x
		}
	}
	return ""
}

func coalesceIface(vs ...interface{}) interface{} {
	for _, v := range vs {
		if v != nil && fmt.Sprint(v) != "" {
			return v
		}
	}
	return nil
}
