package amazon

import (
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// MapAmazonOrder converts one Amazon order + items to PlatformOrder (Amazon types only in this package).
func MapAmazonOrder(order map[string]any, items []map[string]any) platformp.PlatformOrder {
	oid := strFromAny(order["AmazonOrderId"])
	stRaw := strFromAny(order["OrderStatus"])
	cur := strFromAny(order["OrderCurrencyCode"])
	total, cur2 := moneyFromMap(toMap(order["OrderTotal"]))
	if cur == "" {
		cur = cur2
	}
	purchaseDate := parseAmzTime(strFromAny(order["PurchaseDate"]))
	lastUpdate := parseAmzTime(strFromAny(order["LastUpdateDate"]))
	shipService := strFromAny(order["ShipmentServiceLevelCategory"])
	fulfCh := strFromAny(order["FulfillmentChannel"])

	st := mapOrderStatus(stRaw)
	pay := mapPaymentStatus(stRaw)
	fulf := mapFulfillmentStatus(stRaw, order)

	customer := ""
	if bi, ok := order["BuyerInfo"].(map[string]any); ok {
		customer = maskBuyerName(strFromAny(bi["BuyerName"]))
	}

	po := platformp.PlatformOrder{
		ExternalOrderID:   oid,
		OrderNo:           oid,
		CustomerName:      customer,
		Status:            st,
		PaymentStatus:     pay,
		FulfillmentStatus: fulf,
		Currency:          cur,
		TotalAmount:       total,
		OrderedAt:         purchaseDate,
		PaidAt:            purchaseDate,
		ShippedAt:         pickShippedAt(stRaw, lastUpdate),
		DeliveredAt:       nil,
		Items:             mapLineItems(items),
		Shipments:         buildShipmentStub(stRaw, shipService, fulfCh, lastUpdate),
		RawData: map[string]any{
			"amazonOrderId":          oid,
			"orderStatus":            stRaw,
			"fulfillmentChannel":     fulfCh,
			"shipmentServiceLevel":   shipService,
			"lastUpdateDate":         strFromAny(order["LastUpdateDate"]),
			"numberOfItemsShipped":   order["NumberOfItemsShipped"],
			"numberOfItemsUnshipped": order["NumberOfItemsUnshipped"],
			"isReplacementOrder":     order["IsReplacementOrder"],
		},
	}
	return po
}

func toMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func parseAmzTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	tt := t.UTC()
	return &tt
}

func maskBuyerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	r := []rune(name)
	if len(r) <= 3 {
		return "***"
	}
	return string(r[:2]) + "***"
}

func mapOrderStatus(amz string) string {
	switch strings.TrimSpace(amz) {
	case "Shipped":
		return "shipped"
	case "Canceled", "Cancelled":
		return "cancelled"
	case "Pending":
		return "pending"
	case "Unshipped":
		return "paid"
	case "PartiallyShipped":
		return "processing"
	case "PendingAvailability":
		return "pending"
	case "InvoiceUnconfirmed":
		return "processing"
	default:
		return "processing"
	}
}

func mapPaymentStatus(amz string) string {
	switch strings.TrimSpace(amz) {
	case "Canceled", "Cancelled":
		return "unpaid"
	case "Pending", "PendingAvailability":
		return "unpaid"
	default:
		return "paid"
	}
}

func mapFulfillmentStatus(amz string, order map[string]any) string {
	switch strings.TrimSpace(amz) {
	case "Canceled", "Cancelled":
		return "unfulfilled"
	case "Shipped":
		return "fulfilled"
	case "PartiallyShipped":
		return "partial"
	default:
		if n, ok := asInt(order["NumberOfItemsShipped"]); ok && n > 0 {
			if u, ok2 := asInt(order["NumberOfItemsUnshipped"]); ok2 && u > 0 {
				return "partial"
			}
			return "fulfilled"
		}
		return "unfulfilled"
	}
}

func asInt(v any) (int, bool) {
	s := strings.TrimSpace(strFromAny(v))
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func pickShippedAt(amzStatus string, lastUpd *time.Time) *time.Time {
	switch strings.TrimSpace(amzStatus) {
	case "Shipped", "PartiallyShipped":
		return lastUpd
	default:
		return nil
	}
}

func mapLineItems(items []map[string]any) []platformp.PlatformOrderItem {
	out := make([]platformp.PlatformOrderItem, 0, len(items))
	for _, it := range items {
		iid := strFromAny(it["OrderItemId"])
		qty, _ := asInt(it["QuantityOrdered"])
		if qty <= 0 {
			qty = 1
		}
		lineTotal, _ := moneyFromMap(toMap(it["ItemPrice"]))
		unit := lineTotal
		if qty > 1 && lineTotal > 0 {
			unit = lineTotal / float64(qty)
		}
		title := strFromAny(it["Title"])
		sku := strFromAny(it["SellerSKU"])
		asin := strFromAny(it["ASIN"])
		skuName := sku
		if skuName == "" && asin != "" {
			skuName = "ASIN:" + asin
		}
		attrs := map[string]any{}
		if asin != "" {
			attrs["asin"] = asin
		}
		if c := strFromAny(it["ConditionId"]); c != "" {
			attrs["condition"] = c
		}
		out = append(out, platformp.PlatformOrderItem{
			ExternalItemID: iid,
			ProductTitle:   title,
			SKUName:        skuName,
			SKUCode:        sku,
			Quantity:       qty,
			UnitPrice:      unit,
			TotalPrice:     lineTotal,
			ImageURL:       "",
			Attrs:          attrs,
			RawData: map[string]any{
				"orderItemId": iid,
				"asin":        asin,
			},
		})
	}
	return out
}

func buildShipmentStub(amzSt, shipService, fulfCh string, last *time.Time) []platformp.PlatformShipment {
	_ = fulfCh
	switch strings.TrimSpace(amzSt) {
	case "Shipped":
		st := "shipped"
		sh := platformp.PlatformShipment{
			Status:    st,
			ShippedAt: last,
			RawData:   map[string]any{"source": "amazon_order_status", "orderStatus": amzSt},
		}
		if shipService != "" {
			sh.Carrier = shipService
		}
		return []platformp.PlatformShipment{sh}
	case "PartiallyShipped":
		return []platformp.PlatformShipment{{
			Status:    "shipped",
			ShippedAt: last,
			RawData:   map[string]any{"source": "amazon_order_status", "orderStatus": amzSt},
		}}
	default:
		return nil
	}
}
