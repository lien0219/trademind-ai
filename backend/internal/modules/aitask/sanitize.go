package aitask

import (
	"encoding/json"
	"strings"
)

// redactSensitiveJSON walks JSON values and replaces known secret-bearing keys.
// Used for API responses so api_key / tokens never leave the server in task payloads.
func redactSensitiveJSON(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return json.RawMessage(`{}`)
	}
	redactJSONValue(&v)
	out, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return out
}

func redactJSONValue(v *any) {
	if v == nil {
		return
	}
	switch t := (*v).(type) {
	case map[string]any:
		for k, val := range t {
			if sensitiveJSONKey(k) {
				t[k] = "[REDACTED]"
				continue
			}
			redactJSONValue(&val)
			t[k] = val
		}
		*v = t
	case []any:
		for i := range t {
			redactJSONValue(&t[i])
		}
		*v = t
	default:
		// scalars: leave as-is
	}
}

func sensitiveJSONKey(k string) bool {
	lk := strings.ToLower(strings.TrimSpace(k))
	lk = strings.ReplaceAll(lk, "-", "_")
	switch lk {
	case "api_key", "apikey",
		"authorization",
		"secret_key", "secretkey",
		"access_token", "accesstoken",
		"refresh_token", "refreshtoken",
		"password",
		"app_secret", "appsecret",
		"client_secret", "clientsecret":
		return true
	default:
		return false
	}
}
