package safefields

import (
	"net/url"
	"strings"
)

// DefaultSensitiveKeys are field names redacted in logs, outputs and raw responses.
var DefaultSensitiveKeys = []string{
	"access_token", "refresh_token", "app_secret", "secret", "authorization", "code",
	"encrypt_post_receiver", "encrypt_post_tel", "encrypt_post_addr",
	"mobile", "phone", "address", "receiver",
	"accesstoken", "refreshtoken", "appsecret", "client_secret", "clientsecret",
	"password", "passwd", "api_key", "apikey",
}

// RedactValue returns a masked copy of v with sensitive keys replaced recursively.
func RedactValue(v any, extraKeys ...string) any {
	keys := mergeKeys(extraKeys)
	return redactAny(v, keys)
}

// RedactMap redacts a string-keyed map in place (returns new map).
func RedactMap(in map[string]any, extraKeys ...string) map[string]any {
	if in == nil {
		return nil
	}
	keys := mergeKeys(extraKeys)
	out := make(map[string]any, len(in))
	for k, v := range in {
		if isSensitiveKey(k, keys) {
			out[k] = "****"
			continue
		}
		out[k] = redactAny(v, keys)
	}
	return out
}

// RedactString masks known sensitive substrings in free text.
func RedactString(raw string) string {
	msg := strings.TrimSpace(raw)
	if msg == "" {
		return ""
	}
	low := strings.ToLower(msg)
	for _, marker := range DefaultSensitiveKeys {
		if strings.Contains(low, marker) {
			return "[redacted sensitive content]"
		}
	}
	if len(msg) > 500 {
		msg = msg[:500] + "..."
	}
	return msg
}

// RedactURL masks sensitive query parameters.
func RedactURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return RedactString(raw)
	}
	if u.User != nil {
		u.User = url.UserPassword("****", "****")
	}
	q := u.Query()
	for k := range q {
		if isSensitiveKey(k, mergeKeys(nil)) {
			q.Set(k, "****")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// RedactHeaders masks sensitive HTTP header values.
func RedactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if isSensitiveKey(k, mergeKeys(nil)) {
			out[k] = "****"
			continue
		}
		out[k] = RedactString(v)
	}
	return out
}

func mergeKeys(extra []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, k := range DefaultSensitiveKeys {
		out[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}
	for _, k := range extra {
		k = strings.ToLower(strings.TrimSpace(k))
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

func isSensitiveKey(key string, keys map[string]struct{}) bool {
	kl := strings.ToLower(strings.TrimSpace(key))
	if kl == "" {
		return false
	}
	if _, ok := keys[kl]; ok {
		return true
	}
	for marker := range keys {
		if strings.Contains(kl, marker) {
			return true
		}
	}
	return false
}

func redactAny(v any, keys map[string]struct{}) any {
	switch x := v.(type) {
	case string:
		return RedactString(x)
	case map[string]any:
		return RedactMap(x)
	case map[string]string:
		out := map[string]any{}
		for k, val := range x {
			if isSensitiveKey(k, keys) {
				out[k] = "****"
			} else {
				out[k] = RedactString(val)
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(x))
		for i, item := range x {
			if i >= 50 {
				out = append(out, "...truncated")
				break
			}
			out = append(out, redactAny(item, keys))
		}
		return out
	default:
		return v
	}
}
