package collectdomain

import (
	"net/url"
	"strings"
)

var taobaoTmallSupportedProductHosts = map[string]struct{}{
	"item.taobao.com":   {},
	"detail.tmall.com":  {},
	"detail.tmall.hk":   {},
	"world.taobao.com":  {},
	"chaoshi.tmall.com": {},
	"ju.taobao.com":     {},
}

// IsTaobaoEcosystemHost reports whether hostname belongs to taobao/tmall ecosystem.
func IsTaobaoEcosystemHost(hostname string) bool {
	host := strings.ToLower(strings.TrimSpace(hostname))
	if host == "" {
		return false
	}
	if _, ok := taobaoTmallSupportedProductHosts[host]; ok {
		return true
	}
	if host == "taobao.com" || strings.HasSuffix(host, ".taobao.com") {
		return true
	}
	if host == "tmall.com" || host == "tmall.hk" || strings.HasSuffix(host, ".tmall.com") || strings.HasSuffix(host, ".tmall.hk") {
		return true
	}
	return false
}

// ClassifyTaobaoTmallURL returns product_detail, unsupported_taobao, or empty when not taobao ecosystem.
func ClassifyTaobaoTmallURL(urlStr string) string {
	u, err := url.Parse(strings.TrimSpace(urlStr))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return ""
	}
	if _, ok := taobaoTmallSupportedProductHosts[host]; ok {
		return "product_detail"
	}
	if IsTaobaoEcosystemHost(host) {
		return "unsupported_taobao"
	}
	return ""
}

// IsSupportedTaobaoTmallProductURL is true for standard taobao/tmall product detail hosts.
func IsSupportedTaobaoTmallProductURL(urlStr string) bool {
	return ClassifyTaobaoTmallURL(urlStr) == "product_detail"
}
