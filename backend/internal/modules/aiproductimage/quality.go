package aiproductimage

import (
	"encoding/json"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
)

func warningsFromScoreJSON(raw json.RawMessage) []QualityWarning {
	if len(raw) == 0 {
		return nil
	}
	var sc imagetask.ImageScore
	if err := json.Unmarshal(raw, &sc); err != nil {
		return nil
	}
	var out []QualityWarning
	if sc.ClarityScore < 50 {
		out = append(out, QualityWarning{
			Code:    "low_resolution",
			Title:   "分辨率偏低",
			Message: "图片分辨率偏低，建议更换更清晰的图片。",
		})
	}
	if sc.MainSuitabilityScore < 50 {
		out = append(out, QualityWarning{
			Code:    "bad_aspect_ratio",
			Title:   "比例不适合主图",
			Message: "图片比例不适合主图，建议调整裁剪比例。",
		})
	}
	if sc.CleanlinessScore < 50 {
		out = append(out, QualityWarning{
			Code:    "cluttered_background",
			Title:   "背景杂乱",
			Message: "图片背景较杂乱，建议使用白底图提升展示效果。",
		})
	}
	if sc.CompositionScore < 50 {
		out = append(out, QualityWarning{
			Code:    "incomplete_subject",
			Title:   "主体不完整",
			Message: "商品主体可能不完整，建议更换更清晰的图片。",
		})
	}
	for _, issue := range sc.Issues {
		code := strings.ToLower(strings.TrimSpace(issue))
		switch {
		case strings.Contains(code, "watermark"):
			out = append(out, QualityWarning{Code: "watermark_suspected", Title: "疑似有水印", Message: "图片疑似含有水印，建议使用去水印处理。"})
		case strings.Contains(code, "logo"):
			out = append(out, QualityWarning{Code: "logo_suspected", Title: "疑似有 Logo", Message: "图片疑似含有 Logo，建议使用去 Logo 处理。"})
		case strings.Contains(code, "text"):
			out = append(out, QualityWarning{Code: "text_heavy", Title: "文字较多", Message: "图片文字较多，建议翻译或简化后再上架。"})
		}
	}
	return out
}

func checkImageQualityWarnings(publicURL string, accessible bool) []QualityWarning {
	var out []QualityWarning
	if !accessible {
		out = append(out, QualityWarning{
			Code:    "inaccessible",
			Title:   "无法访问",
			Message: "图片无法公开访问，请检查图片链接是否有效。",
		})
	}
	if strings.TrimSpace(publicURL) == "" {
		out = append(out, QualityWarning{
			Code:    "missing_url",
			Title:   "缺少链接",
			Message: "图片缺少有效链接，无法处理。",
		})
	}
	return out
}
