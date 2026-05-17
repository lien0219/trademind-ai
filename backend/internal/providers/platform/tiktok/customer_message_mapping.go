package tiktok

import (
	"fmt"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func extractDataPayload(root map[string]interface{}) map[string]interface{} {
	if root == nil {
		return map[string]interface{}{}
	}
	if d, ok := root["data"].(map[string]interface{}); ok && d != nil {
		return d
	}
	return root
}

func conversationArrayFromPayload(data map[string]interface{}) []interface{} {
	if data == nil {
		return nil
	}
	for _, k := range []string{"conversation_list", "conversations", "conversation_search_list", "list", "data_list"} {
		if v, ok := data[k].([]interface{}); ok {
			return v
		}
	}
	return nil
}

func messageArrayFromPayload(data map[string]interface{}) []interface{} {
	if data == nil {
		return nil
	}
	for _, k := range []string{"message_list", "messages", "list", "data_list"} {
		if v, ok := data[k].([]interface{}); ok {
			return v
		}
	}
	return nil
}

func nextCursorFromPayload(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	return strField(data, "next_cursor", "cursor", "page_token", "next_page_token")
}

func hasMoreFromPayload(data map[string]interface{}, next string) bool {
	if data == nil {
		return false
	}
	if b, ok := data["has_more"].(bool); ok {
		return b
	}
	if b, ok := data["more"].(bool); ok {
		return b
	}
	return strings.TrimSpace(next) != ""
}

func mapPlatformConversation(convObj map[string]interface{}) platformp.PlatformConversation {
	ext := strField(convObj, "conversation_id", "id", "thread_id", "session_id")
	buyer := firstNestedMap(convObj, "buyer", "buyer_info", "user", "customer")
	name := strField(buyer, "nick_name", "nickname", "name", "username", "display_name")
	avatar := strField(buyer, "avatar", "avatar_url", "profile_picture")
	lang := strField(buyer, "language", "locale")
	if lang == "" {
		lang = strField(convObj, "buyer_language", "language", "locale")
	}
	if lang == "" {
		lang = "en"
	}
	st := mapConversationStatus(strField(convObj, "conversation_status", "status", "state"))
	lastAt := parseUnixFlexible(firstIface(convObj, "last_message_time", "last_message_create_time", "update_time", "updated_at"))

	raw := platformp.TrimRawMap(map[string]any{
		"source":              "tiktok",
		"conversationSnippet": strField(convObj, "last_message_preview", "preview"),
	}, 8, 200)

	return platformp.PlatformConversation{
		ExternalConversationID: ext,
		CustomerName:           name,
		CustomerAvatar:         avatar,
		CustomerLanguage:       lang,
		Status:                 st,
		LastMessageAt:          lastAt,
		Messages:               nil,
		RawData:                raw,
	}
}

func mapConversationStatus(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "closed", "close", "ended", "resolved":
		return "closed"
	case "pending", "pending_reply", "waiting_reply", "wait_reply":
		return "pending_reply"
	case "replied", "answered":
		return "replied"
	case "open", "active", "ongoing", "":
		return "open"
	default:
		return "open"
	}
}

func mapPlatformMessage(msgObj map[string]interface{}) platformp.PlatformCustomerMessage {
	ext := strField(msgObj, "message_id", "id", "msg_id")
	role := mapMessageRole(strField(msgObj, "sender_type", "sender_role", "role", "type"))
	mt := mapMessageType(strField(msgObj, "message_type", "type", "msg_type"))
	content := messageTextContent(msgObj)
	lang := strField(msgObj, "language", "locale")
	if lang == "" {
		lang = "en"
	}
	created := parseUnixFlexible(firstIface(msgObj, "create_time", "created_time", "timestamp", "send_time"))
	raw := platformp.TrimRawMap(map[string]any{
		"source":     "tiktok",
		"typeHint":   strField(msgObj, "message_type", "type"),
		"senderHint": strField(msgObj, "sender_type", "sender_role"),
	}, 8, 200)
	return platformp.PlatformCustomerMessage{
		ExternalMessageID: ext,
		Role:              role,
		Content:           content,
		MessageType:       mt,
		Language:          lang,
		CreatedAt:         created,
		RawData:           raw,
	}
}

func messageTextContent(msgObj map[string]interface{}) string {
	if raw, ok := msgObj["text"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	if raw, ok := msgObj["content"].(string); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	if cm, ok := msgObj["content"].(map[string]interface{}); ok && cm != nil {
		if t, ok := cm["text"].(string); ok {
			return strings.TrimSpace(t)
		}
	}
	if bm, ok := msgObj["body"].(map[string]interface{}); ok && bm != nil {
		if t, ok := bm["text"].(string); ok {
			return strings.TrimSpace(t)
		}
	}
	return ""
}

func mapMessageRole(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "seller", "shop", "merchant", "agent", "operator", "business", "assistant":
		return "agent"
	case "buyer", "customer", "user", "consumer":
		return "customer"
	case "system", "platform":
		return "system"
	default:
		return "customer"
	}
}

func mapMessageType(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "text", "message_text", "":
		return "text"
	case "image", "picture", "photo":
		return "image"
	case "order", "product_card", "product", "item":
		return "order"
	case "system":
		return "system"
	default:
		return "text"
	}
}

func summarizePullCodes(convRoot, msgRoot map[string]interface{}) map[string]any {
	out := map[string]any{
		"provider": "tiktok",
	}
	if convRoot != nil {
		out["convApiCode"] = parseBizCodeStr(convRoot)
		if rid, ok := convRoot["request_id"]; ok && fmt.Sprint(rid) != "" {
			out["convRequestId"] = rid
		}
	}
	if msgRoot != nil {
		out["msgApiCode"] = parseBizCodeStr(msgRoot)
	}
	return out
}
