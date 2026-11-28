package failureclassifier

import (
	"strings"
	"testing"
)

func TestClassify_customJdLoginWall(t *testing.T) {
	in := Input{
		TaskType:     taskTypeCollect,
		Platform:     "custom",
		ErrorMessage: "PAGE_BLOCKED_OR_VERIFY_REQUIRED:verification_or_login_page_detected",
		Title:        "item.jd.com · 10159271815523.html",
		RawSummary:   "source=custom https://item.jd.com/10159271815523.html",
	}
	r := Classify(in)
	if r.MatchedRule == "sub:1688_login_verify" {
		t.Fatalf("JD custom should not match 1688 rule, got %q reason=%q", r.MatchedRule, r.Reason)
	}
	if r.MatchedRule != "sub:custom_collector_login_wall" {
		t.Fatalf("want custom_collector_login_wall, got %q reason=%q suggest=%q", r.MatchedRule, r.Reason, r.SuggestedAction)
	}
}

func TestClassify_1688CollectorLogin(t *testing.T) {
	in := Input{
		TaskType:     taskTypeCollect,
		Platform:     "1688",
		ErrorMessage: "verification_or_login_page_detected",
		Title:        "detail.1688.com · offer.html",
	}
	r := Classify(in)
	if r.MatchedRule != "sub:1688_login_verify" {
		t.Fatalf("want 1688_login_verify, got %q", r.MatchedRule)
	}
}

func TestClassify_custom1688URL(t *testing.T) {
	in := Input{
		TaskType:     taskTypeCollect,
		Platform:     "custom",
		ErrorMessage: "verification_or_login_page_detected",
		Title:        "detail.1688.com · offer.html",
		RawSummary:   "source=custom https://detail.1688.com/offer/1.html",
	}
	r := Classify(in)
	if r.MatchedRule != "sub:1688_login_verify" {
		t.Fatalf("custom+1688 url should use 1688 hint, got %q", r.MatchedRule)
	}
}

func TestClassify_pinduoduoLoginRequired(t *testing.T) {
	in := Input{
		TaskType:     taskTypeCollect,
		Platform:     "pinduoduo",
		ErrorCode:    "LOGIN_REQUIRED",
		ErrorMessage: "login_required",
		Title:        "pifa.pinduoduo.com · detail",
		RawSummary:   "source=pinduoduo https://pifa.pinduoduo.com/goods/detail/?gid=123",
	}
	r := Classify(in)
	if r.Category != CategoryLoginRequired {
		t.Fatalf("want category login_required, got %q matched=%q", r.Category, r.MatchedRule)
	}
	if r.MatchedRule != "sub:login_required_collect" {
		t.Fatalf("want sub:login_required_collect, got %q", r.MatchedRule)
	}
	if !strings.Contains(r.SuggestedAction, "拼多多") {
		t.Fatalf("want pinduoduo-specific suggest, got %q", r.SuggestedAction)
	}
	if strings.Contains(r.SuggestedAction, "\u666e\u901a\u5546\u54c1") || strings.Contains(r.SuggestedAction, "\u516c\u5f00\u5546\u54c1\u8be6\u60c5\u9875") {
		t.Fatalf("login recovery must not recommend an unsupported consumer page, got %q", r.SuggestedAction)
	}
	if !strings.Contains(r.SuggestedAction, "\u6279\u53d1") {
		t.Fatalf("login recovery should keep the supported wholesale scope, got %q", r.SuggestedAction)
	}

}
