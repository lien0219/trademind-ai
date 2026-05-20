package collect

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// BatchStatsDTO aggregates derived counters and error-code histogram for a batch.
type BatchStatsDTO struct {
	RetryingCount    int            `json:"retryingCount"`
	BlockedCount     int            `json:"blockedCount"`
	TimeoutCount     int            `json:"timeoutCount"`
	ParseFailedCount int            `json:"parseFailedCount"`
	ErrorSummary     map[string]int `json:"errorSummary"`
}

func blockedErrorCodes() map[string]struct{} {
	return map[string]struct{}{
		"PAGE_BLOCKED_OR_VERIFY_REQUIRED": {},
		"PAGE_BLOCKED":                    {},
		"VERIFY_REQUIRED":                 {},
		"CAPTCHA":                         {},
	}
}

func timeoutErrorCodes() map[string]struct{} {
	return map[string]struct{}{
		"TIMEOUT":           {},
		"PAGE_TIMEOUT":      {},
		"PAGE_LOAD_TIMEOUT": {},
	}
}

func parseFailedErrorCodes() map[string]struct{} {
	return map[string]struct{}{
		"PARSE_FAILED": {},
	}
}

func collectorCodeFromPayload(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m["collectorCode"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.ToUpper(strings.TrimSpace(v))
	}
	return ""
}

func normalizeCollectorErrorCode(code, message string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	msg := strings.ToLower(strings.TrimSpace(message))
	if code != "" && code != "INVALID_URL" {
		return code
	}
	if strings.Contains(msg, "verification_challenge") ||
		strings.Contains(msg, "not_a_1688_offer_detail_page") ||
		strings.Contains(msg, "redirected_off_1688") ||
		strings.Contains(msg, "verification_or_login") ||
		strings.Contains(msg, "offer_path_lost") {
		return "PAGE_BLOCKED_OR_VERIFY_REQUIRED"
	}
	return code
}

func isCollectorCodeRetryable(code string, inBatch bool, policy BatchSourcePolicy) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	switch code {
	case "INVALID_URL", "INVALID_REQUEST", "PROVIDER_NOT_FOUND", "PROVIDER_NOT_IMPLEMENTED",
		"PROVIDER_NOT_AVAILABLE", "PRODUCT_NOT_FOUND", "UNSUPPORTED_URL",
		"LOGIN_REQUIRED", "CUSTOM_RULE_MISSING", "CUSTOM_RULE_INVALID",
		"PARSE_FAILED_TITLE_MISSING", "PARSE_FAILED_IMAGE_MISSING":
		return false
	case "PAGE_BLOCKED_OR_VERIFY_REQUIRED", "PAGE_BLOCKED", "VERIFY_REQUIRED", "CAPTCHA":
		if inBatch && policy.RetryOnBlocked {
			return true
		}
		return false
	case "TIMEOUT", "PAGE_TIMEOUT", "PAGE_LOAD_TIMEOUT", "NAVIGATION_FAILED":
		if inBatch && !policy.RetryOnTimeout {
			return false
		}
		return true
	case "COLLECT_FAILED", "PARSE_FAILED":
		return true
	default:
		return code == ""
	}
}

func collectFailureHint(code string, sameURLSucceeded bool) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if sameURLSucceeded {
		return "该链接单独采集成功，批量失败可能由并发、访问频率或目标站点风控导致。建议降低批量并发或稍后重试。"
	}
	switch code {
	case "LOGIN_REQUIRED":
		return "页面跳转到登录页。请创建采集浏览器 Profile，打开采集浏览器手动登录后，勾选 Profile 再测试规则或提交采集任务。"
	case "PROFILE_NOT_FOUND":
		return "采集浏览器 Profile 不存在或已停用，请重新选择或新建。"
	case "PROFILE_LOGIN_REQUIRED":
		return "所选 Profile 仍未通过登录检测，请打开采集浏览器登录后点击「重新检测登录态」。"
	case "CUSTOM_RULE_MISSING":
		return "未找到匹配的自定义采集规则，请先在「采集规则」创建并启用规则。"
	case "CUSTOM_RULE_INVALID":
		return "采集规则 JSON 无效，请检查 selector 与 type。"
	case "PAGE_BLOCKED_OR_VERIFY_REQUIRED", "PAGE_BLOCKED", "VERIFY_REQUIRED", "CAPTCHA":
		return "目标站点触发验证或风控；1688 链接请先在「设置 → 采集服务」完成登录；自定义采集请稍后重试或降低频率。"
	case "PARSE_FAILED_TITLE_MISSING":
		return "页面已打开但未提取到标题，请在采集规则中检查 title 选择器，或先用规则「测试」验证。"
	case "PARSE_FAILED_IMAGE_MISSING":
		return "页面已打开但未提取到图片，请检查 mainImage / detailImages 选择器，或启用 JSON-LD / OpenGraph fallback。"
	case "PARSE_FAILED":
		return "页面解析不完整，请查看任务详情中的 missingFields / extractDebug。"
	case "TIMEOUT", "PAGE_TIMEOUT", "PAGE_LOAD_TIMEOUT", "NAVIGATION_FAILED":
		return "页面加载超时或导航失败，建议重试或检查网络。"
	default:
		return ""
	}
}

func (s *Service) computeBatchStats(ctx context.Context, batchID uuid.UUID) BatchStatsDTO {
	out := BatchStatsDTO{ErrorSummary: map[string]int{}}
	if s == nil || s.DB == nil {
		return out
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var batch CollectBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return out
	}
	policy := s.batchPolicyForSource(ctx, batch.Source)

	var tasks []CollectTask
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ?", batchID).
		Find(&tasks).Error; err != nil {
		return out
	}

	blocked := blockedErrorCodes()
	timeouts := timeoutErrorCodes()
	parseFailed := parseFailedErrorCodes()

	for i := range tasks {
		t := &tasks[i]
		if t.Status == StatusRetrying {
			out.RetryingCount++
		}
		if t.Status != StatusFailed && t.Status != StatusRetrying {
			continue
		}
		code := s.lastCollectorErrorCode(ctx, t.ID)
		if code == "" {
			continue
		}
		out.ErrorSummary[code]++
		if _, ok := blocked[code]; ok {
			out.BlockedCount++
		}
		if _, ok := timeouts[code]; ok {
			out.TimeoutCount++
		}
		if _, ok := parseFailed[code]; ok {
			out.ParseFailedCount++
		}
		_ = policy
	}
	return out
}

func (s *Service) lastCollectorErrorCode(ctx context.Context, taskID uuid.UUID) string {
	if s == nil || s.DB == nil {
		return ""
	}
	var ev CollectTaskEvent
	err := s.DB.WithContext(ctx).
		Where("task_id = ? AND event_type IN ?", taskID, []string{
			EventTaskFailed,
			EventTaskRetryExhausted,
			EventTaskAutoRetryScheduled,
		}).
		Order("created_at DESC").
		Limit(1).
		Find(&ev).Error
	if err != nil || ev.ID == uuid.Nil {
		return ""
	}
	if code := collectorCodeFromPayload(json.RawMessage(ev.Payload)); code != "" {
		return normalizeCollectorErrorCode(code, ev.ErrorMessage)
	}
	return inferCodeFromMessage(ev.ErrorMessage)
}

func inferCodeFromMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	upper := strings.ToUpper(msg)
	for _, code := range []string{
		"PARSE_FAILED_TITLE_MISSING",
		"PARSE_FAILED_IMAGE_MISSING",
		"CUSTOM_RULE_MISSING",
		"CUSTOM_RULE_INVALID",
		"PAGE_BLOCKED_OR_VERIFY_REQUIRED",
		"NAVIGATION_FAILED",
		"PAGE_LOAD_TIMEOUT",
		"PAGE_TIMEOUT",
		"TIMEOUT",
		"PARSE_FAILED",
		"COLLECT_FAILED",
		"PRODUCT_NOT_FOUND",
		"UNSUPPORTED_URL",
		"INVALID_URL",
	} {
		if strings.Contains(upper, code) {
			if code == "INVALID_URL" && (strings.Contains(upper, "NOT_A_1688_OFFER") ||
				strings.Contains(upper, "VERIFICATION") ||
				strings.Contains(upper, "OFFER_PATH_LOST")) {
				return "PAGE_BLOCKED_OR_VERIFY_REQUIRED"
			}
			return code
		}
	}
	return ""
}

func (s *Service) sameURLCollectSucceeded(ctx context.Context, source, url string, excludeTaskID uuid.UUID) bool {
	if s == nil || s.DB == nil {
		return false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	url = strings.TrimSpace(url)
	source = strings.TrimSpace(source)
	if url == "" || source == "" {
		return false
	}
	var n int64
	_ = s.DB.WithContext(ctx).Model(&CollectTask{}).
		Where("source = ? AND source_url = ? AND status = ? AND id <> ?", source, url, StatusSuccess, excludeTaskID).
		Count(&n).Error
	return n > 0
}

func (s *Service) enrichTaskDTO(ctx context.Context, t *CollectTask) TaskDTO {
	dto := taskToDTO(t)
	if t == nil {
		return dto
	}
	if t.Status == StatusSuccess {
		return dto
	}
	inBatch := t.BatchID != nil
	policy := BatchSourcePolicy{}
	if inBatch {
		policy = s.batchPolicyForSource(ctx, t.Source)
	}
	code := s.lastCollectorErrorCode(ctx, t.ID)
	if code == "" && strings.TrimSpace(t.ErrorMessage) != "" && (t.Status == StatusFailed || t.Status == StatusRetrying) {
		code = inferCodeFromMessage(t.ErrorMessage)
	}
	code = normalizeCollectorErrorCode(code, t.ErrorMessage)
	dto.CollectorErrorCode = code
	retryable := isCollectorCodeRetryable(code, inBatch, policy)
	if code != "" || t.Status == StatusFailed || t.Status == StatusRetrying {
		dto.Retryable = &retryable
	}
	if t.Status == StatusFailed || t.Status == StatusRetrying {
		sameOK := s.sameURLCollectSucceeded(ctx, t.Source, t.SourceURL, t.ID)
		dto.SameUrlSucceededElsewhere = sameOK
		dto.FailureHint = collectFailureHint(code, sameOK)
	}
	return dto
}
