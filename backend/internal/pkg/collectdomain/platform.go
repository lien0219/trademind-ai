package collectdomain

import (
	"net/url"
	"strings"
)

// PlatformID identifies a dedicated collect provider platform from URL hostname.
type PlatformID string

const (
	Platform1688        PlatformID = "1688"
	PlatformAliExpress  PlatformID = "aliexpress"
	PlatformTaobaoTmall PlatformID = "taobao"
	PlatformPdd         PlatformID = "pdd"
	PlatformSheinTemu   PlatformID = "shein_temu"
)

// HostnameFromURL returns lowercase hostname or empty when invalid.
func HostnameFromURL(urlStr string) string {
	s := strings.TrimSpace(urlStr)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(u.Hostname()))
}

func hostMatches1688(host string) bool {
	return host == "1688.com" || strings.HasSuffix(host, ".1688.com")
}

func hostMatchesAliExpress(host string) bool {
	return strings.Contains(host, "aliexpress")
}

func hostMatchesTaobaoTmall(host string) bool {
	if host == "taobao.com" || strings.HasSuffix(host, ".taobao.com") {
		return true
	}
	if host == "tmall.com" || strings.HasSuffix(host, ".tmall.com") {
		return true
	}
	return host == "item.taobao.com" || host == "detail.tmall.com"
}

func hostMatchesPdd(host string) bool {
	switch host {
	case "pinduoduo.com", "yangkeduo.com", "mobile.yangkeduo.com":
		return true
	}
	return strings.HasSuffix(host, ".pinduoduo.com") || strings.HasSuffix(host, ".yangkeduo.com")
}

func hostMatchesSheinTemu(host string) bool {
	if host == "shein.com" || strings.HasSuffix(host, ".shein.com") {
		return true
	}
	return host == "temu.com" || strings.HasSuffix(host, ".temu.com")
}

// DetectPlatform maps a hostname to a dedicated platform id when recognized.
func DetectPlatform(hostname string) (PlatformID, bool) {
	host := strings.ToLower(strings.TrimSpace(hostname))
	if host == "" {
		return "", false
	}
	switch {
	case hostMatches1688(host):
		return Platform1688, true
	case hostMatchesAliExpress(host):
		return PlatformAliExpress, true
	case hostMatchesTaobaoTmall(host):
		return PlatformTaobaoTmall, true
	case hostMatchesPdd(host):
		return PlatformPdd, true
	case hostMatchesSheinTemu(host):
		return PlatformSheinTemu, true
	default:
		return "", false
	}
}

// ProviderSourceForPlatform maps platform id to collect task source key.
func ProviderSourceForPlatform(p PlatformID) string {
	return string(p)
}
