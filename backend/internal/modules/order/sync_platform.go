package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SyncedOrderPayload is provider-neutral input produced by ordersync (maps from platform.PlatformOrder).
type SyncedOrderPayload struct {
	ExternalOrderID   string
	OrderNo           string
	CustomerName      string
	Status            string
	PaymentStatus     string
	FulfillmentStatus string
	Currency          string
	TotalAmount       float64
	OrderedAt         *time.Time
	PaidAt            *time.Time
	ShippedAt         *time.Time
	DeliveredAt       *time.Time
	Items             []SyncedOrderItemPayload
	Shipments         []SyncedShipmentPayload
	RawSummary        map[string]any
}

// SyncedOrderItemPayload is one synced line item.
type SyncedOrderItemPayload struct {
	ExternalItemID string
	ExternalSKUID  string
	SellerSKU      string
	ProductTitle   string
	SKUName        string
	SKUCode        string
	Quantity       int
	UnitPrice      float64
	TotalPrice     float64
	ImageURL       string
	Attrs          map[string]any
	ItemRaw        map[string]any // trimmed subset for order_items.raw_data
}

// SyncedShipmentPayload is one synced shipment segment.
type SyncedShipmentPayload struct {
	Carrier     string
	TrackingNo  string
	TrackingURL string
	Status      string
	ShippedAt   *time.Time
	DeliveredAt *time.Time
}

func normalizeSyncedOrderStatus(v string) string {
	s := strings.TrimSpace(strings.ToLower(v))
	switch s {
	case "", "unknown":
		return StatusPending
	case StatusPending, StatusPaid, StatusProcessing, StatusShipped, StatusDelivered, StatusCancelled, StatusRefunded, StatusClosed:
		return s
	case "complete", "completed":
		return StatusClosed
	case "ship", "shipping":
		return StatusShipped
	default:
		return StatusPending
	}
}

func normalizeSyncedPaymentStatus(v string) string {
	s := strings.TrimSpace(strings.ToLower(v))
	switch s {
	case "", "unknown":
		return PaymentUnpaid
	case PaymentUnpaid, PaymentPaid, PaymentPartiallyRefunded, PaymentRefunded:
		return s
	case "partial_refund":
		return PaymentPartiallyRefunded
	default:
		return PaymentUnpaid
	}
}

func normalizeSyncedFulfillmentStatus(v string) string {
	s := strings.TrimSpace(strings.ToLower(v))
	switch s {
	case "", "unknown":
		return FulfillmentUnfulfilled
	case FulfillmentUnfulfilled, FulfillmentPartial, FulfillmentFulfilled, FulfillmentReturned:
		return s
	default:
		return FulfillmentUnfulfilled
	}
}

func normalizeSyncedShipmentStatus(v string) string {
	s := strings.TrimSpace(strings.ToLower(v))
	switch s {
	case "", "unknown":
		return ShipmentPending
	case ShipmentPending, ShipmentShipped, ShipmentInTransit, ShipmentDelivered, ShipmentException, ShipmentReturned:
		return s
	default:
		return ShipmentPending
	}
}

func compactSyncedItemRaw(it SyncedOrderItemPayload) datatypes.JSON {
	m := map[string]any{}
	if s := strings.TrimSpace(it.ExternalSKUID); s != "" {
		m["externalSkuId"] = s
	}
	if s := strings.TrimSpace(it.SellerSKU); s != "" {
		m["sellerSku"] = s
	}
	for k, v := range it.ItemRaw {
		lk := strings.ToLower(k)
		if lk == "token" || lk == "access_token" || lk == "refresh_token" || lk == "authorization" {
			continue
		}
		m[k] = v
	}
	if len(m) == 0 {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}

func extSkuPtrFromPayload(it SyncedOrderItemPayload) *string {
	s := strings.TrimSpace(it.ExternalSKUID)
	if s == "" {
		return nil
	}
	return &s
}

func compactRawSummary(platformKey string, shopID uuid.UUID, extID string, src map[string]any) datatypes.JSON {
	m := map[string]any{
		"source":          "platform_order_sync",
		"platform":        platformKey,
		"shopId":          shopID.String(),
		"externalOrderId": extID,
		"syncedAt":        time.Now().UTC().Format(time.RFC3339),
		"providerSummary": src,
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}

// UpsertSyncedOrders writes provider-neutral payloads into orders / items / shipments (idempotent by external_order_id per shop).
// Returned order IDs are internal UUIDs successfully upserted in this batch (same order each run for the same external id).
func (s *Service) UpsertSyncedOrders(ctx context.Context, shopID uuid.UUID, shopPlatform string, payloads []SyncedOrderPayload) (orderIDs []uuid.UUID, success int, failed int, err error) {
	if s == nil || s.DB == nil {
		return nil, 0, 0, fmt.Errorf("order: no db")
	}
	platformKey := strings.TrimSpace(shopPlatform)
	if platformKey == "" {
		return nil, 0, 0, fmt.Errorf("platform is required")
	}
	for _, p := range payloads {
		ext := strings.TrimSpace(p.ExternalOrderID)
		if ext == "" {
			failed++
			continue
		}

		var upsertedID uuid.UUID
		txErr := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			st := normalizeSyncedOrderStatus(p.Status)
			ps := normalizeSyncedPaymentStatus(p.PaymentStatus)
			fs := normalizeSyncedFulfillmentStatus(p.FulfillmentStatus)
			if !validOrderStatus(st) || !validPaymentStatus(ps) || !validFulfillmentStatus(fs) {
				return fmt.Errorf("invalid normalized status")
			}

			name := strings.TrimSpace(p.CustomerName)
			if name == "" {
				name = "Unknown customer"
			}
			cur := strings.TrimSpace(p.Currency)
			if cur == "" {
				cur = "USD"
			}
			cur = strings.ToUpper(cur)

			on := strings.TrimSpace(p.OrderNo)
			if on == "" {
				on = fmt.Sprintf("SYNC-%s", ext)
			}

			raw := compactRawSummary(platformKey, shopID, ext, p.RawSummary)

			var existing Order
			q := tx.Where("shop_id = ? AND platform = ? AND external_order_id = ?", shopID, platformKey, ext)
			findErr := q.First(&existing).Error
			if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}

			sid := shopID
			extCopy := ext

			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				o := &Order{
					Platform:          platformKey,
					ShopID:            &sid,
					ExternalOrderID:   &extCopy,
					OrderNo:           on,
					CustomerName:      name,
					Status:            st,
					PaymentStatus:     ps,
					FulfillmentStatus: fs,
					Currency:          cur,
					TotalAmount:       p.TotalAmount,
					PaidAt:            p.PaidAt,
					OrderedAt:         p.OrderedAt,
					ShippedAt:         p.ShippedAt,
					DeliveredAt:       p.DeliveredAt,
					RawData:           raw,
				}
				if err := tx.Create(o).Error; err != nil {
					return err
				}
				upsertedID = o.ID
				return replaceSyncedChildren(tx, o.ID, p)
			}

			existing.Platform = platformKey
			existing.ShopID = &sid
			existing.ExternalOrderID = &extCopy
			existing.OrderNo = on
			existing.CustomerName = name
			existing.Status = st
			existing.PaymentStatus = ps
			existing.FulfillmentStatus = fs
			existing.Currency = cur
			existing.TotalAmount = p.TotalAmount
			existing.PaidAt = p.PaidAt
			existing.OrderedAt = p.OrderedAt
			existing.ShippedAt = p.ShippedAt
			existing.DeliveredAt = p.DeliveredAt
			existing.RawData = raw
			// Remark intentionally preserved (manual ops).

			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			upsertedID = existing.ID
			return replaceSyncedChildren(tx, existing.ID, p)
		})

		if txErr != nil {
			failed++
			continue
		}
		success++
		if upsertedID != uuid.Nil {
			orderIDs = append(orderIDs, upsertedID)
		}
	}
	return orderIDs, success, failed, nil
}

func replaceSyncedChildren(tx *gorm.DB, orderID uuid.UUID, p SyncedOrderPayload) error {
	if err := tx.Where("order_id = ?", orderID).Delete(&OrderShipment{}).Error; err != nil {
		return err
	}

	var existingItems []OrderItem
	if err := tx.Where("order_id = ?", orderID).Find(&existingItems).Error; err != nil {
		return err
	}

	byExt := make(map[string]*OrderItem)
	for i := range existingItems {
		ei := &existingItems[i]
		if ei.ExternalItemID != nil && strings.TrimSpace(*ei.ExternalItemID) != "" {
			k := strings.TrimSpace(*ei.ExternalItemID)
			byExt[k] = ei
		}
	}

	withExtSeen := make(map[string]struct{})

	for _, it := range p.Items {
		extRaw := strings.TrimSpace(it.ExternalItemID)
		if extRaw == "" {
			continue
		}
		withExtSeen[extRaw] = struct{}{}

		title := strings.TrimSpace(it.ProductTitle)
		if title == "" {
			title = strings.TrimSpace(it.SKUCode)
		}
		if title == "" {
			title = "(item)"
		}
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		var attrs datatypes.JSON
		if len(it.Attrs) > 0 {
			attrs = mapAttrs(it.Attrs)
		}
		lineRaw := compactSyncedItemRaw(it)

		prev := byExt[extRaw]
		if prev != nil {
			now := time.Now().UTC()
			if err := tx.Model(prev).Updates(map[string]any{
				"product_title":   title,
				"sku_name":        strings.TrimSpace(it.SKUName),
				"sku_code":        strings.TrimSpace(it.SKUCode),
				"seller_sku":      strings.TrimSpace(it.SellerSKU),
				"external_sku_id": extSkuPtrFromPayload(it),
				"quantity":        qty,
				"unit_price":      it.UnitPrice,
				"total_price":     it.TotalPrice,
				"image_url":       strings.TrimSpace(it.ImageURL),
				"attrs":           attrs,
				"raw_data":        lineRaw,
				"updated_at":      now,
			}).Error; err != nil {
				return err
			}
			continue
		}

		extCopy := extRaw
		row := OrderItem{
			OrderID:        orderID,
			ExternalItemID: &extCopy,
			ExternalSKUID:  extSkuPtrFromPayload(it),
			SellerSKU:      strings.TrimSpace(it.SellerSKU),
			ProductTitle:   title,
			SKUName:        strings.TrimSpace(it.SKUName),
			SKUCode:        strings.TrimSpace(it.SKUCode),
			Quantity:       qty,
			UnitPrice:      it.UnitPrice,
			TotalPrice:     it.TotalPrice,
			ImageURL:       strings.TrimSpace(it.ImageURL),
			Attrs:          attrs,
			RawData:        lineRaw,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}

	for ext, row := range byExt {
		if _, ok := withExtSeen[ext]; ok {
			continue
		}
		if err := tx.Delete(row).Error; err != nil {
			return err
		}
	}

	if err := tx.Where("order_id = ? AND (external_item_id IS NULL OR external_item_id = '')", orderID).
		Delete(&OrderItem{}).Error; err != nil {
		return err
	}

	for _, it := range p.Items {
		if strings.TrimSpace(it.ExternalItemID) != "" {
			continue
		}
		title := strings.TrimSpace(it.ProductTitle)
		if title == "" {
			title = strings.TrimSpace(it.SKUCode)
		}
		if title == "" {
			title = "(item)"
		}
		qty := it.Quantity
		if qty < 1 {
			qty = 1
		}
		var attrs datatypes.JSON
		if len(it.Attrs) > 0 {
			attrs = mapAttrs(it.Attrs)
		}
		lineRaw := compactSyncedItemRaw(it)
		row := OrderItem{
			OrderID:       orderID,
			ExternalSKUID: extSkuPtrFromPayload(it),
			SellerSKU:     strings.TrimSpace(it.SellerSKU),
			ProductTitle:  title,
			SKUName:       strings.TrimSpace(it.SKUName),
			SKUCode:       strings.TrimSpace(it.SKUCode),
			Quantity:      qty,
			UnitPrice:     it.UnitPrice,
			TotalPrice:    it.TotalPrice,
			ImageURL:      strings.TrimSpace(it.ImageURL),
			Attrs:         attrs,
			RawData:       lineRaw,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}

	for _, sh := range p.Shipments {
		cr := strings.TrimSpace(sh.Carrier)
		if cr == "" {
			cr = "unknown"
		}
		tno := strings.TrimSpace(sh.TrackingNo)
		if tno == "" {
			tno = "n/a"
		}
		st := normalizeSyncedShipmentStatus(sh.Status)
		if !validShipmentStatus(st) {
			return fmt.Errorf("invalid shipment status")
		}
		row := OrderShipment{
			OrderID:     orderID,
			Carrier:     cr,
			TrackingNo:  tno,
			TrackingURL: strings.TrimSpace(sh.TrackingURL),
			Status:      st,
			ShippedAt:   sh.ShippedAt,
			DeliveredAt: sh.DeliveredAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
