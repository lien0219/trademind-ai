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
		"PROVIDER_NOT_AVAILABLE", "PRODUCT_NOT_FOUND", "ITEM_NOT_FOUND", "UNSUPPORTED_URL", "UNSUPPORTED_PINDUODUO_URL",
		"UNSUPPORTED_TAOBAO_URL", "LOGIN_REQUIRED", "WECHAT_AUTH_REQUIRED", "APP_REDIRECT", "MAIN_IMAGES_EMPTY",
		"ACCESS_DENIED", "TITLE_NOT_FOUND",
		"CUSTOM_RULE_MISSING", "CUSTOM_RULE_INVALID",
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

func collectFailureHint(code, source string, sameURLSucceeded bool) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	src := strings.TrimSpace(strings.ToLower(source))
	isPdd := src == "pinduoduo" || src == "pdd"
	isTb := src == "taobao_tmall" || src == "taobao"
	if sameURLSucceeded {
		return "该链接单独采集成功，批量失败可能由并发、访问频率或目标站点风控导致。建议降低批量并发或稍后重试。"
	}
	switch code {
	case "LOGIN_REQUIRED":
		if isPdd {
			return "该页面需要登录后才能采集。请打开采集浏览器登录拼多多后重试，或换用公开商品详情页链接。"
		}
		if isTb {
			return "该淘宝/天猫商品页需要登录后才能采集。请打开淘宝/天猫采集浏览器完成登录后重试。"
		}
		return "该商品页需要登录后才能访问，请稍后重试或使用登录状态采集。"
	case "VERIFY_REQUIRED":
		if isTb {
			return "淘宝/天猫页面出现安全验证或滑块，请在采集浏览器中手动完成验证后重试。"
		}
		return "目标网站可能出现验证码或安全验证，请稍后重试，或在采集浏览器中手动完成验证。"
	case "ITEM_NOT_FOUND":
		return "商品不存在、已下架或链接无效。"
	case "MAIN_IMAGES_EMPTY":
		if isTb {
			return "未能识别到商品主图，请确认页面是否完整加载，或在登录/验证完成后重试。"
		}
		return "未能识别到商品主图，请重试采集或手动补充主图。"
	case "PRICE_NOT_FOUND":
		return "未能识别商品价格，草稿已创建，请发布前手动填写价格。"
	case "SKU_INCOMPLETE":
		return "商品规格识别不完整，草稿已创建，请发布前人工核对规格与库存。"
	case "DETAIL_IMAGES_INCOMPLETE":
		return "详情图可能未完全加载，草稿已创建，请发布前核对详情图片。"
	case "ACCESS_DENIED":
		return "页面访问被拒绝，请确认链接是否有效或是否需登录。"
	case "UNSUPPORTED_PINDUODUO_URL":
		if isPdd {
			return "当前链接不是拼多多批发商品详情页。请使用 pifa.pinduoduo.com/goods/detail/?gid= 链接；移动端商品页暂未完整支持。"
		}
		return "当前链接类型暂未支持。"
	case "UNSUPPORTED_TAOBAO_URL":
		if isTb {
			return "当前链接不是标准淘宝/天猫商品详情页，请复制商品详情页链接后重试。"
		}
		return "当前链接类型暂未支持。"
	case "TITLE_NOT_FOUND":
		return "未能识别商品标题，请检查链接是否为有效商品详情页。"
	case "WECHAT_AUTH_REQUIRED":
		return "拼多多登录需要微信扫码授权，请在采集浏览器中完成扫码后再重试。"
	case "APP_REDIRECT":
		return "当前为 App 引导页，请换用拼多多批发商品详情链接。"
	case "PRODUCT_NOT_FOUND":
		return "商品不存在、已下架或链接无效。"
	case "PROFILE_NOT_FOUND":
		return "登录状态不存在或已停用，请重新选择或新建。"
	case "PROFILE_LOGIN_REQUIRED":
		return "尚未完成登录，请打开采集浏览器登录后点击「重新检测登录状态」。"
	case "CUSTOM_RULE_MISSING":
		return "没有找到可用采集规则，请先创建采集规则，或使用「AI 帮我生成规则」。"
	case "CUSTOM_RULE_INVALID":
		return "采集规则内容有误，建议使用「AI 帮我生成规则」重新生成。"
	case "PAGE_BLOCKED_OR_VERIFY_REQUIRED", "PAGE_BLOCKED", "CAPTCHA":
		if isTb {
			return "淘宝/天猫页面出现安全验证或滑块，请在采集浏览器中手动完成验证后重试。"
		}
		return "目标网站可能出现验证码或安全验证，请稍后重试，或在采集浏览器中手动完成验证。"
	case "PARSE_FAILED_TITLE_MISSING":
		return "没有识别到商品标题，请检查规则或重新使用 AI 生成规则。"
	case "PARSE_FAILED_IMAGE_MISSING":
		if isPdd {
			return "系统未识别到商品主图。请重试采集，或进入商品草稿后手动添加主图。"
		}
		return "没有识别到商品图片，请检查主图规则后重新测试。"
	case "PARSE_FAILED":
		return "页面内容识别不完整，请在采集规则页测试采集效果后调整规则。"
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

// InferErrorCodeFromMessage extracts a collector error code from a failure message (exported for task center).
func InferErrorCodeFromMessage(msg string) string {
	return inferCodeFromMessage(msg)
}

func inferCodeFromMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	upper := strings.ToUpper(msg)
	for _, code := range []string{
		"LOGIN_REQUIRED",
		"PROFILE_LOGIN_REQUIRED",
		"UNSUPPORTED_PINDUODUO_URL",
		"UNSUPPORTED_TAOBAO_URL",
		"TITLE_NOT_FOUND",
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
		dto.FailureHint = collectFailureHint(code, t.Source, sameOK)
	}
	return dto
}
