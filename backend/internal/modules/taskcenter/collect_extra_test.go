package taskcenter

import (
	"strings"
	"testing"
)

func TestCollectFailureContextPinduoduoLoginHintStaysWithinSupportedProviderScope(t *testing.T) {
	_, access, suggestion := collectFailureContextExtras(
		"https://pifa.pinduoduo.com/goods/detail/?gid=123",
		"LOGIN_REQUIRED",
		"login_required",
		"",
	)
	if access != "需要登录" {
		t.Fatalf("access status = %q, want login required", access)
	}
	if strings.Contains(suggestion, "普通商品") {
		t.Fatalf("login recovery must not recommend an unsupported consumer page, got %q", suggestion)
	}
	if !strings.Contains(suggestion, "批发商品详情") {
		t.Fatalf("login recovery should retain the supported Pifa context, got %q", suggestion)
	}
}
