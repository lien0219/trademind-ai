package collectdomain

import (
	"net/url"
	"strings"
)

// NormalizeRuleDomain strips scheme/path/port from user input so matching uses a hostname only.
func NormalizeRuleDomain(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return ""
	}
	if strings.Contains(s, "://") {
		if u, err := url.Parse(s); err == nil {
			if h := strings.TrimSpace(u.Hostname()); h != "" {
				return h
			}
		}
	}
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(s, ".")
}

// DomainMatches returns true when host equals domain or is a subdomain of domain.
func DomainMatches(host, domain string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	d := NormalizeRuleDomain(domain)
	if d == "" || h == "" {
		return false
	}
	return h == d || strings.HasSuffix(h, "."+d)
}
