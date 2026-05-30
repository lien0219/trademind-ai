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
	return s.verifyTranslateOutputWithLayout(ctx, sourceBytes, resultBytes, ocr, targetLang, sourceLang, enabled, translateLayoutSummary{})
}

func (s *Service) verifyTranslateOutputWithLayout(
	ctx context.Context,
	sourceBytes, resultBytes []byte,
	ocr *translateOCRResult,
	targetLang, sourceLang string,
	enabled bool,
	layout translateLayoutSummary,
) (translateVerificationMeta, error) {
	meta := translateVerificationMeta{
		ImageChanged:       !imagerender.ImagesEqual(sourceBytes, resultBytes),
		TargetTextDetected: false,
		Confidence:         0,
	}
	if layout.OverflowBlocks > 0 {
		meta.TranslatedTextOverflow = true
	}
	if layout.Simulation.CollisionCount > 0 {
		meta.TextOverlapDetected = true
	}
	if layout.Simulation.ProductOverlapCount > 0 {
		meta.ProductOverlapDetected = true
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

	targetScriptCount := countRunesByScript(resultText, targetIsCJK)

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
	stillBlocks := 0
	if resultText != "" {
		payload := payloadFromImageBytes(resultBytes, http.DetectContentType(resultBytes), "")
		if payload != nil {
			if postOCR, ocrErr := s.runOCROnImage(ctx, payload.DataURL, sourceLang, targetLang, nil); ocrErr == nil && postOCR != nil {
				stillBlocks = countSourceBlocksStillPresent(postOCR, ocr)
				if detectSourceKeywordsNearOriginalBoxes(postOCR, ocr) {
					meta.SourceTextRemainNearBox = true
				}
			}
		}
	}
	if stillBlocks >= sourceEraseRemainThreshold(ocr) {
		meta.SourceTextMayRemain = true
		meta.Confidence *= 0.85
	} else if meta.SourceTextRemainNearBox {
		meta.SourceTextMayRemain = true
		meta.Confidence *= 0.85
	} else if sourceHits > 0 && stillBlocks > 0 {
		meta.SourceTextMayRemain = true
		meta.Confidence *= 0.85
	}
	if meta.Confidence <= 0 {
		meta.Confidence = 0.6
	}
	if meta.SourceTextMayRemain || meta.TranslatedTextOverflow || meta.TextOverlapDetected || meta.ProductOverlapDetected {
		meta.CommercialUsabilityLow = true
	}
	return meta, nil
}

func (s *Service) extractResultText(ctx context.Context, resultBytes []byte) string {
	if len(resultBytes) == 0 {
		return ""
	}
	payload := payloadFromImageBytes(resultBytes, http.DetectContentType(resultBytes), "")
	ocr, err := s.runOCROnImage(ctx, payload.DataURL, "auto", "en", nil)
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

var knownSourceRemainKeywords = []string{
	"雪花白", "炫酷黑", "折叠", "通用手机", "伸缩版",
}

func detectSourceKeywordsNearOriginalBoxes(postOCR, original *translateOCRResult) bool {
	if postOCR == nil || original == nil {
		return false
	}
	keywords := append([]string(nil), knownSourceRemainKeywords...)
	for _, b := range original.Blocks {
		if t := strings.TrimSpace(b.Text); t != "" {
			keywords = append(keywords, t)
		}
	}
	for _, orig := range original.Blocks {
		origText := strings.TrimSpace(orig.Text)
		if len([]rune(origText)) < 2 {
			continue
		}
		expanded := expandBBoxForNearCheck(orig.BBox, 0.35)
		for _, det := range postOCR.Blocks {
			if det.Confidence > 0 && det.Confidence < 0.5 {
				continue
			}
			detected := strings.TrimSpace(det.Text)
			if detected == "" {
				continue
			}
			if bboxOverlapRatio(expanded, det.BBox) < 0.08 {
				continue
			}
			for _, kw := range keywords {
				kw = strings.TrimSpace(kw)
				if kw == "" {
					continue
				}
				if strings.Contains(detected, kw) || strings.Contains(kw, detected) {
					return true
				}
			}
			if strings.EqualFold(detected, origText) {
				return true
			}
		}
	}
	return false
}

func expandBBoxForNearCheck(bb translateTextBBox, padRatio float64) translateTextBBox {
	if bb.Width <= 0 || bb.Height <= 0 {
		return bb
	}
	padX := int(float64(bb.Width) * padRatio)
	padY := int(float64(bb.Height) * padRatio)
	return translateTextBBox{
		X:      bb.X - padX,
		Y:      bb.Y - padY,
		Width:  bb.Width + padX*2,
		Height: bb.Height + padY*2,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
