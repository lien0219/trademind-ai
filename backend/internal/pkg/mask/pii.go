package mask

import (
	"strings"
	"unicode/utf8"
)

// Phone masks middle digits: 138****5678.
func Phone(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	n := len(runes)
	if n <= 4 {
		return strings.Repeat("*", n)
	}
	if n <= 7 {
		return string(runes[:2]) + strings.Repeat("*", n-4) + string(runes[n-2:])
	}
	keepHead, keepTail := 3, 4
	if n < keepHead+keepTail+2 {
		keepHead, keepTail = 2, 2
	}
	return string(runes[:keepHead]) + strings.Repeat("*", n-keepHead-keepTail) + string(runes[n-keepTail:])
}

// Email masks local part: ab***@example.com.
func Email(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 {
		return maskRunes(s, 2, 0)
	}
	local := s[:at]
	domain := s[at:]
	return maskRunes(local, 2, 0) + domain
}

func maskRunes(s string, head, tail int) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return ""
	}
	if head+tail >= n {
		if n <= 2 {
			return strings.Repeat("*", n)
		}
		return string(runes[:1]) + strings.Repeat("*", n-2) + string(runes[n-1:])
	}
	return string(runes[:head]) + strings.Repeat("*", n-head-tail) + string(runes[n-tail:])
}

// Address keeps first few runes for display.
func Address(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= 8 {
		return maskRunes(s, 2, 0)
	}
	return maskRunes(s, 6, 0)
}
