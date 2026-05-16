package imagetask

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var imageErrKeyRedact = regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{8,}\b`)

func redactSensitiveErr(msg string) string {
	return imageErrKeyRedact.ReplaceAllString(msg, "sk-[REDACTED]")
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) > max {
		return strings.TrimSpace(string(r[:max])) + "…"
	}
	return s
}

func processedObjectTag(provider string) string {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "removebg":
		return "removebg"
	case "openai_image":
		return "openai-image"
	default:
		p := strings.TrimSpace(strings.ToLower(provider))
		if p == "" {
			return "processed"
		}
		return strings.ReplaceAll(strings.ReplaceAll(p, "_", "-"), "/", "-")
	}
}
