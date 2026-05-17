package tiktok

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const maxConversationsWithMessagesPerPull = 25

// PullCustomerMessages implements platform.PullMessages for TikTok Shop Customer Service APIs.
func PullCustomerMessages(ctx context.Context, req platformp.PullMessagesRequest) (*platformp.PullMessagesResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ShopCipher) == "" {
		return nil, fmt.Errorf("tiktok shop_cipher missing: complete OAuth so shop_auth_tokens carries cipher")
	}
	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("missing access_token")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	client := http.Client{Timeout: cfg.HTTPTimeout}
	path := customerConvSearchPath(cfg.APIVersion)
	body := map[string]interface{}{
		"page_size": limit,
	}
	if c := strings.TrimSpace(req.Cursor); c != "" {
		body["cursor"] = c
		body["page_token"] = c
	}
	if req.StartTime != nil {
		body["start_time"] = req.StartTime.Unix()
	}
	if req.EndTime != nil {
		body["end_time"] = req.EndTime.Unix()
	}

	convRoot, _, err := postCustomerServiceJSON(ctx, client, cfg, path, access, body)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}
	data := extractDataPayload(convRoot)
	rawRows := conversationArrayFromPayload(data)
	next := nextCursorFromPayload(data)
	more := hasMoreFromPayload(data, next)

	n := len(rawRows)
	if n > maxConversationsWithMessagesPerPull {
		rawRows = rawRows[:maxConversationsWithMessagesPerPull]
	}

	out := make([]platformp.PlatformConversation, 0, len(rawRows))
	var lastMsgRoot map[string]interface{}
	for _, row := range rawRows {
		cm, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		pc := mapPlatformConversation(cm)
		if strings.TrimSpace(pc.ExternalConversationID) == "" {
			continue
		}
		msgs, msgRoot, perr := pullMessagesForConversation(ctx, client, cfg, access, cfg.APIVersion, pc.ExternalConversationID)
		if perr != nil {
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

	summary := summarizePullCodes(convRoot, lastMsgRoot)
	summary["conversationPageSize"] = limit
	summary["conversationsReturned"] = n
	summary["conversationsHydrated"] = len(out)

	return &platformp.PullMessagesResult{
		Conversations: out,
		NextCursor:    next,
		HasMore:       more,
		RawSummary:    platformp.TrimRawMap(summary, 20, 400),
	}, nil
}

func pullMessagesForConversation(ctx context.Context, client http.Client, cfg RuntimeConfig, access, ver, conversationID string) ([]platformp.PlatformCustomerMessage, map[string]interface{}, error) {
	primary := customerMessagesPath(ver, conversationID)
	alt := strings.TrimSuffix(primary, "/search")
	paths := []string{primary}
	if alt != primary {
		paths = append(paths, alt)
	}
	msgBody := map[string]interface{}{
		"page_size": 100,
	}
	var msgRoot map[string]interface{}
	var lastErr error
	for _, p := range paths {
		raw, status, callErr := signedPOSTJSONStatus(ctx, client, cfg, p, access, msgBody)
		if callErr != nil {
			return nil, nil, callErr
		}
		var derr error
		msgRoot, derr = decodeCustomerServiceResponse(raw, status)
		if derr == nil {
			lastErr = nil
			break
		}
		lastErr = derr
		if status == http.StatusNotFound && len(paths) > 1 && p == primary {
			continue
		}
		return nil, msgRoot, derr
	}
	if lastErr != nil {
		return nil, msgRoot, lastErr
	}
	payload := extractDataPayload(msgRoot)
	arr := messageArrayFromPayload(payload)
	out := make([]platformp.PlatformCustomerMessage, 0, len(arr))
	for _, row := range arr {
		mm, ok := row.(map[string]interface{})
		if !ok {
			continue
		}
		pm := mapPlatformMessage(mm)
		if strings.TrimSpace(pm.ExternalMessageID) == "" {
			continue
		}
		out = append(out, pm)
	}
	return out, msgRoot, nil
}

// SendCustomerMessage posts a seller reply via TikTok Customer Service API.
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
	if strings.TrimSpace(cfg.ShopCipher) == "" {
		return nil, fmt.Errorf("tiktok shop_cipher missing: complete OAuth so shop_auth_tokens carries cipher")
	}
	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}

	client := http.Client{Timeout: cfg.HTTPTimeout}
	sendPath := customerSendPath(cfg.APIVersion, extConv)
	body := map[string]interface{}{
		"content": map[string]interface{}{
			"text": reply,
		},
		"message_type": "TEXT",
	}
	if idem := strings.TrimSpace(req.IdempotencyKey); idem != "" {
		// Field name may differ by region/version — adjust against Partner Center if send fails with unknown field.
		body["idempotency_key"] = idem
	}

	root, _, err := postCustomerServiceJSON(ctx, client, cfg, sendPath, access, body)
	if err != nil {
		_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		return nil, err
	}
	data := extractDataPayload(root)
	extMid := strField(data, "message_id", "id", "msg_id")
	var sentAt *time.Time
	sentAt = parseUnixFlexible(firstIface(data, "create_time", "send_time", "created_time"))

	raw := platformp.TrimRawMap(map[string]any{
		"provider":         "tiktok",
		"apiCode":          parseBizCodeStr(root),
		"idempotencyLocal": strings.TrimSpace(req.IdempotencyKey),
	}, 10, 400)

	return &platformp.SendMessageResult{
		ExternalMessageID: extMid,
		SentAt:            sentAt,
		RawSummary:        raw,
	}, nil
}
