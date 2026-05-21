package image

import (
	"fmt"
	"strings"
)

// ProviderStatus describes rollout state for admin UX and task gating.
type ProviderStatus string

const (
	ProviderStatusAvailable ProviderStatus = "available"
	ProviderStatusBeta      ProviderStatus = "beta"
	ProviderStatusPlanned   ProviderStatus = "planned"
	ProviderStatusDisabled  ProviderStatus = "disabled"
)

// ProviderDifficulty is shown in admin settings.
type ProviderDifficulty string

const (
	DifficultyEasy     ProviderDifficulty = "easy"
	DifficultyMedium   ProviderDifficulty = "medium"
	DifficultyAdvanced ProviderDifficulty = "advanced"
)

// RegionFriendly hints domestic vs global access.
type RegionFriendly string

const (
	RegionGlobal RegionFriendly = "global"
	RegionChina  RegionFriendly = "china"
	RegionBoth   RegionFriendly = "both"
)

// ProviderCapability is the public catalog entry (no secrets).
type ProviderCapability struct {
	Provider           string             `json:"provider"`
	DisplayName        string             `json:"displayName"`
	Status             ProviderStatus     `json:"status"`
	Difficulty         ProviderDifficulty `json:"difficulty"`
	RegionFriendly     RegionFriendly     `json:"regionFriendly"`
	RequiresAPIKey     bool               `json:"requiresApiKey"`
	RequiresSelfHosted bool               `json:"requiresSelfHosted"`
	SupportedTasks     []string           `json:"supportedTasks"`
	RecommendedFor     []string           `json:"recommendedFor"`
	DocsURL            string             `json:"docsUrl"`
	Description        string             `json:"description,omitempty"`
}

// AllProviderCapabilities returns the static capability matrix for GET /api/v1/image/providers.
func AllProviderCapabilities() []ProviderCapability {
	return []ProviderCapability{
		{
			Provider: "noop", DisplayName: "占位 / 演示", Status: ProviderStatusAvailable,
			Difficulty: DifficultyEasy, RegionFriendly: RegionBoth,
			RequiresAPIKey: false, RequiresSelfHosted: false,
			SupportedTasks: []string{
				"remove_background", "replace_background", "generate_scene", "resize", "enhance",
				"remove_watermark", "remove_logo", "remove_badge", "remove_qrcode", "cleanup",
				"enhance_detail", "generate_marketing", "generate_main_image", "upscale",
				"score_image", "select_best_main",
			},
			RecommendedFor: []string{"演示", "联调", "评分测试"},
			Description:    "不调用外部服务；resize/enhance 回传原图；评分返回启发式结果。",
		},
		{
			Provider: "removebg", DisplayName: "remove.bg 去背景", Status: ProviderStatusAvailable,
			Difficulty: DifficultyEasy, RegionFriendly: RegionGlobal,
			RequiresAPIKey: true, RequiresSelfHosted: false,
			SupportedTasks: []string{"remove_background"},
			RecommendedFor: []string{"商品去背景", "白底图"},
			Description:    "专业抠图服务，适合商品白底图。",
		},
		{
			Provider: "openai_image", DisplayName: "OpenAI 图片生成", Status: ProviderStatusAvailable,
			Difficulty: DifficultyMedium, RegionFriendly: RegionGlobal,
			RequiresAPIKey: true, RequiresSelfHosted: false,
			SupportedTasks: []string{
				"generate_scene", "replace_background",
				"remove_watermark", "remove_logo", "remove_badge", "remove_qrcode", "cleanup",
				"enhance_detail", "generate_marketing", "generate_main_image", "upscale",
				"score_image", "select_best_main",
			},
			RecommendedFor: []string{"场景图", "替换背景", "图片清理", "营销图"},
			Description:    "OpenAI Images API 或兼容代理，支持去水印/去 Logo/营销图等编辑能力。",
		},
		{
			Provider: "comfyui", DisplayName: "ComfyUI 本地工作流", Status: ProviderStatusAvailable,
			Difficulty: DifficultyAdvanced, RegionFriendly: RegionBoth,
			RequiresAPIKey: false, RequiresSelfHosted: true,
			SupportedTasks: []string{
				"generate_scene", "replace_background",
				"remove_watermark", "remove_logo", "remove_badge", "remove_qrcode", "cleanup",
				"enhance_detail", "generate_marketing", "generate_main_image", "upscale",
			},
			RecommendedFor: []string{"高级自定义", "本地工作流", "图片清理"},
			Description:    "需自行部署 ComfyUI 并配置工作流 JSON。",
		},
		{
			Provider: "dashscope_image", DisplayName: "通义万相", Status: ProviderStatusAvailable,
			Difficulty: DifficultyEasy, RegionFriendly: RegionChina,
			RequiresAPIKey: true, RequiresSelfHosted: false,
			SupportedTasks: []string{"generate_scene"},
			RecommendedFor: []string{"国内场景图", "商品场景图"},
			Description:    "阿里云 DashScope 万相文生图。",
		},
		{
			Provider: "volcengine_image", DisplayName: "火山方舟", Status: ProviderStatusAvailable,
			Difficulty: DifficultyMedium, RegionFriendly: RegionChina,
			RequiresAPIKey: true, RequiresSelfHosted: false,
			SupportedTasks: []string{"generate_scene"},
			RecommendedFor: []string{"国内场景图"},
			Description:    "火山引擎 Ark 图像生成（OpenAI 兼容接口）。",
		},
		{
			Provider: "siliconflow_image", DisplayName: "硅基流动", Status: ProviderStatusBeta,
			Difficulty: DifficultyMedium, RegionFriendly: RegionChina,
			RequiresAPIKey: true, RequiresSelfHosted: false,
			SupportedTasks: []string{"generate_scene"},
			RecommendedFor: []string{"国内场景图", "多模型图像"},
			Description:    "SiliconFlow 图像生成 API。",
		},
		{
			Provider: "hunyuan_image", DisplayName: "腾讯混元", Status: ProviderStatusPlanned,
			Difficulty: DifficultyMedium, RegionFriendly: RegionChina,
			RequiresAPIKey: true, RequiresSelfHosted: false,
			SupportedTasks: []string{},
			RecommendedFor: []string{"后续版本"},
			Description:    "预留配置项，当前版本暂不支持真实调用。",
		},
	}
}

// CapabilityByProvider returns capability for name or nil.
func CapabilityByProvider(name string) *ProviderCapability {
	n := strings.TrimSpace(strings.ToLower(name))
	for i := range AllProviderCapabilities() {
		c := &AllProviderCapabilities()[i]
		if c.Provider == n {
			return c
		}
	}
	return nil
}

// SupportsTask reports whether provider can run taskType (respects planned/disabled).
func SupportsTask(providerName, taskType string) bool {
	cap := CapabilityByProvider(providerName)
	if cap == nil {
		return false
	}
	if cap.Status == ProviderStatusPlanned || cap.Status == ProviderStatusDisabled {
		return false
	}
	tt := strings.TrimSpace(strings.ToLower(taskType))
	for _, t := range cap.SupportedTasks {
		if t == tt {
			return true
		}
	}
	return false
}

// IsRunnableProvider is true when tasks may be created and executed.
func IsRunnableProvider(name string) bool {
	cap := CapabilityByProvider(name)
	if cap == nil {
		return false
	}
	return cap.Status == ProviderStatusAvailable || cap.Status == ProviderStatusBeta
}

// TaskTypeDisplayName maps task_type to Chinese label for errors.
func TaskTypeDisplayName(taskType string) string {
	switch strings.TrimSpace(strings.ToLower(taskType)) {
	case "remove_background":
		return "去背景"
	case "replace_background":
		return "替换背景"
	case "generate_scene":
		return "生成场景图"
	case "resize":
		return "缩放"
	case "enhance":
		return "增强"
	case "translate_image":
		return "图片翻译"
	case "poster_generate":
		return "海报生成"
	case "remove_watermark":
		return "去水印"
	case "remove_logo":
		return "去 Logo"
	case "remove_badge":
		return "去角标"
	case "remove_qrcode":
		return "去二维码"
	case "cleanup":
		return "综合清理"
	case "enhance_detail":
		return "详情图增强"
	case "generate_marketing":
		return "营销图生成"
	case "generate_main_image":
		return "主图生成"
	case "batch_generate_main":
		return "批量主图生成"
	case "upscale":
		return "高清修复"
	case "score_image":
		return "商品图评分"
	case "select_best_main":
		return "自动选最佳主图"
	default:
		return taskType
	}
}

// UnsupportedTaskError returns a user-facing Chinese error.
func UnsupportedTaskError(providerName, taskType string) error {
	cap := CapabilityByProvider(providerName)
	pn := providerName
	if cap != nil && cap.DisplayName != "" {
		pn = cap.DisplayName
	}
	if cap != nil && cap.Status == ProviderStatusPlanned {
		return fmt.Errorf("「%s」尚未开放，请选择其他图片 AI 服务或在后续版本再试", pn)
	}
	return fmt.Errorf("当前 Provider「%s」不支持「%s」，请更换图片 AI 服务或任务类型", pn, TaskTypeDisplayName(taskType))
}
