package aiproducttext

import "strings"

func itemStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case ItemPending:
		return "等待处理"
	case ItemRunning:
		return "生成中"
	case ItemSuccess, ItemPendingReview:
		return "待复核"
	case ItemFailed:
		return "生成失败"
	case ItemApplied:
		return "已应用"
	case ItemRejected:
		return "已放弃"
	case ItemConflict:
		return "内容有冲突"
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

// OperationTypeLabel returns user-facing label for title/description ops.
func OperationTypeLabel(op string) string {
	return operationTypeLabel(op)
}

func operationTypeLabel(op string) string {
	switch strings.TrimSpace(op) {
	case OpTitle:
		return "商品标题"
	case OpDescription:
		return "商品描述"
	default:
		return op
	}
}

func checkStatusLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "ready":
		return "可以生成"
	case "warning":
		return "可生成（有提醒）"
	case "blocked":
		return "不可生成"
	default:
		return status
	}
}
