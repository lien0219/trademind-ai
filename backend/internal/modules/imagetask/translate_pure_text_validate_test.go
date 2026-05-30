package imagetask

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func TestValidatePureTextReplaceHardFailures(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	layout := translateLayoutSummary{}
	rq := translateRenderQuality{
		TextAppliedScore:         90,
		SourceTextRemovedScore:   88,
		LayoutScore:              85,
		StyleConsistencyScore:    84,
		ReadabilityScore:         86,
		ProductPreservationScore: 90,
		CommercialUsabilityScore: 87,
	}
	blocks := []translateRenderBlock{
		{
			ID: "b1", BlockClass: blockClassTitle, GroupType: groupTypeMainTitle,
			Lines: []string{"Snow White"}, FontSize: 28,
			BBox:         translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			OriginalBBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			Style:        translateTextStyle{Color: "#ffffff"},
		},
		{
			ID: "b2", BlockClass: blockClassBadge, GroupType: groupTypeBadge,
			Lines: []string{"Universal Stand"}, FontSize: 16,
			BBox:         translateTextBBox{X: 550, Y: 180, Width: 120, Height: 48},
			OriginalBBox: translateTextBBox{X: 550, Y: 180, Width: 120, Height: 48},
			EraseBBox:    translateTextBBox{X: 550, Y: 180, Width: 120, Height: 48},
			Style:        translateTextStyle{Color: "#111111", FontWeight: "bold", Align: "center"},
		},
	}
	got := validatePureTextReplace(verify, layout, rq, blocks, nil, 900, 900)
	if !got.HardPassed {
		t.Fatalf("expected hard pass, got failures=%v", got.HardFailures)
	}
	if got.ValidationMode != ValidationModePureTextReplace {
		t.Fatalf("validationMode = %q", got.ValidationMode)
	}
	if resolvePureTextFinalStatus(got) != StatusSuccess {
		t.Fatalf("status = %q, want success", resolvePureTextFinalStatus(got))
	}
}

func TestValidatePureTextReplaceSourceRemainFails(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true, SourceTextMayRemain: true}
	layout := translateLayoutSummary{}
	rq := translateRenderQuality{CommercialUsabilityScore: 90}
	got := validatePureTextReplace(verify, layout, rq, nil, nil, 900, 900)
	if got.HardPassed {
		t.Fatal("source remain should hard fail")
	}
	if resolvePureTextFinalStatus(got) != StatusFailedValidation {
		t.Fatalf("status = %q, want failed_render_validation", resolvePureTextFinalStatus(got))
	}
}

func TestValidatePureTextReplaceLowStyleSuccessWithReview(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	layout := translateLayoutSummary{}
	rq := translateRenderQuality{
		TextAppliedScore:         78,
		SourceTextRemovedScore:   82,
		LayoutScore:              68,
		StyleConsistencyScore:    62,
		ReadabilityScore:         70,
		ProductPreservationScore: 88,
	}
	blocks := []translateRenderBlock{
		{
			ID: "b1", BlockClass: blockClassTitle, Lines: []string{"Cool Black"}, FontSize: 26,
			BBox:         translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			OriginalBBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			Style:        titleGroupStyle(),
		},
		{
			ID: "b2", BlockClass: blockClassBadge, Lines: []string{"Universal Phone Stand"}, FontSize: 14,
			BBox:         translateTextBBox{X: 550, Y: 180, Width: 330, Height: 48},
			OriginalBBox: translateTextBBox{X: 550, Y: 180, Width: 330, Height: 48},
			EraseBBox:    translateTextBBox{X: 550, Y: 180, Width: 330, Height: 48},
			Style:        translateTextStyle{Color: "#111111", FontWeight: "bold", Align: "center"},
		},
	}
	got := validatePureTextReplace(verify, layout, rq, blocks, nil, 900, 900)
	if !got.HardPassed {
		t.Fatalf("expected hard pass with soft style issues, failures=%v", got.HardFailures)
	}
	status := resolvePureTextFinalStatus(got)
	if status != StatusSuccessWithReview {
		t.Fatalf("status = %q, want success_with_review (score=%d)", status, got.OverallScore)
	}
}

func TestValidatePureTextReplaceExtraBackgroundFails(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	layout := translateLayoutSummary{}
	rq := translateRenderQuality{CommercialUsabilityScore: 80}
	blocks := []translateRenderBlock{
		{
			ID: "b1", Lines: []string{"Snow White"}, FontSize: 20,
			BBox:  translateTextBBox{X: 10, Y: 10, Width: 100, Height: 30},
			Style: translateTextStyle{BackgroundColor: "#ffffff", BorderRadius: 12},
		},
	}
	got := validatePureTextReplace(verify, layout, rq, blocks, nil, 900, 900)
	if got.HardPassed {
		t.Fatal("extra background should hard fail")
	}
	for _, f := range got.HardFailures {
		if f == hardFailExtraBackground {
			return
		}
	}
	t.Fatalf("expected extra background hard fail, got %v", got.HardFailures)
}

func TestDetectSourceKeywordsNearOriginalBoxes(t *testing.T) {
	orig := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "b1", Text: "雪花白", BBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70}},
		},
	}
	post := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "p1", Text: "雪花", BBox: translateTextBBox{X: 620, Y: 85, Width: 60, Height: 40}, Confidence: 0.9},
		},
	}
	if !detectSourceKeywordsNearOriginalBoxes(post, orig) {
		t.Fatal("expected keyword near original box")
	}
	post2 := &translateOCRResult{
		Blocks: []translateTextBlock{
			{ID: "p2", Text: "Snow White", BBox: translateTextBBox{X: 620, Y: 85, Width: 120, Height: 40}, Confidence: 0.9},
		},
	}
	if detectSourceKeywordsNearOriginalBoxes(post2, orig) {
		t.Fatal("english replacement should not trigger source remain")
	}
}

func TestResolveTranslateFinalStatusForPureTextMode(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	layout := translateLayoutSummary{}
	rq := translateRenderQuality{
		TextAppliedScore: 70, SourceTextRemovedScore: 70, LayoutScore: 65,
		StyleConsistencyScore: 62, ReadabilityScore: 68, ProductPreservationScore: 80,
		CommercialUsabilityScore: 69,
	}
	blocks := []translateRenderBlock{
		{ID: "b1", BlockClass: blockClassTitle, Lines: []string{"Cool Black"}, FontSize: 24,
			BBox:         translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			OriginalBBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
		},
	}
	status := resolveTranslateFinalStatusForMode(
		rq.CommercialUsabilityScore, verify, layout, rq, false, RenderModePureTextReplace, blocks, nil, 900, 900,
	)
	if status != StatusSuccessWithReview {
		t.Fatalf("pure text low style = %q, want success_with_review", status)
	}
}

func TestBuildPureTextQualityRetryPlanSourceRemain(t *testing.T) {
	verify := translateVerificationMeta{SourceTextMayRemain: true}
	validation := pureTextValidationResult{SourceTextRemainDetected: true, HardPassed: false}
	plan := buildPureTextQualityRetryPlan(verify, translateLayoutSummary{}, translateRenderQuality{}, validation)
	if plan.MaskDilateExtra < 1 || !plan.SecondaryInpaint || !plan.ForcePillErase {
		t.Fatalf("unexpected retry plan: %+v", plan)
	}
}

func TestBuildPureTextQualityRetryPlanExtraBackground(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	validation := pureTextValidationResult{
		TargetTextDetected:           true,
		ExtraBackgroundLayerDetected: true,
		HardPassed:                   false,
	}
	plan := buildPureTextQualityRetryPlan(verify, translateLayoutSummary{}, translateRenderQuality{}, validation)
	if !plan.ForcePureTextReplace {
		t.Fatalf("expected force pure text replace, got %+v", plan)
	}
}

func TestPureTextRenderQualityUsesIndependentScoring(t *testing.T) {
	verify := translateVerificationMeta{TargetTextDetected: true}
	layout := translateLayoutSummary{LayoutTemplate: layoutTemplateTitleBadge}
	blocks := []translateRenderBlock{
		{ID: "b1", BlockClass: blockClassTitle, Lines: []string{"Cool Black"}, FontSize: 24,
			BBox:         translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			OriginalBBox: translateTextBBox{X: 610, Y: 80, Width: 180, Height: 70},
			Style:        titleGroupStyle(),
		},
	}
	rq := buildPureTextRenderQuality(
		translateQualitySummary{}, layout, verify,
		translateRenderOptions{RenderMode: RenderModePureTextReplace},
		blocks, &imagerender.Result{PatchAreaRatio: 0.01}, 900, 900,
	)
	if rq.CommercialUsabilityScore < 60 {
		t.Fatalf("score too low for clean pure text render: %d", rq.CommercialUsabilityScore)
	}
	if !rq.Passed && rq.CommercialUsabilityScore >= 75 {
		t.Fatalf("score %d should pass when hard checks pass", rq.CommercialUsabilityScore)
	}
}
