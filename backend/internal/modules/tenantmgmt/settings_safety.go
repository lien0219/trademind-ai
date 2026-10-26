package tenantmgmt

import (
	"errors"
	"strings"
)

// Enterprise extension metadata is not an encrypted secrets store. Reject
// obvious credentials and JSON prototype-pollution keys at the API boundary,
// regardless of whether the caller uses the graphical settings editor.
func rejectSensitiveTenantSettings(value any, depth int) error {
	if depth > 32 {
		return errors.New("tenant settings nesting is too deep")
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if unsafeTenantSettingsKey(key) {
				return errors.New("tenant settings contain forbidden credential or unsafe field")
			}
			if err := rejectSensitiveTenantSettings(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range typed {
			if err := rejectSensitiveTenantSettings(child, depth+1); err != nil {
				return err
			}
		}
	case string:
		normalized := strings.TrimSpace(strings.ToLower(typed))
		if strings.HasPrefix(normalized, "bearer ") ||
			strings.Contains(normalized, "-----begin private key-----") ||
			(strings.HasPrefix(normalized, "sk-") && len(normalized) >= 19) {
			return errors.New("tenant settings contain a possible credential")
		}
	}
	return nil
}

func unsafeTenantSettingsKey(key string) bool {
	lowered := strings.ToLower(key)
	if lowered == "__proto__" || lowered == "constructor" || lowered == "prototype" {
		return true
	}
	var builder strings.Builder
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
	}
	normalized := builder.String()
	for _, part := range []string{
		"password", "passwd", "secret", "token", "credential",
		"apikey", "privatekey", "authorization", "authheader", "cookie", "sessionid",
	} {
		if strings.Contains(normalized, part) {
			return true
		}
	}
	return false
}
