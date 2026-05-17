package lazada

// API path names (for signing); full URL = api_rest_base + path (api base includes /rest).
const (
	PathAuthTokenCreate  = "/auth/token/create"
	PathAuthTokenRefresh = "/auth/token/refresh"

	PathSellerGet = "/seller/get"

	PathOrdersGet     = "/orders/get"
	PathOrderGet      = "/order/get"
	PathOrderItemsGet = "/order/items/get"

	// Instant Messaging (buyer–seller chat). Paths match Lazada Open Platform IM APIs;
	// response field names may differ by region/version — align parsers with official docs when needed.
	PathIMSessionList = "/im/session/list"
	PathIMMessageList = "/im/message/list"
	PathIMMessageSend = "/im/message/send"

	PathProductCreate = "/product/create"
	PathImageMigrate  = "/image/migrate"
	PathImageUpload   = "/image/upload"
)
