package imagetask

import (
	"context"
	"image"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
)

const maxQualityScoreRetries = 1

type translateRenderPipelineResult struct {
	Result            *imagerender.Result
	VerifyMeta        translateVerificationMeta
	RenderQuality     translateRenderQuality
	RenderBlocks      []translateRenderBlock
	ImageBlocks       []imagerender.TextBlock
	DebugArtifacts    map[string]any
	QualityRetried    bool
	RetryStrategies   []string
	EraseSourceRemain bool
}

func (s *Service) runTranslateRenderPipeline(
	ctx context.Context,
	task *ImageTask,
	sourceBytes []byte,
	renderBlocks []translateRenderBlock,
	imageBlocks []imagerender.TextBlock,
	imOpts imagerender.Options,
	outputFormat string,
	ocr *translateOCRResult,
	sourceLang, targetLang string,
	renderOpts translateRenderOptions,
	quality translateQualitySummary,
	layoutSummary translateLayoutSummary,
) (*translateRenderPipelineResult, error) {
	img, _, err := imagerender.Decode(sourceBytes)
	if err != nil {
		return nil, err
	}

	attempt := func(blocks []imagerender.TextBlock, drawBlocks []translateRenderBlock, label string) (*translateRenderAttemptResult, translateVerificationMeta, translateRenderQuality, error) {
		attemptOpts := imOpts
		attemptOpts.PureTextReplace = isPureTextReplaceMode(renderOpts.RenderMode)
		attemptOut, aErr := renderTranslateWithEraseVerify(
			s, ctx, sourceBytes, blocks, attemptOpts, outputFormat,
			ocr, sourceLang, renderOpts.VerifyOutputText,
		)
		if aErr != nil {
			return nil, translateVerificationMeta{}, translateRenderQuality{}, aErr
		}
		if attemptOut != nil && attemptOut.Result != nil && label != "" {
			attemptOut.Result.RetryStrategies = append(attemptOut.Result.RetryStrategies, label)
		}
		verifyMeta, vErr := s.verifyTranslateOutputWithLayout(ctx, sourceBytes, attemptOut.Result.Data, ocr, targetLang, sourceLang, renderOpts.VerifyOutputText, layoutSummary)
		if vErr != nil {
			return attemptOut, verifyMeta, translateRenderQuality{}, vErr
		}
		rq := buildTranslateRenderQuality(quality, layoutSummary, verifyMeta, renderOpts, drawBlocks, attemptOut.Result)
		if attemptOpts.PureTextReplace && verifyMeta.SourceTextMayRemain && !attemptOpts.ForceTextBoundsCleanup {
			cleanupOpts := attemptOpts
			cleanupOpts.ForceTextBoundsCleanup = true
			cleanupOut, cleanupErr := renderTranslateWithEraseVerify(
				s, ctx, sourceBytes, blocks, cleanupOpts, outputFormat,
				ocr, sourceLang, false,
			)
			if cleanupErr == nil && cleanupOut != nil && cleanupOut.Result != nil {
				cleanupOut.Result.RetryStrategies = append(cleanupOut.Result.RetryStrategies, "localized_source_cleanup")
				cleanupVerify, cleanupVerifyErr := s.verifyTranslateOutputWithLayout(ctx, sourceBytes, cleanupOut.Result.Data, ocr, targetLang, sourceLang, renderOpts.VerifyOutputText, layoutSummary)
				if cleanupVerifyErr == nil {
					cleanupQuality := buildTranslateRenderQuality(quality, layoutSummary, cleanupVerify, renderOpts, drawBlocks, cleanupOut.Result)
					if cleanupQuality.CommercialUsabilityScore >= rq.CommercialUsabilityScore || !cleanupVerify.SourceTextMayRemain {
						return cleanupOut, cleanupVerify, cleanupQuality, nil
					}
				}
			}
		}
		return attemptOut, verifyMeta, rq, nil
	}

	currentImageBlocks := cloneImageRenderBlocks(imageBlocks)
	currentRenderBlocks := renderBlocks
	var best *translateRenderPipelineResult
	bestScore := -1

	runAndTrack := func(blocks []imagerender.TextBlock, drawBlocks []translateRenderBlock, label string) error {
		attemptOut, verifyMeta, rq, aErr := attempt(blocks, drawBlocks, label)
		if aErr != nil || attemptOut == nil || attemptOut.Result == nil {
			return aErr
		}
		score := rq.CommercialUsabilityScore
		if score > bestScore {
			bestScore = score
			best = &translateRenderPipelineResult{
				Result:            attemptOut.Result,
				VerifyMeta:        verifyMeta,
				RenderQuality:     rq,
				RenderBlocks:      drawBlocks,
				ImageBlocks:       blocks,
				RetryStrategies:   append([]string(nil), attemptOut.Result.RetryStrategies...),
				EraseSourceRemain: attemptOut.EraseSourceRemain,
			}
		}
		return nil
	}

	_ = runAndTrack(currentImageBlocks, currentRenderBlocks, "text_pixel_mask")

	if best != nil {
		imageW, imageH := imageDimensionsFromRenderBlocks(currentRenderBlocks)
		validation := validatePureTextReplace(
			best.VerifyMeta, layoutSummary, best.RenderQuality, currentRenderBlocks, best.Result, imageW, imageH,
		)
		if shouldQualityAutoRetry(
			best.RenderQuality.CommercialUsabilityScore, best.VerifyMeta, layoutSummary, best.RenderQuality,
			renderOpts.RenderMode, validation, best.QualityRetried,
		) {
			var plan translateQualityRetryPlan
			if isPureTextReplaceMode(renderOpts.RenderMode) {
				plan = buildPureTextQualityRetryPlan(best.VerifyMeta, layoutSummary, best.RenderQuality, validation)
			} else {
				plan = buildQualityRetryPlan(best.VerifyMeta, layoutSummary, best.RenderQuality)
			}
			retryImageBlocks := applyQualityRetryToImageBlocks(currentImageBlocks, plan)
			retryRenderBlocks := applyQualityRetryToRenderBlocks(currentRenderBlocks, ocr, plan)
			if plan.UseShorterText || plan.ReduceFontSize {
				retryImageBlocks = buildImageRenderBlocks(retryRenderBlocks)
			}
			label := "quality_auto_retry"
			if plan.DrawTextOnlyRetry {
				label = "draw_text_only_retry"
			}
			if plan.ForcePureTextReplace {
				label = "force_pure_text_replace"
			}
			_ = runAndTrack(retryImageBlocks, retryRenderBlocks, label)
			best.QualityRetried = true
		}
	}

	if best == nil {
		return nil, imagerender.ErrEraseMaskEmpty
	}

	// Debug artifacts from last best result path
	rgba, stats, usedErase, debugArt, dbgErr := imagerender.EraseRegionsWithDebug(img, best.ImageBlocks, imOpts)
	if dbgErr == nil {
		finalData := best.Result.Data
		var debugMap map[string]any
		if task != nil {
			debugMap = buildTranslateDebugOutput(ctx, s, task, debugArt.OriginalPNG, debugArt.MaskPNG, debugArt.ErasedPNG, finalData)
		}
		best.DebugArtifacts = debugMap
		if best.Result != nil {
			best.Result.EraseMode = usedErase
			best.Result.EraseAreaRatio = float64(stats.ErasePixels) / float64(maxInt(1, rgba.Bounds().Dx()*rgba.Bounds().Dy()))
		}
	}

	return best, nil
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

	currentBlocks := cloneImageRenderBlocks(blocks)
	eraseSourceRemain := false
	var rgba *image.RGBA
	var stats imagerender.EraseStats
	var usedErase string

	maxRetries := maxEraseResidueRetries
	if opts.PureTextReplace {
		maxRetries = maxPureTextEraseRetries
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		attemptBlocks := currentBlocks
		if attempt > 0 {
			attemptBlocks = expandMaskDilateBlocks(currentBlocks, 1)
		}
		attemptOpts := opts
		rgba, stats, usedErase, err = imagerender.EraseRegions(img, attemptBlocks, attemptOpts)
		if err != nil {
			return nil, err
		}
		if opts.PureTextReplace {
			b := rgba.Bounds()
			boundsCleanup := imagerender.ForceEraseSourceBlockBounds(rgba, attemptBlocks, maxInt(1, b.Dx()*b.Dy()))
			if boundsCleanup.ErasePixels > 0 {
				stats = mergeEraseStats(stats, boundsCleanup)
				if usedErase == "" {
					usedErase = "source_bounds_cleanup"
				} else if !strings.Contains(usedErase, "source_bounds_cleanup") {
					usedErase += "+source_bounds_cleanup"
				}
			}
		}
		if !verifyErase || s == nil || attempt >= maxRetries {
			break
		}
		preview, _, encErr := imagerender.Encode(rgba, outputFormat)
		if encErr != nil {
			break
		}
		if !s.checkSourceTextAfterErase(ctx, preview, ocr, sourceLang) {
			break
		}
		eraseSourceRemain = true
		currentBlocks = attemptBlocks
	}
	if opts.PureTextReplace && (eraseSourceRemain || opts.ForceTextBoundsCleanup) {
		b := rgba.Bounds()
		cleanup := imagerender.ForceEraseTextMaskBounds(rgba, currentBlocks, maxInt(1, b.Dx()*b.Dy()))
		if cleanup.ErasePixels > 0 {
			stats = mergeEraseStats(stats, cleanup)
			if usedErase == "" {
				usedErase = "localized_cleanup"
			} else if !strings.Contains(usedErase, "localized_cleanup") {
				usedErase += "+localized_cleanup"
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
			LargePatchDetected:   stats.LargePatchDetected || float64(stats.PatchPixels)/float64(imageArea) > imagerender.MaxEraseMaskRatioTotal,
		},
		EraseSourceRemain: eraseSourceRemain,
	}, nil
}

func mergeEraseStats(a, b imagerender.EraseStats) imagerender.EraseStats {
	out := a
	out.ErasePixels += b.ErasePixels
	out.PatchPixels += b.PatchPixels
	out.BackgroundDeltaScore += b.BackgroundDeltaScore
	if out.FlatFillRatio == 0 {
		out.FlatFillRatio = b.FlatFillRatio
	}
	out.LargePatchDetected = out.LargePatchDetected || b.LargePatchDetected
	return out
}

func secondaryInpaintOnMaskOnly(
	rgba *image.RGBA,
	blocks []imagerender.TextBlock,
	imageArea int,
) {
	for _, block := range blocks {
		class := strings.TrimSpace(strings.ToLower(block.BlockClass))
		if isCapsuleBlockClassForRender(class) {
			continue
		}
		_, _ = imagerender.SecondaryInpaintTextMask(rgba, block, imageArea, 2)
	}
}

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
			"id":                    b.ID,
			"text":                  b.Text,
			"blockClass":            b.BlockClass,
			"standard_translation":  b.StandardTranslation,
			"standardTranslation":   b.StandardTranslation,
			"compact_translation":   firstNonEmptyString(b.CompactTranslation, b.ShortTranslatedText),
			"compactTranslation":    firstNonEmptyString(b.CompactTranslation, b.ShortTranslatedText),
			"badge_translation":     b.BadgeTranslation,
			"badgeTranslation":      b.BadgeTranslation,
			"fixedShortTranslation": b.FixedShortTranslation,
			"erase_bbox":            sourceBBoxForBlock(b),
			"layout_bbox":           b.BBox,
		})
	}
	return out
}

const (
	warningBadgeShapeAbnormal = "badge_shape_abnormal"
	warningTextOverlap        = "text_overlap"
	warningEraseFailed        = "erase_failed"
	maxEraseResidueRetries    = 2
	maxPureTextEraseRetries   = 3
)

func expandMaskDilateBlocks(blocks []imagerender.TextBlock, extra int) []imagerender.TextBlock {
	out := cloneImageRenderBlocks(blocks)
	for i := range out {
		out[i].MaskDilate = maxInt(1, out[i].MaskDilate+extra)
		if out[i].MaskDilate > 2 {
			out[i].MaskDilate = 2
		}
	}
	return out
}
