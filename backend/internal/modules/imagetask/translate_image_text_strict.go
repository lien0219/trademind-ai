package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/aimodelparse"
)

const layoutWarningOCRFiltered = "ocr_hallucination_filtered"

func ocrStrictRules() string {
	return `
STRICT RULES — literal OCR only:
- Read ONLY text that is literally visible in the image pixels.
- Do NOT invent, guess, or add typical ecommerce marketing text (prices, flash sale, discounts, coupons, "限时抢购", "原价", "现价", etc.) unless those exact words are clearly visible.
- Do NOT translate product features that are not written as text overlays.
- If text is unclear or not visible, omit it — do not fabricate.
- Each block must correspond to one visible text region on THIS image.
- translatedText must be a faithful translation of the visible text only, not marketing rewrite.`
}

func ocrPromptBase(sourceLang, targetLang string, imageW, imageH int) string {
	srcHint := "auto-detect the source language"
	if sourceLang != "" && !strings.EqualFold(sourceLang, "auto") {
		srcHint = fmt.Sprintf("assume source language is %s", langDisplayName(sourceLang))
	}
	targetName := langDisplayName(targetLang)
	sizeHint := ""
	if imageW > 0 && imageH > 0 {
		sizeHint = fmt.Sprintf("Image size: %d x %d pixels.\n", imageW, imageH)
	}
	return fmt.Sprintf(`You are a strict literal OCR engine for ecommerce product images.
%s%s
%s
Detect each DISTINCT visible text overlay (title lines, small labels, badges with readable text).
For each block return: original text exactly as shown, faithful translated text in %s, confidence (0-1), bounding box (x, y, width, height).
Each block must have a distinct y coordinate matching its on-image vertical position (do not return y=0 for every block).

Return ONLY valid JSON:
{
  "detectedLanguage": "zh|en|...",
  "textBlocksCount": number,
  "blocks": [
    {
      "text": "original",
      "translatedText": "translated",
      "confidence": 0.92,
      "bbox": {"x": 0, "y": 0, "width": 100, "height": 40}
    }
  ]
}
If no readable overlay text exists, return {"detectedLanguage":"","textBlocksCount":0,"blocks":[]}.`, sizeHint, srcHint, ocrStrictRules(), targetName)
}

func translateDefaultNegativePrompt() string {
	return strings.Join([]string{
		"new text", "extra text", "added text", "flash sale", "discount", "price tag",
		"promotional banner", "watermark", "sticker", "invented marketing copy",
		"限时抢购", "原价", "现价", "coupon", "new badge", "sale tag",
		"generated poster", "redesigned layout", "additional labels",
	}, ", ")
}

func isLikelyHallucinatedMarketingText(text string, confidence float64) bool {
	if confidence >= 0.93 {
		return false
	}
	t := strings.TrimSpace(text)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	markers := []string{
		"限时抢购", "flash sale", "原价", "现价", "秒杀", "促销", "特惠", "立减",
		"was:", "now:", "coupon", "discount", "off sale",
	}
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return true
		}
	}
	if strings.Contains(t, "¥") || strings.Contains(t, "$") || strings.Contains(t, "€") {
		// Price-like text needs high confidence
		if confidence < 0.9 {
			return true
		}
	}
	return false
}

func (s *Service) verifyOCRBlocksLiteral(ctx context.Context, imageRef string, blocks []translateTextBlock) ([]translateTextBlock, int) {
	if s == nil || len(blocks) == 0 || strings.TrimSpace(imageRef) == "" {
		return blocks, 0
	}
	var lines []string
	for i, b := range blocks {
		lines = append(lines, fmt.Sprintf(`%d. %q`, i+1, strings.TrimSpace(b.Text)))
	}
	prompt := fmt.Sprintf(`Look at this product image. For each candidate text, decide if that EXACT text (or nearly identical) is literally visible as written text in the image.
Do NOT assume typical ecommerce overlays exist. Mark visible:false if the text is guessed, inferred, or common marketing copy not shown in the image.

Candidates:
%s

Return ONLY JSON:
{"verified":[{"index":1,"text":"original","visible":true,"confidence":0.95}]}
Use visible:true ONLY when the text is clearly readable in the image.`, strings.Join(lines, "\n"))

	content, err := s.chatVisionJSON(ctx, prompt, imageRef, 1200)
	if err != nil {
		return filterOCRBlocksHeuristic(blocks)
	}
	normalized := aimodelparse.NormalizeJSONContent(content)
	var parsed struct {
		Verified []struct {
			Index      int     `json:"index"`
			Text       string  `json:"text"`
			Visible    bool    `json:"visible"`
			Confidence float64 `json:"confidence"`
		} `json:"verified"`
	}
	if err := json.Unmarshal([]byte(normalized), &parsed); err != nil || len(parsed.Verified) == 0 {
		return filterOCRBlocksHeuristic(blocks)
	}

	visibleByIndex := map[int]bool{}
	visibleByText := map[string]bool{}
	for _, v := range parsed.Verified {
		if v.Visible && v.Confidence >= 0.5 {
			if v.Index > 0 {
				visibleByIndex[v.Index] = true
			}
			if t := strings.TrimSpace(v.Text); t != "" {
				visibleByText[strings.ToLower(t)] = true
			}
		}
	}

	out := make([]translateTextBlock, 0, len(blocks))
	filtered := 0
	for i, b := range blocks {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			filtered++
			continue
		}
		ok := visibleByIndex[i+1] || visibleByText[strings.ToLower(text)]
		if !ok {
			filtered++
			continue
		}
		if isLikelyHallucinatedMarketingText(text, b.Confidence) {
			filtered++
			continue
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return filterOCRBlocksHeuristic(blocks)
	}
	return out, filtered
}

func filterOCRBlocksHeuristic(blocks []translateTextBlock) ([]translateTextBlock, int) {
	out := make([]translateTextBlock, 0, len(blocks))
	filtered := 0
	for _, b := range blocks {
		text := strings.TrimSpace(b.Text)
		if text == "" || isLikelyHallucinatedMarketingText(text, b.Confidence) {
			filtered++
			continue
		}
		out = append(out, b)
	}
	return out, filtered
}

func (s *Service) filterAndVerifyOCR(ctx context.Context, ocr *translateOCRResult, imageRef string) *translateOCRResult {
	if ocr == nil {
		return ocr
	}
	originalCount := len(ocr.Blocks)
	pre, preFiltered := filterOCRBlocksHeuristic(ocr.Blocks)
	blocks, vFiltered := s.verifyOCRBlocksLiteral(ctx, imageRef, pre)
	if len(blocks) == 0 && len(pre) > 0 {
		blocks = pre
	} else if len(blocks) == 0 {
		blocks, preFiltered = filterOCRBlocksHeuristic(ocr.Blocks)
	}
	ocr.Blocks = blocks
	ocr.TextBlocksCount = len(blocks)
	totalFiltered := originalCount - len(blocks)
	if totalFiltered < 0 {
		totalFiltered = preFiltered + vFiltered
	}
	if totalFiltered > 0 {
		ocr.FilteredBlocksCount = totalFiltered
	}
	return ocr
}
