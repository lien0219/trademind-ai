package order

const (
	StatusPending    = "pending"
	StatusPaid       = "paid"
	StatusProcessing = "processing"
	StatusShipped    = "shipped"
	StatusDelivered  = "delivered"
	StatusCancelled  = "cancelled"
	StatusRefunded   = "refunded"
	StatusClosed     = "closed"
)

const (
	PaymentUnpaid            = "unpaid"
	PaymentPaid              = "paid"
	PaymentPartiallyRefunded = "partially_refunded"
	PaymentRefunded          = "refunded"
)

const (
	FulfillmentUnfulfilled = "unfulfilled"
	FulfillmentPartial     = "partial"
	FulfillmentFulfilled   = "fulfilled"
	FulfillmentReturned    = "returned"
)

const (
	ShipmentPending   = "pending"
	ShipmentShipped   = "shipped"
	ShipmentInTransit = "in_transit"
	ShipmentDelivered = "delivered"
	ShipmentException = "exception"
	ShipmentReturned  = "returned"
)
