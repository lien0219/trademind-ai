package opslabels

import "testing"

func TestLocalizeCollectWarning(t *testing.T) {
	title, msg := LocalizeCollectWarning("DETAIL_IMAGES_INCOMPLETE")
	if title != "详情图不完整" {
		t.Fatalf("title=%q", title)
	}
	if msg == "" || msg == "DETAIL_IMAGES_INCOMPLETE" {
		t.Fatalf("msg=%q", msg)
	}
}

func TestLocalizeReadinessIssueUnknownCode(t *testing.T) {
	iss := LocalizeReadinessIssue("UNKNOWN_TEST_CODE", "error", "UNKNOWN_TEST_CODE", "", "", "", "")
	if iss.Title == "UNKNOWN_TEST_CODE" {
		t.Fatalf("expected fallback title, got raw code")
	}
	if iss.TechnicalDetails["rawCode"] != "UNKNOWN_TEST_CODE" {
		t.Fatalf("missing rawCode in technical details")
	}
}

func TestStatusLabelReady(t *testing.T) {
	if StatusLabel("ready") != "已准备好" {
		t.Fatalf("ready label mismatch")
	}
	if StatusLabel("warning") != "建议检查" {
		t.Fatalf("warning label mismatch")
	}
}

func TestPublishCapabilityLabel(t *testing.T) {
	if PublishCapabilityLabel("local_draft_only") != "仅生成本地草稿" {
		t.Fatalf("capability label mismatch")
	}
}
