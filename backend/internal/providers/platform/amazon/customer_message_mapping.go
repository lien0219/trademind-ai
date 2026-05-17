package amazon

import (
	"fmt"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const amazonMessagingConvPrefix = "amazon"

// amazonSendTemplates accepts a plain-text body {"text":"..."} per Messaging API models.
var amazonSendTemplatesOrdered = []string{
	"confirmOrderDetails",
	"confirmDeliveryDetails",
	"confirmCustomizationDetails",
	"unexpectedProblem",
}

func buildAmazonExternalConversationID(marketplaceID, amazonOrderID string) string {
	mp := strings.TrimSpace(marketplaceID)
	oid := strings.TrimSpace(amazonOrderID)
	return fmt.Sprintf("%s:%s:%s", amazonMessagingConvPrefix, mp, oid)
}

func parseAmazonExternalConversationID(ext string, fallbackMarketplace string) (amazonOrderID string, marketplaceID string, ok bool) {
	s := strings.TrimSpace(ext)
	if s == "" {
		return "", "", false
	}
	if strings.HasPrefix(strings.ToLower(s), amazonMessagingConvPrefix+":") {
		parts := strings.SplitN(s, ":", 3)
		if len(parts) != 3 {
			return "", "", false
		}
		if strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
			return "", "", false
		}
		return strings.TrimSpace(parts[2]), strings.TrimSpace(parts[1]), true
	}
	// Legacy: plain AmazonOrderId only.
	if fallbackMarketplace == "" {
		return "", "", false
	}
	return s, strings.TrimSpace(fallbackMarketplace), true
}

func amazonBuyerLocale(attrs map[string]any) string {
	if attrs == nil {
		return ""
	}
	buyer, _ := attrs["buyer"].(map[string]any)
	if buyer == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(buyer["locale"]))
}

func amazonMaskedBuyerName(order map[string]any) string {
	if order == nil {
		return ""
	}
	bi, _ := order["BuyerInfo"].(map[string]any)
	if bi == nil {
		return ""
	}
	return maskBuyerName(strings.TrimSpace(fmt.Sprint(bi["BuyerName"])))
}

func amazonLastUpdated(order map[string]any) *time.Time {
	if order == nil {
		return nil
	}
	t := parseAmzTime(strings.TrimSpace(fmt.Sprint(order["LastUpdateDate"])))
	if t != nil {
		return t
	}
	return parseAmzTime(strings.TrimSpace(fmt.Sprint(order["PurchaseDate"])))
}

func templateStubExternalMessageID(amazonOrderID, template string) string {
	return fmt.Sprintf("amazon-template:%s:%s", strings.TrimSpace(amazonOrderID), strings.TrimSpace(template))
}

func buildAmazonMessagingConversation(
	mp string,
	order map[string]any,
	actionNames []string,
	buyerLocale string,
) platformp.PlatformConversation {
	oid := strings.TrimSpace(fmt.Sprint(order["AmazonOrderId"]))
	ext := buildAmazonExternalConversationID(mp, oid)
	name := amazonBuyerName(order)
	if strings.TrimSpace(name) == "" {
		name = "Customer"
	}
	lang := strings.TrimSpace(buyerLocale)
	if lang == "" {
		lang = "en"
	}
	lastAt := amazonLastUpdated(order)

	msgs := make([]platformp.PlatformCustomerMessage, 0, len(actionNames)+1)
	msgs = append(msgs, platformp.PlatformCustomerMessage{
		ExternalMessageID: fmt.Sprintf("amazon-messaging-note:%s", oid),
		Role:              "system",
		MessageType:       "system",
		Content:           "Amazon SP-API Messaging exposes seller-initiated templates per order; buyer-authored Buyer-Seller Messaging history is not available via this API.",
		Language:          lang,
		CreatedAt:         lastAt,
		RawData: platformp.TrimRawMap(map[string]any{
			"kind": "amazonMessagingDisclaimer",
		}, 8, 240),
	})
	for _, act := range actionNames {
		a := strings.TrimSpace(act)
		if a == "" {
			continue
		}
		msgs = append(msgs, platformp.PlatformCustomerMessage{
			ExternalMessageID: templateStubExternalMessageID(oid, a),
			Role:              "system",
			MessageType:       "system",
			Content:           fmt.Sprintf("[Amazon Messaging] Template available for this order: %s", a),
			Language:          lang,
			CreatedAt:         lastAt,
			RawData: platformp.TrimRawMap(map[string]any{
				"template":      a,
				"amazonOrderId": oid,
			}, 8, 240),
		})
	}

	st := strings.TrimSpace(fmt.Sprint(order["OrderStatus"]))
	status := "open"
	switch strings.ToLower(st) {
	case "canceled", "cancelled":
		status = "closed"
	case "pending", "pendingavailability", "unshipped":
		status = "pending_reply"
	default:
		status = "open"
	}

	return platformp.PlatformConversation{
		ExternalConversationID: ext,
		CustomerName:           name,
		CustomerAvatar:         "",
		CustomerLanguage:       lang,
		Status:                 status,
		LastMessageAt:          lastAt,
		Messages:               msgs,
		RawData: platformp.TrimRawMap(map[string]any{
			"amazonOrderId": oid,
			"marketplaceId": mp,
			"orderStatus":   st,
			"buyerLocale":   buyerLocale,
			"templates":     strings.Join(actionNames, ","),
		}, 12, 400),
	}
}

func amazonBuyerName(order map[string]any) string {
	n := amazonMaskedBuyerName(order)
	if n != "" {
		return n
	}
	return ""
}
