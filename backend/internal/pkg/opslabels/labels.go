package opslabels

import "strings"

// Collect warning codes from collector / raw product data.
var collectWarningDefs = map[string]struct {
	Title   string
	Message string
}{
	"DETAIL_IMAGES_INCOMPLETE": {
		Title:   "详情图不完整",
		Message: "建议补充商品详情图，帮助买家了解商品细节。",
	},
	"ATTRIBUTES_EMPTY": {
		Title:   "商品参数未完善",
		Message: "未识别到商品参数，建议手动补充品牌、材质等信息。",
	},
	"STOCK_UNKNOWN": {
		Title:   "库存信息不明确",
		Message: "库存状态未知，发布前请人工确认各规格库存。",
	},
	"PRICE_NOT_FOUND": {
		Title:   "销售价格未设置",
		Message: "未识别到商品价格，请手动填写后再发布。",
	},
	"SKU_INCOMPLETE": {
		Title:   "规格信息不完整",
		Message: "商品规格识别不完整，请发布前人工核对规格与库存。",
	},
	"MAIN_IMAGES_EMPTY": {
		Title:   "商品主图缺失",
		Message: "采集结果没有可用主图，请上传或处理图片。",
	},
	"PRICE_MISSING": {
		Title:   "销售价格未设置",
		Message: "请为商品或规格填写有效销售价。",
	},
}

// Readiness / publish check codes (internal dotted codes and uppercase collect codes).
var readinessCodeDefs = map[string]struct {
	Title      string
	Message    string
	Suggestion string
}{
	"DETAIL_IMAGES_INCOMPLETE": {
		Title: "详情图不完整", Message: "建议补充商品详情图，帮助买家了解商品细节。",
		Suggestion: "请核对商品介绍区域图片，必要时手动补充。",
	},
	"ATTRIBUTES_EMPTY": {
		Title: "商品参数未完善", Message: "未识别到商品参数，建议手动补充。",
		Suggestion: "请补充品牌、材质、规格等参数。",
	},
	"STOCK_UNKNOWN": {
		Title: "库存信息不明确", Message: "库存状态未知，发布前请人工确认。",
		Suggestion: "请确认各规格库存后再创建刊登草稿。",
	},
	"PRICE_MISSING": {
		Title: "销售价格未设置", Message: "请为商品或规格填写有效销售价。",
		Suggestion: "前往规格页设置销售价格。",
	},
	"PRICE_INVALID": {
		Title: "销售价格不正确", Message: "销售价格无效或低于成本保护线。",
		Suggestion: "请检查定价规则与各规格售价。",
	},
	"PRICE_PROFIT_TOO_LOW": {
		Title: "预计利润率低于保护线", Message: "当前售价预计利润低于系统保护线。",
		Suggestion: "请调整售价或成本后再发布。",
	},
	"MAIN_IMAGES_EMPTY": {
		Title: "商品主图缺失", Message: "至少需要一张有效主图。",
		Suggestion: "请上传或同步主图到平台存储。",
	},
	"MAIN_IMAGES_NOT_UPLOADED": {
		Title: "主图还未同步到平台", Message: "主图仍为外链或未上传到目标平台。",
		Suggestion: "请先同步图片到平台存储或上传到抖店。",
	},
	"DESCRIPTION_EMPTY": {
		Title: "商品描述待完善", Message: "商品描述内容不足。",
		Suggestion: "请填写描述或生成 AI 描述后确认。",
	},
	"TITLE_EMPTY": {
		Title: "商品标题待完善", Message: "商品标题缺失或过短。",
		Suggestion: "请填写清晰的商品标题。",
	},
	"TITLE_TOO_LONG": {
		Title: "商品标题过长", Message: "标题超出平台允许长度。",
		Suggestion: "请缩短标题或应用 AI 优化标题。",
	},
	"SKU_INCOMPLETE": {
		Title: "规格信息不完整", Message: "规格、价格或库存信息不完整。",
		Suggestion: "请核对各规格名称、售价与库存。",
	},
	"SKU_PRICE_MISSING": {
		Title: "规格价格未设置", Message: "部分规格缺少有效售价。",
		Suggestion: "请为每个规格填写售价。",
	},
	"SKU_STOCK_MISSING": {
		Title: "规格库存未确认", Message: "部分规格库存未确认。",
		Suggestion: "请确认各规格库存数量。",
	},
	"CATEGORY_REQUIRED": {
		Title: "平台类目未选择", Message: "尚未选择目标平台商品类目。",
		Suggestion: "请在刊登配置中选择平台类目。",
	},
	"PLATFORM_ATTRIBUTES_REQUIRED": {
		Title: "平台必填属性未完善", Message: "平台要求的必填属性尚未填写完整。",
		Suggestion: "请补齐平台类目下的必填属性。",
	},
	"SHOP_NOT_AUTHORIZED": {
		Title: "店铺尚未授权", Message: "目标店铺未完成授权或授权已失效。",
		Suggestion: "请前往店铺管理完成授权。",
	},
	"PLATFORM_NOT_SUPPORTED": {
		Title: "当前平台暂未接入真实发布", Message: "该平台当前仅支持生成本地刊登草稿。",
		Suggestion: "可先创建本地草稿预览，待平台接入后再同步。",
	},
	"PUBLISH_CONFIG_MISSING": {
		Title: "刊登配置未完成", Message: "平台刊登预设或商品刊登配置尚未完成。",
		Suggestion: "请补齐平台刊登配置后再试。",
	},
	"product.title_missing": {
		Title: "商品标题缺失", Message: "商品标题缺失。",
		Suggestion: "请填写商品标题或应用 AI 标题。",
	},
	"collect.warning_requires_confirmation": {
		Title: "采集提示需检查", Message: "采集结果存在需人工确认的提示。",
		Suggestion: "请在商品详情确认采集提示后再发布。",
	},
}

// StatusLabels maps internal status tokens to user-facing Chinese.
var StatusLabels = map[string]string{
	"ready":           "已准备好",
	"draft":           "草稿",
	"draft_created":   "平台草稿已创建",
	"pending":         "等待处理",
	"running":         "处理中",
	"success":         "成功",
	"failed":          "失败",
	"partial_success": "部分成功",
	"warning":         "建议检查",
	"blocked":         "暂不能继续",
	"passed":          "检查通过",
	"error":           "错误",
	"cancelled":       "已取消",
	"checking":        "检查中",
	"publishing":      "刊登中",
}

// PublishCapabilityLabels for multi-platform publish center.
var PublishCapabilityLabels = map[string]string{
	"real_draft_create": "可创建平台草稿",
	"local_draft_only":  "仅生成本地草稿",
	"not_configured":    "尚未配置",
	"not_authorized":    "店铺未授权",
	"disabled":          "已停用",
}

// PlatformLabels cross-border platform display names.
var PlatformLabels = map[string]string{
	"douyin_shop": "抖店",
	"tiktok":      "TikTok Shop",
	"shopee":      "Shopee",
	"lazada":      "Lazada",
	"amazon":      "Amazon",
	"mock":        "模拟",
	"manual":      "手动",
}

// FieldLabels maps common English field names to Chinese.
var FieldLabels = map[string]string{
	"specs":          "规格",
	"main images":    "主图",
	"detail images":  "详情图",
	"external links": "外链图片",
	"platform":       "平台",
	"shop":           "店铺",
	"sku":            "规格编码",
	"stock":          "库存",
	"price":          "售价",
	"category":       "类目",
	"attributes":     "商品参数",
	"main_images":    "主图",
	"detail_images":  "详情图",
}

// LocalizedIssue is a user-facing check/issue item.
type LocalizedIssue struct {
	Code                string         `json:"code"`
	Title               string         `json:"title"`
	Message             string         `json:"message"`
	Severity            string         `json:"severity"`
	Suggestion          string         `json:"suggestion,omitempty"`
	TechnicalDetails    map[string]any `json:"technicalDetails,omitempty"`
	Group               string         `json:"group,omitempty"`
	RelatedResourceType string         `json:"relatedResourceType,omitempty"`
	RelatedResourceID   string         `json:"relatedResourceId,omitempty"`
}

// StatusLabel returns Chinese label for a status token with fallback.
func StatusLabel(status string) string {
	k := strings.TrimSpace(strings.ToLower(status))
	if k == "" {
		return "—"
	}
	if v, ok := StatusLabels[k]; ok {
		return v
	}
	if v, ok := StatusLabels[strings.ToUpper(status)]; ok {
		return v
	}
	return status
}

// PlatformLabel returns Chinese platform name.
func PlatformLabel(platform string) string {
	k := strings.TrimSpace(strings.ToLower(platform))
	if k == "" {
		return "—"
	}
	if v, ok := PlatformLabels[k]; ok {
		return v
	}
	return platform
}

// PublishCapabilityLabel returns user-facing capability text.
func PublishCapabilityLabel(cap string) string {
	k := strings.TrimSpace(cap)
	if v, ok := PublishCapabilityLabels[k]; ok {
		return v
	}
	return cap
}

// FieldLabel returns Chinese field label.
func FieldLabel(field string) string {
	k := strings.TrimSpace(strings.ToLower(field))
	if v, ok := FieldLabels[k]; ok {
		return v
	}
	return field
}

// LocalizeCollectWarning maps a raw collector warning code to title + message.
func LocalizeCollectWarning(raw string) (title, message string) {
	code := strings.TrimSpace(raw)
	upper := strings.ToUpper(code)
	if def, ok := collectWarningDefs[upper]; ok {
		return def.Title, def.Message
	}
	if def, ok := readinessCodeDefs[upper]; ok {
		msg := def.Message
		if msg == "" {
			msg = def.Title
		}
		return def.Title, msg
	}
	if looksLikeInternalCode(code) {
		return "采集提示需检查", "采集结果存在需人工确认的信息，请核对商品内容后再发布。"
	}
	return "采集提示需检查", code
}

// LocalizeReadinessIssue localizes one readiness check item.
func LocalizeReadinessIssue(code, level, message, suggestion, group, resType, resID string) LocalizedIssue {
	rawCode := strings.TrimSpace(code)
	upper := strings.ToUpper(rawCode)
	title := strings.TrimSpace(message)
	msg := strings.TrimSpace(message)
	sug := strings.TrimSpace(suggestion)
	severity := strings.TrimSpace(strings.ToLower(level))
	if severity == "failed" {
		severity = "error"
	}
	if severity == "" {
		severity = "warning"
	}

	if def, ok := readinessCodeDefs[rawCode]; ok {
		title = def.Title
		if msg == "" || msg == rawCode || looksLikeInternalCode(msg) {
			msg = def.Message
		}
		if sug == "" {
			sug = def.Suggestion
		}
	} else if def, ok := readinessCodeDefs[upper]; ok {
		title = def.Title
		if msg == "" || msg == rawCode || looksLikeInternalCode(msg) {
			msg = def.Message
		}
		if sug == "" {
			sug = def.Suggestion
		}
	} else if strings.HasPrefix(rawCode, "collect.") && (title == "" || looksLikeInternalCode(title)) {
		title = "采集提示需检查"
		if msg == "" {
			msg = "采集结果存在需人工确认的提示。"
		}
	} else if title == "" || looksLikeInternalCode(title) {
		title = fallbackTitle(rawCode)
		if msg == "" || looksLikeInternalCode(msg) {
			msg = title
		}
	}

	tech := map[string]any{}
	if rawCode != "" {
		tech["rawCode"] = rawCode
	}
	if strings.TrimSpace(message) != "" && strings.TrimSpace(message) != msg {
		tech["rawMessage"] = strings.TrimSpace(message)
	}

	return LocalizedIssue{
		Code:                rawCode,
		Title:               title,
		Message:             msg,
		Severity:            severity,
		Suggestion:          sug,
		TechnicalDetails:    tech,
		Group:               group,
		RelatedResourceType: resType,
		RelatedResourceID:   resID,
	}
}

func fallbackTitle(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "需要检查"
	}
	if looksLikeInternalCode(code) {
		return "需要检查"
	}
	return code
}

func looksLikeInternalCode(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, ".") {
		return true
	}
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r == '_' || r == '-' {
			continue
		}
		return false
	}
	return strings.ToUpper(s) == s && strings.Contains(s, "_")
}

// LocalizeCollectWarnings converts raw warning codes to localized display strings.
func LocalizeCollectWarnings(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, w := range raw {
		title, msg := LocalizeCollectWarning(w)
		line := title
		if msg != "" && msg != title {
			line = title + "：" + msg
		}
		key := strings.ToLower(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, line)
	}
	return out
}
