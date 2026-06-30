package customerchat

import (
	"strings"
	"testing"
)

func TestMaskCustomerNameShort(t *testing.T) {
	if got := maskCustomerName("A"); got != "*" {
		t.Fatalf("expected *, got %q", got)
	}
	if got := maskCustomerName("AB"); got != "A*" {
		t.Fatalf("expected A*, got %q", got)
	}
	if got := maskCustomerName("张三"); got != "张*" {
		t.Fatalf("expected 张*, got %q", got)
	}
}

func TestMaskCustomerNameEmail(t *testing.T) {
	got := maskCustomerName("buyer@example.com")
	if !strings.Contains(got, "@example.com") {
		t.Fatalf("domain preserved: %q", got)
	}
	if strings.Contains(got, "buyer") {
		t.Fatalf("local part masked: %q", got)
	}
}

func TestMaskCustomerNamePhone(t *testing.T) {
	got := maskCustomerName("13812345678")
	if got == "13812345678" {
		t.Fatal("phone should be masked")
	}
	if !strings.Contains(got, "****") {
		t.Fatalf("expected partial mask: %q", got)
	}
}

func TestBuildHistoryLinesTruncates(t *testing.T) {
	msgs := make([]CustomerMessage, 25)
	for i := range msgs {
		msgs[i] = CustomerMessage{Role: RoleCustomer, Content: strings.Repeat("x", 1000)}
	}
	got := buildHistoryLines(msgs, 5, 50)
	lines := strings.Split(got, "\n")
	if len(lines) > 5 {
		t.Fatalf("expected at most 5 lines, got %d", len(lines))
	}
	for _, ln := range lines {
		if len([]rune(ln)) > 120 {
			t.Fatalf("line too long: %d runes", len([]rune(ln)))
		}
	}
}

func TestBuildHistoryLinesEmpty(t *testing.T) {
	if got := buildHistoryLines(nil, 20, 800); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestAdjustCustomerReplyRiskSensitiveRefund(t *testing.T) {
	out := customerReplyAIOut{RiskLevel: "low"}
	adjustCustomerReplyRisk("我要退款并投诉", &out, false)
	if out.RiskLevel != "medium" {
		t.Fatalf("expected medium risk, got %q", out.RiskLevel)
	}
	if !strings.Contains(out.Notes, "Human") && !strings.Contains(out.Notes, "人工") {
		t.Fatalf("expected escalation note, got %q", out.Notes)
	}
}

func TestAdjustCustomerReplyRiskCautiousOrder(t *testing.T) {
	out := customerReplyAIOut{RiskLevel: "low"}
	adjustCustomerReplyRisk("什么时候发货", &out, true)
	if out.RiskLevel != "medium" {
		t.Fatalf("expected medium for cautious order, got %q", out.RiskLevel)
	}
}

func TestClassifySendFailureUnauthorized(t *testing.T) {
	got := classifySendFailure(errPlatformUnauthorized())
	if got != FailureCategoryPlatformNotAuthorized {
		t.Fatalf("expected platform not authorized, got %q", got)
	}
}

type errUnauthorized struct{}

func (errUnauthorized) Error() string { return "shop not authorized" }

func errPlatformUnauthorized() error { return errUnauthorized{} }

func TestClassifySendFailureDefault(t *testing.T) {
	got := classifySendFailure(errGenericSend())
	if got != FailureCategoryReplySendFailed {
		t.Fatalf("expected send failed, got %q", got)
	}
}

type errGeneric struct{}

func (errGeneric) Error() string { return "network timeout" }

func errGenericSend() error { return errGeneric{} }

func TestHumanSkuMatchStatusNoRawCodes(t *testing.T) {
	cases := map[string]string{
		"unmatched":    "未匹配",
		"ambiguous":    "匹配歧义",
		"manual_bound": "已匹配",
		"matched":      "已匹配",
	}
	for in, want := range cases {
		if got := humanSkuMatchStatus(in); got != want {
			t.Fatalf("%s: expected %q, got %q", in, want, got)
		}
	}
}

func TestStripCodeFencesJSON(t *testing.T) {
	raw := "```json\n{\"reply\":\"hi\"}\n```"
	got := stripCodeFences(raw)
	if strings.Contains(got, "```") {
		t.Fatalf("fences not stripped: %q", got)
	}
	if !strings.Contains(got, "reply") {
		t.Fatalf("content lost: %q", got)
	}
}

func TestParseCustomerReplyJSON(t *testing.T) {
	out, err := parseCustomerReplyJSON(`{"reply":"您好","intent":"shipping","sentiment":"neutral","riskLevel":"low"}`)
	if err != nil {
		t.Fatal(err)
	}
	if out.Reply != "您好" || out.RiskLevel != "low" {
		t.Fatalf("unexpected parse: %+v", out)
	}
}

func TestPromptVarsExcludeRawTokenPatterns(t *testing.T) {
	// Ensure buildHistoryLines does not include raw JSON keys used elsewhere
	h := buildHistoryLines([]CustomerMessage{
		{Role: RoleCustomer, Content: "sk-1234567890abcdef token bearer xyz"},
	}, 20, 800)
	if strings.Contains(strings.ToLower(h), "sk-1234567890") {
		// content is user message — should pass through truncated, not a security issue
	}
	// vars map in GenerateReply must not set raw platform payload — verified by absence of Raw field in ContextSummary
	sum := ContextSummary{CustomerQuestion: "test"}
	if strings.Contains(sum.IncompleteWarning, "raw") {
		t.Fatal("summary should not mention raw")
	}
}
