package collectdomain

import "testing"

func TestDetectPlatform(t *testing.T) {
	cases := []struct {
		url  string
		want PlatformID
		ok   bool
	}{
		{"https://detail.1688.com/offer/1.html", Platform1688, true},
		{"https://m.1688.com/", Platform1688, true},
		{"https://login.1688.com/", Platform1688, true},
		{"https://www.aliexpress.com/item/1.html", PlatformAliExpress, true},
		{"https://es.aliexpress.com/item/1.html", PlatformAliExpress, true},
		{"https://item.taobao.com/item.htm?id=1", PlatformTaobaoTmall, true},
		{"https://detail.tmall.com/item.htm?id=1", PlatformTaobaoTmall, true},
		{"https://detail.tmall.hk/item.htm?id=1", PlatformTaobaoTmall, true},
		{"https://world.taobao.com/item/123.htm", PlatformTaobaoTmall, true},
		{"https://mobile.yangkeduo.com/goods.html", PlatformPdd, true},
		{"https://www.shein.com/p-1.html", PlatformSheinTemu, true},
		{"https://www.temu.com/goods.html", PlatformSheinTemu, true},
		{"https://example.com/product/1", "", false},
		{"not-a-url", "", false},
	}
	for _, tc := range cases {
		host := HostnameFromURL(tc.url)
		got, ok := DetectPlatform(host)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("url=%q host=%q got=%q ok=%v want=%q ok=%v", tc.url, host, got, ok, tc.want, tc.ok)
		}
	}
}
