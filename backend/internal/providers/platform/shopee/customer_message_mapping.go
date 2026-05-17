package shopee

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// shopeeChatTextMessageType is the documented int code for plain text chat messages (confirm in Partner Center if send fails).
const shopeeChatTextMessageType int64 = 1

func formatConversationExternalID(v any) string {
	switch t := v.(type) {
	case float64:
		if t <= 0 {
			return ""
		}
		return strconv.FormatInt(int64(t), 10)
	case int64:
		if t <= 0 {
			return ""
		}
		return strconv.FormatInt(t, 10)
	case string:
		s := strings.TrimSpace(t)
		return s
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		return s
	}
}

func mapPlatformConversationFromListRow(m map[string]any) platformp.PlatformConversation {
	if m == nil {
		return platformp.PlatformConversation{CustomerLanguage: "en"}
	}
	ext := formatConversationExternalID(firstAny(m, "conversation_id", "conversation_id_str", "chat_id"))
	name := pickStr(m, "buyer_name", "buyer_username", "user_name", "name", "to_name")
	avatar := pickStr(m, "buyer_avatar", "user_portrait", "portrait", "to_avatar")
	lang := pickStr(m, "buyer_language", "language")
	if lang == "" {
		lang = "en"
	}
	last := parseUnixAny(firstAny(m, "last_message_time", "last_reply_time", "last_message_timestamp", "update_time", "create_time"))
	st := inferConversationStatus(m, last)
	raw := platformp.TrimRawMap(map[string]any{
		"source":            "shopee",
		"conversation_id":   ext,
		"unread_count":      m["unread_count"],
		"last_message_time": m["last_message_time"],
	}, 12, 400)

	return platformp.PlatformConversation{
		ExternalConversationID: ext,
		CustomerName:           name,
		CustomerAvatar:         avatar,
		CustomerLanguage:       lang,
		Status:                 st,
		LastMessageAt:          last,
		RawData:                raw,
	}
}

func inferConversationStatus(m map[string]any, last *time.Time) string {
	// Prefer explicit flags when present; otherwise infer from unread / timestamps.
	if u, ok := m["unread_count"].(float64); ok && u > 0 {
		return "pending_reply"
	}
	if pickStr(m, "reply_status", "conversation_status") != "" {
		s := strings.ToLower(pickStr(m, "reply_status", "conversation_status"))
		if strings.Contains(s, "close") {
			return "closed"
		}
		if strings.Contains(s, "reply") || strings.Contains(s, "wait") || strings.Contains(s, "pending") {
			return "pending_reply"
		}
	}
	if last != nil {
		return "replied"
	}
	return "open"
}

func mapPlatformMessage(m map[string]any) platformp.PlatformCustomerMessage {
	if m == nil {
		return platformp.PlatformCustomerMessage{}
	}
	ext := formatConversationExternalID(firstAny(m, "message_id", "msg_id", "id"))
	content := pickStr(m, "content", "text")
	if content == "" {
		if c, ok := m["content"].(map[string]any); ok {
			content = pickStr(c, "text", "message")
		}
	}
	role := inferMessageRole(m)
	mt := mapMessageType(m)
	lang := pickStr(m, "language")
	if lang == "" {
		lang = "en"
	}
	created := parseUnixAny(firstAny(m, "created_timestamp", "timestamp", "create_time", "message_time"))

	raw := platformp.TrimRawMap(map[string]any{
		"source":       "shopee",
		"message_id":   ext,
		"message_type": mt,
		"from_shop":    m["from_shop_id"],
		"from_user":    m["from_user_id"],
	}, 12, 400)

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

func inferMessageRole(m map[string]any) string {
	rawType := strings.TrimSpace(fmt.Sprint(m["message_type"]))
	if strings.EqualFold(rawType, "system") {
		return "system"
	}
	// Shop/seller-originated messages (field names vary by API version).
	if fs, ok := m["from_shop_id"].(float64); ok && fs != 0 {
		return "agent"
	}
	if fs, ok := m["from_shop_id"].(int64); ok && fs != 0 {
		return "agent"
	}
	if s := strings.TrimSpace(pickStr(m, "from_shop_id")); s != "" && s != "0" {
		return "agent"
	}
	if b, ok := m["from_shop"].(bool); ok && b {
		return "agent"
	}
	if s := strings.ToLower(pickStr(m, "sender_role", "source")); s == "seller" || s == "shop" {
		return "agent"
	}
	return "customer"
}

func mapMessageType(m map[string]any) string {
	// May be int or string in upstream JSON.
	switch t := m["message_type"].(type) {
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		switch s {
		case "text", "":
			return "text"
		case "image", "photo", "picture":
			return "image"
		case "order", "order_card":
			return "order"
		case "system":
			return "system"
		default:
			if strings.Contains(s, "image") || strings.Contains(s, "pic") {
				return "image"
			}
			if strings.Contains(s, "order") {
				return "order"
			}
			return "text"
		}
	case float64:
		n := int64(t)
		// Heuristic mapping — align enum integers with Partner Center Seller Chat docs.
		switch n {
		case 1:
			return "text"
		case 2, 3:
			return "image"
		case 8, 9, 10:
			return "order"
		default:
			if n == 0 {
				return "text"
			}
			return "text"
		}
	default:
		return "text"
	}
}

func firstAny(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func conversationSliceFrom(root map[string]any) []any {
	if root == nil {
		return nil
	}
	for _, k := range []string{"conversation_list", "conversations", "conversation_data_list"} {
		if a, ok := root[k].([]any); ok {
			return a
		}
	}
	return nil
}

func messageSliceFrom(root map[string]any) []any {
	if root == nil {
		return nil
	}
	for _, k := range []string{"message_list", "messages", "message_data_list"} {
		if a, ok := root[k].([]any); ok {
			return a
		}
	}
	return nil
}

func pickCursor(root map[string]any) string {
	return pickStr(root, "next_cursor", "cursor", "next_offset", "offset")
}

func pickMore(root map[string]any, next string) bool {
	if b, ok := root["more"].(bool); ok {
		return b
	}
	if b, ok := root["has_more"].(bool); ok {
		return b
	}
	return strings.TrimSpace(next) != ""
}

func parsePositiveIntID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty id")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return v, nil
}
