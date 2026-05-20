package failureclassifier

import "testing"

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

func TestClassify_custom1688Url(t *testing.T) {
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
