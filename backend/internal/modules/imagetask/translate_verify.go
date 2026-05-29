package imagetask

import (
	"context"
	"net/http"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

func (s *Service) verifyTranslateOutput(
	ctx context.Context,
	sourceBytes, resultBytes []byte,
	ocr *translateOCRResult,
	targetLang, sourceLang string,
	enabled bool,
) (translateVerificationMeta, error) {
	meta := translateVerificationMeta{
		ImageChanged:       !imagerender.ImagesEqual(sourceBytes, resultBytes),
		TargetTextDetected: false,
		Confidence:         0,
	}
	if !enabled {
		meta.ImageChanged = len(sourceBytes) > 0 && len(resultBytes) > 0 && !imagerender.ImagesEqual(sourceBytes, resultBytes)
		meta.TargetTextDetected = true
		meta.Confidence = 0.75
		return meta, nil
	}
	if imagerender.ImagesEqual(sourceBytes, resultBytes) {
		return meta, newTranslateErr(errCodeImageNotChanged, "生成图片没有变化，请重新生成或切换处理方式")
	}
	meta.ImageChanged = true

	targetKeywords := collectTargetKeywords(ocr, targetLang)
	sourceKeywords := collectSourceKeywords(ocr)
	resultText := s.extractResultText(ctx, resultBytes)

	if resultText == "" {
		if meta.ImageChanged && countTranslatedBlocks(ocr) > 0 {
			meta.TargetTextDetected = true
			meta.Confidence = 0.55
			meta.OutputTextVerifySkipped = true
			return meta, nil
		}
		meta.OutputTextVerifyFailed = true
		return meta, newTranslateErr(errCodeImageTextNotApplied, "翻译文字没有成功写入图片，请重新生成")
	}

	targetHits := countKeywordHits(resultText, targetKeywords)
	sourceHits := countKeywordHits(resultText, sourceKeywords)
	targetIsCJK := isCJKLang(targetLang)
	sourceIsCJK := isCJKLang(sourceLang)
	if strings.EqualFold(sourceLang, "auto") {
		sourceIsCJK = true
	}

	targetScriptCount := countRunesByScript(resultText, targetIsCJK)
	sourceScriptCount := countRunesByScript(resultText, sourceIsCJK)

	meta.TargetTextDetected = targetHits > 0 ||
		(!targetIsCJK && targetScriptCount >= 2) ||
		(targetIsCJK && targetScriptCount >= 1)
	if targetHits > 0 {
		meta.Confidence = float64(targetHits) / float64(maxInt(1, len(targetKeywords)))
		if meta.Confidence > 1 {
			meta.Confidence = 1
		}
	} else if meta.TargetTextDetected {
		meta.Confidence = 0.72
	}

	if !meta.TargetTextDetected {
		return meta, newTranslateErr(errCodeImageTextNotApplied, "翻译文字没有成功写入图片，请重新生成")
	}

	if sourceHits > 0 && targetHits == 0 {
		return meta, newTranslateErr(errCodeImageTextNotApplied, "翻译文字没有成功写入图片，请重新生成")
	}
	if sourceHits > 0 || (sourceIsCJK && sourceScriptCount >= 2 && strings.EqualFold(targetLang, "en")) {
		meta.SourceTextMayRemain = true
		meta.Confidence *= 0.85
	}
	if meta.Confidence <= 0 {
		meta.Confidence = 0.6
	}
	return meta, nil
}

func (s *Service) extractResultText(ctx context.Context, resultBytes []byte) string {
	if len(resultBytes) == 0 {
		return ""
	}
	payload := payloadFromImageBytes(resultBytes, http.DetectContentType(resultBytes), "")
	ocr, err := s.runOCROnImage(ctx, payload.DataURL, "auto", "en")
	if err != nil || ocr == nil {
		return ""
	}
	var parts []string
	for _, b := range ocr.Blocks {
		if t := strings.TrimSpace(b.TranslatedText); t != "" {
			parts = append(parts, t)
		}
		if t := strings.TrimSpace(b.Text); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func countTranslatedBlocks(ocr *translateOCRResult) int {
	if ocr == nil {
		return 0
	}
	n := 0
	for _, b := range ocr.Blocks {
		if strings.TrimSpace(b.TranslatedText) != "" {
			n++
		}
	}
	return n
}

func collectTargetKeywords(ocr *translateOCRResult, targetLang string) []string {
	if ocr == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[strings.ToLower(s)] {
			return
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	for _, b := range ocr.Blocks {
		add(b.TranslatedText)
		add(b.ShortTranslatedText)
		for _, w := range strings.Fields(b.TranslatedText) {
			if len(w) >= 3 {
				add(w)
			}
		}
	}
	_ = targetLang
	return out
}

func collectSourceKeywords(ocr *translateOCRResult) []string {
	if ocr == nil {
		return nil
	}
	var out []string
	for _, b := range ocr.Blocks {
		if t := strings.TrimSpace(b.Text); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func countKeywordHits(text string, keywords []string) int {
	lower := strings.ToLower(text)
	hits := 0
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(kw)) {
			hits++
		}
	}
	return hits
}

func countAllSourceText(ocr *translateOCRResult) int {
	if ocr == nil {
		return 0
	}
	n := 0
	for _, b := range ocr.Blocks {
		n += len([]rune(strings.TrimSpace(b.Text)))
	}
	return maxInt(1, n)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
