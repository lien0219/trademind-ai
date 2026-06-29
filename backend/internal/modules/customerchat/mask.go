package customerchat

import (
	"strings"
	"unicode/utf8"
)

// maskCustomerName masks buyer name for list/detail display (phone/email-like strings fully masked).
func maskCustomerName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return "—"
	}
	if looksLikeSensitiveContact(s) {
		return maskSensitiveContact(s)
	}
	rs := []rune(s)
	if len(rs) <= 1 {
		return "*"
	}
	if len(rs) == 2 {
		return string(rs[0]) + "*"
	}
	return string(rs[0]) + strings.Repeat("*", len(rs)-2) + string(rs[len(rs)-1])
}

func looksLikeSensitiveContact(s string) bool {
	if strings.Contains(s, "@") {
		return true
	}
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 7
}

func maskSensitiveContact(s string) string {
	if strings.Contains(s, "@") {
		parts := strings.SplitN(s, "@", 2)
		local := parts[0]
		if utf8.RuneCountInString(local) <= 1 {
			local = "*"
		} else {
			local = string([]rune(local)[0]) + "***"
		}
		return local + "@" + parts[1]
	}
	rs := []rune(s)
	if len(rs) <= 4 {
		return "****"
	}
	return string(rs[:3]) + "****" + string(rs[len(rs)-2:])
}
