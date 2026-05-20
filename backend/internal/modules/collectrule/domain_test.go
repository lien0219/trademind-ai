package collectrule

import "testing"

func TestNormalizeRuleDomain(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"1688.com", "1688.com"},
		{"https://www.1688.com/", "www.1688.com"},
		{"HTTP://DETAIL.1688.COM/path", "detail.1688.com"},
		{"www.1688.com:443", "www.1688.com"},
	}
	for _, tc := range tests {
		if got := NormalizeRuleDomain(tc.in); got != tc.want {
			t.Fatalf("NormalizeRuleDomain(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRuleMatchesURL_1688Detail(t *testing.T) {
	s := &Service{}
	rule := &CollectRule{
		Domain: "https://www.1688.com/",
	}
	url := "https://detail.1688.com/offer/817299221966.html"
	if s.ruleMatchesURL(rule, url) {
		t.Fatal("www.1688.com rule should not match detail.1688.com url")
	}
	rule.Domain = "1688.com"
	if !s.ruleMatchesURL(rule, url) {
		t.Fatal("1688.com rule should match detail.1688.com url")
	}
}

func TestDomainMatches(t *testing.T) {
	if !domainMatches("detail.1688.com", "1688.com") {
		t.Fatal("expected suffix match")
	}
	if domainMatches("detail.1688.com", "www.1688.com") {
		t.Fatal("sibling subdomains should not match")
	}
}
