package admin

import (
	"strings"
	"unicode"
)

// NormalizePhoneDigits extracts digits for phone lookup (allows +86 / spaces; keeps 10–15 digits).
func NormalizePhoneDigits(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if len(d) >= 13 && strings.HasPrefix(d, "86") && d[2] == '1' {
		// e.g. 86138xxxxxxxx -> 11-digit domestic
		d = d[len(d)-11:]
	}
	return d
}

// ParseLoginAccount classifies input as email or phone. ok is false if empty or invalid.
func ParseLoginAccount(account string) (email string, phone string, ok bool) {
	raw := strings.TrimSpace(account)
	if raw == "" {
		return "", "", false
	}
	if strings.Contains(raw, "@") {
		at := strings.LastIndex(raw, "@")
		if at <= 0 || at >= len(raw)-1 {
			return "", "", false
		}
		local := strings.TrimSpace(raw[:at])
		domain := strings.TrimSpace(raw[at+1:])
		if local == "" || domain == "" {
			return "", "", false
		}
		return strings.ToLower(strings.TrimSpace(raw)), "", true
	}
	phone = NormalizePhoneDigits(raw)
	if len(phone) < 10 || len(phone) > 15 {
		return "", "", false
	}
	return "", phone, true
}
