package taskcenter

import "unicode/utf8"

func truncateRunes(s string, n int) string {
	s = trimmed(s)
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	var rb []rune
	for _, r := range s {
		rb = append(rb, r)
		if len(rb) >= n {
			break
		}
	}
	return string(rb) + "…"
}

func trimmed(s string) string {
	// callers pass trimmed-ish strings; keep light trim
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t' || s[i] == '\r') {
		i++
	}
	for i < j && (s[j-1] == ' ' || s[j-1] == '\n' || s[j-1] == '\t' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}
