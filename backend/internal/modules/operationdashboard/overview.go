package operationdashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/configstatus"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
)

// OverviewSection is one domain bucket in GET /dashboard/overview.
type OverviewSection struct {
	Key       string         `json:"key"`
	Title     string         `json:"title"`
	Count     int64          `json:"count"`
	Status    string         `json:"status"`
	Priority  string         `json:"priority"`
	Link      string         `json:"link"`
	EmptyHint string         `json:"emptyHint,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// OverviewDTO is GET /api/v1/dashboard/overview.
type OverviewDTO struct {
	GeneratedAt string            `json:"generatedAt"`
	Sections    map[string]any    `json:"sections"`
	Cards       []OverviewSection `json:"cards"`
}

// UnifiedTodo is one actionable item for GET /dashboard/todos.
type UnifiedTodo struct {
	Type        string     `json:"type"`
	Priority    string     `json:"priority"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Module      string     `json:"module"`
	ShopID      string     `json:"shopId,omitempty"`
	Link        string     `json:"link"`
	Count       int64      `json:"count"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}

// TodosDTO is GET /api/v1/dashboard/todos.
type TodosDTO struct {
	GeneratedAt string        `json:"generatedAt"`
	Items       []UnifiedTodo `json:"items"`
}

// HealthDTO is GET /api/v1/dashboard/health.
type HealthDTO struct {
	GeneratedAt string                         `json:"generatedAt"`
	Modules     []HealthModule                 `json:"modules"`
	Config      *configstatus.DashboardSummary `json:"config,omitempty"`
}

// HealthModule is one subsystem health row.
type HealthModule struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Summary string `json:"summary,omitempty"`
	Link    string `json:"link,omitempty"`
}

// GetOverview returns modular overview cards with RBAC-scoped counts.
func (s *Service) GetOverview(ctx context.Context, q Query, sc Scope) (*OverviewDTO, error) {
	dash, err := s.GetProductOperationDashboard(ctx, q, sc)
	if err != nil {
		return nil, err
	}
	sum := dash.Summary
	cards := []OverviewSection{
		overviewCard("collect_today", "今日采集任务", sum.CollectFailedCount, sumStatus(sum.CollectFailedCount), priorityFromCount(sum.CollectFailedCount, "P1"), "/collect/tasks", "暂无采集任务，可前往采集中心添加链接"),
		overviewCard("product_drafts", "商品草稿", sum.DraftTotal, "normal", "P2", "/product/drafts", "还没有商品草稿，可先采集或手动创建"),
		overviewCard("ai_pending_review", "AI 待复核", sum.AiPendingProducts+sum.AiReplySuggestionPendingCount, sumStatus(sum.AiPendingProducts+sum.AiReplySuggestionPendingCount), priorityFromCount(sum.AiPendingProducts, "P1"), "/ai/operation-workbench", "暂无待复核 AI 文案或图片"),
		overviewCard("readiness_blocked", "发布检查问题", sum.ReadinessBlocked, sumStatus(sum.ReadinessBlocked), priorityFromCount(sum.ReadinessBlocked, "P1"), "/product/drafts?readiness=blocked", "发布检查均通过"),
		overviewCard("publish_failed", "刊登任务异常", sum.PublishFailedTasks, sumStatus(sum.PublishFailedTasks), priorityFromCount(sum.PublishFailedTasks, "P0"), "/product/publish-tasks?status=failed", "暂无刊登异常"),
		overviewCard("order_exceptions", "订单异常", sum.OrderExceptionTotal, sumStatus(sum.OrderExceptionTotal), priorityFromCount(sum.OrderExceptionTotal, "P0"), "/orders/exceptions", "暂无订单异常"),
		overviewCard("inventory_alerts", "库存异常", sum.LowStockSkus+sum.OutOfStockSkus+sum.InventorySyncFailedCount, sumStatus(sum.LowStockSkus+sum.OutOfStockSkus+sum.InventorySyncFailedCount), priorityFromCount(sum.InventorySyncFailedCount, "P1"), "/inventory/alerts", "库存状态正常"),
		overviewCard("customer_pending", "客服待回复", sum.CustomerPendingReplyCount, sumStatus(sum.CustomerPendingReplyCount), priorityFromCount(sum.CustomerPendingReplyCount, "P1"), "/customer/conversations?status=pending_reply", "暂无待回复会话"),
		overviewCard("failed_tasks", "失败任务", sum.FailedTaskTotal, sumStatus(sum.FailedTaskTotal), priorityFromCount(sum.FailedTaskTotal, "P0"), "/ops/task-center/failures", "暂无失败任务"),
	}
	var cfgSum *configstatus.DashboardSummary
	if s.ConfigStatus != nil {
		if cs, err := s.ConfigStatus.DashboardSummary(ctx); err == nil {
			cfgSum = cs
			cfgCount := int64(0)
			if cs != nil {
				cfgCount = int64(cs.RiskCount)
			}
			cards = append(cards, overviewCard("config_risk", "配置风险", cfgCount, sumStatus(cfgCount), priorityFromCount(cfgCount, "P1"), "/settings/config-status", "核心配置已完成"))
		}
	}
	sections := map[string]any{
		"collect":      compactSection(sum.CollectFailedCount, sum.TodayNewProducts),
		"products":     compactSection(sum.DraftTotal, sum.Publishable),
		"ai":           compactSection(sum.AiPendingProducts, sum.ImageTaskPending),
		"publish":      compactSection(sum.PublishFailedTasks, sum.PublishPendingTasks),
		"orders":       compactSection(sum.OrderExceptionTotal, sum.SKUUnmatchedOrderItems),
		"inventory":    compactSection(sum.LowStockSkus+sum.OutOfStockSkus, sum.InventorySyncFailedCount),
		"customer":     compactSection(sum.CustomerPendingReplyCount, sum.AiReplySuggestionPendingCount),
		"taskCenter":   compactSection(sum.FailedTaskTotal, sum.OpenAlertCount),
		"configStatus": cfgSum,
	}
	return &OverviewDTO{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Sections:    sections,
		Cards:       cards,
	}, nil
}

// GetTodos returns unified prioritized todo stream.
func (s *Service) GetTodos(ctx context.Context, q Query, sc Scope) (*TodosDTO, error) {
	dash, err := s.GetProductOperationDashboard(ctx, q, sc)
	if err != nil {
		return nil, err
	}
	sum := dash.Summary
	items := []UnifiedTodo{
		unifiedTodo("collect_failed", "P0", "采集失败", "商品链接采集未成功，可重试", "collect", "", "/collect/tasks?status=failed", sum.CollectFailedCount),
		unifiedTodo("missing_ai_title", "P1", "商品待补齐 · AI 标题", "这些商品还没有 AI 标题", "product", "", "/product/drafts?missingAiTitle=1", sum.MissingAiTitleCount),
		unifiedTodo("missing_ai_description", "P1", "商品待补齐 · AI 描述", "这些商品还没有 AI 描述", "product", "", "/product/drafts?missingAiDescription=1", sum.MissingAiDescriptionCount),
		unifiedTodo("ai_text_review", "P1", "AI 文案待复核", "批量文案任务有待人工确认项", "ai", "", "/ai/text-batches?status=pending_review", sum.AiBatchRunningCount),
		unifiedTodo("ai_image_review", "P1", "AI 图片待复核", "图片批次或任务待确认", "ai", "", "/ai/image-batches?status=pending_review", sum.ImageTaskPending),
		unifiedTodo("readiness_blocked", "P1", "发布检查未通过", "缺标题、主图、规格或价格等", "product", "", "/product/drafts?readiness=blocked", sum.ReadinessBlocked),
		unifiedTodo("publish_failed", "P0", "刊登失败", "刊登到平台时出错", "publish", "", "/product/publish-tasks?status=failed", sum.PublishFailedTasks),
		unifiedTodo("order_sku_unmatched", "P0", "订单 SKU 未匹配", "平台订单行尚未绑定本地 SKU", "order", "", "/orders/exceptions?type=sku_unmatched", sum.SKUUnmatchedOrderItems),
		unifiedTodo("inventory_sync_failed", "P1", "库存同步失败", "平台库存同步未成功", "inventory", "", "/inventory/sync-tasks?status=failed", sum.InventorySyncFailedCount),
		unifiedTodo("low_stock", "P1", "低库存 / 零库存", "建议补货或调整预警线", "inventory", "", "/inventory/alerts", sum.LowStockSkus+sum.OutOfStockSkus),
		unifiedTodo("customer_pending", "P1", "客服待回复", "买家消息等待人工处理", "customer", "", "/customer/conversations?status=pending_reply", sum.CustomerPendingReplyCount),
		unifiedTodo("customer_send_failed", "P0", "客服发送失败", "回复发送失败需重试", "customer", "", "/customer/conversations?status=open", 0),
		unifiedTodo("failed_tasks", "P0", "失败任务待处理", "统一失败任务中心有待处理项", "taskCenter", "", "/ops/task-center/failures", sum.FailedTaskTotal),
	}
	if s.ConfigStatus != nil {
		if cs, err := s.ConfigStatus.DashboardSummary(ctx); err == nil && cs != nil && cs.RiskCount > 0 {
			items = append(items, unifiedTodo("config_incomplete", "P1", "配置未完成", "系统配置存在未完成或异常项", "config", "", "/settings/config-status", int64(cs.RiskCount)))
		}
	}
	// Sort: P0 first, then by count desc
	sortTodos(items)
	return &TodosDTO{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Items:       items,
	}, nil
}

// GetHealth returns subsystem health summary (degrades per module).
func (s *Service) GetHealth(ctx context.Context, q Query, sc Scope) (*HealthDTO, error) {
	out := &HealthDTO{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Modules:     []HealthModule{},
	}
	dash, err := s.GetProductOperationDashboard(ctx, q, sc)
	if err == nil && dash != nil {
		sum := dash.Summary
		out.Modules = append(out.Modules,
			healthMod("collect", "采集", healthFromCount(sum.CollectFailedCount), "/collect/tasks"),
			healthMod("product", "商品草稿", healthFromCount(sum.ReadinessBlocked), "/product/drafts"),
			healthMod("publish", "刊登", healthFromCount(sum.PublishFailedTasks), "/product/publish-tasks"),
			healthMod("order", "订单", healthFromCount(sum.OrderExceptionTotal), "/orders/exceptions"),
			healthMod("inventory", "库存", healthFromCount(sum.InventorySyncFailedCount+sum.LowStockSkus), "/inventory/alerts"),
			healthMod("customer", "客服", healthFromCount(sum.CustomerPendingReplyCount), "/customer/hub"),
			healthMod("tasks", "失败任务", healthFromCount(sum.FailedTaskTotal), "/ops/task-center/failures"),
		)
	}
	if s.ConfigStatus != nil {
		if cs, err := s.ConfigStatus.DashboardSummary(ctx); err == nil {
			out.Config = cs
			st := "healthy"
			if cs.RiskCount > 0 {
				st = "warning"
			}
			out.Modules = append(out.Modules, HealthModule{
				Key: "config", Title: "配置状态", Status: st,
				Summary: fmt.Sprintf("风险项 %d", cs.RiskCount),
				Link:    "/settings/config-status",
			})
		}
	}
	return out, nil
}

func overviewCard(key, title string, count int64, status, priority, link, emptyHint string) OverviewSection {
	return OverviewSection{
		Key: key, Title: title, Count: count, Status: status, Priority: priority,
		Link: link, EmptyHint: emptyHint,
	}
}

func compactSection(primary, secondary int64) map[string]any {
	return map[string]any{"count": primary, "secondary": secondary}
}

func unifiedTodo(typ, priority, title, desc, module, shopID, link string, count int64) UnifiedTodo {
	return UnifiedTodo{
		Type: typ, Priority: priority, Title: title, Description: desc,
		Module: module, ShopID: shopID, Link: link, Count: count,
	}
}

func sumStatus(count int64) string {
	if count > 0 {
		return "attention"
	}
	return "normal"
}

func priorityFromCount(count int64, defaultP string) string {
	if count <= 0 {
		return "P2"
	}
	return defaultP
}

func healthFromCount(count int64) string {
	if count > 0 {
		return "warning"
	}
	return "healthy"
}

func healthMod(key, title, status, link string) HealthModule {
	return HealthModule{Key: key, Title: title, Status: status, Link: link}
}

func sortTodos(items []UnifiedTodo) {
	prioRank := map[string]int{"P0": 0, "P1": 1, "P2": 2}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			pi := prioRank[strings.ToUpper(items[i].Priority)]
			pj := prioRank[strings.ToUpper(items[j].Priority)]
			if pj < pi || (pj == pi && items[j].Count > items[i].Count) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func severityToPriority(severity string) string {
	switch strings.ToLower(severity) {
	case failureclassifier.SeverityCritical, failureclassifier.SeverityHigh:
		return "P0"
	case failureclassifier.SeverityMedium:
		return "P1"
	default:
		return "P2"
	}
}

// shopFilterUUID returns explicit shop filter or first allowed shop when scoped.
func shopFilterUUID(q Query, sc Scope) *uuid.UUID {
	if raw := strings.TrimSpace(q.ShopID); raw != "" {
		u, err := uuid.Parse(raw)
		if err == nil {
			return &u
		}
	}
	return nil
}
