package imagetask

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/imagerender"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

func (s *Service) executeTranslateDeterministic(
	ctx context.Context,
	task *ImageTask,
	hints map[string]any,
	renderOpts translateRenderOptions,
	ocr *translateOCRResult,
	renderBlocks []translateRenderBlock,
	layoutSummary translateLayoutSummary,
	quality translateQualitySummary,
	sourceLang, targetLang string,
	sourceBytes []byte,
) error {
	_, _, err := imagerender.Decode(sourceBytes)
	if err != nil {
		return newTranslateErr(errCodeTranslateRenderFail, "无法解码原图："+err.Error())
	}

	imageBlocks := buildImageRenderBlocks(renderBlocks)
	if len(imageBlocks) == 0 {
		return newTranslateErr(errCodeTranslateRenderFail, "没有可绘制的翻译文字块")
	}

	eraseMode := effectiveEraseMode(renderOpts)

	if eraseMode == "ai_inpaint" {
		quality.Warnings = appendUniqueWarning(quality.Warnings, "AI 局部擦除服务未配置，已使用程序擦除方式处理。")
	}

	imOpts := imagerender.Options{
		EraseMode:       eraseMode,
		MaskPadding:     renderOpts.MaskPadding,
		TextPadding:     renderOpts.TextPadding,
		LineHeight:      floatFromAny(hints["lineHeightRatio"]),
		PureTextReplace: isPureTextReplaceMode(renderOpts.RenderMode),
	}
	if imOpts.LineHeight <= 0 {
		imOpts.LineHeight = 1.15
	}

	var res *imagerender.Result
	var verifyMeta translateVerificationMeta
	var renderQuality translateRenderQuality
	var lastRenderErr error
	var lastVerifyErr error
	var debugArtifacts map[string]any
	var qualityRetried bool

	pipelineRes, pipeErr := s.runTranslateRenderPipeline(
		ctx, task, sourceBytes, renderBlocks, imageBlocks, imOpts, renderOpts.OutputFormat,
		ocr, sourceLang, targetLang, renderOpts, quality, layoutSummary,
	)
	if pipeErr == nil && pipelineRes != nil && pipelineRes.Result != nil {
		res = pipelineRes.Result
		verifyMeta = pipelineRes.VerifyMeta
		renderQuality = pipelineRes.RenderQuality
		debugArtifacts = pipelineRes.DebugArtifacts
		qualityRetried = pipelineRes.QualityRetried
		if pipelineRes.EraseSourceRemain {
			quality.Warnings = appendUniqueCodeWarning(quality.Warnings, warningEraseFailed)
			if isPureTextReplaceMode(renderOpts.RenderMode) {
				quality.Warnings = appendUniqueCodeWarning(quality.Warnings, warningPureTextSourceNotErased)
				renderQuality.SourceTextRemovedScore = minInt(renderQuality.SourceTextRemovedScore, 35)
				renderQuality.Warnings = appendUniqueCodeWarning(renderQuality.Warnings, warningPureTextSourceNotErased)
				renderQuality.Passed = false
			}
		}
	} else {
		lastRenderErr = pipeErr
		bestScore := -1
		for _, attempt := range translateRenderAttempts(imageBlocks, imOpts, eraseMode, renderOpts.EraseMode) {
			for _, mode := range attempt.Modes {
				attemptOpts := attempt.Options
				attemptOpts.EraseMode = mode
				attemptOut, err := renderTranslateWithEraseVerify(
					s, ctx, sourceBytes, attempt.Blocks, attemptOpts, renderOpts.OutputFormat,
					ocr, sourceLang, renderOpts.VerifyOutputText,
				)
				if err != nil {
					lastRenderErr = err
					continue
				}
				attemptRes := attemptOut.Result
				attemptRes.RetryStrategies = append(attemptRes.RetryStrategies, attempt.Name, mode)
				if attemptOut.EraseSourceRemain {
					quality.Warnings = appendUniqueCodeWarning(quality.Warnings, warningEraseFailed)
				}
				attemptVerify, verifyErr := s.verifyTranslateOutputWithLayout(ctx, sourceBytes, attemptRes.Data, ocr, targetLang, sourceLang, renderOpts.VerifyOutputText, layoutSummary)
				if verifyErr != nil {
					lastVerifyErr = verifyErr
					continue
				}
				attemptQuality := buildTranslateRenderQuality(quality, layoutSummary, attemptVerify, renderOpts, renderBlocks, attemptRes)
				score := attemptQuality.CommercialUsabilityScore
				if score > bestScore {
					bestScore = score
					res, verifyMeta, renderQuality = attemptRes, attemptVerify, attemptQuality
				}
				if attemptQuality.Passed {
					break
				}
			}
			if res != nil && renderQuality.Passed {
				break
			}
		}
	}
	if res == nil {
		if lastRenderErr != nil {
			if errors.Is(lastRenderErr, imagerender.ErrEraseMaskTooLarge) {
				return newTranslateErr(errCodeTranslateEraseFail, "擦除区域过大，mask 生成异常："+lastRenderErr.Error())
			}
		}
		img, _, decErr := imagerender.Decode(sourceBytes)
		if decErr == nil && !strings.EqualFold(effectiveEraseMode(renderOpts), imagerender.EraseTextPixelMask) {
			fallbackOpts := imOpts
			fallbackOpts.EraseMode = imagerender.EraseOpenCVInpaint
			if fallbackRes, fbErr := imagerender.RenderAndEncode(img, sourceBytes, imageBlocks, fallbackOpts, renderOpts.OutputFormat); fbErr == nil {
				fallbackVerify, _ := s.verifyTranslateOutputWithLayout(ctx, sourceBytes, fallbackRes.Data, ocr, targetLang, sourceLang, renderOpts.VerifyOutputText, layoutSummary)
				fallbackQuality := buildTranslateRenderQuality(quality, layoutSummary, fallbackVerify, renderOpts, renderBlocks, fallbackRes)
				quality.Warnings = appendUniqueCodeWarning(quality.Warnings, warningEraseFailed)
				res, verifyMeta, renderQuality = fallbackRes, fallbackVerify, fallbackQuality
			}
		}
	}
	if res == nil {
		if lastVerifyErr != nil {
			s.logTranslateAudit(ctx, task, "ai_image.translate_text.failed", "failed",
				translateAuditMsg(task, map[string]any{"errorCode": translateErrCode(lastVerifyErr), "err": lastVerifyErr.Error()}))
			return lastVerifyErr
		}
		msg := "未知错误"
		if lastRenderErr != nil {
			msg = lastRenderErr.Error()
		}
		return newTranslateErr(errCodeTranslateRenderFail, "翻译文字绘制失败："+msg)
	}

	s.logTranslateAudit(ctx, task, "ai_image.translate_text.rendered", "success",
		translateAuditMsg(task, map[string]any{"renderedBlocks": res.BlocksDrawn, "eraseMode": res.EraseMode}))

	s.logTranslateAudit(ctx, task, "ai_image.translate_text.verified", "success",
		translateAuditMsg(task, map[string]any{
			"imageChanged":       verifyMeta.ImageChanged,
			"targetTextDetected": verifyMeta.TargetTextDetected,
			"confidence":         verifyMeta.Confidence,
		}))

	s.logTranslateAudit(ctx, task, "ai_image.translate_text.verified", "success",
		translateAuditMsg(task, map[string]any{
			"imageChanged":       verifyMeta.ImageChanged,
			"targetTextDetected": verifyMeta.TargetTextDetected,
			"confidence":         verifyMeta.Confidence,
		}))

	imgResult := &imgprov.ImageResult{
		RawPayload:         res.Data,
		PayloadContentType: res.ContentType,
		Meta: map[string]any{
			"renderMode": renderOpts.RenderMode,
			"eraseMode":  res.EraseMode,
		},
	}
	finalURL, finalFID, storageKey, persistErr := s.persistProviderResult(ctx, task, imgResult, hints)
	if persistErr != nil {
		return newTranslateErr(errCodeStorageUploadFailed, "翻译结果上传失败："+persistErr.Error())
	}

	return s.finalizeTranslateSuccess(ctx, task, hints, ocr, quality, renderQuality, layoutSummary, renderOpts, renderBlocks, res, verifyMeta, sourceLang, targetLang, finalURL, finalFID, storageKey, debugArtifacts, qualityRetried)
}

func (s *Service) executeTranslateAIEdit(
	ctx context.Context,
	task *ImageTask,
	hints map[string]any,
	ocr *translateOCRResult,
	layoutPlans []translateBlockLayoutPlan,
	layoutSummary translateLayoutSummary,
	quality translateQualitySummary,
	sourceLang, targetLang string,
) error {
	hints = prepareTranslateHints(task, hints, ocr, layoutPlans)
	provName := strings.TrimSpace(strings.ToLower(task.Provider))
	prov, err := imgprov.NewForTask(ctx, provName, s.Settings)
	if err != nil {
		return newTranslateErr(errCodeImageEditFailed, "图片编辑服务不可用："+err.Error())
	}
	src := strings.TrimSpace(task.SourceImageURL)
	var res *imgprov.ImageResult
	switch provName {
	case "openai_image", "dashscope_image":
		rb, rbErr := s.resolveOpenAIEditSource(ctx, task)
		if rbErr != nil {
			return newTranslateErr(errCodeImageFetchFailed, "无法读取原图用于编辑："+rbErr.Error())
		}
		if rb.File != nil {
			defer rb.File.Close()
		}
		res, err = prov.ReplaceBackground(ctx, imgprov.ReplaceBackgroundRequest{
			ImageRequest: imgprov.ImageRequest{
				SourceURL:         rb.PublicURL,
				SourceFile:        rb.File,
				SourceFilename:    rb.Filename,
				SourceContentType: rb.ContentType,
				Input:             hints,
			},
		})
	case "comfyui":
		res, err = prov.ReplaceBackground(ctx, imgprov.ReplaceBackgroundRequest{
			ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
		})
	default:
		return imgprov.UnsupportedTaskError(task.Provider, task.TaskType)
	}
	if err != nil {
		return newTranslateErr(errCodeImageEditFailed, "图片文字替换失败："+err.Error())
	}
	if res == nil {
		return newTranslateErr(errCodeImageEditFailed, "图片编辑未返回结果")
	}
	finalURL, finalFID, storageKey, persistErr := s.persistProviderResult(ctx, task, res, hints)
	if persistErr != nil {
		return newTranslateErr(errCodeStorageUploadFailed, "翻译结果上传失败："+persistErr.Error())
	}
	m, _ := s.Settings.PlainByGroup(ctx, 0, "image")
	defaultEraseMode := strings.TrimSpace(m["erase_mode"])
	renderOpts := parseTranslateRenderOptions(hints, defaultEraseMode)
	renderOpts.RenderMode = RenderModeAIEdit
	verifyMeta := translateVerificationMeta{ImageChanged: true, TargetTextDetected: true, Confidence: 0.5}
	renderQuality := buildTranslateRenderQuality(quality, layoutSummary, verifyMeta, renderOpts, nil, nil)
	return s.finalizeTranslateSuccess(ctx, task, hints, ocr, quality, renderQuality, layoutSummary, renderOpts, nil, nil, verifyMeta, sourceLang, targetLang, finalURL, finalFID, storageKey, nil, false)
}

func (s *Service) finalizeTranslateSuccess(
	ctx context.Context,
	task *ImageTask,
	hints map[string]any,
	ocr *translateOCRResult,
	quality translateQualitySummary,
	renderQuality translateRenderQuality,
	layoutSummary translateLayoutSummary,
	renderOpts translateRenderOptions,
	renderBlocks []translateRenderBlock,
	renderRes *imagerender.Result,
	verifyMeta translateVerificationMeta,
	sourceLang, targetLang, finalURL string,
	finalFID *uuid.UUID,
	storageKey string,
	debugArtifacts map[string]any,
	qualityRetried bool,
) error {
	detectedLang := strings.TrimSpace(ocr.DetectedLanguage)
	if detectedLang == "" && !strings.EqualFold(sourceLang, "auto") {
		detectedLang = sourceLang
	}
	renderedCount := quality.TranslatedBlocksCount
	if renderRes != nil {
		renderedCount = renderRes.BlocksDrawn
	}
	verifiedCount := renderedCount
	if !verifyMeta.TargetTextDetected {
		verifiedCount = 0
	}
	meta := translateResultMeta{
		Translate: translateSummaryMeta{
			SourceLanguage:        sourceLang,
			TargetLanguage:        targetLang,
			TextBlocksCount:       len(ocr.Blocks),
			TranslatedBlocksCount: quality.TranslatedBlocksCount,
			RenderedBlocksCount:   renderedCount,
			VerifiedBlocksCount:   verifiedCount,
		},
		Layout: translateLayoutMeta{
			RenderMode:        renderOpts.RenderMode,
			EraseMode:         layoutSummaryEraseMode(renderOpts, renderRes),
			LayoutTemplate:    layoutSummary.LayoutTemplate,
			AutoWrappedBlocks: layoutSummary.AutoWrappedBlocks,
			FontResizedBlocks: layoutSummary.FontResizedBlocks,
			SimplifiedBlocks:  layoutSummary.SimplifiedBlocks,
			OverflowBlocks:    layoutSummary.OverflowBlocks,
			MinFontSizeUsed:   layoutSummary.MinFontSizeUsed,
			Simulation:        layoutSummary.Simulation,
			Warnings:          layoutSummary.Warnings,
		},
		Verification:  verifyMeta,
		RenderQuality: renderQuality,
	}
	if renderRes != nil {
		meta.Layout.EraseAreaRatio = renderRes.EraseAreaRatio
		meta.Layout.PatchAreaRatio = renderRes.PatchAreaRatio
		meta.Layout.BackgroundDelta = renderRes.BackgroundDeltaScore
		meta.Layout.FlatFillRatio = renderRes.FlatFillRatio
		meta.Layout.LargePatchDetected = renderRes.LargePatchDetected
		meta.Layout.RetryStrategies = append([]string(nil), renderRes.RetryStrategies...)
	}
	if verifyMeta.SourceTextMayRemain {
		quality.Warnings = appendUniqueCodeWarning(quality.Warnings, verifyWarningSourceTextRemain)
	}
	for _, w := range renderQuality.Warnings {
		quality.Warnings = appendUniqueCodeWarning(quality.Warnings, w)
	}
	coordMeta := map[string]any{}
	if ocr.CoordinateMeta != nil {
		coordMeta = map[string]any{
			"originalImageWidth":    ocr.CoordinateMeta.OriginalImageWidth,
			"originalImageHeight":   ocr.CoordinateMeta.OriginalImageHeight,
			"ocrImageWidth":         ocr.CoordinateMeta.OCRImageWidth,
			"ocrImageHeight":        ocr.CoordinateMeta.OCRImageHeight,
			"renderImageWidth":      ocr.CoordinateMeta.RenderImageWidth,
			"renderImageHeight":     ocr.CoordinateMeta.RenderImageHeight,
			"cropOffsetX":           ocr.CoordinateMeta.CropOffsetX,
			"cropOffsetY":           ocr.CoordinateMeta.CropOffsetY,
			"cropRenderWidth":       ocr.CoordinateMeta.CropRenderWidth,
			"cropRenderHeight":      ocr.CoordinateMeta.CropRenderHeight,
			"cropScaleX":            ocr.CoordinateMeta.CropScaleX,
			"cropScaleY":            ocr.CoordinateMeta.CropScaleY,
			"scaleX":                ocr.CoordinateMeta.ScaleX,
			"scaleY":                ocr.CoordinateMeta.ScaleY,
			"aspectRatioDiff":       ocr.CoordinateMeta.AspectRatioDiff,
			"mappingMode":           ocr.CoordinateMeta.MappingMode,
			"mappingTier":           ocr.CoordinateMeta.MappingTier,
			"groupRelayoutFallback": ocr.CoordinateMeta.GroupRelayoutFallback,
			"fallback":              ocr.CoordinateMeta.Fallback,
			"coordScaleApplied":     ocr.CoordinateMeta.CoordScaleApplied,
			"bboxCorrectionCount":   ocr.CoordinateMeta.BBoxCorrectionCount,
			"coordMappingValid":     ocr.CoordinateMeta.CoordMappingValid,
			"finalMappedBoxes":      ocr.CoordinateMeta.FinalMappedBoxes,
		}
	}
	ocrSummary := map[string]any{
		"provider":              ocr.Provider,
		"apiName":               ocr.APIName,
		"configuredOcrProvider": ocr.ConfiguredProvider,
		"actualOcrProvider":     firstNonEmptyString(ocr.ActualProvider, ocr.Provider),
		"ocrFallbackUsed":       ocr.Fallback,
		"ocrFallbackReason":     ocr.FallbackReason,
		"ocrErrorCode":          ocr.FallbackErrorCode,
		"fallback":              ocr.Fallback,
		"detectedLanguage":      detectedLang,
		"textBlocksCount":       len(ocr.Blocks),
		"ocrBlocksCount":        len(ocr.Blocks),
		"averageConfidence":     ocr.AverageConfidence,
		"ocrAverageConfidence":  ocr.AverageConfidence,
		"filteredBlocksCount":   ocr.FilteredBlocksCount,
		"errorMessage":          ocr.ErrorMessage,
		"blocks":                ocr.Blocks,
		"coordinateMeta":        coordMeta,
	}
	eraseBlocks := 0
	if renderRes != nil {
		eraseBlocks = renderRes.EraseBlocks
	}
	badgeCount := countRenderBlocksByGroup(renderBlocks, groupTypeBadge, groupTypeBottomBadge)
	abnormalBadgeCount := countAbnormalBadgeRenderBlocks(renderBlocks)
	eraseBBoxCount := countRenderBlocksWithEraseBBox(renderBlocks)
	layoutBBoxCount := countRenderBlocksWithLayoutBBox(renderBlocks)
	backgroundPatchScore := 0.0
	if renderRes != nil {
		backgroundPatchScore = renderRes.BackgroundDeltaScore + renderRes.PatchAreaRatio*100
	}
	outObj := map[string]any{
		"resultUrl":               finalURL,
		"storageKey":              storageKey,
		"provider":                task.Provider,
		"taskType":                task.TaskType,
		"configuredOcrProvider":   ocr.ConfiguredProvider,
		"actualOcrProvider":       firstNonEmptyString(ocr.ActualProvider, ocr.Provider),
		"ocrFallbackUsed":         ocr.Fallback,
		"ocrFallbackReason":       ocr.FallbackReason,
		"ocrErrorCode":            ocr.FallbackErrorCode,
		"ocrBlocksCount":          len(ocr.Blocks),
		"ocrAverageConfidence":    ocr.AverageConfidence,
		"sourceLanguage":          sourceLang,
		"targetLanguage":          targetLang,
		"renderMode":              renderOpts.RenderMode,
		"pureTextReplaceMode":     isPureTextReplaceMode(renderOpts.RenderMode),
		"ocr":                     ocrSummary,
		"coordinateMeta":          coordMeta,
		"eraseBlocks":             eraseBlocks,
		"eraseRetryCount":         len(meta.Layout.RetryStrategies),
		"renderedTextCount":       renderedCount,
		"overflowTextCount":       layoutSummary.OverflowBlocks,
		"blockClassifications":    buildBlockClassificationSummary(ocr),
		"eraseBBoxCount":          eraseBBoxCount,
		"layoutBBoxCount":         layoutBBoxCount,
		"badgeCount":              badgeCount,
		"abnormalBadgeCount":      abnormalBadgeCount,
		"backgroundPatchScore":    backgroundPatchScore,
		"overlapScore":            layoutSummary.Simulation.OverlapScore,
		"badgeShapeAbnormal":      abnormalBadgeCount > 0,
		"textOverlap":             verifyMeta.SourceTextMayRemain || layoutSummary.Simulation.CollisionCount > 0,
		"quality":                 quality,
		"renderQuality":           renderQuality,
		"translate":               meta.Translate,
		"layout":                  meta.Layout,
		"verification":            meta.Verification,
		"detected_source_blocks":  len(ocr.Blocks),
		"translated_blocks":       quality.TranslatedBlocksCount,
		"rendered_blocks":         renderedCount,
		"target_language_present": verifyMeta.TargetTextDetected,
		"source_language_residue": verifyMeta.SourceTextMayRemain,
		"overflow_blocks":         layoutSummary.OverflowBlocks,
		"style_mismatch_count":    countStyleMismatchWarnings(renderQuality.Warnings),
		"patch_area_ratio":        meta.Layout.PatchAreaRatio,
		"render_quality_score":    renderQuality.CommercialUsabilityScore,
		"overall_confidence":      verifyMeta.Confidence,
	}
	if finalFID != nil {
		outObj["resultFileId"] = finalFID.String()
	}
	imageW, imageH := imageDimensionsFromRenderBlocks(renderBlocks)
	status := resolveTranslateFinalStatusForMode(
		renderQuality.CommercialUsabilityScore, verifyMeta, layoutSummary, renderQuality, qualityRetried, renderOpts.RenderMode,
		renderBlocks, renderRes, imageW, imageH,
	)
	pureValidation := validatePureTextReplace(verifyMeta, layoutSummary, renderQuality, renderBlocks, renderRes, imageW, imageH)
	if isPureTextReplaceMode(renderOpts.RenderMode) {
		outObj["validationMode"] = pureValidation.ValidationMode
		outObj["pureTextValidation"] = pureValidation
		if len(pureValidation.HardFailures) > 0 {
			labels := make([]string, 0, len(pureValidation.HardFailures))
			for _, f := range pureValidation.HardFailures {
				labels = append(labels, pureTextHardFailureReasonLabel(f))
			}
			outObj["validationFailureReasons"] = labels
		}
	}
	if qualityRetried {
		outObj["qualityAutoRetried"] = true
	}
	attachDebugToOutput(outObj, debugArtifacts)
	outObj["finalQualityStatus"] = status
	scoreJSON, _ := json.Marshal(meta)
	attachTranslateArtifactPaths(outObj, storageKey, finalURL)
	arts := translateArtifactsFromPersist(storageKey, finalFID, finalURL)
	if status == StatusFailedValidation {
		s.discardUnusableTranslateOutput(ctx, task, arts, status)
		outObj["resultUrl"] = ""
		outObj["previewPath"] = ""
		outObj["outputPath"] = ""
		outObj["tempOutputPath"] = ""
		delete(outObj, "resultFileId")
		outObj["resultUnavailable"] = true
		scoreJSON, _ = json.Marshal(meta)
		return s.finalizeTaskSuccessWithStatus(ctx, task, "", nil, "", outObj, scoreJSON, false, status)
	}
	return s.finalizeTaskSuccessWithStatus(ctx, task, finalURL, finalFID, storageKey, outObj, scoreJSON, false, status)
}

func hasSevereTranslateQualityWarning(groups ...[]string) bool {
	for _, warnings := range groups {
		for _, w := range warnings {
			switch w {
			case verifyWarningSourceTextRemain, layoutWarningPatchVisible, "erase_area_too_large",
				"text_overflow", warningBadgeShapeAbnormal, warningTextOverlap, warningEraseFailed,
				layoutWarningOverflow, layoutWarningUnbalanced, layoutWarningProductSubjectOverlap,
				warningPureTextSourceNotErased, warningPureTextExtraBackground, warningPureTextOverlap:
				return true
			}
		}
	}
	return false
}

func layoutSummaryEraseMode(opts translateRenderOptions, res *imagerender.Result) string {
	if res != nil && res.EraseMode != "" {
		return res.EraseMode
	}
	return effectiveEraseMode(opts)
}

func translateErrCode(err error) string {
	if te, ok := err.(*translateTaskError); ok && te != nil {
		return te.Code
	}
	return ""
}

func countRunesByScript(text string, cjk bool) int {
	n := 0
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		isCJK := unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) ||
			unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
		if cjk && isCJK {
			n++
		} else if !cjk && !isCJK && unicode.IsLetter(r) {
			n++
		}
	}
	return n
}
