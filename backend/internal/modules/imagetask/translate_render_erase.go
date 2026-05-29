package imagetask

import (
	"context"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

const (
	warningBadgeShapeAbnormal = "badge_shape_abnormal"
	warningTextOverlap        = "text_overlap"
	warningEraseFailed        = "erase_failed"
)

// countSourceBlocksStillPresent counts how many original OCR text blocks still appear after erase.
func countSourceBlocksStillPresent(postOCR *translateOCRResult, original *translateOCRResult) int {
	if postOCR == nil || original == nil {
		return 0
	}
	still := 0
	for _, orig := range original.Blocks {
		origText := strings.TrimSpace(orig.Text)
		if len([]rune(origText)) < 2 {
			continue
		}
		for _, b := range postOCR.Blocks {
			if b.Confidence > 0 && b.Confidence < 0.55 {
				continue
			}
			detected := strings.TrimSpace(b.Text)
			if detected == "" {
				continue
			}
			if strings.EqualFold(detected, origText) ||
				strings.Contains(detected, origText) ||
				strings.Contains(origText, detected) {
				still++
				break
			}
		}
	}
	return still
}

func sourceEraseRemainThreshold(original *translateOCRResult) int {
	if original == nil || len(original.Blocks) == 0 {
		return 2
	}
	n := 0
	for _, b := range original.Blocks {
		if len([]rune(strings.TrimSpace(b.Text))) >= 2 {
			n++
		}
	}
	if n <= 1 {
		return 1
	}
	return (n + 1) / 2
}

func (s *Service) checkSourceTextAfterErase(
	ctx context.Context,
	erasedBytes []byte,
	ocr *translateOCRResult,
	sourceLang string,
) bool {
	if s == nil || len(erasedBytes) == 0 || ocr == nil {
		return false
	}
	payload := payloadFromImageBytes(erasedBytes, "", "")
	if payload == nil {
		return false
	}
	postOCR, err := s.runOCROnImage(ctx, payload.DataURL, sourceLang, "en", nil)
	if err != nil || postOCR == nil {
		return false
	}
	still := countSourceBlocksStillPresent(postOCR, ocr)
	return still >= sourceEraseRemainThreshold(ocr)
}

func strengthenEraseBlocks(blocks []imagerender.TextBlock) []imagerender.TextBlock {
	out := cloneImageRenderBlocks(blocks)
	for i := range out {
		out[i].ErasePadding = maxInt(out[i].ErasePadding+4, 8)
		if out[i].ErasePadding > 14 {
			out[i].ErasePadding = 14
		}
	}
	return out
}

func detectBadgeShapeAbnormal(blocks []translateRenderBlock) bool {
	return countAbnormalBadgeRenderBlocks(blocks) > 0
}

func countAbnormalBadgeRenderBlocks(blocks []translateRenderBlock) int {
	n := 0
	for _, b := range blocks {
		if b.GroupType != groupTypeBadge && b.GroupType != groupTypeBottomBadge {
			continue
		}
		if isDarkLabelStyle(b.Style) && badgeRenderExceedsHardLimit(b) {
			n++
			continue
		}
		w, h := b.BBox.Width, b.BBox.Height
		if w <= 0 || h <= 0 {
			continue
		}
		ratio := float64(w) / float64(h)
		area := w * h
		if area > 12000 && ratio > 0.75 && ratio < 1.35 {
			n++
			continue
		}
		if w > h*3 && h > 80 {
			n++
		}
	}
	return n
}

func badgeRenderExceedsHardLimit(b translateRenderBlock) bool {
	orig := b.OriginalBBox
	if orig.Width <= 0 || orig.Height <= 0 {
		orig = b.EraseBBox
	}
	if orig.Width <= 0 || orig.Height <= 0 {
		return false
	}
	return float64(b.BBox.Width) > float64(orig.Width)*1.35+1 ||
		float64(b.BBox.Height) > float64(orig.Height)*1.25+1
}

type translateRenderAttemptResult struct {
	Result            *imagerender.Result
	EraseSourceRemain bool
}

func renderTranslateWithEraseVerify(
	s *Service,
	ctx context.Context,
	sourceBytes []byte,
	blocks []imagerender.TextBlock,
	opts imagerender.Options,
	outputFormat string,
	ocr *translateOCRResult,
	sourceLang string,
	verifyErase bool,
) (*translateRenderAttemptResult, error) {
	img, _, err := imagerender.Decode(sourceBytes)
	if err != nil {
		return nil, err
	}
	rgba, stats, usedErase, err := imagerender.EraseRegions(img, blocks, opts)
	if err != nil {
		return nil, err
	}

	eraseSourceRemain := false
	if verifyErase && s != nil {
		preview, _, encErr := imagerender.Encode(rgba, outputFormat)
		if encErr == nil && s.checkSourceTextAfterErase(ctx, preview, ocr, sourceLang) {
			eraseSourceRemain = true
			strongOpts := opts
			strongOpts.EraseMode = imagerender.EraseOpenCVInpaint
			strongBlocks := strengthenEraseBlocks(blocks)
			if strongRGBA, strongStats, strongUsed, reErr := imagerender.EraseRegions(rgba, strongBlocks, strongOpts); reErr == nil {
				rgba = strongRGBA
				stats.ErasePixels += strongStats.ErasePixels
				stats.PatchPixels += strongStats.PatchPixels
				stats.BackgroundDeltaScore += strongStats.BackgroundDeltaScore
				stats.LargePatchDetected = stats.LargePatchDetected || strongStats.LargePatchDetected
				preview2, _, enc2 := imagerender.Encode(rgba, outputFormat)
				if enc2 == nil && !s.checkSourceTextAfterErase(ctx, preview2, ocr, sourceLang) {
					eraseSourceRemain = false
				}
				if strings.TrimSpace(strongUsed) != "" {
					usedErase = strongUsed
				} else {
					usedErase = imagerender.EraseOpenCVInpaint
				}
			}
		}
	}

	drawn, err := imagerender.DrawRegions(rgba, blocks, opts)
	if err != nil {
		return nil, err
	}
	data, ct, err := imagerender.Encode(rgba, outputFormat)
	if err != nil {
		return nil, err
	}
	b := rgba.Bounds()
	imageArea := maxInt(1, b.Dx()*b.Dy())
	return &translateRenderAttemptResult{
		Result: &imagerender.Result{
			Data:                 data,
			ContentType:          ct,
			EraseMode:            usedErase,
			BlocksDrawn:          drawn,
			EraseBlocks:          countEraseBlocksFromImage(blocks),
			SourceSHA256:         imagerender.SHA256Hex(sourceBytes),
			OutputSHA256:         imagerender.SHA256Hex(data),
			EraseAreaRatio:       float64(stats.ErasePixels) / float64(imageArea),
			PatchAreaRatio:       float64(stats.PatchPixels) / float64(imageArea),
			BackgroundDeltaScore: stats.BackgroundDeltaScore,
			FlatFillRatio:        stats.FlatFillRatio / float64(maxInt(1, stats.PatchPixels)),
			LargePatchDetected:   stats.LargePatchDetected || float64(stats.PatchPixels)/float64(imageArea) > 0.08,
		},
		EraseSourceRemain: eraseSourceRemain,
	}, nil
}

func countRenderBlocksByGroup(blocks []translateRenderBlock, types ...string) int {
	if len(blocks) == 0 || len(types) == 0 {
		return 0
	}
	allowed := map[string]bool{}
	for _, t := range types {
		allowed[t] = true
	}
	n := 0
	for _, b := range blocks {
		if !allowed[b.GroupType] {
			continue
		}
		if (b.GroupType == groupTypeBadge || b.GroupType == groupTypeBottomBadge) && !isDarkLabelStyle(b.Style) {
			continue
		}
		if allowed[b.GroupType] {
			n++
		}
	}
	return n
}

func countEraseBlocksFromImage(blocks []imagerender.TextBlock) int {
	n := 0
	for _, b := range blocks {
		eb := b.EraseBBox
		if eb.Width <= 0 || eb.Height <= 0 {
			eb = b.BBox
		}
		if eb.Width > 0 && eb.Height > 0 {
			n++
		}
	}
	return n
}

func countRenderBlocksWithEraseBBox(blocks []translateRenderBlock) int {
	n := 0
	for _, b := range blocks {
		if b.EraseBBox.Width > 0 && b.EraseBBox.Height > 0 {
			n++
		}
	}
	return n
}

func countRenderBlocksWithLayoutBBox(blocks []translateRenderBlock) int {
	n := 0
	for _, b := range blocks {
		if b.BBox.Width > 0 && b.BBox.Height > 0 {
			n++
		}
	}
	return n
}

func buildBlockClassificationSummary(ocr *translateOCRResult) []map[string]any {
	if ocr == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(ocr.Blocks))
	for _, b := range ocr.Blocks {
		out = append(out, map[string]any{
			"id":                   b.ID,
			"text":                 b.Text,
			"blockClass":           b.BlockClass,
			"standard_translation": b.StandardTranslation,
			"standardTranslation":  b.StandardTranslation,
			"compact_translation":  firstNonEmptyString(b.CompactTranslation, b.ShortTranslatedText),
			"compactTranslation":   firstNonEmptyString(b.CompactTranslation, b.ShortTranslatedText),
			"erase_bbox":           b.BBox,
		})
	}
	return out
}
