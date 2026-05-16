package tiktok

import (
	"strings"
)

// MapOrderStatus maps TikTok order statuses to internal order.status (unknown -> processing with note in raw elsewhere).
func MapOrderStatus(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "", "unknown":
		return "processing"
	case "unpaid", "awaiting_payment", "pending_payment":
		return "pending"
	case "paid", "payment_received", "completed_payment", "paid_enough":
		return "paid"
	case "processing", "processing_order", "confirmed", "seller_confirmed":
		return "processing"
	case "shipping", "sent", "shipped", "in_transit":
		return "shipped"
	case "delivered":
		return "delivered"
	case "cancelled", "canceled":
		return "cancelled"
	case "refunded":
		return "refunded"
	case "closed", "complete", "completed":
		return "closed"
	default:
		return "processing"
	}
}

func MapPaymentStatus(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "", "unknown":
		return "unpaid"
	case "paid", "payment_received", "completed_payment":
		return "paid"
	case "unpaid", "awaiting_payment":
		return "unpaid"
	case "partially_refunded", "partial_refund":
		return "partially_refunded"
	case "refunded":
		return "refunded"
	default:
		return "unpaid"
	}
}

func MapFulfillmentStatus(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "", "unknown":
		return "unfulfilled"
	case "unfulfilled", "pending_fulfillment":
		return "unfulfilled"
	case "partial":
		return "partial"
	case "fulfilled", "shipped_all", "completed":
		return "fulfilled"
	case "returned", "return_requested":
		return "returned"
	default:
		return "unfulfilled"
	}
}

func MapShipmentStatus(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "", "unknown":
		return "pending"
	case "pending":
		return "pending"
	case "shipped", "sent":
		return "shipped"
	case "in_transit", "transit":
		return "in_transit"
	case "delivered":
		return "delivered"
	case "exception", "failed_delivery":
		return "exception"
	case "returned":
		return "returned"
	default:
		return "pending"
	}
}
