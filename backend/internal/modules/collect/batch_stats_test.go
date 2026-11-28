package collect

import (
	"strings"
	"testing"
)

func TestCollectFailureHintPinduoduoUsesSupportedWholesaleScope(t *testing.T) {
	unsupported := collectFailureHint("UNSUPPORTED_PINDUODUO_URL", "pinduoduo", false)
	if !strings.Contains(unsupported, "pifa.pinduoduo.com/goods/detail/?gid=") {
		t.Fatalf("unsupported URL hint must name the supported Pifa detail pattern, got %q", unsupported)
	}
	if strings.Contains(unsupported, "移动端商品页暂未完整支持") {
		t.Fatalf("unsupported URL hint must not imply mobile collection is partially supported, got %q", unsupported)
	}

	login := collectFailureHint("LOGIN_REQUIRED", "pinduoduo", false)
	if strings.Contains(login, "普通商品") || strings.Contains(login, "公开商品详情页") {
		t.Fatalf("login recovery must not recommend an unsupported consumer page, got %q", login)
	}
}
