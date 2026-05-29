package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
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

type translateTextBlock struct {
	Text           string            `json:"text"`
	TranslatedText string            `json:"translatedText"`
	Confidence     float64           `json:"confidence"`
	BBox           translateTextBBox `json:"bbox"`
}

type translateOCRResult struct {
	DetectedLanguage string               `json:"detectedLanguage"`
	TextBlocksCount  int                  `json:"textBlocksCount"`
	Blocks           []translateTextBlock `json:"blocks"`
}

type translateQualitySummary struct {
	TextBlocksCount          int      `json:"textBlocksCount"`
	TranslatedBlocksCount    int      `json:"translatedBlocksCount"`
	LowConfidenceBlocksCount int      `json:"lowConfidenceBlocksCount"`
	LayoutPreserved          bool     `json:"layoutPreserved"`
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

func (s *Service) runOCROnImage(ctx context.Context, imageURL, sourceLang, targetLang string) (*translateOCRResult, error) {
	if s == nil || s.AIGateway == nil {
		return nil, newTranslateErr(errCodeOCRFailed, "未配置 AI 服务，无法进行文字识别（请在「设置 → AI」配置）")
	}
	srcHint := "auto-detect the source language"
	if sourceLang != "" && !strings.EqualFold(sourceLang, "auto") {
		srcHint = fmt.Sprintf("assume source language is %s", langDisplayName(sourceLang))
	}
	targetName := langDisplayName(targetLang)
	prompt := fmt.Sprintf(`You are an OCR engine for ecommerce product images.
Analyze the product image at this URL: %s
%s
Detect ALL visible text overlays, labels, badges, and promotional text (not product packaging text printed on the physical product if possible to distinguish).
For each text block return: original text, translated text in %s, confidence (0-1), bounding box (x, y, width, height in pixels relative to image top-left).

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
If no readable text exists, return {"detectedLanguage":"","textBlocksCount":0,"blocks":[]}.`, imageURL, srcHint, targetName)

	resp, err := s.AIGateway.Chat(ctx, aigate.ChatRequest{
		Messages: []aigate.Message{{Role: "user", Content: prompt}},
		ResponseFormat: &aigate.ResponseFormat{
			Type: "json_object",
		},
		MaxTokens: 2000,
	})
	if err != nil {
		return nil, newTranslateErr(errCodeOCRFailed, "文字识别失败，请稍后重试或更换图片")
	}
	content := strings.TrimSpace(resp.Content)
	var ocr translateOCRResult
	if err := json.Unmarshal([]byte(content), &ocr); err != nil {
		return nil, newTranslateErr(errCodeOCRFailed, "文字识别结果解析失败，请稍后重试")
	}
	if ocr.TextBlocksCount <= 0 {
		ocr.TextBlocksCount = len(ocr.Blocks)
	}
	return &ocr, nil
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
	}
	return nil
}

func buildTranslateQuality(ocr *translateOCRResult, hints map[string]any) translateQualitySummary {
	q := translateQualitySummary{
		TextBlocksCount:       len(ocr.Blocks),
		TranslatedBlocksCount: 0,
		LayoutPreserved:       boolFromHints(hints, "preserveLayout", true),
		Warnings:              []string{},
	}
	const lowConf = 0.70
	const longRatio = 2.0
	const longAbs = 80
	for _, b := range ocr.Blocks {
		if strings.TrimSpace(b.TranslatedText) != "" {
			q.TranslatedBlocksCount++
		}
		if b.Confidence > 0 && b.Confidence < lowConf {
			q.LowConfidenceBlocksCount++
		}
		origLen := len([]rune(strings.TrimSpace(b.Text)))
		trLen := len([]rune(strings.TrimSpace(b.TranslatedText)))
		if origLen > 0 && (trLen > int(float64(origLen)*longRatio) || trLen > longAbs) {
			hasLong := false
			for _, w := range q.Warnings {
				if w == "部分翻译文字较长，可能影响图片排版。" {
					hasLong = true
					break
				}
			}
			if !hasLong {
				q.Warnings = append(q.Warnings, "部分翻译文字较长，可能影响图片排版。")
			}
		}
	}
	if q.LowConfidenceBlocksCount > 0 {
		q.Warnings = append(q.Warnings, "部分文字识别置信度较低，请人工检查翻译结果。")
	}
	return q
}

func translateEditPrompt(ocr *translateOCRResult, hints map[string]any, sourceLang, targetLang string) string {
	preserveLayout := boolFromHints(hints, "preserveLayout", true)
	removeOriginal := boolFromHints(hints, "removeOriginalText", true)
	keepProduct := boolFromHints(hints, "keepProductUnchanged", true)

	var pairs []string
	for _, b := range ocr.Blocks {
		orig := strings.TrimSpace(b.Text)
		tr := strings.TrimSpace(b.TranslatedText)
		if orig == "" || tr == "" {
			continue
		}
		pairs = append(pairs, fmt.Sprintf(`"%s" → "%s"`, orig, tr))
	}

	instruction := fmt.Sprintf(
		"Translate all visible text in this product image from %s to %s.",
		langDisplayName(sourceLang),
		langDisplayName(targetLang),
	)
	if strings.EqualFold(sourceLang, "auto") && ocr != nil && strings.TrimSpace(ocr.DetectedLanguage) != "" {
		instruction = fmt.Sprintf(
			"Translate all visible text in this product image from %s to %s.",
			langDisplayName(ocr.DetectedLanguage),
			langDisplayName(targetLang),
		)
	}
	if removeOriginal {
		instruction += " Erase/remove the original text overlays completely before writing translated text."
	}
	if preserveLayout {
		instruction += " Preserve the original text layout, position, size, and alignment as closely as possible."
	}
	if keepProduct {
		instruction += " Do NOT alter the product itself, colors, shape, or background except where text overlays exist."
	}
	if len(pairs) > 0 {
		instruction += " Text replacements:\n" + strings.Join(pairs, "\n")
	}
	instruction += " Output a clean professional ecommerce product photo with translated text only."
	userPrompt := strings.TrimSpace(stringFromMap(hints, "prompt"))
	if userPrompt != "" {
		instruction += " " + userPrompt
	}
	neg := strings.TrimSpace(stringFromMap(hints, "negativePrompt"))
	if neg != "" {
		instruction += " Avoid: " + neg + "."
	}
	return instruction
}

func prepareTranslateHints(task *ImageTask, hints map[string]any, ocr *translateOCRResult) map[string]any {
	if hints == nil {
		hints = map[string]any{}
	}
	sourceLang, targetLang := resolveTranslateLanguages(hints)
	assembled := translateEditPrompt(ocr, hints, sourceLang, targetLang)
	hints["assembled_prompt"] = assembled
	hints["prompt"] = assembled
	hints["targetLanguage"] = targetLang
	hints["sourceLanguage"] = sourceLang
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

	sourceLang, targetLang := resolveTranslateLanguages(hints)
	ocr, err := s.runOCROnImage(ctx, src, sourceLang, targetLang)
	if err != nil {
		return err
	}
	if ocr == nil || len(ocr.Blocks) == 0 {
		return newTranslateErr(errCodeNoTextDetected, "未识别到可翻译文字，请确认图片中包含清晰可见的文字")
	}
	if err := s.ensureBlocksTranslated(ctx, ocr, targetLang); err != nil {
		return err
	}

	quality := buildTranslateQuality(ocr, hints)
	if quality.TranslatedBlocksCount == 0 {
		return newTranslateErr(errCodeTranslateFailed, "文字翻译失败，未能生成有效翻译结果")
	}

	hints = prepareTranslateHints(task, hints, ocr)

	provName := strings.TrimSpace(strings.ToLower(task.Provider))
	prov, err := imgprov.NewForTask(ctx, provName, s.Settings)
	if err != nil {
		return newTranslateErr(errCodeImageEditFailed, "图片编辑服务不可用："+err.Error())
	}

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

	detectedLang := strings.TrimSpace(ocr.DetectedLanguage)
	if detectedLang == "" && !strings.EqualFold(sourceLang, "auto") {
		detectedLang = sourceLang
	}

	ocrSummary := map[string]any{
		"detectedLanguage": detectedLang,
		"textBlocksCount":  len(ocr.Blocks),
		"blocks":           ocr.Blocks,
	}
	outObj := map[string]any{
		"resultUrl":      finalURL,
		"storageKey":     storageKey,
		"provider":       task.Provider,
		"taskType":       task.TaskType,
		"sourceLanguage": sourceLang,
		"targetLanguage": targetLang,
		"ocr":            ocrSummary,
		"quality":        quality,
	}
	if finalFID != nil {
		outObj["resultFileId"] = finalFID.String()
	}

	status := StatusSuccess
	if len(quality.Warnings) > 0 {
		status = StatusSuccessWithWarnings
	}
	return s.finalizeTaskSuccessWithStatus(ctx, task, finalURL, finalFID, storageKey, outObj, nil, false, status)
}
