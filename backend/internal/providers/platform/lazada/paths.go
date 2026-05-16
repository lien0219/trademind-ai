package lazada

// API path names (for signing); full URL = api_rest_base + path (api base includes /rest).
const (
	PathAuthTokenCreate  = "/auth/token/create"
	PathAuthTokenRefresh = "/auth/token/refresh"

	PathSellerGet = "/seller/get"

	PathOrdersGet     = "/orders/get"
	PathOrderGet      = "/order/get"
	PathOrderItemsGet = "/order/items/get"
)
