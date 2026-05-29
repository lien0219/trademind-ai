package imagetask

import (
	"context"
	"encoding/base64"
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
	w, h := 0, 0
	if cfg, _, dErr := image.DecodeConfig(bytesReader(data)); dErr == nil {
		w, h = cfg.Width, cfg.Height
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/jpeg"
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	return &translateImagePayload{
		DataURL:  fmt.Sprintf("data:%s;base64,%s", ct, b64),
		RawBytes: data,
		Width:    w,
		Height:   h,
	}, nil
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

func (s *Service) runOCROnImage(ctx context.Context, imageURL, sourceLang, targetLang string) (*translateOCRResult, error) {
	if s == nil || s.AIGateway == nil {
		return nil, newTranslateErr(errCodeOCRFailed, "未配置 AI 服务，无法进行文字识别（请在「设置 → AI」配置）")
	}

	payload, loadErr := s.loadTranslateImagePayload(ctx, imageURL)
	var ocr *translateOCRResult
	var lastVisionContent string
	var visionErr error

	if loadErr == nil && payload != nil {
		prompt := ocrPromptBase(sourceLang, targetLang, payload.Width, payload.Height)
		refs := visionImageRefs(imageURL, payload)
		content, vErr := s.chatVisionJSONWithRefs(ctx, prompt, refs, 2500)
		if vErr == nil {
			lastVisionContent = content
			ocr, visionErr = parseOCRJSON(content)
		} else {
			visionErr = vErr
		}
	}

	if (ocr == nil || len(ocr.Blocks) == 0) && strings.TrimSpace(lastVisionContent) != "" {
		if parsed, pErr := parseOCRJSON(lastVisionContent); pErr == nil && len(parsed.Blocks) > 0 {
			ocr = parsed
		}
	}

	if ocr == nil || len(ocr.Blocks) == 0 {
		// Last resort: text-only (may work if model can fetch public URL).
		targetName := langDisplayName(targetLang)
		srcHint := "auto-detect the source language"
		if sourceLang != "" && !strings.EqualFold(sourceLang, "auto") {
			srcHint = fmt.Sprintf("assume source language is %s", langDisplayName(sourceLang))
		}
		prompt := fmt.Sprintf(`You are a strict literal OCR engine for ecommerce product images.
Analyze the product image at this URL: %s
%s
%s
Return ONLY valid JSON with blocks containing text, translatedText in %s, confidence, bbox.
If no readable text exists, return {"detectedLanguage":"","textBlocksCount":0,"blocks":[]}.`, imageURL, srcHint, ocrStrictRules(), targetName)
		resp, fErr := s.AIGateway.Chat(ctx, aigate.ChatRequest{
			Messages:  []aigate.Message{{Role: "user", Content: prompt}},
			MaxTokens: 2000,
		})
		if fErr != nil {
			if visionErr != nil {
				return nil, newTranslateErr(errCodeOCRFailed, "文字识别失败，请确认 AI 设置已配置支持视觉的模型（如 qwen-vl-plus 或 gpt-4o-mini）")
			}
			return nil, newTranslateErr(errCodeOCRFailed, "文字识别失败，请稍后重试或更换图片")
		}
		var parseErr error
		ocr, parseErr = parseOCRJSON(strings.TrimSpace(resp.Content))
		if parseErr != nil {
			return nil, newTranslateErr(errCodeOCRFailed, "文字识别结果解析失败，请检查 AI 模型是否支持 JSON 输出并重试")
		}
	}

	if ocr == nil || len(ocr.Blocks) == 0 {
		return nil, newTranslateErr(errCodeNoTextDetected, "未识别到可翻译文字，请确认图片中包含清晰可见的文字")
	}
	return ocr, nil
}
