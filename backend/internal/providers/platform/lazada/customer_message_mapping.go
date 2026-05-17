package lazada

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

// Lazada IM template_id values (Partner Center IM docs). Adjust when official enums change.
const (
	lazadaTplText       = 1
	lazadaTplEmoji      = 4
	lazadaTplImage      = 3
	lazadaTplProduct    = 10006
	lazadaTplOrder      = 10007
	lazadaTplPromotion  = 10008
	lazadaTplInviteSubs = 10010
)

func parseFlexibleInt(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err == nil {
			return n
		}
	}
	return 0
}

func unixMillisPtr(ms int64) *time.Time {
	if ms <= 0 {
		return nil
	}
	var sec int64
	var nsec int64
	if ms > 1_000_000_000_000 {
		sec = ms / 1000
		nsec = (ms % 1000) * 1e6
	} else {
		sec = ms
		nsec = 0
	}
	t := time.Unix(sec, nsec).UTC()
	return &t
}

func buyerIDStringFromSession(sess map[string]any) string {
	if sess == nil {
		return ""
	}
	n := parseFlexibleInt(sess["buyer_id"])
	if n != 0 {
		return strconv.FormatInt(n, 10)
	}
	return strings.TrimSpace(fmt.Sprint(sess["buyer_id"]))
}

func extractTxtFromContentJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return raw
	}
	for _, k := range []string{"txt", "text", "translatText", "translate_text"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func lazadaTemplateMessageType(tplID int64) string {
	switch tplID {
	case lazadaTplImage:
		return "image"
	case lazadaTplOrder:
		return "order"
	case lazadaTplProduct, lazadaTplPromotion, lazadaTplInviteSubs:
		return "system"
	default:
		return "text"
	}
}

func mapLazadaIMMessage(row map[string]any, buyerAccountID string) platformp.PlatformCustomerMessage {
	if row == nil {
		return platformp.PlatformCustomerMessage{}
	}
	tpl := parseFlexibleInt(row["template_id"])
	from := strings.TrimSpace(fmt.Sprint(row["from_account_id"]))
	sendMs := parseFlexibleInt(row["send_time"])

	var role string
	if pickBool(row, "auto_reply") {
		role = "system"
	} else if buyerAccountID != "" && from == buyerAccountID {
		role = "customer"
	} else {
		role = "agent"
	}

	mt := lazadaTemplateMessageType(tpl)
	if tpl == lazadaTplText || tpl == lazadaTplEmoji {
		mt = "text"
	}

	content := extractTxtFromContentJSON(strings.TrimSpace(fmt.Sprint(row["content"])))
	if content == "" && mt != "text" {
		content = fmt.Sprintf("[%s]", mt)
	}

	extMid := strings.TrimSpace(fmt.Sprint(row["message_id"]))
	raw := platformp.TrimRawMap(map[string]any{
		"provider":        "lazada",
		"templateId":      tpl,
		"typeField":       parseFlexibleInt(row["type"]),
		"statusField":     parseFlexibleInt(row["status"]),
		"siteId":          pickStr(row, "site_id"),
		"fromAccountType": parseFlexibleInt(row["from_account_type"]),
	}, 10, 240)

	return platformp.PlatformCustomerMessage{
		ExternalMessageID: extMid,
		Role:              role,
		Content:           content,
		MessageType:       mt,
		Language:          "en",
		CreatedAt:         unixMillisPtr(sendMs),
		RawData:           raw,
	}
}

func mapLazadaIMSession(sess map[string]any) platformp.PlatformConversation {
	if sess == nil {
		return platformp.PlatformConversation{}
	}
	ext := strings.TrimSpace(fmt.Sprint(sess["session_id"]))
	title := pickStr(sess, "title", "summary")
	head := strings.TrimSpace(fmt.Sprint(sess["head_url"]))
	lastMs := parseFlexibleInt(sess["last_message_time"])
	unread := parseFlexibleInt(sess["unread_count"])

	st := "open"
	if unread > 0 {
		st = "pending_reply"
	}

	raw := platformp.TrimRawMap(map[string]any{
		"provider":       "lazada",
		"buyerIdPresent": buyerIDStringFromSession(sess) != "",
		"siteId":         pickStr(sess, "site_id"),
		"lastMsgSnippet": trimOneLine(sess["summary"], 160),
	}, 10, 280)

	pc := platformp.PlatformConversation{
		ExternalConversationID: ext,
		CustomerName:           title,
		CustomerAvatar:         head,
		CustomerLanguage:       "en",
		Status:                 st,
		LastMessageAt:          unixMillisPtr(lastMs),
		Messages:               nil,
		RawData:                raw,
	}
	return pc
}

func trimOneLine(v any, max int) string {
	s := strings.TrimSpace(fmt.Sprint(v))
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func pickBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true") || strings.TrimSpace(t) == "1"
	case float64:
		return t != 0
	default:
		return false
	}
}
