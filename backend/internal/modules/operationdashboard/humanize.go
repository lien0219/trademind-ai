package operationdashboard

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
)

// humanizeCollectorError maps collector error codes to user-facing Chinese labels.
func humanizeCollectorError(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	switch c {
	case "LOGIN_REQUIRED", "PROFILE_LOGIN_REQUIRED":
		return "需要登录后重试"
	case "WECHAT_AUTH_REQUIRED":
		return "需要微信扫码授权"
	case "PARSE_FAILED", "PARSE_FAILED_TITLE_MISSING":
		return "商品识别失败"
	case "PARSE_FAILED_IMAGE_MISSING", "NO_MAIN_IMAGES", "NO_MAIN_IMAGES_WARNING":
		return "缺少主图"
	case "PAGE_BLOCKED_OR_VERIFY_REQUIRED", "PAGE_BLOCKED", "VERIFY_REQUIRED", "CAPTCHA":
		return "页面需要验证"
	case "NAVIGATION_FAILED":
		return "页面无法打开"
	case "TIMEOUT", "PAGE_TIMEOUT", "PAGE_LOAD_TIMEOUT":
		return "页面加载超时"
	case "PRODUCT_NOT_FOUND":
		return "商品不存在或已下架"
	case "INVALID_URL", "UNSUPPORTED_PINDUODUO_URL":
		return "链接无效或暂不支持"
	case "APP_REDIRECT":
		return "当前为 App 引导页"
	default:
		if c != "" {
			return "采集失败"
		}
		return ""
	}
}

// humanizeTaskStatus maps internal task status to Chinese labels.
func humanizeTaskStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending":
		return "等待中"
	case "running":
		return "处理中"
	case "success":
		return "已完成"
	case "success_with_warnings":
		return "成功（有警告）"
	case "partial_success":
		return "部分成功"
	case "failed":
		return "失败"
	case "retrying":
		return "重试中"
	case "cancelled":
		return "已取消"
	default:
		if status == "" {
			return "—"
		}
		return status
	}
}

func severityToLevel(sev string) string {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical", "high":
		return "danger"
	case "medium":
		return "warning"
	default:
		return "info"
	}
}

func humanizeImageTaskType(taskType string) string {
	switch strings.TrimSpace(taskType) {
	case "remove_background":
		return "去背景"
	case "replace_background":
		return "换背景"
	case "generate_scene":
		return "场景图"
	case "remove_watermark":
		return "去水印"
	case "remove_logo":
		return "去 Logo"
	case "remove_badge":
		return "去角标/贴纸"
	case "remove_qrcode":
		return "去二维码"
	case "cleanup":
		return "综合清理"
	case "enhance_detail":
		return "详情图增强"
	case "upscale":
		return "高清修复"
	case "generate_marketing":
		return "营销图生成"
	case "generate_main_image":
		return "主图生成"
	case "batch_generate_main":
		return "批量主图生成"
	case "score_image":
		return "商品图评分"
	case "select_best_main":
		return "自动选最佳主图"
	case "translate_image_text":
		return "图片文字翻译"
	case "resize":
		return "缩放"
	case "enhance":
		return "增强"
	default:
		if taskType == "" {
			return "AI 图片任务"
		}
		return taskType
	}
}

func humanizeBatchOperationType(op string) string {
	switch strings.TrimSpace(strings.ToLower(op)) {
	case "title_optimize":
		return "批量标题优化"
	case "description_generate":
		return "批量描述生成"
	case "image_remove_background":
		return "批量去背景"
	case "image_generate_scene":
		return "批量场景图"
	case "image_replace_background":
		return "批量换背景"
	case "image_batch_generate_main":
		return "批量主图生成"
	case "image_score":
		return "批量图片评分"
	case "image_select_best_main":
		return "批量自动选主图"
	default:
		if op == "" {
			return ""
		}
		return op
	}
}

func humanizeProductSource(source string) string {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case "1688":
		return "1688"
	case "pinduoduo", "pdd":
		return "拼多多"
	case "taobao":
		return "淘宝"
	case "aliexpress":
		return "速卖通"
	case "custom":
		return "自定义链接"
	case "manual":
		return "手动创建"
	default:
		return strings.TrimSpace(source)
	}
}

var translateLayoutWarningLabels = map[string]string{
	"translated_text_too_long":   "部分翻译文字较长，可能影响图片排版，请检查结果图。",
	"translated_text_overflow":   "部分翻译文字较长，可能影响图片排版，请检查结果图。",
	"font_size_auto_adjusted":    "系统已自动调整部分文字大小。",
	"translated_text_simplified": "系统已自动精简部分翻译文案以适配排版。",
	"partial_text_detected":      "部分图片文字可能未全部识别，请检查结果图是否仍有未翻译文字。",
	"ocr_hallucination_filtered": "已过滤疑似非原图文字，仅翻译图片中真实可见的文字。",
	"source_text_may_remain":     "图片中可能仍有部分原文字，请检查结果图。",
	"IMAGE_NOT_CHANGED":          "生成图片没有变化，请重新生成或切换处理方式。",
	"IMAGE_TEXT_NOT_APPLIED":     "翻译文字没有成功写入图片，请重新生成。",
	"OUTPUT_TEXT_VERIFY_FAILED":  "系统无法确认文字替换效果，请人工检查图片。",
}

func humanizeTranslateWarningCode(code string) string {
	key := strings.TrimSpace(code)
	if key == "" {
		return ""
	}
	if label, ok := translateLayoutWarningLabels[key]; ok {
		return label
	}
	return key
}

func humanizeImageTaskSubtitle(taskType, status string, outputJSON []byte, errorMessage string) string {
	if status == imagetask.StatusFailed {
		return clip(errorMessage, 80)
	}
	if status != imagetask.StatusSuccessWithWarnings || taskType != imagetask.TaskTypeTranslateImageText {
		return ""
	}
	if len(outputJSON) == 0 {
		return ""
	}
	var out map[string]any
	if err := json.Unmarshal(outputJSON, &out); err != nil {
		return ""
	}
	quality, _ := out["quality"].(map[string]any)
	if quality == nil {
		return ""
	}
	seen := map[string]bool{}
	addFirst := func(msg string) string {
		msg = strings.TrimSpace(msg)
		if msg == "" || seen[msg] {
			return ""
		}
		seen[msg] = true
		return clip(msg, 80)
	}
	if warnings, ok := quality["warnings"].([]any); ok {
		for _, w := range warnings {
			if s := addFirst(fmt.Sprint(w)); s != "" {
				return s
			}
		}
	}
	layout, _ := quality["layout"].(map[string]any)
	if layout != nil {
		if warnings, ok := layout["warnings"].([]any); ok {
			for _, w := range warnings {
				if s := addFirst(humanizeTranslateWarningCode(fmt.Sprint(w))); s != "" {
					return s
				}
			}
		}
	}
	return ""
}
