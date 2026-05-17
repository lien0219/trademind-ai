package shopee

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const maxShopeeConversationsPerPull = 25
const maxShopeeMessagePages = 20

// PullCustomerMessages lists Seller Chat conversations and hydrates each with message history.
func PullCustomerMessages(ctx context.Context, req platformp.PullMessagesRequest) (*platformp.PullMessagesResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}

	access, a2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	sid, err := parseShopID(a2)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("missing access_token")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	q := url.Values{}
	q.Set("direction", "latest")
	q.Set("type", "all")
	q.Set("page_size", fmt.Sprintf("%d", limit))
	if c := strings.TrimSpace(req.Cursor); c != "" {
		q.Set("cursor", c)
	}

	listRoot, httpSt, err := getShopWithStatus(ctx, cfg, PathSellerChatGetConversationList, sid, access, q)
	if err != nil {
		err = classifyShopeeCustomerMessageError(httpSt, listRoot, err)
		if !errors.Is(err, platformp.ErrPlatformCustomerMessagePermissionDenied) {
			_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		}
		return nil, err
	}

	rows := conversationSliceFrom(listRoot)
	next := pickCursor(listRoot)
	more := pickMore(listRoot, next)

	n := len(rows)
	if n > maxShopeeConversationsPerPull {
		rows = rows[:maxShopeeConversationsPerPull]
		more = true
	}

	out := make([]platformp.PlatformConversation, 0, len(rows))
	var lastMsgRoot map[string]any
	for _, row := range rows {
		cm, ok := row.(map[string]any)
		if !ok {
			continue
		}
		pc := mapPlatformConversationFromListRow(cm)
		if strings.TrimSpace(pc.ExternalConversationID) == "" {
			continue
		}
		msgs, msgRoot, perr := pullMessagesForShopeeConversation(ctx, cfg, sid, access, pc.ExternalConversationID)
		if perr != nil {
			_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
			return nil, perr
		}
		lastMsgRoot = msgRoot
		sort.SliceStable(msgs, func(i, j int) bool {
			var ti, tj time.Time
			if msgs[i].CreatedAt != nil {
				ti = *msgs[i].CreatedAt
			}
			if msgs[j].CreatedAt != nil {
				tj = *msgs[j].CreatedAt
			}
			return ti.Before(tj)
		})
		pc.Messages = msgs
		if len(msgs) > 0 {
			last := msgs[len(msgs)-1]
			if last.CreatedAt != nil {
				t := *last.CreatedAt
				pc.LastMessageAt = &t
			}
		}
		out = append(out, pc)
	}

	summary := map[string]any{
		"provider":              "shopee",
		"listHttpStatus":        httpSt,
		"conversationsPageSize": limit,
		"conversationsReturned": n,
		"conversationsHydrated": len(out),
		"nextCursor":            next,
		"note":                  "Shopee sellerchat list filters may not support arbitrary time_from/time_to; align PullMessages start/end when Partner Center documents them.",
	}
	if lastMsgRoot != nil {
		summary["lastMessageKeys"] = strings.Join(shallowKeys(lastMsgRoot, 10), ",")
	}

	return &platformp.PullMessagesResult{
		Conversations: out,
		NextCursor:    next,
		HasMore:       more,
		RawSummary:    platformp.TrimRawMap(summary, 20, 400),
	}, nil
}

func shallowKeys(m map[string]any, max int) []string {
	if m == nil || max <= 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
		if len(out) >= max {
			break
		}
	}
	return out
}

func pullMessagesForShopeeConversation(ctx context.Context, cfg RuntimeConfig, shopID int64, access, convID string) ([]platformp.PlatformCustomerMessage, map[string]any, error) {
	var all []platformp.PlatformCustomerMessage
	msgCursor := ""
	var last map[string]any

	for page := 0; page < maxShopeeMessagePages; page++ {
		q := url.Values{}
		q.Set("conversation_id", convID)
		q.Set("page_size", "100")
		if msgCursor != "" {
			q.Set("cursor", msgCursor)
		}

		root, httpSt, err := getShopWithStatus(ctx, cfg, PathSellerChatGetMessage, shopID, access, q)
		if err != nil {
			return nil, nil, classifyShopeeCustomerMessageError(httpSt, root, err)
		}
		last = root
		arr := messageSliceFrom(root)
		for _, row := range arr {
			mm, ok := row.(map[string]any)
			if !ok {
				continue
			}
			pm := mapPlatformMessage(mm)
			if strings.TrimSpace(pm.ExternalMessageID) == "" {
				continue
			}
			all = append(all, pm)
		}
		msgCursor = pickCursor(root)
		if !pickMore(root, msgCursor) {
			break
		}
		if strings.TrimSpace(msgCursor) == "" {
			break
		}
	}
	return all, last, nil
}

// SendCustomerMessage posts a seller text reply via v2.sellerchat.send_message.
func SendCustomerMessage(ctx context.Context, req platformp.SendMessageRequest) (*platformp.SendMessageResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	reply := strings.TrimSpace(req.Reply)
	if reply == "" {
		return nil, fmt.Errorf("reply is required")
	}
	extConv := strings.TrimSpace(req.ExternalConversationID)
	if extConv == "" {
		return nil, fmt.Errorf("external conversation id required")
	}

	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}

	access, a2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	sid, err := parseShopID(a2)
	if err != nil {
		return nil, err
	}
	if _, err := parsePositiveIntID(extConv); err != nil {
		return nil, fmt.Errorf("invalid shopee conversation id")
	}

	q := url.Values{}
	q.Set("conversation_id", extConv)
	one, httpSt, err := getShopWithStatus(ctx, cfg, PathSellerChatGetOneConversation, sid, access, q)
	if err != nil {
		return nil, classifyShopeeCustomerMessageError(httpSt, one, err)
	}

	toIDStr := pickStr(one, "to_id", "user_id", "buyer_id", "receiver_id", "buyer_user_id")
	var toNum int64
	if strings.TrimSpace(toIDStr) != "" {
		toNum, err = strconv.ParseInt(strings.TrimSpace(toIDStr), 10, 64)
		if err != nil {
			toNum = 0
		}
	}
	if toNum <= 0 {
		if f, ok := one["to_id"].(float64); ok {
			toNum = int64(f)
		}
	}
	if toNum <= 0 {
		return nil, fmt.Errorf("shopee: could not resolve buyer to_id for send_message (check get_one_conversation response fields vs Partner Center docs)")
	}

	body := map[string]any{
		"to_id":        toNum,
		"message_type": shopeeChatTextMessageType,
		"content":      map[string]any{"text": reply},
	}

	res, httpSt2, err := postShopWithStatus(ctx, cfg, PathSellerChatSendMessage, sid, access, body)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, classifyShopeeCustomerMessageError(httpSt2, res, err)
	}

	extMid := pickStr(res, "message_id", "msg_id", "id")
	var sentAt *time.Time
	sentAt = parseUnixAny(firstAny(res, "timestamp", "create_time", "created_timestamp"))
	raw := platformp.TrimRawMap(map[string]any{
		"provider":         "shopee",
		"httpStatus":       httpSt2,
		"idempotencyLocal": strings.TrimSpace(req.IdempotencyKey),
		"conversationId":   extConv,
	}, 12, 400)

	return &platformp.SendMessageResult{
		ExternalMessageID: extMid,
		SentAt:            sentAt,
		RawSummary:        raw,
	}, nil
}
