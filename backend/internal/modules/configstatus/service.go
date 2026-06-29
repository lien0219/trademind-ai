package configstatus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	"github.com/trademind-ai/trademind/backend/internal/providers/image"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/gorm"
)

const (
	StatusConfigured         = "已配置"
	StatusNotConfigured      = "未配置"
	StatusConfigError        = "配置异常"
	StatusAwaitingCredential = "待真实凭证"
	StatusAwaitingPublicURL  = "待公网 Storage"
	StatusUnsupported        = "当前服务暂不支持"
	StatusDisabled           = "已关闭"
	StatusRunning            = "运行中"
	StatusAbnormal           = "异常"
)

// Item is one config health row for the status center.
type Item struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Summary     string `json:"summary,omitempty"`
	NextAction  string `json:"nextAction,omitempty"`
	SettingsURL string `json:"settingsUrl,omitempty"`
}

// Overview is GET /api/v1/settings/config-status.
type Overview struct {
	GeneratedAt string `json:"generatedAt"`
	Items       []Item `json:"items"`
	DemoData    Item   `json:"demoData"`
}

// Service aggregates configuration health.
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	Redis    *rdb.Client
	Config   *config.Config
	Shops    *shop.Service
}

// Handler serves config status HTTP API.
type Handler struct {
	Svc *Service
}

func (h *Handler) requireView(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "配置状态中心不可用")
		return false
	}
	return adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermSettingsManage)
}

// GetOverview GET /api/v1/settings/config-status
func (h *Handler) GetOverview(c *gin.Context) {
	if !h.requireView(c) {
		return
	}
	out, err := h.Svc.Build(c.Request.Context())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, out)
}

// Build returns config status center snapshot.
func (s *Service) Build(ctx context.Context) (*Overview, error) {
	if s == nil || s.Settings == nil {
		return nil, fmt.Errorf("configstatus: unavailable")
	}
	out := &Overview{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Items:       make([]Item, 0, 16),
	}
	out.Items = append(out.Items, s.aiTextItem(ctx))
	out.Items = append(out.Items, s.aiImageItem(ctx))
	out.Items = append(out.Items, s.ocrItem(ctx))
	out.Items = append(out.Items, s.storageItem(ctx))
	out.Items = append(out.Items, s.storagePublicItem(ctx))
	out.Items = append(out.Items, s.redisWorkerItem(ctx))
	out.Items = append(out.Items, s.collectorItem(ctx))
	out.Items = append(out.Items, s.douyinCredentialItem(ctx))
	out.Items = append(out.Items, s.platformPublishItem(ctx))
	out.Items = append(out.Items, s.orderSyncItem(ctx))
	out.Items = append(out.Items, s.inventorySyncItem(ctx))
	out.Items = append(out.Items, s.customerSyncItem(ctx))
	out.DemoData = s.demoDataItem(ctx)
	return out, nil
}

func (s *Service) aiTextItem(ctx context.Context) Item {
	it := Item{
		Key:         "ai_text_provider",
		Title:       "AI 文案 Provider",
		SettingsURL: "/settings/ai",
		NextAction:  "前往 AI 设置配置 Provider、Base URL 与 API Key",
	}
	ai, err := s.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		it.Status = StatusConfigError
		it.Summary = "读取 AI 配置失败"
		return it
	}
	pname := strings.TrimSpace(ai["provider"])
	if pname == "" {
		pname = "openai_compatible"
	}
	if aigate.ResolveProviderAPIKey(ai, pname) == "" || aigate.ResolveProviderBaseURL(ai, pname) == "" {
		it.Status = StatusNotConfigured
		it.Summary = "API Key 或 Base URL 未配置"
		return it
	}
	it.Status = StatusConfigured
	it.Summary = fmt.Sprintf("Provider=%s Model=%s", pname, aigate.ResolveProviderModel(ai, pname, ""))
	return it
}

func (s *Service) aiImageItem(ctx context.Context) Item {
	it := Item{
		Key:         "ai_image_provider",
		Title:       "AI 图片 Provider",
		SettingsURL: "/settings/image",
		NextAction:  "前往图片 AI 设置选择 Provider 并填写密钥",
	}
	img, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		it.Status = StatusConfigError
		it.Summary = "读取图片 AI 配置失败"
		return it
	}
	prov := strings.TrimSpace(img["provider"])
	if prov == "" {
		it.Status = StatusNotConfigured
		it.Summary = "未选择图片 Provider"
		return it
	}
	st := image.ConfigStatus(prov, img)
	switch st {
	case "configured", "ready":
		it.Status = StatusConfigured
		it.Summary = fmt.Sprintf("Provider=%s", prov)
	default:
		it.Status = StatusAwaitingCredential
		it.Summary = fmt.Sprintf("Provider=%s 待补全凭证", prov)
	}
	return it
}

func (s *Service) ocrItem(ctx context.Context) Item {
	it := Item{
		Key:         "ocr_provider",
		Title:       "OCR Provider",
		SettingsURL: "/settings/image",
		NextAction:  "在图片 AI 设置中配置 OCR Provider 凭证",
	}
	img, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		it.Status = StatusConfigError
		return it
	}
	ocrProv := strings.TrimSpace(img["ocr_provider"])
	if ocrProv == "" {
		it.Status = StatusNotConfigured
		it.Summary = "未选择 OCR Provider"
		return it
	}
	if strings.TrimSpace(img["ocr_api_key"]) == "" && strings.TrimSpace(img["ocr_access_key_id"]) == "" {
		it.Status = StatusAwaitingCredential
		it.Summary = fmt.Sprintf("OCR=%s 待填写密钥", ocrProv)
		return it
	}
	it.Status = StatusConfigured
	it.Summary = fmt.Sprintf("OCR=%s", ocrProv)
	return it
}

func (s *Service) storageItem(ctx context.Context) Item {
	it := Item{
		Key:         "storage",
		Title:       "Storage",
		SettingsURL: "/settings/storage",
		NextAction:  "配置存储 Provider；本地存储默认可用",
	}
	st, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		it.Status = StatusConfigError
		return it
	}
	kind := strings.TrimSpace(st["kind"])
	if kind == "" {
		kind = "local"
	}
	if kind == "local" {
		it.Status = StatusConfigured
		it.Summary = "本地存储已启用"
		return it
	}
	if settingsStorageConfigured(kind, st) {
		it.Status = StatusConfigured
		it.Summary = fmt.Sprintf("云存储 %s 凭证已填写", kind)
		return it
	}
	it.Status = StatusAwaitingCredential
	it.Summary = fmt.Sprintf("云存储 %s 凭证不完整", kind)
	return it
}

func settingsStorageConfigured(kind string, m map[string]string) bool {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "s3", "r2", "minio":
		return strings.TrimSpace(m["s3_bucket"]) != "" &&
			strings.TrimSpace(m["s3_access_key_id"]) != "" &&
			strings.TrimSpace(m["s3_secret_access_key"]) != ""
	case "cos":
		return strings.TrimSpace(m["cos_bucket"]) != "" &&
			strings.TrimSpace(m["cos_secret_id"]) != "" &&
			strings.TrimSpace(m["cos_secret_key"]) != ""
	case "oss":
		return strings.TrimSpace(m["oss_bucket"]) != "" &&
			strings.TrimSpace(m["oss_access_key_id"]) != "" &&
			strings.TrimSpace(m["oss_access_key_secret"]) != ""
	default:
		return false
	}
}

func (s *Service) storagePublicItem(ctx context.Context) Item {
	it := Item{
		Key:         "storage_public_access",
		Title:       "Storage 公网访问",
		SettingsURL: "/settings/storage",
		NextAction:  "在存储设置填写 public_base_url 或手动测试公网访问",
	}
	st, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		it.Status = StatusConfigError
		return it
	}
	pub := strings.TrimSpace(st["public_base_url"])
	if pub == "" {
		it.Status = StatusAwaitingPublicURL
		it.Summary = "未配置 public_base_url"
		return it
	}
	it.Status = StatusConfigured
	it.Summary = "已配置公网访问前缀（需人工点击测试确认）"
	return it
}

func (s *Service) redisWorkerItem(ctx context.Context) Item {
	it := Item{
		Key:         "redis_worker",
		Title:       "Redis / Worker",
		SettingsURL: "/workers/monitor",
	}
	redisOK := false
	if s.Redis != nil && s.Redis.Client != nil {
		redisOK = s.Redis.Client.Ping(ctx).Err() == nil
	}
	wb := worker.BuildHealthWorkersBlock(ctx, s.DB, s.Config)
	if redisOK && !wb.Degraded {
		it.Status = StatusRunning
		it.Summary = fmt.Sprintf("Redis 可用；Worker 运行 %d / 过期 %d", wb.Running, wb.Stale)
		return it
	}
	it.Status = StatusAbnormal
	parts := make([]string, 0, 2)
	if !redisOK {
		parts = append(parts, "Redis 不可用或未连接")
	}
	if wb.Degraded {
		parts = append(parts, "Worker 心跳异常")
	}
	it.Summary = strings.Join(parts, "；")
	it.NextAction = "检查 docker-compose Redis 与 Worker 进程"
	return it
}

func (s *Service) collectorItem(ctx context.Context) Item {
	it := Item{
		Key:         "collector",
		Title:       "采集服务",
		SettingsURL: "/settings/collector",
	}
	enabled := s.Config != nil && s.Config.CollectorBaseURL != ""
	if !enabled {
		it.Status = StatusNotConfigured
		it.Summary = "未配置采集服务地址"
		it.NextAction = "启动 collector 服务并配置 COLLECTOR_BASE_URL"
		return it
	}
	it.Status = StatusRunning
	it.Summary = fmt.Sprintf("采集服务地址 %s（健康检查见 /health）", strings.TrimSpace(s.Config.CollectorBaseURL))
	return it
}

func (s *Service) douyinCredentialItem(ctx context.Context) Item {
	it := Item{
		Key:         "douyin_credential",
		Title:       "抖店凭证",
		SettingsURL: "/settings/platforms",
		NextAction:  "配置抖店开放平台应用并完成店铺 OAuth 授权",
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "platform_douyinshop")
	if err != nil {
		it.Status = StatusConfigError
		return it
	}
	appKey := strings.TrimSpace(plain["app_key"])
	if appKey == "" {
		it.Status = StatusAwaitingCredential
		it.Summary = "抖店 App Key 未配置"
		return it
	}
	var authed int64
	if s.DB != nil {
		_ = s.DB.WithContext(ctx).Model(&shop.Shop{}).
			Where("platform = ? AND auth_status = ?", "douyinshop", "authorized").Count(&authed).Error
	}
	if authed > 0 {
		it.Status = StatusConfigured
		it.Summary = fmt.Sprintf("应用已配置；已授权店铺 %d 家", authed)
		return it
	}
	it.Status = StatusAwaitingCredential
	it.Summary = "应用已配置；尚无已授权抖店店铺"
	return it
}

func (s *Service) platformPublishItem(ctx context.Context) Item {
	it := Item{
		Key:         "platform_publish",
		Title:       "平台发布能力",
		SettingsURL: "/settings/platform-publish",
	}
	sys, err := s.Settings.PlainByGroup(ctx, 0, "system")
	if err != nil {
		it.Status = StatusConfigError
		return it
	}
	if strings.EqualFold(strings.TrimSpace(sys["product_publish_enabled"]), "false") {
		it.Status = StatusDisabled
		it.Summary = "平台刊登功能已关闭"
		return it
	}
	it.Status = StatusConfigured
	it.Summary = "平台刊登功能已开启（真实 E2E 需人工验收）"
	return it
}

func (s *Service) orderSyncItem(ctx context.Context) Item {
	it := Item{
		Key:         "order_sync",
		Title:       "订单同步开关",
		SettingsURL: "/settings/inventory",
	}
	enabled := s.Config != nil && s.Config.OrderSyncQueueEnabled
	if !enabled {
		it.Status = StatusDisabled
		it.Summary = "订单同步队列未启用"
		return it
	}
	block := ordersync.BuildOrderSyncQueueHealthBlock(ctx, s.Redis, enabled, "order:sync:tasks", 1)
	if block.RedisAvailable {
		it.Status = StatusRunning
		it.Summary = "订单同步队列已启用"
		return it
	}
	it.Status = StatusAbnormal
	it.Summary = "订单同步已启用但 Redis 队列不可用"
	return it
}

func (s *Service) inventorySyncItem(ctx context.Context) Item {
	it := Item{
		Key:         "inventory_sync",
		Title:       "库存同步开关",
		SettingsURL: "/settings/inventory",
	}
	inv, err := s.Settings.PlainByGroup(ctx, 0, "inventory")
	if err != nil {
		it.Status = StatusConfigError
		return it
	}
	if strings.EqualFold(strings.TrimSpace(inv["inventory_sync_enabled"]), "false") {
		it.Status = StatusDisabled
		it.Summary = "库存同步已关闭"
		return it
	}
	enabled := s.Config != nil && s.Config.InventorySyncQueueEnabled
	block := inventory.BuildInventorySyncQueueHealthBlock(ctx, s.Redis, enabled, "inventory:sync:tasks", 1)
	if enabled && block.RedisAvailable {
		it.Status = StatusRunning
		it.Summary = "库存同步队列运行中"
		return it
	}
	if !enabled {
		it.Status = StatusDisabled
		it.Summary = "库存同步 Worker 未启用"
		return it
	}
	it.Status = StatusAbnormal
	it.Summary = "库存同步队列异常"
	return it
}

func (s *Service) customerSyncItem(ctx context.Context) Item {
	it := Item{
		Key:         "customer_sync",
		Title:       "客服消息同步开关",
		SettingsURL: "/settings/integrations",
	}
	enabled := s.Config != nil && s.Config.CustomerMessageSyncQueueEnabled
	if !enabled {
		it.Status = StatusDisabled
		it.Summary = "客服消息同步未启用"
		return it
	}
	block := customersync.BuildCustomerMessageSyncQueueHealthBlock(ctx, s.Redis, enabled, "customer:message:sync:tasks", 1)
	if block.RedisAvailable {
		it.Status = StatusRunning
		it.Summary = "客服消息同步队列运行中"
		return it
	}
	it.Status = StatusAbnormal
	it.Summary = "客服同步已启用但 Redis 不可用"
	return it
}

func (s *Service) demoDataItem(ctx context.Context) Item {
	it := Item{
		Key:         "demo_data",
		Title:       "Demo 数据状态",
		SettingsURL: "/docs/DEMO_DATASET.md",
	}
	if s.DB == nil {
		it.Status = StatusAbnormal
		return it
	}
	var products, orders, convs int64
	_ = s.DB.WithContext(ctx).Table("products").Where("deleted_at IS NULL").Count(&products).Error
	_ = s.DB.WithContext(ctx).Table("orders").Where("deleted_at IS NULL").Count(&orders).Error
	_ = s.DB.WithContext(ctx).Table("customer_conversations").Count(&convs).Error
	if products > 5 && orders > 0 {
		it.Status = StatusConfigured
		it.Summary = fmt.Sprintf("商品 %d / 订单 %d / 会话 %d", products, orders, convs)
		return it
	}
	it.Status = StatusNotConfigured
	it.Summary = "Demo 种子数据较少，可运行 scripts/seed-demo-data"
	it.NextAction = "执行 seed-demo-data 脚本导入演示数据"
	return it
}
