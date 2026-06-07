package imagetask

import (
	"testing"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func TestVerifyDetectsUnchangedImage(t *testing.T) {
	s := &Service{}
	src := []byte{1, 2, 3, 4}
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "金属底座", TranslatedText: "Metal Base"},
		},
	}
	_, err := s.verifyTranslateOutput(t.Context(), src, src, ocr, "en", "zh", true)
	if err == nil {
		t.Fatal("expected error for unchanged image")
	}
	if translateErrCode(err) != errCodeImageNotChanged {
		t.Fatalf("got code %q want %q", translateErrCode(err), errCodeImageNotChanged)
	}
}

func TestRuleBasedShortTextPhoneStand(t *testing.T) {
	cases := map[string]string{
		"金属底座":  "Metal Base",
		"折叠支架":  "Foldable Stand",
		"手机/平板": "Phone/Tablet",
		"暗夜黑":   "Midnight Black",
	}
	for orig, want := range cases {
		got := ruleBasedShortText(orig, orig, "en")
		if got != want {
			t.Fatalf("%q => %q want %q", orig, got, want)
		}
	}
}

func TestCollectTargetKeywordsIncludesRenderedTranslationVariants(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{
				Text:                  "炫酷黑",
				TranslatedText:        "Dark Black",
				StandardTranslation:   "Cool Black",
				CompactTranslation:    "Cool Black",
				BadgeTranslation:      "Cool Black",
				FixedShortTranslation: "Cool Black",
			},
			{
				Text:               "折叠伸缩版/通用手机",
				TranslatedText:     "Foldable Telescopic Version / Universal Phone",
				CompactTranslation: "Universal Stand",
				BadgeTranslation:   "Universal Stand",
			},
		},
	}
	keywords := collectTargetKeywords(ocr, "en")
	if countKeywordHits("Cool Black Universal Stand", keywords) < 2 {
		t.Fatalf("expected rendered variants in target keywords, got %v", keywords)
	}
}

func TestFinalSourceRemainThresholdRequiresMoreEvidenceWhenTargetHit(t *testing.T) {
	ocr := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "炫酷黑"},
			{Text: "折叠伸缩版/通用手机"},
		},
	}
	if got := finalSourceRemainThreshold(ocr, true, 2); got != 2 {
		t.Fatalf("threshold with target hit = %d, want 2", got)
	}
	if got := finalSourceRemainThreshold(ocr, false, 0); got != 1 {
		t.Fatalf("threshold without target hit = %d, want 1", got)
	}
}

func TestDetectSourceKeywordsNearOriginalBoxesIgnoresTinyFragments(t *testing.T) {
	orig := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "折叠伸缩版/通用手机", BBox: translateTextBBox{X: 600, Y: 180, Width: 300, Height: 60}},
		},
	}
	post := &translateOCRResult{
		Blocks: []translateTextBlock{
			{Text: "版", BBox: translateTextBBox{X: 620, Y: 190, Width: 20, Height: 20}, Confidence: 0.9},
		},
	}
	if detectSourceKeywordsNearOriginalBoxes(post, orig) {
		t.Fatal("single-character OCR fragment should not count as source residue")
	}
	post.Blocks[0].Text = "通用手机"
	if !detectSourceKeywordsNearOriginalBoxes(post, orig) {
		t.Fatal("known source phrase near original box should count as source residue")
	}
}

func TestImagerenderChangesBytes(t *testing.T) {
	a := []byte("abc")
	b := []byte("abd")
	if imagerender.ImagesEqual(a, b) {
		t.Fatal("expected different hashes")
	}
}
