package imagetask

import (
	"context"
	"encoding/json"
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
		EraseMode:   eraseMode,
		MaskPadding: renderOpts.MaskPadding,
		TextPadding: renderOpts.TextPadding,
		LineHeight:  floatFromAny(hints["lineHeightRatio"]),
	}
	if imOpts.LineHeight <= 0 {
		imOpts.LineHeight = 1.15
	}

	var res *imagerender.Result
	var verifyMeta translateVerificationMeta
	var renderQuality translateRenderQuality
	var lastRenderErr error
	var lastVerifyErr error
	for _, attempt := range translateRenderAttempts(imageBlocks, imOpts, eraseMode, renderOpts.EraseMode) {
		for _, mode := range attempt.Modes {
			img, _, err := imagerender.Decode(sourceBytes)
			if err != nil {
				return newTranslateErr(errCodeTranslateRenderFail, "无法解码原图："+err.Error())
			}
			attemptOpts := attempt.Options
			attemptOpts.EraseMode = mode
			attemptRes, err := imagerender.RenderAndEncode(img, sourceBytes, attempt.Blocks, attemptOpts, renderOpts.OutputFormat)
			if err != nil {
				lastRenderErr = err
				continue
			}
			attemptRes.RetryStrategies = append(attemptRes.RetryStrategies, attempt.Name, mode)
			attemptVerify, verifyErr := s.verifyTranslateOutput(ctx, sourceBytes, attemptRes.Data, ocr, targetLang, sourceLang, renderOpts.VerifyOutputText)
			if verifyErr != nil {
				lastVerifyErr = verifyErr
				continue
			}
			attemptQuality := buildTranslateRenderQuality(quality, layoutSummary, attemptVerify, renderOpts, renderBlocks, attemptRes)
			res, verifyMeta, renderQuality = attemptRes, attemptVerify, attemptQuality
			if attemptQuality.Passed || !shouldRetryTranslateRender(attemptQuality.Warnings) {
				break
			}
		}
		if res != nil && (renderQuality.Passed || !shouldRetryTranslateRender(renderQuality.Warnings)) {
			break
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

	return s.finalizeTranslateSuccess(ctx, task, hints, ocr, quality, renderQuality, layoutSummary, renderOpts, res, verifyMeta, sourceLang, targetLang, finalURL, finalFID, storageKey)
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
	return s.finalizeTranslateSuccess(ctx, task, hints, ocr, quality, renderQuality, layoutSummary, renderOpts, nil, verifyMeta, sourceLang, targetLang, finalURL, finalFID, storageKey)
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
	renderRes *imagerender.Result,
	verifyMeta translateVerificationMeta,
	sourceLang, targetLang, finalURL string,
	finalFID *uuid.UUID,
	storageKey string,
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
	ocrSummary := map[string]any{
		"detectedLanguage": detectedLang,
		"textBlocksCount":  len(ocr.Blocks),
		"blocks":           ocr.Blocks,
	}
	outObj := map[string]any{
		"resultUrl":               finalURL,
		"storageKey":              storageKey,
		"provider":                task.Provider,
		"taskType":                task.TaskType,
		"sourceLanguage":          sourceLang,
		"targetLanguage":          targetLang,
		"renderMode":              renderOpts.RenderMode,
		"ocr":                     ocrSummary,
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
	status := StatusSuccess
	if !verifyMeta.TargetTextDetected {
		status = StatusFailedValidation
	} else if renderQuality.CommercialUsabilityScore > 0 && renderQuality.CommercialUsabilityScore < 60 {
		status = StatusLowQuality
	} else if len(quality.Warnings) > 0 || verifyMeta.SourceTextMayRemain || !renderQuality.Passed {
		status = StatusSuccessWithWarnings
	}
	scoreJSON, _ := json.Marshal(meta)
	return s.finalizeTaskSuccessWithStatus(ctx, task, finalURL, finalFID, storageKey, outObj, scoreJSON, false, status)
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
