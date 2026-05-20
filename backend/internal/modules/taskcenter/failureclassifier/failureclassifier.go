package failureclassifier

import "strings"

// Mirrors taskcenter package constants without importing taskcenter (avoid cycles).
const (
	taskTypeCollect  = "collect"
	taskTypeImage    = "image"
	normLeaseExpired = "lease_expired"
)

// Failure categories (failure_category uniform enum).
const (
	CategoryPlatformAuth             = "platform_auth"
	CategoryPlatformPermission       = "platform_permission"
	CategoryPlatformRateLimit        = "platform_rate_limit"
	CategoryPlatformAPIError         = "platform_api_error"
	CategoryPlatformConfigIncomplete = "platform_config_incomplete"
	CategoryNetworkTimeout           = "network_timeout"
	CategoryCollectorBlocked         = "collector_blocked"
	CategoryCollectorPlatformLogin   = "collector_platform_login"
	CategoryLoginRequired            = "login_required"
	CategoryCollectorMissingImages   = "collector_missing_images"
	CategoryCollectorMissingPrice    = "collector_missing_price"
	CategoryCollectorEvaluateScript  = "collector_evaluate_script"
	CategoryCollectorInvalidURL      = "collector_invalid_url"
	CategoryAIProviderError          = "ai_provider_error"
	CategoryAIConfigIncomplete       = "ai_config_incomplete"
	CategoryImageProviderError       = "image_provider_error"
	CategoryStorageError             = "storage_error"
	CategoryValidationError          = "validation_error"
	CategoryInventoryMappingMissing  = "inventory_mapping_missing"
	CategorySKUMappingMissing        = "sku_mapping_missing"
	CategoryWorkerLeaseExpired       = "worker_lease_expired"
	CategorySystemError              = "system_error"
	CategoryUnknown                  = "unknown"
)

// Severity levels.
const (
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"
)

// Input is a minimal projection for rule matching (no secrets / no large JSON).
type Input struct {
	TaskType         string
	Platform         string
	NormalizedStatus string
	ErrorMessage     string
	ErrorCode        string
	Title            string
	RawSummary       string
}

// Result is the outcome of rule-based classification.
type Result struct {
	Category        string
	Severity        string
	Reason          string
	MatchedRule     string
	SuggestedAction string
}

type rule struct {
	id         string
	substrs    []string // any match (OR)
	category   string
	severity   string
	reason     string
	suggest    string
	taskTypes  []string // empty = all
	onlyIfNorm string   // empty = ignore; if set, NormalizedStatus must equal
}

func defaultSeverity(cat string) string {
	switch cat {
	case CategoryPlatformAuth, CategoryPlatformPermission, CategoryPlatformConfigIncomplete,
		CategoryInventoryMappingMissing, CategorySKUMappingMissing, CategoryStorageError:
		return SeverityHigh
	case CategoryPlatformRateLimit, CategoryNetworkTimeout, CategoryCollectorBlocked,
		CategoryCollectorPlatformLogin, CategoryLoginRequired,
		CategoryAIProviderError, CategoryImageProviderError, CategoryPlatformAPIError,
		CategoryWorkerLeaseExpired:
		return SeverityMedium
	case CategoryCollectorInvalidURL, CategoryValidationError, CategoryAIConfigIncomplete:
		return SeverityMedium
	case CategorySystemError:
		return SeverityHigh
	default:
		return SeverityLow
	}
}

var rules = []rule{
	// Strong signals from normalized status
	{
		id: "norm:lease_expired", substrs: nil, category: CategoryWorkerLeaseExpired,
		severity:   SeverityMedium,
		reason:     "任务租约已过期或 Worker 回收，需重新排队或检查 Worker 心跳。",
		suggest:    "请在「运维 → Worker 监控」确认实例与队列健康，必要时人工重试任务。",
		onlyIfNorm: normLeaseExpired,
	},
	// Platform auth
	{
		id: "sub:token_expired", substrs: []string{"token expired", "access token expired", "invalid access token", "unauthorized", "refresh token failed", "invalid refresh token"},
		category: CategoryPlatformAuth, severity: SeverityHigh,
		reason:  "鉴权失败或 Token 失效。",
		suggest: "请重新完成店铺授权或检查 Token 是否过期，并在平台开放后台确认应用状态。",
	},
	// Platform permission
	{
		id: "sub:permission", substrs: []string{"permission denied", "forbidden", "scope missing", "not authorized", "insufficient scope", "403"},
		category: CategoryPlatformPermission, severity: SeverityHigh,
		reason:  "平台返回权限不足或禁止访问。",
		suggest: "请确认已在平台开放后台申请对应权限并重新授权店铺。",
	},
	// Rate limit
	{
		id: "sub:rate_limit", substrs: []string{"429", "rate limit", "too many requests", "throttl"},
		category: CategoryPlatformRateLimit, severity: SeverityMedium,
		reason:  "平台限流或请求过于频繁。",
		suggest: "平台限流，建议稍后重试或降低同步频率。",
	},
	// Config incomplete
	{
		id: "sub:platform_config", substrs: []string{"platform config incomplete", "publish config incomplete", "missing warehouse_id", "missing marketplace_id", "missing fulfillment_channel", "config incomplete", "inventory config incomplete"},
		category: CategoryPlatformConfigIncomplete, severity: SeverityHigh,
		reason:  "平台或刊登相关配置不完整。",
		suggest: "请检查「设置 → 平台开放配置」或「设置 → 平台刊登配置」。",
	},
	// Inventory / SKU mapping
	{
		id: "sub:inv_mapping", substrs: []string{"product publication sku mapping incomplete", "external_sku_id missing", "external_product_id missing", "mapping incomplete"},
		category: CategoryInventoryMappingMissing, severity: SeverityHigh,
		reason:  "刊登或库存同步所需的平台 SKU 映射不完整。",
		suggest: "请检查商品刊登映射 product_publications / product_publication_skus。",
	},
	{
		id: "sub:sku_mapping", substrs: []string{"sku unmatched", "product_sku_id missing", "order item sku unmatched", "no matching sku"},
		category: CategorySKUMappingMissing, severity: SeverityHigh,
		reason:  "订单行与本地 SKU 未匹配。",
		suggest: "请在订单 SKU 匹配页面人工绑定本地 SKU。",
	},
	// Network
	{
		id: "sub:network", substrs: []string{"timeout", "deadline exceeded", "connection reset", "network unreachable", "i/o timeout", "context deadline", "eof", "connection refused"},
		category: CategoryNetworkTimeout, severity: SeverityMedium,
		reason:  "网络超时或连接异常。",
		suggest: "请检查本机与平台网络，稍后重试；持续出现需排查代理与防火墙。",
	},
	// Collector — page.evaluate 脚本注入失败
	{
		id: "sub:evaluate_script", substrs: []string{"__name is not defined", "evaluate_script_error"},
		category: CategoryCollectorEvaluateScript, severity: SeverityHigh,
		reason:    "采集脚本执行错误。",
		suggest:   "页面解析脚本注入失败，请检查 page.evaluate 中是否引用了构建工具 helper 或外部函数。",
		taskTypes: []string{taskTypeCollect},
	},
	// Collector — 1688 字段缺失（优先于 unknown）
	{
		id:       "sub:missing_main_images",
		substrs:  []string{"missing_main_images"},
		category: CategoryCollectorMissingImages, severity: SeverityMedium,
		reason:    "商品页已打开，但未提取到主图字段。",
		suggest:   "页面已打开，但未提取到主图。请打开原始快照确认页面是否正常显示主图；如果快照正常，请检查 1688 图片选择器或 JSON 提取逻辑。",
		taskTypes: []string{taskTypeCollect},
	},
	{
		id: "sub:no_main_images",
		substrs: []string{
			"no_main_images",
			"PARSE_FAILED_IMAGE_MISSING:no_main_images",
			"NO_MAIN_IMAGES_WARNING",
		},
		category: CategoryCollectorMissingImages, severity: SeverityMedium,
		reason:    "系统未识别到商品主图。",
		suggest:   "系统未识别到商品主图。请重试采集，或进入商品草稿后手动添加主图。",
		taskTypes: []string{taskTypeCollect},
	},
	{
		id:       "sub:missing_price",
		substrs:  []string{"missing_price", "missing_core_fields"},
		category: CategoryCollectorMissingPrice, severity: SeverityMedium,
		reason:    "商品页已解析，但价格字段缺失。",
		suggest:   "价格可能需要登录、权限或异步接口加载。请检查 1688 登录态和价格提取规则。",
		taskTypes: []string{taskTypeCollect},
	},
	{
		id: "sub:wechat_auth_pinduoduo",
		substrs: []string{
			"wechat_auth_required",
			"open.weixin.qq.com",
		},
		category:  CategoryLoginRequired,
		severity:  SeverityMedium,
		reason:    "拼多多登录需要微信扫码授权。",
		suggest:   "请打开拼多多采集浏览器，在弹出的微信授权页面完成扫码登录后，再重试采集任务。",
		taskTypes: []string{taskTypeCollect},
	},
	{
		id: "sub:app_redirect_pinduoduo",
		substrs: []string{
			"app_redirect",
		},
		category:  CategoryCollectorInvalidURL,
		severity:  SeverityMedium,
		reason:    "当前为 App 引导页，不是商品详情页。",
		suggest:   "请换用拼多多批发商品详情链接（pifa.pinduoduo.com/goods/detail/?gid=）。",
		taskTypes: []string{taskTypeCollect},
	},
	{
		id: "sub:unsupported_pinduoduo_url",
		substrs: []string{
			"unsupported_pinduoduo_url",
			"wholesale_homepage",
			"goods_detail",
		},
		category:  CategoryCollectorInvalidURL,
		severity:  SeverityMedium,
		reason:    "链接不是拼多多批发商品详情页，或当前版本暂未支持该链接类型。",
		suggest:   "请使用 pifa.pinduoduo.com/goods/detail/?gid= 链接；移动端商品页暂未完整支持。",
		taskTypes: []string{taskTypeCollect},
	},
	// 采集任务 — 需要登录（全平台；优先于 unknown）
	{
		id: "sub:login_required_collect",
		substrs: []string{
			"login_required",
			"profile_login_required",
			"page_login_required",
		},
		category:  CategoryLoginRequired,
		severity:  SeverityMedium,
		reason:    "页面需要登录后才能访问。",
		suggest:   "该页面需要登录后才能访问。请打开采集浏览器登录对应平台后重试，或换用公开商品详情页链接。",
		taskTypes: []string{taskTypeCollect},
	},
	// 自定义采集 — LOGIN_REQUIRED 错误码（非 1688 专属检测）
	{
		id:        "sub:collect_login_required",
		substrs:   []string{"login_required"},
		category:  CategoryCollectorPlatformLogin,
		severity:  SeverityMedium,
		reason:    "页面疑似需要登录后才能查看商品详情。",
		suggest:   "自定义链接采集器不做自动登录与账号密码保存。请确认该商品页是否公开可访问，或使用带登录态的采集浏览器（后续 Profile 扩展）。",
		taskTypes: []string{taskTypeCollect},
	},
	// 自定义采集器 — 非 1688 站点登录墙（须在 1688 规则之前，且仅 custom 源）
	{
		id: "sub:custom_collector_login_wall",
		substrs: []string{
			"verification_or_login_page_detected",
			"login_wall_detected",
			"login_or_verification_redirect",
			"verification_page_title",
			"empty_fields_likely_login_wall",
			"page_blocked_or_verify_required",
		},
		category:  CategoryCollectorBlocked,
		severity:  SeverityMedium,
		reason:    "自定义采集打开的是目标站点登录/验证页，不是商品详情。",
		suggest:   "当前为「自定义链接采集器」且目标站（如京东）需登录才能查看商品；系统未提供该站登录 Profile。请核对规则 CSS 选择器，或确认该链接是否必须在已登录浏览器中打开。",
		taskTypes: []string{taskTypeCollect},
	},
	// Collector — 1688 登录/验证失效（仅 1688 采集器或 custom+1688 链接）
	{
		id: "sub:1688_login_verify",
		substrs: []string{
			"verification_or_login_page_detected",
			"verification_challenge_or_offer_unreadable",
			"verification_or_login",
			"offer_path_lost_with_no_product_data",
			"captcha_redirect_url",
		},
		category:  CategoryCollectorPlatformLogin,
		severity:  SeverityHigh,
		reason:    "1688 采集浏览器未登录或登录态/安全验证已失效。",
		suggest:   "请在采集设置中打开 1688 采集浏览器，重新登录或完成安全验证后再重试。",
		taskTypes: []string{taskTypeCollect},
	},
	// Collector
	{
		id: "sub:collector_blocked", substrs: []string{"captcha", "verify required", "page_blocked_or_verify_required", "人机", "风控"},
		category: CategoryCollectorBlocked, severity: SeverityMedium,
		reason:    "采集目标可能触发验证或风控。",
		suggest:   "目标站点可能触发风控，请稍后重试或检查采集规则。",
		taskTypes: []string{taskTypeCollect},
	},
	{
		id: "sub:invalid_url", substrs: []string{"invalid_url", "invalid url", "non offer url"},
		category: CategoryCollectorInvalidURL, severity: SeverityLow,
		reason:    "链接无效或非商品详情页。",
		suggest:   "请核对采集链接是否为有效商品详情 URL。",
		taskTypes: []string{taskTypeCollect},
	},
	// AI / image (image tasks)
	{
		id: "sub:ai_config", substrs: []string{"api key", "missing api", "config incomplete", "no api key"},
		category: CategoryAIConfigIncomplete, severity: SeverityMedium,
		reason:    "AI 或图片服务配置不完整。",
		suggest:   "请检查「设置 → AI」或「设置 → 图片 AI」中的密钥与必填项。",
		taskTypes: []string{taskTypeImage},
	},
	{
		id: "sub:image_provider", substrs: []string{"removebg", "comfyui", "openai", "image", "workflow", "multipart"},
		category: CategoryImageProviderError, severity: SeverityMedium,
		reason:    "图片处理提供方返回错误。",
		suggest:   "请核对图片 AI 提供方配置、额度与源图可访问性。",
		taskTypes: []string{taskTypeImage},
	},
	// Storage
	{
		id: "sub:storage", substrs: []string{"storage", "s3", "cos", "oss", "upload failed", "put object", "bucket"},
		category: CategoryStorageError, severity: SeverityHigh,
		reason:  "对象存储读写失败。",
		suggest: "请检查「设置 → 存储」与 Bucket/密钥/网络。",
	},
	// Validation
	{
		id: "sub:validation", substrs: []string{"invalid request", "validation", "bad request", "malformed", "422"},
		category: CategoryValidationError, severity: SeverityLow,
		reason:  "请求参数或业务校验未通过。",
		suggest: "请根据错误摘要修正任务参数或关联数据后重试。",
	},
	// Generic platform API error (after specific rules)
	{
		id: "sub:platform_api", substrs: []string{"platform api", "sp-api", "api error", "500", "502", "503", "504", "internal error"},
		category: CategoryPlatformAPIError, severity: SeverityMedium,
		reason:  "平台接口返回错误或服务端异常。",
		suggest: "请稍后重试；若持续失败需对照平台状态与错误码排查。",
	},
	// System
	{
		id: "sub:system", substrs: []string{"internal server error", "database", "sql", "panic", "gorm"},
		category: CategorySystemError, severity: SeverityHigh,
		reason:  "系统内部异常。",
		suggest: "请查看服务日志与数据库健康；必要时联系运维。",
	},
	// Lease keywords in message
	{
		id: "sub:lease_msg", substrs: []string{"lease expired", "locked_until expired", "stale worker", "worker lease"},
		category: CategoryWorkerLeaseExpired, severity: SeverityMedium,
		reason:  "租约或 Worker 锁相关提示。",
		suggest: "请在「运维 → Worker 监控」检查任务租约与实例心跳，必要时人工重试。",
	},
}

func joinText(in Input) string {
	var b strings.Builder
	parts := []string{in.ErrorMessage, in.ErrorCode, in.Title, in.RawSummary}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			b.WriteString(" ")
			b.WriteString(p)
		}
	}
	return strings.ToLower(b.String())
}

func appliesToTaskTypes(r rule, tt string) bool {
	if len(r.taskTypes) == 0 {
		return true
	}
	for _, x := range r.taskTypes {
		if strings.EqualFold(x, tt) {
			return true
		}
	}
	return false
}

func anySubstr(text string, subs []string) bool {
	for _, s := range subs {
		if s != "" && strings.Contains(text, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

func is1688CollectContext(in Input) bool {
	text := joinText(in)
	if strings.Contains(text, "1688.com") {
		return true
	}
	pl := strings.TrimSpace(strings.ToLower(in.Platform))
	return pl == "1688"
}

func isPinduoduoCollectContext(in Input) bool {
	pl := strings.TrimSpace(strings.ToLower(in.Platform))
	if pl == "pinduoduo" || pl == "pdd" {
		return true
	}
	text := joinText(in)
	return strings.Contains(text, "pinduoduo.com") ||
		strings.Contains(text, "yangkeduo.com") ||
		strings.Contains(text, "pifa.pinduoduo.com")
}

func pinduoduoLoginSuggest(in Input) string {
	if !isPinduoduoCollectContext(in) {
		return ""
	}
	text := joinText(in)
	if strings.Contains(text, "open.weixin.qq.com") ||
		strings.Contains(text, "wechat_auth") ||
		strings.Contains(text, "微信扫码") {
		return "请打开拼多多采集浏览器，在弹出的微信授权页面完成扫码登录后，再重试采集任务。"
	}
	return "该页面需要登录后才能采集。请打开采集浏览器登录拼多多后重试，或换用普通商品详情页链接。"
}

func skipCollectFailureRule(rID string, in Input) bool {
	switch rID {
	case "sub:login_required_collect":
		return false
	case "sub:collect_login_required":
		if !strings.EqualFold(strings.TrimSpace(in.Platform), "custom") {
			return true
		}
		return is1688CollectContext(in)
	case "sub:custom_collector_login_wall":
		if !strings.EqualFold(strings.TrimSpace(in.Platform), "custom") {
			return true
		}
		return is1688CollectContext(in)
	case "sub:1688_login_verify":
		return !is1688CollectContext(in)
	default:
		return false
	}
}

// Classify applies ordered keyword rules plus status hints. No AI, no IO.
func Classify(in Input) Result {
	in.TaskType = strings.TrimSpace(strings.ToLower(in.TaskType))
	for _, r := range rules {
		if !appliesToTaskTypes(r, in.TaskType) {
			continue
		}
		if r.onlyIfNorm != "" && !strings.EqualFold(strings.TrimSpace(in.NormalizedStatus), r.onlyIfNorm) {
			continue
		}
		if len(r.substrs) == 0 && r.onlyIfNorm != "" && strings.EqualFold(strings.TrimSpace(in.NormalizedStatus), r.onlyIfNorm) {
			return Result{
				Category: r.category, Severity: r.severity, Reason: r.reason,
				MatchedRule: r.id, SuggestedAction: r.suggest,
			}
		}
		if len(r.substrs) == 0 {
			continue
		}
		if skipCollectFailureRule(r.id, in) {
			continue
		}
		text := joinText(in)
		if anySubstr(text, r.substrs) {
			sev := r.severity
			if sev == "" {
				sev = defaultSeverity(r.category)
			}
			suggest := r.suggest
			if r.id == "sub:login_required_collect" {
				if pdd := pinduoduoLoginSuggest(in); pdd != "" {
					suggest = pdd
				}
			}
			return Result{
				Category: r.category, Severity: sev, Reason: r.reason,
				MatchedRule: r.id, SuggestedAction: suggest,
			}
		}
	}
	// Fallback: image task residual -> image_provider_error
	if in.TaskType == taskTypeImage {
		return Result{
			Category: CategoryImageProviderError, Severity: SeverityMedium,
			Reason:          "未能匹配细分规则的图片任务失败。",
			MatchedRule:     "fallback:image_task",
			SuggestedAction: "请核对图片任务类型、Provider 配置与源图可读性后重试。",
		}
	}
	return Result{
		Category: CategoryUnknown, Severity: SeverityLow,
		Reason:          "暂无匹配规则，已归入未知类别。",
		MatchedRule:     "fallback:unknown",
		SuggestedAction: "请结合错误摘要与上游模块文档人工判断，必要时联系我们完善规则。",
	}
}
