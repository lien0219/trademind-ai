package collectrule

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeRuleDomain strips scheme/path/port from user input so matching uses a hostname only.
// Examples: "https://www.1688.com/" → "www.1688.com", "1688.com" → "1688.com".
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

func ruleMatchError(host string, rule *CollectRule) error {
	if rule == nil {
		return fmt.Errorf("url does not match rule domain or pattern")
	}
	d := NormalizeRuleDomain(rule.Domain)
	mp := strings.TrimSpace(rule.MatchPattern)
	if !domainMatches(host, d) {
		hint := siblingSubdomainHint(host, d)
		return fmt.Errorf("url host %q does not match rule domain %q%s", host, d, hint)
	}
	if mp != "" {
		return fmt.Errorf("url does not match rule pattern %q", mp)
	}
	return fmt.Errorf("url does not match rule domain or pattern")
}

func siblingSubdomainHint(host, domain string) string {
	if host == "" || domain == "" || host == domain || domainMatches(host, domain) {
		return ""
	}
	partsH := strings.Split(host, ".")
	partsD := strings.Split(domain, ".")
	if len(partsH) < 2 || len(partsD) < 2 {
		return ""
	}
	baseH := strings.Join(partsH[len(partsH)-2:], ".")
	baseD := strings.Join(partsD[len(partsD)-2:], ".")
	if baseH == baseD {
		return fmt.Sprintf("；链接主机为 %q，规则域名为 %q，请改用 %q 以匹配所有子域", host, domain, baseH)
	}
	return ""
}
