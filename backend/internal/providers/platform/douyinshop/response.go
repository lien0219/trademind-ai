package douyinshop

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type envelope struct {
	Code      string
	Message   string
	RequestID string
	Data      any
	Raw       map[string]any
}

func parseEnvelope(body []byte) (*envelope, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, NewError(CodeDouyinResponseParseFailed, "douyin openapi response parse failed", "", err.Error(), "")
	}
	env := &envelope{
		Code:      apiCode(raw),
		Message:   apiMessage(raw),
		RequestID: pickRawString(raw, "log_id", "logId", "request_id", "requestId"),
		Data:      raw["data"],
		Raw:       raw,
	}
	return env, nil
}

func (e *envelope) success() bool {
	if e == nil {
		return false
	}
	code := strings.TrimSpace(e.Code)
	if code == "" || code == "0" || code == "10000" || strings.EqualFold(code, "success") {
		return true
	}
	return false
}

func (e *envelope) decodeData(out any) error {
	if out == nil {
		return nil
	}
	if e == nil {
		return NewError(CodeDouyinResponseParseFailed, "douyin openapi response parse failed", "", "empty envelope", "")
	}
	data := e.Data
	if data == nil {
		data = e.Raw
	}
	b, err := json.Marshal(data)
	if err != nil {
		return NewError(CodeDouyinResponseParseFailed, "douyin openapi response parse failed", "", err.Error(), e.RequestID)
	}
	if err := json.Unmarshal(b, out); err != nil {
		return NewError(CodeDouyinResponseParseFailed, "douyin openapi response parse failed", "", err.Error(), e.RequestID)
	}
	return nil
}

func apiCode(env map[string]any) string {
	for _, k := range []string{"code", "err_no", "errno"} {
		if s := stringFromAny(env[k]); s != "" {
			return s
		}
	}
	return ""
}

func apiMessage(env map[string]any) string {
	for _, k := range []string{"msg", "message", "sub_msg", "subMsg"} {
		if s := stringFromAny(env[k]); s != "" {
			return s
		}
	}
	return ""
}

func pickRawString(data map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := stringFromAny(data[k]); s != "" {
			return s
		}
	}
	return ""
}

func stringFromAny(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case float64:
		if float64(int64(x)) == x {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case json.Number:
		return strings.TrimSpace(x.String())
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func int64FromAny(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	default:
		return 0
	}
}

func pickString(data map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := stringFromAny(data[k]); s != "" {
			return s
		}
	}
	return ""
}

func pickExpiresAt(now time.Time, data map[string]any, keys ...string) *time.Time {
	for _, k := range keys {
		if n := int64FromAny(data[k]); n > 0 {
			var t time.Time
			if n > now.Unix() {
				t = time.Unix(n, 0).UTC()
			} else {
				t = now.Add(time.Duration(n) * time.Second).UTC()
			}
			return &t
		}
	}
	return nil
}
