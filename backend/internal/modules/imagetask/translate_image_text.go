package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

const (
	TaskTypeTranslateImageText = "translate_image_text"

	errCodeImageFetchFailed    = "IMAGE_FETCH_FAILED"
	errCodeOCRFailed           = "OCR_FAILED"
	errCodeNoTextDetected      = "NO_TEXT_DETECTED"
	errCodeTranslateFailed     = "TRANSLATE_FAILED"
	errCodeImageEditFailed     = "IMAGE_EDIT_FAILED"
	errCodeStorageUploadFailed = "STORAGE_UPLOAD_FAILED"
)

type translateTextBBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type translateTextStyle struct {
	Color           string `json:"color,omitempty"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
	FontWeight      string `json:"fontWeight,omitempty"`
	Align           string `json:"align,omitempty"`
	BorderRadius    int    `json:"borderRadius,omitempty"`
}

type translateTextBlock struct {
	ID                  string               `json:"id,omitempty"`
	BlockClass          string               `json:"blockClass,omitempty"`
	Text                string               `json:"text"`
	TranslatedText      string               `json:"translatedText"`
	StandardTranslation string               `json:"standardTranslation,omitempty"`
	ShortTranslatedText string               `json:"shortTranslatedText,omitempty"`
	CompactTranslation  string               `json:"compactTranslation,omitempty"`
	DrawText            string               `json:"drawText,omitempty"`
	Confidence          float64              `json:"confidence"`
	BBox                translateTextBBox    `json:"bbox"`
	Polygon             []translateTextPoint `json:"polygon,omitempty"`
	Angle               float64              `json:"angle,omitempty"`
	Direction           string               `json:"direction,omitempty"`
	Style               translateTextStyle   `json:"style,omitempty"`
}

type translateTextPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type translateOCRResult struct {
	Provider            string                   `json:"provider,omitempty"`
	APIName             string                   `json:"apiName,omitempty"`
	ConfiguredProvider  string                   `json:"configuredOcrProvider,omitempty"`
	ActualProvider      string                   `json:"actualOcrProvider,omitempty"`
	Fallback            bool                     `json:"fallback,omitempty"`
	FallbackReason      string                   `json:"ocrFallbackReason,omitempty"`
	FallbackErrorCode   string                   `json:"ocrErrorCode,omitempty"`
	DetectedLanguage    string                   `json:"detectedLanguage"`
	TextBlocksCount     int                      `json:"textBlocksCount"`
	SupplementedBlocks  int                      `json:"supplementedBlocks,omitempty"`
	FilteredBlocksCount int                      `json:"filteredBlocksCount,omitempty"`
	AverageConfidence   float64                  `json:"averageConfidence,omitempty"`
	ErrorMessage        string                   `json:"errorMessage,omitempty"`
	Blocks              []translateTextBlock     `json:"blocks"`
	Groups              []translateTextGroup     `json:"groups,omitempty"`
	CoordinateMeta      *translateCoordinateMeta `json:"coordinateMeta,omitempty"`
}

type translateQualitySummary struct {
	TextBlocksCount          int                    `json:"textBlocksCount"`
	TranslatedBlocksCount    int                    `json:"translatedBlocksCount"`
	LowConfidenceBlocksCount int                    `json:"lowConfidenceBlocksCount"`
	LayoutPreserved          bool                   `json:"layoutPreserved"`
	Layout                   translateLayoutSummary `json:"layout"`
	Warnings                 []string               `json:"warnings"`
}

type translateRenderQuality struct {
	TextAppliedScore         int      `json:"textAppliedScore"`
	SourceTextRemovedScore   int      `json:"sourceTextRemovedScore"`
	LayoutScore              int      `json:"layoutScore"`
	StyleConsistencyScore    int      `json:"styleConsistencyScore"`
	ReadabilityScore         int      `json:"readabilityScore"`
	ProductPreservationScore int      `json:"productPreservationScore"`
	CommercialUsabilityScore int      `json:"commercialUsabilityScore"`
	Passed                   bool     `json:"passed"`
	Warnings                 []string `json:"warnings"`
}

type translateTaskError struct {
	Code    string
	Message string
}

func (e *translateTaskError) Error() string {
	if e == nil {
		return ""
	}
	code := strings.TrimSpace(e.Code)
	msg := strings.TrimSpace(e.Message)
	if code != "" {
		return fmt.Sprintf("[%s] %s", code, msg)
	}
	return msg
}

func newTranslateErr(code, msg string) error {
	return &translateTaskError{Code: code, Message: msg}
}

// IsTranslateTaskType reports image text translation tasks.
func IsTranslateTaskType(t string) bool {
	return strings.TrimSpace(strings.ToLower(t)) == TaskTypeTranslateImageText
}

func langDisplayName(code string) string {
	switch strings.TrimSpace(strings.ToLower(code)) {
	case "zh", "cn", "chinese":
		return "Chinese"
	case "en", "english":
		return "English"
	case "ja":
		return "Japanese"
	case "ko":
		return "Korean"
	default:
		return code
	}
}

func resolveTranslateLanguages(hints map[string]any) (sourceLang, targetLang string) {
	sourceLang = strings.TrimSpace(stringFromMap(hints, "sourceLanguage"))
	if sourceLang == "" {
		sourceLang = strings.TrimSpace(stringFromMap(hints, "sourceLang"))
	}
	if sourceLang == "" {
		sourceLang = "auto"
	}
	targetLang = strings.TrimSpace(stringFromMap(hints, "targetLanguage"))
	if targetLang == "" {
		targetLang = strings.TrimSpace(stringFromMap(hints, "targetLang"))
	}
	if targetLang == "" {
		targetLang = "en"
	}
	return sourceLang, targetLang
}

func boolFromHints(hints map[string]any, key string, def bool) bool {
	if hints == nil {
		return def
	}
	v, ok := hints[key]
	if !ok {
		return def
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s == "true" || s == "1" || s == "yes"
	default:
		return def
	}
}

func verifyImageAccessible(ctx context.Context, imageURL string) error {
	u := strings.TrimSpace(imageURL)
	if u == "" {
		return newTranslateErr(errCodeImageFetchFailed, "图片地址为空，无法读取原图")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return newTranslateErr(errCodeImageFetchFailed, "无法访问原图，请检查图片链接是否有效")
	}
	cli := &http.Client{Timeout: 20 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		// Some storage providers may not support HEAD; try GET with range.
		getReq, gErr := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if gErr != nil {
			return newTranslateErr(errCodeImageFetchFailed, "无法访问原图，请检查图片链接是否有效")
		}
		getReq.Header.Set("Range", "bytes=0-1023")
		resp, err = cli.Do(getReq)
		if err != nil {
			return newTranslateErr(errCodeImageFetchFailed, "无法访问原图，请检查图片链接是否有效")
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return newTranslateErr(errCodeImageFetchFailed, "无法访问原图，请检查图片链接是否有效")
	}
	return nil
}

func (s *Service) ensureBlocksTranslated(ctx context.Context, ocr *translateOCRResult, targetLang string) error {
	if s == nil || s.AIGateway == nil || ocr == nil {
		return newTranslateErr(errCodeTranslateFailed, "未配置 AI 服务，无法翻译文字")
	}
	targetName := langDisplayName(targetLang)
	var needTranslate []translateTextBlock
	for _, b := range ocr.Blocks {
		if strings.TrimSpace(b.Text) == "" {
			continue
		}
		if strings.TrimSpace(b.TranslatedText) == "" {
			needTranslate = append(needTranslate, b)
		}
	}
	if len(needTranslate) == 0 {
		return nil
	}
	var lines []string
	for _, b := range needTranslate {
		lines = append(lines, fmt.Sprintf("- %s", strings.TrimSpace(b.Text)))
	}
	prompt := fmt.Sprintf(`Translate the following product image text lines to %s.
Return ONLY valid JSON: {"translations":[{"text":"original","translatedText":"..."}]}
Lines:
%s`, targetName, strings.Join(lines, "\n"))

	resp, err := s.AIGateway.Chat(ctx, aigate.ChatRequest{
		Messages: []aigate.Message{{Role: "user", Content: prompt}},
		ResponseFormat: &aigate.ResponseFormat{
			Type: "json_object",
		},
		MaxTokens: 1500,
	})
	if err != nil {
		return newTranslateErr(errCodeTranslateFailed, "文字翻译失败，请稍后重试")
	}
	var parsed struct {
		Translations []struct {
			Text           string `json:"text"`
			TranslatedText string `json:"translatedText"`
		} `json:"translations"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &parsed); err != nil {
		return newTranslateErr(errCodeTranslateFailed, "翻译结果解析失败，请稍后重试")
	}
	lookup := map[string]string{}
	for _, t := range parsed.Translations {
		lookup[strings.TrimSpace(t.Text)] = strings.TrimSpace(t.TranslatedText)
	}
	for i := range ocr.Blocks {
		key := strings.TrimSpace(ocr.Blocks[i].Text)
		if key == "" {
			continue
		}
		if strings.TrimSpace(ocr.Blocks[i].TranslatedText) == "" {
			if tr, ok := lookup[key]; ok && tr != "" {
				ocr.Blocks[i].TranslatedText = tr
			}
		}
		if strings.TrimSpace(ocr.Blocks[i].StandardTranslation) == "" {
			ocr.Blocks[i].StandardTranslation = strings.TrimSpace(ocr.Blocks[i].TranslatedText)
		}
	}
	return nil
}

func (s *Service) simplifyLongTranslations(ctx context.Context, blocks []translateTextBlock, targetLang string, allow bool) error {
	ensureRuleCompact := func(i int) {
		if strings.TrimSpace(blocks[i].ShortTranslatedText) == "" {
			blocks[i].ShortTranslatedText = ruleBasedShortText(blocks[i].Text, blocks[i].TranslatedText, targetLang)
		}
		if strings.TrimSpace(blocks[i].CompactTranslation) == "" {
			blocks[i].CompactTranslation = strings.TrimSpace(blocks[i].ShortTranslatedText)
		}
		if strings.TrimSpace(blocks[i].StandardTranslation) == "" {
			blocks[i].StandardTranslation = strings.TrimSpace(blocks[i].TranslatedText)
		}
	}
	if !allow || s == nil || s.AIGateway == nil {
		for i := range blocks {
			ensureRuleCompact(i)
		}
		return nil
	}
	var need []translateTextBlock
	for _, b := range blocks {
		if needsShortText(b.Text, b.TranslatedText) && strings.TrimSpace(b.ShortTranslatedText) == "" {
			need = append(need, b)
		}
	}
	for i := range blocks {
		ensureRuleCompact(i)
	}
	if len(need) == 0 {
		return nil
	}
	var lines []string
	for _, b := range need {
		lines = append(lines, fmt.Sprintf(`- original: %q translated: %q`, strings.TrimSpace(b.Text), strings.TrimSpace(b.TranslatedText)))
	}
	targetName := langDisplayName(targetLang)
	prompt := fmt.Sprintf(`Shorten the following ecommerce product image marketing translations to fit small text overlays.
Keep meaning but use concise marketing copy (2-4 words when possible, Title Case for English).
Target language: %s
Return ONLY valid JSON: {"items":[{"text":"original source","translatedText":"full translation","shortTranslatedText":"short version"}]}
Items:
%s`, targetName, strings.Join(lines, "\n"))

	resp, err := s.AIGateway.Chat(ctx, aigate.ChatRequest{
		Messages: []aigate.Message{{Role: "user", Content: prompt}},
		ResponseFormat: &aigate.ResponseFormat{
			Type: "json_object",
		},
		MaxTokens: 1200,
	})
	if err != nil {
		return nil // fallback already applied via rules
	}
	var parsed struct {
		Items []struct {
			Text                string `json:"text"`
			TranslatedText      string `json:"translatedText"`
			ShortTranslatedText string `json:"shortTranslatedText"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &parsed); err != nil {
		return nil
	}
	lookup := map[string]string{}
	for _, it := range parsed.Items {
		key := strings.TrimSpace(it.Text)
		if key == "" {
			key = strings.TrimSpace(it.TranslatedText)
		}
		short := strings.TrimSpace(it.ShortTranslatedText)
		if key != "" && short != "" {
			lookup[key] = short
		}
	}
	for i := range blocks {
		origKey := strings.TrimSpace(blocks[i].Text)
		trKey := strings.TrimSpace(blocks[i].TranslatedText)
		if short, ok := lookup[origKey]; ok && short != "" {
			blocks[i].ShortTranslatedText = short
			blocks[i].CompactTranslation = short
		} else if short, ok := lookup[trKey]; ok && short != "" {
			blocks[i].ShortTranslatedText = short
			blocks[i].CompactTranslation = short
		}
		if strings.TrimSpace(blocks[i].CompactTranslation) == "" {
			blocks[i].CompactTranslation = strings.TrimSpace(blocks[i].ShortTranslatedText)
		}
		if strings.TrimSpace(blocks[i].StandardTranslation) == "" {
			blocks[i].StandardTranslation = strings.TrimSpace(blocks[i].TranslatedText)
		}
	}
	return nil
}

func applyLayoutPlansToBlocks(blocks []translateTextBlock, plans []translateBlockLayoutPlan) {
	planIdx := 0
	for i := range blocks {
		if strings.TrimSpace(blocks[i].TranslatedText) == "" {
			continue
		}
		if planIdx >= len(plans) {
			break
		}
		plan := plans[planIdx]
		planIdx++
		if plan.DisplayText != "" {
			blocks[i].TranslatedText = plan.DisplayText
			blocks[i].DrawText = plan.DisplayText
		}
		if plan.UsedShortText && strings.TrimSpace(blocks[i].ShortTranslatedText) == "" {
			blocks[i].ShortTranslatedText = plan.DisplayText
		}
		blocks[i].BBox = plan.BBox
	}
}

func buildTranslateQuality(ocr *translateOCRResult, hints map[string]any, layoutSummary translateLayoutSummary) translateQualitySummary {
	q := translateQualitySummary{
		TextBlocksCount:       len(ocr.Blocks),
		TranslatedBlocksCount: 0,
		LayoutPreserved:       boolFromHints(hints, "preserveLayout", true),
		Layout:                layoutSummary,
		Warnings:              []string{},
	}
	const lowConf = 0.70
	for _, b := range ocr.Blocks {
		if strings.TrimSpace(b.TranslatedText) != "" {
			q.TranslatedBlocksCount++
		}
		if b.Confidence > 0 && b.Confidence < lowConf {
			q.LowConfidenceBlocksCount++
		}
	}
	for _, w := range layoutSummary.Warnings {
		switch w {
		case layoutWarningTextTooLong:
			q.Warnings = appendUniqueWarning(q.Warnings, "部分翻译文字较长，可能影响图片排版，请检查结果图。")
		case layoutWarningFontAdjusted:
			q.Warnings = appendUniqueWarning(q.Warnings, "系统已自动调整部分文字大小。")
		case layoutWarningSimplified:
			q.Warnings = appendUniqueWarning(q.Warnings, "系统已自动精简部分翻译文案以适配排版。")
		case layoutWarningOverflow:
			q.Warnings = appendUniqueWarning(q.Warnings, "部分翻译文字较长，可能影响图片排版，请检查结果图。")
		case layoutWarningPartialOCR:
			q.Warnings = appendUniqueWarning(q.Warnings, "部分图片文字可能未全部识别，请检查结果图是否仍有未翻译文字。")
		case layoutWarningOCRFiltered:
			q.Warnings = appendUniqueWarning(q.Warnings, "已过滤疑似非原图文字，仅翻译图片中真实可见的文字。")
		}
	}
	if q.LowConfidenceBlocksCount > 0 {
		q.Warnings = appendUniqueWarning(q.Warnings, "部分文字识别置信度较低，请人工检查翻译结果。")
	}

	if ocr != nil {
		if ocr.Provider == "ai_vision" {
			q.Warnings = appendUniqueWarning(q.Warnings, "当前使用 AI 视觉识别文字，复杂图片可能需要人工检查。")
		} else if ocr.Provider == "paddleocr" {
			q.Warnings = appendUniqueWarning(q.Warnings, "当前使用本地 OCR 识别文字。")
		} else if ocr.Provider == "tencent" {
			q.Warnings = appendUniqueWarning(q.Warnings, "当前使用腾讯云 OCR 识别文字。")
		}
	}

	return q
}

func appendUniqueWarning(warnings []string, msg string) []string {
	for _, w := range warnings {
		if w == msg {
			return warnings
		}
	}
	return append(warnings, msg)
}

func appendUniqueCodeWarning(warnings []string, code string) []string {
	for _, w := range warnings {
		if w == code {
			return warnings
		}
	}
	return append(warnings, code)
}

func translateEditPrompt(ocr *translateOCRResult, hints map[string]any, sourceLang, targetLang string, plans []translateBlockLayoutPlan) string {
	preserveLayout := boolFromHints(hints, "preserveLayout", true)
	removeOriginal := boolFromHints(hints, "removeOriginalText", true)
	keepProduct := boolFromHints(hints, "keepProductUnchanged", true)
	layoutOpts := parseTranslateLayoutOptions(hints, targetLang)

	var pairs []string
	var layoutInstructions []string
	planIdx := 0
	for _, b := range ocr.Blocks {
		orig := strings.TrimSpace(b.Text)
		tr := strings.TrimSpace(b.TranslatedText)
		if orig == "" || tr == "" {
			continue
		}
		short := strings.TrimSpace(b.ShortTranslatedText)
		display := tr
		if short != "" && short != tr {
			pairs = append(pairs, fmt.Sprintf(`"%s" → "%s" (preferred short: "%s")`, orig, tr, short))
		} else {
			pairs = append(pairs, fmt.Sprintf(`"%s" → "%s"`, orig, tr))
		}

		if layoutOpts.AutoLayout && planIdx < len(plans) {
			plan := plans[planIdx]
			planIdx++
			if plan.DisplayText != "" {
				display = plan.DisplayText
			}
			bb := plan.BBox
			lineHint := strings.Join(plan.Lines, " | ")
			if lineHint == "" {
				lineHint = display
			}
			instr := fmt.Sprintf(
				"- Region (%d,%d %dx%d): write %q at ~%dpx, lines: [%s], keep inside box, do not overlap product",
				bb.X, bb.Y, bb.Width, bb.Height, display, plan.FontSize, lineHint,
			)
			if plan.Wrapped {
				instr += ", word-wrap enabled"
			}
			if plan.FontResized {
				instr += ", auto font size"
			}
			layoutInstructions = append(layoutInstructions, instr)
		}
	}

	instruction := fmt.Sprintf(
		"STRICT image text translation task — replace existing overlay text only. This is NOT marketing image generation or poster design.",
	)
	if strings.EqualFold(sourceLang, "auto") && ocr != nil && strings.TrimSpace(ocr.DetectedLanguage) != "" {
		sourceLang = ocr.DetectedLanguage
	}
	instruction += fmt.Sprintf(
		" Translate ONLY the %d listed text block(s) from %s to %s. Do not translate or add any other text.",
		len(pairs),
		langDisplayName(sourceLang),
		langDisplayName(targetLang),
	)
	if removeOriginal {
		instruction += " Erase/remove ONLY the original text at each listed region before writing the translated text."
	}
	if preserveLayout {
		instruction += " Keep the same position, alignment, font style, and layout as the original text for each region."
	}
	if layoutOpts.AutoLayout {
		instruction += " Fit translated text inside each original text region; wrap or shrink font if needed."
	}
	if keepProduct {
		instruction += " Do NOT alter the product, props, colors, background, or any pixels outside the listed text regions."
	}
	instruction += " FORBIDDEN: adding new text, prices, flash sale, discount tags, promotional banners, stickers, watermarks, coupons, or any text not in the replacement list."
	instruction += " The output must look like the same photo with only the listed texts translated — nothing added."
	if len(pairs) > 0 {
		instruction += "\nONLY these replacements are allowed (exact regions, no extras):\n" + strings.Join(pairs, "\n")
	}
	if len(layoutInstructions) > 0 {
		instruction += "\nLayout per block:\n" + strings.Join(layoutInstructions, "\n")
	}
	neg := strings.TrimSpace(stringFromMap(hints, "negativePrompt"))
	if neg == "" {
		neg = translateDefaultNegativePrompt()
	}
	instruction += " Avoid: " + neg + "."
	return instruction
}

func prepareTranslateHints(task *ImageTask, hints map[string]any, ocr *translateOCRResult, plans []translateBlockLayoutPlan) map[string]any {
	if hints == nil {
		hints = map[string]any{}
	}
	sourceLang, targetLang := resolveTranslateLanguages(hints)
	assembled := translateEditPrompt(ocr, hints, sourceLang, targetLang, plans)
	hints["assembled_prompt"] = assembled
	hints["prompt"] = assembled
	hints["targetLanguage"] = targetLang
	hints["sourceLanguage"] = sourceLang
	if strings.TrimSpace(stringFromMap(hints, "negativePrompt")) == "" {
		hints["negativePrompt"] = translateDefaultNegativePrompt()
	}
	hints["strictLiteralTranslation"] = true
	return hints
}

func (s *Service) executeTranslateImageTextTask(ctx context.Context, task *ImageTask, hints map[string]any) error {
	if s == nil || task == nil {
		return fmt.Errorf("imagetask: invalid translate task")
	}
	src := strings.TrimSpace(task.SourceImageURL)
	if src == "" {
		return newTranslateErr(errCodeImageFetchFailed, "缺少源图地址，无法读取原图")
	}
	if err := verifyImageAccessible(ctx, src); err != nil {
		return err
	}

	m, _ := s.Settings.PlainByGroup(ctx, 0, "image")
	defaultEraseMode := strings.TrimSpace(m["erase_mode"])
	renderOpts := parseTranslateRenderOptions(hints, defaultEraseMode)
	sourceLang, targetLang := resolveTranslateLanguages(hints)
	payload, loadErr := s.loadTranslateImagePayload(ctx, src)
	if loadErr != nil || payload == nil {
		return newTranslateErr(errCodeImageFetchFailed, "无法下载原图用于翻译")
	}
	sourceBytes, err := decodePayloadBytes(payload)
	if err != nil {
		return newTranslateErr(errCodeImageFetchFailed, "无法读取原图数据："+err.Error())
	}

	s.logTranslateAudit(ctx, task, "ai_image.translate_text.started", "success",
		translateAuditMsg(task, map[string]any{"renderMode": renderOpts.RenderMode}))

	ocr, err := s.runOCROnImage(ctx, src, sourceLang, targetLang, hints)
	if err != nil {
		return err
	}
	if ocr == nil || len(ocr.Blocks) == 0 {
		return newTranslateErr(errCodeNoTextDetected, "未识别到可翻译文字，请确认图片中包含清晰可见的文字")
	}
	imgRef := payload.DataURL
	if imgRef == "" {
		imgRef = src
	}
	ocr = s.filterAndVerifyOCR(ctx, ocr, imgRef)
	if ocr == nil || len(ocr.Blocks) == 0 {
		return newTranslateErr(errCodeNoTextDetected, "未识别到可翻译文字，请确认图片中包含清晰可见的文字")
	}
	assignOCRBlockIDs(ocr.Blocks)
	imageW, imageH := payload.Width, payload.Height
	if rw, rh, dErr := renderDimensionsFromBytes(sourceBytes); dErr == nil && rw > 0 && rh > 0 {
		imageW, imageH = rw, rh
	}
	if imageW <= 0 {
		imageW = intFromAny(hints["imageWidth"])
		imageH = intFromAny(hints["imageHeight"])
	}
	coordMeta, coordErr := applyOCRCoordinateMapping(ocr, imageW, imageH)
	if coordErr != nil {
		return coordErr
	}
	ocr.CoordinateMeta = &coordMeta
	bboxRepaired := false
	if needsOCRBBoxRepair(ocr.Blocks) {
		if repaired := s.repairOCRBlockBBoxes(ctx, imgRef, ocr.Blocks, imageW, imageH); len(repaired) > 0 {
			ocr.Blocks = clampOCRBlockBBoxes(repaired, imageW, imageH)
			bboxRepaired = true
		}
	} else {
		ocr.Blocks = clampOCRBlockBBoxes(ocr.Blocks, imageW, imageH)
	}
	inferBlockStyles(payload, ocr.Blocks)
	classifyTranslateBlocks(ocr.Blocks, imageW, imageH)

	s.logTranslateAudit(ctx, task, "ai_image.translate_text.ocr_done", "success",
		translateAuditMsg(task, map[string]any{"textBlocksCount": len(ocr.Blocks)}))

	if err := s.ensureBlocksTranslated(ctx, ocr, targetLang); err != nil {
		return err
	}
	s.logTranslateAudit(ctx, task, "ai_image.translate_text.translated", "success",
		translateAuditMsg(task, map[string]any{"translatedBlocksCount": len(ocr.Blocks)}))

	layoutOpts := parseTranslateLayoutOptions(hints, targetLang)
	_ = s.simplifyLongTranslations(ctx, ocr.Blocks, targetLang, layoutOpts.AllowTextSimplify)

	if imageW <= 0 {
		imageW = intFromAny(hints["imageWidth"])
		imageH = intFromAny(hints["imageHeight"])
	}
	textGroups, layoutTemplate := buildTranslateTextGroups(ocr.Blocks, hints, imageW, imageH)
	groupPlans, layoutSummary := computeTranslateGroupLayouts(textGroups, layoutOpts, imageW, imageH, layoutTemplate)
	sim := simulateTranslateGroupLayouts(groupPlans, imageW, imageH)
	layoutSummary.Simulation = sim
	for _, w := range sim.Warnings {
		layoutSummary.Warnings = appendUniqueCodeWarning(layoutSummary.Warnings, w)
	}
	if bboxRepaired {
		layoutSummary.Warnings = appendUniqueCodeWarning(layoutSummary.Warnings, layoutWarningBBoxRepaired)
	}
	if ocr.CoordinateMeta != nil && ocr.CoordinateMeta.CoordScaleApplied {
		layoutSummary.Warnings = appendUniqueCodeWarning(layoutSummary.Warnings, warningOCRCoordScaled)
	}
	if ocr.FilteredBlocksCount > 0 {
		layoutSummary.Warnings = appendUniqueCodeWarning(layoutSummary.Warnings, layoutWarningOCRFiltered)
	}
	if ocr.SupplementedBlocks > 0 || (len(ocr.Blocks) == 1 && imageW > 400 && imageH > 400) {
		layoutSummary.Warnings = appendUniqueCodeWarning(layoutSummary.Warnings, layoutWarningPartialOCR)
	}
	applyGroupPlansToOCR(textGroups, groupPlans, ocr)

	quality := buildTranslateQuality(ocr, hints, layoutSummary)
	if quality.TranslatedBlocksCount == 0 {
		return newTranslateErr(errCodeTranslateFailed, "文字翻译失败，未能生成有效翻译结果")
	}

	if renderOpts.RenderMode == RenderModeAIEdit {
		layoutPlans, _ := computeTranslateLayouts(ocr.Blocks, layoutOpts, imageW, imageH)
		return s.executeTranslateAIEdit(ctx, task, hints, ocr, layoutPlans, layoutSummary, quality, sourceLang, targetLang)
	}
	if renderOpts.RenderMode == RenderModeDeterministic || renderOpts.RenderMode == RenderModeHybrid {
		renderBlocks := buildRenderBlocksFromGroups(textGroups, groupPlans)
		return s.executeTranslateDeterministic(ctx, task, hints, renderOpts, ocr, renderBlocks, layoutSummary, quality, sourceLang, targetLang, sourceBytes)
	}
	renderOpts.RenderMode = RenderModeHybrid
	renderBlocks := buildRenderBlocksFromGroups(textGroups, groupPlans)
	return s.executeTranslateDeterministic(ctx, task, hints, renderOpts, ocr, renderBlocks, layoutSummary, quality, sourceLang, targetLang, sourceBytes)
}
