package settings

// IntegrationFieldSchema documents one settings item for admin UX (static registry).
type IntegrationFieldSchema struct {
	Name         string                    `json:"name"`
	Label        string                    `json:"label"`
	Type         string                    `json:"type"`
	Required     bool                      `json:"required"`
	Sensitive    bool                      `json:"sensitive"`
	Placeholder  string                    `json:"placeholder,omitempty"`
	Help         string                    `json:"help,omitempty"`
	DefaultValue any                       `json:"defaultValue,omitempty"`
	Options      []IntegrationSelectOption `json:"options,omitempty"`
}

// IntegrationSelectOption is a select option for IntegrationFieldSchema.
type IntegrationSelectOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// IntegrationConfigSchema groups third-party capabilities for documentation-driven admin forms.
type IntegrationConfigSchema struct {
	Key         string                   `json:"key"`
	Title       string                   `json:"title"`
	Description string                   `json:"description"`
	GroupKey    string                   `json:"groupKey"`
	Category    string                   `json:"category"`
	Fields      []IntegrationFieldSchema `json:"fields"`
}

// IntegrationConfigDefinitions returns the static integration registry (OpenAPI-style docs for settings groups).
func IntegrationConfigDefinitions() []IntegrationConfigSchema {
	return []IntegrationConfigSchema{
		{
			Key:         "ai",
			Title:       "AI 大模型（文本）",
			Category:    "ai",
			Description: "用于标题优化、描述生成、客服建议回复等。支持 OpenAI、OpenAI Compatible、DeepSeek、通义千问（Qwen）；请自行申请 API Key；贸灵不在仓库内置密钥，前端不直连模型，仅后端通过 AI Gateway 调用。",
			GroupKey:    "ai",
			Fields: []IntegrationFieldSchema{
				{Name: "provider", Label: "Provider 类型", Type: "select", Required: true, Options: []IntegrationSelectOption{
					{Label: "OpenAI", Value: "openai"},
					{Label: "OpenAI Compatible", Value: "openai_compatible"},
					{Label: "DeepSeek", Value: "deepseek"},
					{Label: "通义千问 / Qwen", Value: "qwen"},
				}},
				{Name: "base_url", Label: "Base URL", Type: "text", Required: true, Placeholder: "https://api.openai.com/v1", Help: "不含 /chat/completions"},
				{Name: "api_key", Label: "API Key", Type: "password", Required: true, Sensitive: true},
				{Name: "model", Label: "默认模型", Type: "text", Required: true, Placeholder: "gpt-4o-mini"},
				{Name: "temperature", Label: "Temperature", Type: "text", Required: false, Help: "可选，默认 0.7"},
				{Name: "max_tokens", Label: "Max tokens", Type: "text", Required: false, Help: "可选，默认 512"},
				{Name: "timeout_sec", Label: "超时（秒）", Type: "number", Required: true, DefaultValue: 120, Help: "单次 Chat 上限；标题/描述等大 max_tokens 时后端会自动抬高下限（≥120～180 秒）"},
				{Name: "ai_batch_enabled", Label: "启用批量 AI", Type: "text", Required: false, Help: "true / false；关闭后批量接口拒绝执行"},
				{Name: "ai_batch_max_size", Label: "单次批量最大商品数", Type: "text", Required: false, Help: "默认 100；上限 5000"},
				{Name: "ai_batch_concurrency", Label: "批量并发数", Type: "text", Required: false, Help: "文本 AI 并行调用数，1–16，默认 2"},
				{Name: "ai_batch_auto_save_ai_field", Label: "默认保存到 AI 字段", Type: "text", Required: false, Help: "true / false；未传 applyMode 时是否默认写入 ai_title / ai_description"},
			},
		},
		{
			Key:         "image",
			Title:       "图片 AI",
			Category:    "image",
			Description: "remove.bg、OpenAI Image、ComfyUI 等。remove.bg / OpenAI Image 需自行申请 Key；ComfyUI 需自行部署可访问服务并配置工作流。OpenAI Image 使用本分组 openai_image_*，不回退 settings.ai.api_key。",
			GroupKey:    "image",
			Fields: []IntegrationFieldSchema{
				{Name: "provider", Label: "默认 Provider", Type: "text", Required: false, Help: "noop / removebg / openai_image / comfyui"},
				{Name: "removebg_api_key", Label: "remove.bg API Key", Type: "password", Required: false, Sensitive: true},
				{Name: "removebg_base_url", Label: "remove.bg Base URL", Type: "text", Required: false},
				{Name: "openai_image_api_key", Label: "OpenAI Image API Key", Type: "password", Required: false, Sensitive: true},
				{Name: "openai_image_base_url", Label: "OpenAI Image Base URL", Type: "text", Required: false},
				{Name: "openai_image_model", Label: "OpenAI Image 模型", Type: "text", Required: false},
				{Name: "comfyui_base_url", Label: "ComfyUI Base URL", Type: "text", Required: false, Placeholder: "http://127.0.0.1:8188"},
				{Name: "comfyui_api_key", Label: "ComfyUI API Key（可选）", Type: "password", Required: false, Sensitive: true},
				{Name: "comfyui_workflow_json", Label: "ComfyUI Workflow JSON", Type: "textarea", Required: false, Help: "非密钥；请勿在操作日志全量落库"},
				{Name: "timeout_sec", Label: "通用超时（秒）", Type: "number", Required: false},
			},
		},
		{
			Key:         "storage",
			Title:       "对象存储",
			Category:    "storage",
			Description: "local / S3 兼容（含 R2、MinIO）/ 腾讯云 COS / 阿里云 OSS。AccessKey 与 Secret 加密保存；浏览器不经由前端 SDK 直传云端，上传走 POST /api/v1/files/upload 与后端探活 test-storage。",
			GroupKey:    "storage",
			Fields: []IntegrationFieldSchema{
				{Name: "kind", Label: "存储类型", Type: "select", Required: true, Options: []IntegrationSelectOption{
					{Label: "本地", Value: "local"}, {Label: "S3", Value: "s3"},
					{Label: "Cloudflare R2", Value: "r2"}, {Label: "MinIO", Value: "minio"},
					{Label: "腾讯云 COS", Value: "cos"}, {Label: "阿里云 OSS", Value: "oss"},
				}},
				{Name: "local_root", Label: "本地目录 local_root", Type: "text", Required: false},
				{Name: "public_base", Label: "公开 URL 前缀 public_base", Type: "text", Required: false},
				{Name: "s3_endpoint", Label: "S3 Endpoint", Type: "text", Required: false},
				{Name: "s3_access_key_id", Label: "S3 Access Key ID", Type: "password", Required: false, Sensitive: true},
				{Name: "s3_secret_access_key", Label: "S3 Secret", Type: "password", Required: false, Sensitive: true},
				{Name: "cos_secret_id", Label: "COS SecretId", Type: "password", Required: false, Sensitive: true},
				{Name: "cos_secret_key", Label: "COS SecretKey", Type: "password", Required: false, Sensitive: true},
				{Name: "oss_access_key_id", Label: "OSS AccessKeyId", Type: "password", Required: false, Sensitive: true},
				{Name: "oss_access_key_secret", Label: "OSS AccessKeySecret", Type: "password", Required: false, Sensitive: true},
			},
		},
		{
			Key:         "mail",
			Title:       "邮箱（SMTP）",
			Category:    "mail",
			Description: "注册验证码与系统通知。请自行准备企业邮箱、QQ/网易客户端授权码、云邮件推送或 SendGrid 等 SMTP。密钥加密保存；推荐使用分组 mail（兼容 legacy email）。",
			GroupKey:    "mail",
			Fields: []IntegrationFieldSchema{
				{Name: "provider", Label: "Provider", Type: "text", Required: false, DefaultValue: "smtp"},
				{Name: "smtp_host", Label: "SMTP Host", Type: "text", Required: true},
				{Name: "smtp_port", Label: "SMTP Port", Type: "number", Required: true},
				{Name: "smtp_username", Label: "SMTP Username", Type: "text", Required: false},
				{Name: "smtp_password", Label: "SMTP Password / 授权码", Type: "password", Required: false, Sensitive: true},
				{Name: "smtp_from", Label: "发件人邮箱", Type: "text", Required: true},
				{Name: "smtp_from_name", Label: "发件人名称", Type: "text", Required: false},
				{Name: "smtp_use_tls", Label: "STARTTLS", Type: "switch", Required: false},
				{Name: "smtp_use_ssl", Label: "SSL", Type: "switch", Required: false},
				{Name: "timeout_sec", Label: "超时（秒）", Type: "number", Required: false},
			},
		},
		{
			Key:         "alert_notify",
			Title:       "告警外部通知",
			Category:    "ops",
			Description: "任务告警的邮件 / Webhook / 飞书·企业微信（后两者首版预留）。Webhook URL / Secret 与机器人地址加密保存；SMTP 使用「邮件设置」。详见管理端「告警通知」页。",
			GroupKey:    "alert_notify",
			Fields: []IntegrationFieldSchema{
				{Name: "enabled", Label: "启用外部通知配置", Type: "switch", Required: false, Help: "与 taskcenter.enable_external_notifications 同时生效"},
				{Name: "channels", Label: "通道列表（JSON）", Type: "textarea", Required: false, Help: `如 ["mail","webhook"]`},
				{Name: "mail_enabled", Label: "邮件通知", Type: "switch", Required: false},
				{Name: "mail_to", Label: "收件人（逗号分隔）", Type: "text", Required: false},
				{Name: "mail_cc", Label: "抄送", Type: "text", Required: false},
				{Name: "mail_subject_prefix", Label: "主题前缀", Type: "text", Required: false},
				{Name: "webhook_enabled", Label: "Webhook", Type: "switch", Required: false},
				{Name: "webhook_url", Label: "Webhook URL", Type: "password", Required: false, Sensitive: true},
				{Name: "webhook_method", Label: "HTTP 方法", Type: "text", Required: false, Help: "留空则发送时默认 POST"},
				{Name: "webhook_secret", Label: "签名密钥（HMAC-SHA256）", Type: "password", Required: false, Sensitive: true},
				{Name: "webhook_timeout_seconds", Label: "HTTP 超时（秒）", Type: "number", Required: false, Help: "留空则使用后端安全默认值"},
				{Name: "webhook_template", Label: "模板（预留）", Type: "textarea", Required: false},
				{Name: "feishu_enabled", Label: "飞书（预留，planned）", Type: "switch", Required: false},
				{Name: "feishu_webhook_url", Label: "飞书 Webhook", Type: "password", Required: false, Sensitive: true},
				{Name: "feishu_secret", Label: "飞书 Secret", Type: "password", Required: false, Sensitive: true},
				{Name: "wecom_enabled", Label: "企业微信（预留，planned）", Type: "switch", Required: false},
				{Name: "wecom_webhook_url", Label: "企业微信 Webhook", Type: "password", Required: false, Sensitive: true},
			},
		},
		{
			Key:         "collect_rules",
			Title:       "自定义采集规则",
			Category:    "collector",
			Description: "声明式 CSS Selector / JSON-LD / OpenGraph / Meta，用于自定义链接采集。非第三方密钥；规则 JSON 有大小限制，不执行用户 JS。管理页：采集 → 采集规则。",
			GroupKey:    "",
			Fields:      nil,
		},
		{
			Key:         "inventory",
			Title:       "库存与订单",
			Category:    "inventory",
			Description: "订单扣减 / 取消回滚库存、平台同步后的自动扣库策略。密钥无关；全部为布尔开关字符串（true/false）。手动创建订单可在请求体勾选扣库；平台拉单仅在「自动扣平台订单库存」开启且订单符合条件时异步扣库。",
			GroupKey:    "inventory",
			Fields: []IntegrationFieldSchema{
				{Name: "auto_deduct_manual_orders", Label: "创建手工订单默认自动扣库", Type: "switch", Required: false, Help: "仅影响后台创建订单默认勾选；仍可在单笔创建时改写。", DefaultValue: false},
				{Name: "auto_deduct_platform_orders", Label: "平台同步订单自动扣库", Type: "switch", Required: false, Help: "仅对已付款等可履约状态的平台订单生效；同步失败不回滚本地库存。", DefaultValue: false},
				{Name: "auto_restore_cancelled_orders", Label: "订单取消 / 作废时自动回滚库存", Type: "switch", Required: false, DefaultValue: true},
				{Name: "auto_sync_platform_inventory_after_deduct", Label: "扣库后触发平台库存同步任务", Type: "switch", Required: false, Help: "依赖店铺刊登与 outbound 路由。", DefaultValue: false},
				{Name: "allow_negative_stock", Label: "允许 SKU 库存为负", Type: "switch", Required: false, DefaultValue: false},
				{Name: "inventory_sync_batch_max_size", Label: "单次批量库存同步最多创建任务数", Type: "number", Required: false, Help: "上限建议 ≤500。", DefaultValue: 500},
				{Name: "inventory_stock_settings_batch_max_size", Label: "单次批量设置预警线最多影响 SKU 数", Type: "number", Required: false, Help: "仅限制批量修改预警线/安全线，不影响库存同步批次大小。", DefaultValue: 500},
				{Name: "inventory_sync_platform_rate_limit_enabled", Label: "启用库存同步 Worker 基础节流（Redis）", Type: "switch", Required: false, DefaultValue: true},
				{Name: "inventory_sync_platform_rate_limit_per_minute_tiktok", Label: "TikTok 每分钟起始配额（计数近似）", Type: "number", Required: false, DefaultValue: 60},
				{Name: "inventory_sync_platform_rate_limit_per_minute_shopee", Label: "Shopee 每分钟起始配额", Type: "number", Required: false, DefaultValue: 60},
				{Name: "inventory_sync_platform_rate_limit_per_minute_lazada", Label: "Lazada 每分钟起始配额", Type: "number", Required: false, DefaultValue: 60},
				{Name: "inventory_sync_platform_rate_limit_per_minute_amazon", Label: "Amazon 每分钟起始配额", Type: "number", Required: false, DefaultValue: 30},
			},
		},
	}
}
