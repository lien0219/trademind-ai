package aiopsworkbench

// TypeLabel returns Chinese label for todo type.
func TypeLabel(t string) string {
	switch t {
	case TodoTypeAITextReview:
		return "AI 文案待复核"
	case TodoTypeAITextConflict:
		return "AI 文案内容冲突"
	case TodoTypeAIImageReview:
		return "AI 图片待复核"
	case TodoTypeAIImageConflict:
		return "AI 图片有冲突"
	case TodoTypePublishCheckFailed:
		return "发布检查未通过"
	case TodoTypePublishCheckWarning:
		return "发布检查建议处理"
	case TodoTypePublishBatchFailed:
		return "刊登任务失败"
	case TodoTypePublishBatchPartial:
		return "刊登任务部分成功"
	case TodoTypeTaskCenterFailure:
		return "系统失败任务"
	default:
		return "待处理事项"
	}
}

// PriorityLabel returns Chinese label for priority.
func PriorityLabel(p string) string {
	switch p {
	case PriorityP0:
		return "紧急"
	case PriorityP1:
		return "阻断"
	case PriorityP2:
		return "建议处理"
	case PriorityP3:
		return "普通提醒"
	default:
		return p
	}
}
