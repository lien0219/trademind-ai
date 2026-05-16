package httppublic

import (
	"net"
	"net/url"
	"strings"
)

// IsPublicHTTPURL reports whether raw looks like a public http(s) URL that third-party
// services (e.g. remove.bg image_url) can typically fetch. Domains are treated as public
// unless the host resolves syntactically to a blocked address (no DNS lookup).
// See PROGRESS.md for limitations (e.g. intranet hostname pointing to private IP).
func IsPublicHTTPURL(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(s), "file:") {
		return false
	}
	// Relative or path-only URLs are not publicly reachable from the internet.
	if !strings.Contains(s, "://") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u == nil {
		return false
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return false
	}
	hl := strings.ToLower(host)
	if hl == "localhost" || hl == "0.0.0.0" || strings.HasSuffix(hl, ".localhost") {
		return false
	}
	switch hl {
	case "127.0.0.1", "::1":
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false
		}
		return true
	}
	return true
}
