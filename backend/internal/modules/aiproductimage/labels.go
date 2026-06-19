package aiproductimage

import "strings"

func itemStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case ItemPending:
		return "等待处理"
	case ItemRunning:
		return "处理中"
	case ItemSuccess, ItemPendingReview:
		return "待复核"
	case ItemFailed:
		return "处理失败"
	case ItemApplied:
		return "已应用"
	case ItemRejected:
		return "已放弃"
	case ItemConflict:
		return "图片有冲突"
	case ItemCancelled:
		return "已取消"
	default:
		return status
	}
}

func batchStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case BatchPending:
		return "等待处理"
	case BatchRunning:
		return "处理中"
	case BatchSuccess:
		return "已完成"
	case BatchPartialSuccess:
		return "部分成功"
	case BatchFailed:
		return "失败"
	case BatchCancelled:
		return "已取消"
	default:
		return status
	}
}

// OperationTypeLabel returns user-facing label.
func OperationTypeLabel(op string) string {
	return operationTypeLabel(op)
}

func operationTypeLabel(op string) string {
	switch strings.TrimSpace(op) {
	case OpQualityCheck:
		return "图片质量检查"
	case OpRemoveWatermark:
		return "去水印"
	case OpRemoveLogo:
		return "去 Logo"
	case OpWhiteBackground:
		return "白底图"
	case OpOptimizeBackground:
		return "优化背景"
	case OpTranslateText:
		return "翻译图片文字"
	case OpSelectBestMain:
		return "主图优选建议"
	default:
		return op
	}
}

// ImageTypeLabel returns user-facing image type label.
func ImageTypeLabel(t string) string {
	return imageTypeLabel(t)
}

func imageTypeLabel(t string) string {
	switch strings.TrimSpace(t) {
	case "main":
		return "主图"
	case "detail":
		return "详情图"
	case "sku":
		return "规格图"
	case "marketing":
		return "营销图"
	case "ai_generated":
		return "AI 生成图"
	default:
		return "图片"
	}
}

func applyModeLabel(mode string) string {
	switch strings.TrimSpace(mode) {
	case ApplySetMain:
		return "设置为主图"
	case ApplyAddDetail:
		return "添加为详情图"
	case ApplyReplaceImage:
		return "替换原图片"
	case ApplySaveToGallery:
		return "保存到商品图库"
	default:
		return mode
	}
}

func checkStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "ready":
		return "可以处理"
	case "warning":
		return "可处理（有提醒）"
	case "blocked":
		return "不可处理"
	default:
		return status
	}
}
