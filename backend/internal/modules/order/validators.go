package order

func validOrderStatus(s string) bool {
	switch s {
	case StatusPending, StatusPaid, StatusProcessing, StatusShipped, StatusDelivered, StatusCancelled, StatusRefunded, StatusClosed:
		return true
	default:
		return false
	}
}

func validPaymentStatus(s string) bool {
	switch s {
	case PaymentUnpaid, PaymentPaid, PaymentPartiallyRefunded, PaymentRefunded:
		return true
	default:
		return false
	}
}

func validFulfillmentStatus(s string) bool {
	switch s {
	case FulfillmentUnfulfilled, FulfillmentPartial, FulfillmentFulfilled, FulfillmentReturned:
		return true
	default:
		return false
	}
}

func validShipmentStatus(s string) bool {
	switch s {
	case ShipmentPending, ShipmentShipped, ShipmentInTransit, ShipmentDelivered, ShipmentException, ShipmentReturned:
		return true
	default:
		return false
	}
}
