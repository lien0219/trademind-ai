package productcheck

import "testing"

func TestLocalizeReadinessResult(t *testing.T) {
	res := &CheckProductReadinessResult{
		Status: "warning",
		Result: "warning",
		Checks: []CheckItem{{
			Code:    "DETAIL_IMAGES_INCOMPLETE",
			Level:   "warning",
			Message: "DETAIL_IMAGES_INCOMPLETE",
		}},
	}
	out := LocalizeReadinessResult(res)
	if out.StatusLabel != "建议检查" {
		t.Fatalf("statusLabel=%q", out.StatusLabel)
	}
	if len(out.Checks) != 1 {
		t.Fatalf("checks len")
	}
	if out.Checks[0].Title != "详情图不完整" {
		t.Fatalf("title=%q", out.Checks[0].Title)
	}
	if out.Checks[0].TechnicalDetails["rawCode"] != "DETAIL_IMAGES_INCOMPLETE" {
		t.Fatalf("technical details missing rawCode")
	}
}
