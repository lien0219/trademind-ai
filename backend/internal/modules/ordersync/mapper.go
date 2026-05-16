package ordersync

import (
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// ToSyncedPayloads maps neutral platform orders into DB payloads (drops heavyweight RawData blobs).
func ToSyncedPayloads(in []platformp.PlatformOrder) []order.SyncedOrderPayload {
	out := make([]order.SyncedOrderPayload, 0, len(in))
	for _, po := range in {
		ext := strings.TrimSpace(po.ExternalOrderID)
		if ext == "" {
			continue
		}
		sum := summaryFromRaw(po.RawData)

		items := make([]order.SyncedOrderItemPayload, 0, len(po.Items))
		for _, it := range po.Items {
			items = append(items, order.SyncedOrderItemPayload{
				ExternalItemID: strings.TrimSpace(it.ExternalItemID),
				ProductTitle:   it.ProductTitle,
				SKUName:        it.SKUName,
				SKUCode:        it.SKUCode,
				Quantity:       it.Quantity,
				UnitPrice:      it.UnitPrice,
				TotalPrice:     it.TotalPrice,
				ImageURL:       it.ImageURL,
				Attrs:          it.Attrs,
			})
		}

		ships := make([]order.SyncedShipmentPayload, 0, len(po.Shipments))
		for _, sh := range po.Shipments {
			ships = append(ships, order.SyncedShipmentPayload{
				Carrier:     sh.Carrier,
				TrackingNo:  sh.TrackingNo,
				TrackingURL: sh.TrackingURL,
				Status:      sh.Status,
				ShippedAt:   sh.ShippedAt,
				DeliveredAt: sh.DeliveredAt,
			})
		}

		out = append(out, order.SyncedOrderPayload{
			ExternalOrderID:   ext,
			OrderNo:           strings.TrimSpace(po.OrderNo),
			CustomerName:      strings.TrimSpace(po.CustomerName),
			Status:            po.Status,
			PaymentStatus:     po.PaymentStatus,
			FulfillmentStatus: po.FulfillmentStatus,
			Currency:          po.Currency,
			TotalAmount:       po.TotalAmount,
			OrderedAt:         po.OrderedAt,
			PaidAt:            po.PaidAt,
			ShippedAt:         po.ShippedAt,
			DeliveredAt:       po.DeliveredAt,
			Items:             items,
			Shipments:         ships,
			RawSummary:        sum,
		})
	}
	return out
}

func summaryFromRaw(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{"mock": false}
	}
	out := map[string]any{}
	for k, v := range raw {
		switch strings.ToLower(k) {
		case "token", "access_token", "refresh_token", "app_secret", "secret", "authorization":
			continue
		default:
			out[k] = v
		}
	}
	if len(out) == 0 {
		return map[string]any{"truncated": true}
	}
	return out
}
