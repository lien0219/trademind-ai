package amazon

// SP-API Messaging v1 paths (relative to sp_api_base_url).
// Ref: selling-partner-api-models messaging-api-model/messaging.json
const (
	PathMessagingOrderActions = "/messaging/v1/orders/{amazonOrderId}"
	PathMessagingAttributes   = "/messaging/v1/orders/{amazonOrderId}/attributes"
	// POST /messaging/v1/orders/{amazonOrderId}/messages/{template}
	PathMessagingSendTemplate = "/messaging/v1/orders/{amazonOrderId}/messages/{template}"
)
