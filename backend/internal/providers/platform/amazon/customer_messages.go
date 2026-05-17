package amazon

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const amazonMessagingOrdersCap = 25

const amazonMessagingMaxTextRunes = 2000

// PullCustomerMessages hydrates normalized conversations from Orders + Messaging APIs (seller templates only).
func PullCustomerMessages(ctx context.Context, req platformp.PullMessagesRequest) (*platformp.PullMessagesResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}
	mp := EffectiveMarketplaceID(req.Auth, cfg)
	if mp == "" {
		return nil, fmt.Errorf("missing marketplace_id")
	}
	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}

	pageLimit := req.Limit
	if pageLimit <= 0 {
		pageLimit = 50
	}
	if pageLimit > amazonMessagingOrdersCap {
		pageLimit = amazonMessagingOrdersCap
	}

	rawOrders, next, err := FetchOrdersPage(ctx, cfg, access, mp, req.Cursor, pageLimit, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}

	conversations := make([]platformp.PlatformConversation, 0, len(rawOrders))
	for _, ord := range rawOrders {
		oid := strings.TrimSpace(fmt.Sprint(ord["AmazonOrderId"]))
		if oid == "" {
			continue
		}
		actions, err := getMessagingActionsForOrder(ctx, cfg, access, mp, oid)
		if err != nil {
			return nil, err
		}
		attrRoot := getMessagingAttributes(ctx, cfg, access, mp, oid)
		locale := amazonBuyerLocale(attrRoot)
		pc := buildAmazonMessagingConversation(mp, ord, actions, locale)
		conversations = append(conversations, pc)
	}

	summary := map[string]any{
		"provider":             "amazon",
		"ordersPageCount":      len(rawOrders),
		"conversationsBuilt":   len(conversations),
		"ordersApiHasMore":     next != "",
		"messagingNote":        "SP-API Messaging lists seller templates per order; buyer-authored inbox history is not exposed.",
		"messagingThrottleRps": "~1 rps enforced client-side between Messaging calls",
	}
	return &platformp.PullMessagesResult{
		Conversations: conversations,
		NextCursor:    next,
		HasMore:       strings.TrimSpace(next) != "",
		RawSummary:    platformp.TrimRawMap(summary, 14, 400),
	}, nil
}

// SendCustomerMessage posts a templated Messaging API payload using the best eligible `{text}` template for the order.
func SendCustomerMessage(ctx context.Context, req platformp.SendMessageRequest) (*platformp.SendMessageResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	reply := strings.TrimSpace(req.Reply)
	if reply == "" {
		return nil, fmt.Errorf("reply is required")
	}
	ext := strings.TrimSpace(req.ExternalConversationID)
	if ext == "" {
		return nil, fmt.Errorf("external conversation id required")
	}

	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}
	mpFallback := EffectiveMarketplaceID(req.Auth, cfg)
	oid, mpID, ok := parseAmazonExternalConversationID(ext, mpFallback)
	if !ok || oid == "" || mpID == "" {
		return nil, fmt.Errorf("invalid amazon external conversation id")
	}

	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}

	actions, err := getMessagingActionsForOrder(ctx, cfg, access, mpID, oid)
	if err != nil {
		return nil, err
	}
	tpl := pickAmazonSendTemplate(actions)
	if tpl == "" {
		return nil, fmt.Errorf("amazon messaging: no eligible templated send action for free-text reply on this order; available templates depend on order state and Buyer-Seller Messaging permissions")
	}

	text := truncateAmazonMessagingText(reply, amazonMessagingMaxTextRunes)
	return sendAmazonMessagingTextTemplate(ctx, cfg, access, mpID, oid, tpl, text, req.IdempotencyKey)
}
