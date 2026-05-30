package imagetask

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/ocrerror"
)

const layoutWarningPartialOCR = "partial_text_detected"

type translateImagePayload struct {
	DataURL  string
	RawBytes []byte
	Width    int
	Height   int
}

func (s *Service) loadTranslateImagePayload(ctx context.Context, imageURL string) (*translateImagePayload, error) {
	u := strings.TrimSpace(imageURL)
	if u == "" {
		return nil, fmt.Errorf("empty image url")
	}
	if strings.HasPrefix(strings.ToLower(u), "data:") {
		return payloadFromDataURL(u)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	cli := &http.Client{Timeout: 30 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download image HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 12<<20))
	if err != nil {
		return nil, err
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/jpeg"
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	return payloadFromImageBytes(data, ct, fmt.Sprintf("data:%s;base64,%s", ct, b64)), nil
}

func payloadFromDataURL(dataURL string) (*translateImagePayload, error) {
	dataURL = strings.TrimSpace(dataURL)
	comma := strings.Index(dataURL, ",")
	if comma < 0 {
		return nil, fmt.Errorf("invalid data url")
	}
	meta := strings.TrimSpace(dataURL[:comma])
	payloadPart := strings.TrimSpace(dataURL[comma+1:])
	if payloadPart == "" {
		return nil, fmt.Errorf("empty data url payload")
	}

	var data []byte
	var err error
	if strings.Contains(strings.ToLower(meta), ";base64") {
		data, err = base64.StdEncoding.DecodeString(payloadPart)
	} else {
		data = []byte(payloadPart)
	}
	if err != nil {
		return nil, fmt.Errorf("decode data url: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data url payload")
	}

	ct := "image/jpeg"
	if strings.HasPrefix(strings.ToLower(meta), "data:") {
		rest := meta[5:]
		if semi := strings.Index(rest, ";"); semi >= 0 {
			ct = strings.TrimSpace(rest[:semi])
		} else if rest != "" {
			ct = strings.TrimSpace(rest)
		}
	}
	if ct == "" || !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/jpeg"
	}
	return payloadFromImageBytes(data, ct, dataURL), nil
}

func payloadFromImageBytes(data []byte, contentType, dataURL string) *translateImagePayload {
	w, h := 0, 0
	if cfg, _, dErr := image.DecodeConfig(bytesReader(data)); dErr == nil {
		w, h = cfg.Width, cfg.Height
	}
	if strings.TrimSpace(dataURL) == "" {
		dataURL = fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data))
	}
	return &translateImagePayload{
		DataURL:  dataURL,
		RawBytes: data,
		Width:    w,
		Height:   h,
	}
}

func (s *Service) chatVisionJSON(ctx context.Context, prompt, imageDataURL string, maxTokens int) (string, error) {
	if s == nil || s.AIGateway == nil {
		return "", fmt.Errorf("ai gateway not configured")
	}
	buildReq := func(useJSON bool) aigate.ChatRequest {
		req := aigate.ChatRequest{
			Messages: []aigate.Message{{
				Role:      "user",
				Content:   prompt,
				ImageURLs: []string{imageDataURL},
			}},
			MaxTokens: maxTokens,
		}
		if useJSON {
			req.ResponseFormat = &aigate.ResponseFormat{Type: "json_object"}
		}
		return req
	}
	resp, err := s.AIGateway.Chat(ctx, buildReq(true))
	if err != nil {
		resp, err = s.AIGateway.Chat(ctx, buildReq(false))
		if err != nil {
			return "", err
		}
	}
	content := strings.TrimSpace(resp.Content)
	if content == "" {
		return "", fmt.Errorf("empty vision response")
	}
	return content, nil
}

func visionImageRefs(imageURL string, payload *translateImagePayload) []string {
	var refs []string
	if payload != nil && strings.TrimSpace(payload.DataURL) != "" {
		refs = append(refs, payload.DataURL)
	}
	u := strings.TrimSpace(imageURL)
	if u != "" && strings.HasPrefix(strings.ToLower(u), "http") {
		refs = append(refs, u)
	}
	return refs
}

func (s *Service) chatVisionJSONWithRefs(ctx context.Context, prompt string, imageRefs []string, maxTokens int) (string, error) {
	var lastContent string
	var lastErr error
	seen := map[string]bool{}
	for _, ref := range imageRefs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		content, err := s.chatVisionJSON(ctx, prompt, ref, maxTokens)
		if err != nil {
			lastErr = err
			continue
		}
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		lastContent = content
		if _, pErr := parseOCRJSON(content); pErr == nil {
			return content, nil
		}
	}
	if lastContent != "" {
		return lastContent, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("vision ocr failed for all image refs")
}

func normalizeOCRBlocks(blocks []translateTextBlock) []translateTextBlock {
	out := make([]translateTextBlock, 0, len(blocks))
	seen := map[string]bool{}
	for _, b := range blocks {
		text := strings.TrimSpace(b.Text)
		if text == "" {
			continue
		}
		key := text + fmt.Sprintf("|%d,%d", b.BBox.X, b.BBox.Y)
		if seen[key] {
			continue
		}
		seen[key] = true
		b.Text = text
		b.TranslatedText = strings.TrimSpace(b.TranslatedText)
		out = append(out, b)
	}
	return out
}

func mergeOCRBlocks(base, extra []translateTextBlock) []translateTextBlock {
	if len(extra) == 0 {
		return base
	}
	merged := append([]translateTextBlock{}, base...)
	for _, b := range extra {
		dup := false
		for _, ex := range merged {
			if strings.EqualFold(strings.TrimSpace(ex.Text), strings.TrimSpace(b.Text)) {
				dup = true
				break
			}
			if bboxOverlap(ex.BBox, b.BBox) && textSimilar(strings.TrimSpace(ex.Text), strings.TrimSpace(b.Text)) {
				dup = true
				break
			}
		}
		if !dup {
			merged = append(merged, b)
		}
	}
	return merged
}

func bboxOverlap(a, b translateTextBBox) bool {
	if a.Width <= 0 || a.Height <= 0 || b.Width <= 0 || b.Height <= 0 {
		return false
	}
	return a.X < b.X+b.Width && a.X+a.Width > b.X && a.Y < b.Y+b.Height && a.Y+a.Height > b.Y
}

func textSimilar(a, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

func (s *Service) supplementOCRBlocks(ctx context.Context, imageDataURL string, existing *translateOCRResult, sourceLang, targetLang string, imageW, imageH int) ([]translateTextBlock, error) {
	_, _, _, _, _ = ctx, imageDataURL, existing, sourceLang, targetLang
	_, _ = imageW, imageH
	// Disabled: supplement pass often hallucinates marketing text not on the image.
	return nil, nil
}

func (s *Service) runOCROnImage(ctx context.Context, imageURL, sourceLang, targetLang string, hints map[string]any) (*translateOCRResult, error) {
	if s == nil || s.Settings == nil {
		return nil, newTranslateErr(errCodeOCRFailed, "服务配置异常，无法读取设置")
	}

	m, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return nil, newTranslateErr(errCodeOCRFailed, "无法读取图片配置，请检查系统设置")
	}

	providerName := strings.TrimSpace(m["ocr_provider"])
	if providerName == "" {
		return nil, newTranslateErr("OCR_NOT_CONFIGURED", "图片文字翻译需要 OCR 服务。请先到「设置 → 图片 AI 设置」选择 OCR 服务并测试通过")
	}
	configuredProvider := providerName
	timeoutRaw := strings.TrimSpace(m["ocr_timeout_sec"])
	if timeoutRaw == "" {
		timeoutRaw = strings.TrimSpace(m["ocr_timeout_seconds"])
	}
	timeoutSec := comfyIntSetting(timeoutRaw, 60, 5, 300)
	minConfidenceRaw := strings.TrimSpace(m["ocr_min_confidence"])
	if minConfidenceRaw == "" {
		minConfidenceRaw = "0.55"
	}

	payload, loadErr := s.loadTranslateImagePayload(ctx, imageURL)
	if loadErr != nil || payload == nil {
		return nil, newTranslateErr(errCodeOCRFailed, "无法加载图片")
	}

	var ocrRes *translateOCRResult
	if providerName != "ai_vision" {
		var ocrErr error
		ocrRes, ocrErr = s.runExternalOCR(ctx, providerName, m, imageURL, payload, sourceLang, targetLang, timeoutSec)
		if ocrErr == nil && (ocrRes == nil || len(ocrRes.Blocks) == 0) {
			ocrErr = ocrerror.New(ocrerror.CodeEmptyBlocks, "OCR 未识别到文字，请更换图片或降低最低置信度")
		}
		if ocrErr != nil {
			failureCode, failureReason := classifyOCRError(ocrErr)
			taskCode := "OCR_DETECT_FAILED"
			switch failureCode {
			case ocrerror.CodeSecretMissing:
				taskCode = "OCR_PROVIDER_NOT_CONFIGURED"
			case ocrerror.CodeEmptyBlocks:
				taskCode = "OCR_EMPTY_BLOCKS"
			}
			return nil, newTranslateErr(taskCode, "当前 OCR 服务不可用，任务已停止。请先到「设置 → 图片 AI 设置」完成 OCR 配置并测试通过。"+failureCode+" / "+failureReason)
		}
	}

	if ocrRes == nil || len(ocrRes.Blocks) == 0 {
		if providerName != "ai_vision" {
			return nil, newTranslateErr("OCR_EMPTY_BLOCKS", "当前 OCR 服务未识别到文字，任务已停止。请更换图片或降低最低置信度")
		}
		if s.AIGateway == nil {
			return nil, newTranslateErr("OCR_PROVIDER_NOT_CONFIGURED", "当前 AI 视觉 OCR 未配置，请先到「设置 → AI 设置」配置可用视觉模型")
		}

		var lastVisionContent string
		var visionErr error

		prompt := ocrPromptBase(sourceLang, targetLang, payload.Width, payload.Height)
		refs := visionImageRefs(imageURL, payload)

		// Use a context with timeout for vision
		vCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()

		content, vErr := s.chatVisionJSONWithRefs(vCtx, prompt, refs, 2500)
		if vErr == nil {
			lastVisionContent = content
			ocrRes, visionErr = parseOCRJSON(content)
		} else {
			visionErr = vErr
		}

		if (ocrRes == nil || len(ocrRes.Blocks) == 0) && strings.TrimSpace(lastVisionContent) != "" {
			if parsed, pErr := parseOCRJSON(lastVisionContent); pErr == nil && len(parsed.Blocks) > 0 {
				ocrRes = parsed
			}
		}

		if (ocrRes == nil || len(ocrRes.Blocks) == 0) && payload != nil && strings.TrimSpace(payload.DataURL) != "" {
			targetName := langDisplayName(targetLang)
			srcHint := "auto-detect the source language"
			if sourceLang != "" && !strings.EqualFold(sourceLang, "auto") {
				srcHint = fmt.Sprintf("assume source language is %s", langDisplayName(sourceLang))
			}
			retryPrompt := fmt.Sprintf(`You are a strict literal OCR engine for ecommerce product images.
%s
%s
Return ONLY valid JSON with blocks containing text, translatedText in %s, confidence, bbox.
If no readable text exists, return {"detectedLanguage":"","textBlocksCount":0,"blocks":[]}.`, srcHint, ocrStrictRules(), targetName)
			content, vErr2 := s.chatVisionJSON(vCtx, retryPrompt, payload.DataURL, 2500)
			if vErr2 == nil {
				if parsed, pErr := parseOCRJSON(strings.TrimSpace(content)); pErr == nil {
					ocrRes = parsed
				} else if strings.TrimSpace(content) != "" {
					return nil, newTranslateErr(errCodeOCRFailed, "文字识别结果解析失败，请检查 AI 模型是否支持 JSON 输出并重试")
				}
			} else if visionErr == nil {
				visionErr = vErr2
			}
		}

		if ocrRes == nil || len(ocrRes.Blocks) == 0 {
			if visionErr != nil {
				return nil, newTranslateErr(errCodeOCRFailed, "文字识别失败，请确认 AI 设置已配置支持视觉的模型（如 qwen3-vl-plus 或 gpt-4o-mini）")
			}
			return nil, newTranslateErr(errCodeNoTextDetected, "未识别到可翻译文字，请确认图片中包含清晰可见的文字")
		}

		if ocrRes.Provider == "" {
			ocrRes.Provider = "ai_vision"
		}
	}

	if ocrRes != nil {
		ocrRes.Fallback = false
		ocrRes.ConfiguredProvider = configuredProvider
		if ocrRes.Provider == "" {
			ocrRes.Provider = providerName
		}
		ocrRes.ActualProvider = ocrRes.Provider
	}

	return ocrRes, nil
}

func (s *Service) runExternalOCR(ctx context.Context, providerName string, m map[string]string, imageURL string, payload *translateImagePayload, sourceLang, targetLang string, timeoutSec int) (*translateOCRResult, error) {
	prov, err := ocr.NewProvider(providerName, m)
	if err != nil {
		return nil, err
	}

	base64Data := ""
	if len(payload.RawBytes) > 0 {
		base64Data = base64.StdEncoding.EncodeToString(payload.RawBytes)
	}

	oCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	ocrURL := ""
	if base64Data == "" {
		ocrURL = firstNonEmptyString(imageURL, payload.DataURL)
	}
	res, err := prov.DetectText(oCtx, ocr.OCRRequest{
		ImageURL:          ocrURL,
		ImageBase64:       base64Data,
		SourceLanguage:    sourceLang,
		TargetLanguage:    targetLang,
		Languages:         []string{sourceLang, targetLang},
		DetectOrientation: true,
		ImageWidth:        payload.Width,
		ImageHeight:       payload.Height,
	})
	if err != nil {
		return nil, err
	}

	if res == nil || len(res.Blocks) == 0 {
		return &translateOCRResult{
			DetectedLanguage: "auto",
			TextBlocksCount:  0,
			Blocks:           []translateTextBlock{},
		}, nil
	}

	blocks := make([]translateTextBlock, 0, len(res.Blocks))
	confSum := 0.0
	confCount := 0
	for _, b := range res.Blocks {
		polygon := make([]translateTextPoint, 0, len(b.Polygon))
		for _, p := range b.Polygon {
			polygon = append(polygon, translateTextPoint{X: p.X, Y: p.Y})
		}
		if b.Confidence > 0 {
			confSum += b.Confidence
			confCount++
		}
		blocks = append(blocks, translateTextBlock{
			ID:         b.ID,
			Text:       b.Text,
			Confidence: b.Confidence,
			BBox: translateTextBBox{
				X:      b.BBox.X,
				Y:      b.BBox.Y,
				Width:  b.BBox.Width,
				Height: b.BBox.Height,
			},
			Polygon:   polygon,
			Angle:     b.Angle,
			Direction: b.Direction,
		})
	}
	avgConfidence := 0.0
	if confCount > 0 {
		avgConfidence = confSum / float64(confCount)
	}

	return &translateOCRResult{
		Provider:            res.Provider,
		APIName:             res.APIName,
		DetectedLanguage:    res.DetectedLanguage,
		TextBlocksCount:     len(blocks),
		FilteredBlocksCount: res.FilteredBlocksCount,
		AverageConfidence:   avgConfidence,
		Blocks:              blocks,
	}, nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func classifyOCRError(err error) (code, message string) {
	if err == nil {
		return "", ""
	}
	var oe *ocrerror.Error
	if errors.As(err, &oe) && oe != nil {
		code = strings.TrimSpace(oe.Code)
		message = strings.TrimSpace(oe.Message)
		if code == "" {
			code = ocrerror.CodeUnknown
		}
		if message == "" {
			message = err.Error()
		}
		return code, message
	}
	msg := strings.TrimSpace(err.Error())
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "i/o timeout"):
		return ocrerror.CodeTimeout, msg
	case strings.Contains(lower, "secret") && (strings.Contains(lower, "空") || strings.Contains(lower, "未配置") || strings.Contains(lower, "required")):
		return ocrerror.CodeSecretMissing, msg
	case strings.Contains(lower, "auth") || strings.Contains(lower, "signature") || strings.Contains(lower, "invalidaccesskey"):
		return ocrerror.CodeAuthFailed, msg
	case strings.Contains(lower, "permission") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden"):
		return ocrerror.CodePermissionDenied, msg
	case strings.Contains(lower, "not open") || strings.Contains(lower, "not enabled") || strings.Contains(lower, "未开通"):
		return ocrerror.CodeServiceNotOpen, msg
	case strings.Contains(lower, "image") && (strings.Contains(lower, "access") || strings.Contains(lower, "访问")):
		return ocrerror.CodeImageURLInaccessible, msg
	default:
		return ocrerror.CodeUnknown, msg
	}
}
