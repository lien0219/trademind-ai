package lazada

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	maxLazadaIMSessionsPerPull = 25
	maxLazadaIMMessagePages    = 40
	lazadaIMConvCursorPrefix   = "lz-im-v1:"
)

type lzIMConvCursor struct {
	NextStartTime int64  `json:"nst"`
	LastSessionID string `json:"lsid,omitempty"`
}

func encodeLZIMConvCursor(c lzIMConvCursor) string {
	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return lazadaIMConvCursorPrefix + base64.RawURLEncoding.EncodeToString(b)
}

func decodeLZIMConvCursor(raw string) (lzIMConvCursor, bool) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, lazadaIMConvCursorPrefix) {
		return lzIMConvCursor{}, false
	}
	payload := strings.TrimPrefix(s, lazadaIMConvCursorPrefix)
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return lzIMConvCursor{}, false
	}
	var c lzIMConvCursor
	if json.Unmarshal(decoded, &c) != nil {
		return lzIMConvCursor{}, false
	}
	return c, true
}

func imDataPayload(root map[string]any) map[string]any {
	if root == nil {
		return nil
	}
	d, _ := root["data"].(map[string]any)
	return d
}

func pickBoolAny(d map[string]any, keys ...string) bool {
	if d == nil {
		return false
	}
	for _, k := range keys {
		if pickBool(d, k) {
			return true
		}
		if v, ok := d[k]; ok {
			if f, ok2 := v.(float64); ok2 && f != 0 {
				return true
			}
		}
	}
	return false
}

func pickStrAny(d map[string]any, keys ...string) string {
	if d == nil {
		return ""
	}
	for _, k := range keys {
		if s := pickStr(d, k); s != "" {
			return s
		}
	}
	return ""
}

func sliceFromIM(d map[string]any, keys ...string) []any {
	if d == nil {
		return nil
	}
	for _, k := range keys {
		if arr, ok := d[k].([]any); ok {
			return arr
		}
	}
	return nil
}

// PullCustomerMessages lists IM sessions then hydrates message history per session (Lazada Open Platform `/im/*`).
func PullCustomerMessages(ctx context.Context, req platformp.PullMessagesRequest) (*platformp.PullMessagesResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, err
	}

	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("missing access_token")
	}

	pageSize := req.Limit
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}

	var cur lzIMConvCursor
	if c, ok := decodeLZIMConvCursor(req.Cursor); ok {
		cur = c
	}

	startMs := time.Now().UTC().UnixMilli()
	if cur.NextStartTime > 0 {
		startMs = cur.NextStartTime
	} else if req.StartTime != nil {
		startMs = req.StartTime.UnixMilli()
	}

	sessExtra := map[string]string{
		"start_time": strconv.FormatInt(startMs, 10),
		"page_size":  strconv.FormatInt(int64(pageSize), 10),
	}
	if sid := strings.TrimSpace(cur.LastSessionID); sid != "" {
		sessExtra["last_session_id"] = sid
	}

	listRoot, httpSt, err := signedGET(ctx, cfg, cfg.APIRESTBase, PathIMSessionList, access, sessExtra)
	err = classifyLazadaCustomerMessageError(httpSt, listRoot, err)
	if err != nil {
		if !errors.Is(err, platformp.ErrPlatformCustomerMessagePermissionDenied) {
			_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		}
		return nil, err
	}

	data := imDataPayload(listRoot)
	sessions := sliceFromIM(data, "session_list", "sessions")
	hasMore := pickBoolAny(data, "has_more", "hasMore")
	nextStart := parseFlexibleInt(firstAny(data, "next_start_time", "nextStartTime"))
	lastSess := pickStrAny(data, "last_session_id", "lastSessionID")

	nextCursor := ""
	if hasMore && (nextStart > 0 || strings.TrimSpace(lastSess) != "") {
		nextCursor = encodeLZIMConvCursor(lzIMConvCursor{
			NextStartTime: nextStart,
			LastSessionID: strings.TrimSpace(lastSess),
		})
	}

	n := len(sessions)
	if n > maxLazadaIMSessionsPerPull {
		sessions = sessions[:maxLazadaIMSessionsPerPull]
		hasMore = true
		if nextCursor == "" {
			nextCursor = encodeLZIMConvCursor(lzIMConvCursor{
				NextStartTime: nextStart,
				LastSessionID: strings.TrimSpace(lastSess),
			})
		}
	}

	out := make([]platformp.PlatformConversation, 0, len(sessions))
	var lastMsgRoot map[string]any

	winStart := req.StartTime
	winEnd := req.EndTime

	for _, row := range sessions {
		sm, ok := row.(map[string]any)
		if !ok {
			continue
		}
		pc := mapLazadaIMSession(sm)
		if strings.TrimSpace(pc.ExternalConversationID) == "" {
			continue
		}
		buyer := buyerIDStringFromSession(sm)

		msgs, msgRoot, perr := pullLazadaMessagesForSession(ctx, cfg, access, pc.ExternalConversationID, buyer, winStart, winEnd)
		if perr != nil {
			if !errors.Is(perr, platformp.ErrPlatformCustomerMessagePermissionDenied) {
				_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
			}
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
		"provider":              "lazada",
		"sessionListHttpStatus": httpSt,
		"sessionsReturned":      n,
		"sessionsHydrated":      len(out),
		"pageSize":              pageSize,
		"sessionHasMore":        hasMore,
		"note":                  "IM session/message fields vary by Lazada venture; verify mappings against latest Partner IM docs.",
	}
	if lastMsgRoot != nil {
		d := imDataPayload(lastMsgRoot)
		summary["lastMessageBlockKeys"] = strings.Join(shallowKeyHints(d, 10), ",")
	}

	return &platformp.PullMessagesResult{
		Conversations: out,
		NextCursor:    nextCursor,
		HasMore:       hasMore,
		RawSummary:    platformp.TrimRawMap(summary, 20, 400),
	}, nil
}

func shallowKeyHints(m map[string]any, max int) []string {
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

func firstAny(m map[string]any, keys ...string) any {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func pullLazadaMessagesForSession(ctx context.Context, cfg RuntimeConfig, access, sessionID, buyer string, winStart, winEnd *time.Time) ([]platformp.PlatformCustomerMessage, map[string]any, error) {
	listStart := int64(0)
	if winStart != nil {
		listStart = winStart.UnixMilli()
	}

	lastMID := ""
	var all []platformp.PlatformCustomerMessage
	var lastRoot map[string]any

	for page := 0; page < maxLazadaIMMessagePages; page++ {
		extra := map[string]string{
			"session_id": strings.TrimSpace(sessionID),
			"start_time": strconv.FormatInt(listStart, 10),
			"page_size":  "50",
		}
		if strings.TrimSpace(lastMID) != "" {
			extra["last_message_id"] = strings.TrimSpace(lastMID)
		}

		root, httpSt, err := signedGET(ctx, cfg, cfg.APIRESTBase, PathIMMessageList, access, extra)
		err = classifyLazadaCustomerMessageError(httpSt, root, err)
		if err != nil {
			return nil, nil, err
		}
		lastRoot = root
		data := imDataPayload(root)

		arr := sliceFromIM(data, "message_list", "messages")
		for _, row := range arr {
			mm, ok := row.(map[string]any)
			if !ok {
				continue
			}
			pm := mapLazadaIMMessage(mm, buyer)
			if strings.TrimSpace(pm.ExternalMessageID) == "" {
				continue
			}
			if winStart != nil && pm.CreatedAt != nil && pm.CreatedAt.Before(*winStart) {
				continue
			}
			if winEnd != nil && pm.CreatedAt != nil && pm.CreatedAt.After(*winEnd) {
				continue
			}
			all = append(all, pm)
		}

		hasMore := pickBoolAny(data, "has_more", "hasMore")
		nextStart := parseFlexibleInt(firstAny(data, "next_start_time", "nextStartTime"))
		lastMID = pickStrAny(data, "last_message_id", "lastMessageId")

		if !hasMore || strings.TrimSpace(lastMID) == "" {
			break
		}
		if nextStart > 0 {
			listStart = nextStart
		}
	}

	return all, lastRoot, nil
}

// SendCustomerMessage posts a seller text reply via Lazada `/im/message/send` (template_id=1).
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

	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}

	extra := map[string]string{
		"session_id":  extConv,
		"template_id": strconv.Itoa(lazadaTplText),
		"txt":         reply,
	}

	root, httpSt, err := signedPOSTForm(ctx, cfg, cfg.APIRESTBase, PathIMMessageSend, access, extra)
	err = classifyLazadaCustomerMessageError(httpSt, root, err)
	if err != nil {
		if !errors.Is(err, platformp.ErrPlatformCustomerMessagePermissionDenied) {
			_ = setAuthStatusMaybe(ctx, req.ShopID, "error")
		}
		return nil, err
	}

	data := imDataPayload(root)
	extMid := strings.TrimSpace(fmt.Sprint(firstAny(data, "message_id", "msg_id")))
	sendMs := parseFlexibleInt(firstAny(data, "current_time", "send_time"))

	raw := platformp.TrimRawMap(map[string]any{
		"provider":           "lazada",
		"httpStatus":         httpSt,
		"idempotencyLocal":   strings.TrimSpace(req.IdempotencyKey),
		"sessionIdTruncated": trimOneLine(extConv, 48),
	}, 12, 400)

	return &platformp.SendMessageResult{
		ExternalMessageID: extMid,
		SentAt:            unixMillisPtr(sendMs),
		RawSummary:        raw,
	}, nil
}
