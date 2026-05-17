package shopee

// Shopee Open Platform API v2 paths (host comes from settings auth_base_url / api_base_url).
const (
	PathAuthPartner     = "/api/v2/shop/auth_partner"
	PathAuthTokenGet    = "/api/v2/auth/token/get"
	PathAuthAccessToken = "/api/v2/auth/access_token/get"
	PathGetShopInfo     = "/api/v2/shop/get_shop_info"
	PathGetOrderList    = "/api/v2/order/get_order_list"
	PathGetOrderDetail  = "/api/v2/order/get_order_detail"

	// Seller chat (Open API v2). Paths must match Partner Center naming; adjust if Shopee renames endpoints.
	PathSellerChatGetConversationList = "/api/v2/sellerchat/get_conversation_list"
	PathSellerChatGetMessage          = "/api/v2/sellerchat/get_message"
	PathSellerChatGetOneConversation  = "/api/v2/sellerchat/get_one_conversation"
	PathSellerChatSendMessage         = "/api/v2/sellerchat/send_message"
)
