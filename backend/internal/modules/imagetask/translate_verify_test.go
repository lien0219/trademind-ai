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

func TestImagerenderChangesBytes(t *testing.T) {
	a := []byte("abc")
	b := []byte("abd")
	if imagerender.ImagesEqual(a, b) {
		t.Fatal("expected different hashes")
	}
}
