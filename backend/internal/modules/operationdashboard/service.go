package operationdashboard

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aioperationbatch"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/configstatus"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/orderexception"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"gorm.io/gorm"
)

const (
	recentLimit = 10
	descShort   = 60
)

// Service builds read-only product operations dashboard aggregates.
type Service struct {
	DB              *gorm.DB
	Inventory       *inventory.Service
	TaskCenter      *taskcenter.Service
	OrderExceptions *orderexception.Service
	ConfigStatus    *configstatus.Service
	flags           schemaFlags
}

// GetProductOperationDashboard returns dashboard data (local DB only; no side effects).
func (s *Service) GetProductOperationDashboard(ctx context.Context, q Query, sc Scope) (*ProductOperationsDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("operationdashboard: no db")
	}
	var shopPtr *uuid.UUID
	if raw := strings.TrimSpace(q.ShopID); raw != "" {
		u, err := uuid.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid shopId")
		}
		shopPtr = &u
	}

	out := &ProductOperationsDTO{
		Charts:     map[string]any{},
		QuickLinks: defaultQuickLinks(),
		FiltersEcho: map[string]any{
			"platform": strings.TrimSpace(q.Platform),
			"shopId":   strings.TrimSpace(q.ShopID),
			"source":   strings.TrimSpace(q.Source),
		},
	}
	if q.Start != nil {
		out.FiltersEcho["start"] = q.Start.UTC().Format(time.RFC3339)
	}
	if q.End != nil {
		out.FiltersEcho["end"] = q.End.UTC().Format(time.RFC3339)
	}

	prodTx := s.productTimeScope(ctx, q)
	sum := &out.Summary

	_ = prodTx.Session(&gorm.Session{}).Where("status = ?", product.StatusDraft).Count(&sum.DraftProducts).Error
	_ = prodTx.Session(&gorm.Session{}).Where("status = ?", product.StatusReady).Count(&sum.ReadyProducts).Error
	_ = prodTx.Session(&gorm.Session{}).Where("status = ?", product.StatusPublished).Count(&sum.PublishedProducts).Error
	_ = prodTx.Session(&gorm.Session{}).Where("status = ?", product.StatusArchived).Count(&sum.ArchivedProducts).Error
	_ = prodTx.Session(&gorm.Session{}).Count(&sum.TotalProducts).Error

	blockedTx := s.readinessBlockedTx(ctx, q)
	_ = blockedTx.Session(&gorm.Session{}).Count(&sum.ReadinessBlocked).Error

	missingTitleTx := s.missingAiTitleTx(ctx, q)
	_ = missingTitleTx.Session(&gorm.Session{}).Count(&sum.MissingAiTitleCount).Error
	missingDescTx := s.missingAiDescriptionTx(ctx, q)
	_ = missingDescTx.Session(&gorm.Session{}).Count(&sum.MissingAiDescriptionCount).Error

	titleOrDesc := s.productTimeScope(ctx, q)
	descMissing := s.aiDescMissingExpr(ctx)
	titleOrDesc = titleOrDesc.Where("products.status <> ?", product.StatusArchived).
		Where(`(
			(TRIM(COALESCE(products.ai_title,'')) = '' AND (TRIM(COALESCE(products.title,'')) <> '' OR TRIM(COALESCE(products.original_title,'')) <> ''))
			OR
			(`+descMissing+` AND (TRIM(COALESCE(products.description,'')) = '' OR LENGTH(TRIM(COALESCE(products.description,''))) < ?))
		)`, descShort)
	_ = titleOrDesc.Count(&sum.AiPendingProducts).Error

	warnTx := s.readinessWarningTx(ctx, q)
	_ = warnTx.Session(&gorm.Session{}).Count(&sum.ReadinessWarningProducts).Error
	readyTx := s.readinessReadyTx(ctx, q)
	_ = readyTx.Session(&gorm.Session{}).Count(&sum.ReadinessReadyProducts).Error

	pubTask := s.publishTaskScope(ctx, q)
	_ = pubTask.Session(&gorm.Session{}).Where("status = ?", productpublish.TaskPending).Count(&sum.PublishPendingTasks).Error
	_ = pubTask.Session(&gorm.Session{}).Where("status = ?", productpublish.TaskRunning).Count(&sum.PublishRunningTasks).Error
	_ = pubTask.Session(&gorm.Session{}).Where("status = ?", productpublish.TaskFailed).Count(&sum.PublishFailedTasks).Error

	pubRec := s.publicationScope(ctx, q).Where("status = ? AND deleted_at IS NULL", productpublish.StatusPublishedRecord)
	_ = pubRec.Session(&gorm.Session{}).Count(&sum.PublishedPublicationCount).Error

	// AI tasks (product text; exclude customer_reply_generate noise for this MVP board)
	aiFailTx := s.DB.WithContext(ctx).Model(&aitask.AITask{}).
		Where("status = ?", aitask.StatusFailed).
		Where("task_type IN ?", []string{"title_optimize", "product_description_generate"})
	if q.Start != nil {
		aiFailTx = aiFailTx.Where("updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		aiFailTx = aiFailTx.Where("updated_at <= ?", *q.End)
	}
	_ = aiFailTx.Count(&sum.AiTaskFailedCount).Error

	_ = s.DB.WithContext(ctx).Model(&aioperationbatch.AIOperationBatch{}).
		Where("status = ?", aioperationbatch.StatusRunning).Count(&sum.AiBatchRunningCount).Error
	_ = s.DB.WithContext(ctx).Model(&aioperationbatch.AIOperationBatch{}).
		Where("status = ?", aioperationbatch.StatusFailed).Count(&sum.AiBatchFailedCount).Error

	// Customer
	custOpen := s.customerConvScope(ctx, q).Where("status = ?", customerchat.StatusOpen)
	_ = custOpen.Session(&gorm.Session{}).Count(&sum.CustomerOpenConversations).Error
	custPending := s.customerConvScope(ctx, q).Where("status = ?", customerchat.StatusPendingReply)
	_ = custPending.Session(&gorm.Session{}).Count(&sum.CustomerPendingReplyCount).Error
	sum.CustomerPendingConversations = sum.CustomerOpenConversations + sum.CustomerPendingReplyCount

	_ = s.suggestionPendingScope(ctx, q).Count(&sum.AiReplySuggestionPendingCount).Error

	// Inventory alert totals via existing policy-aware listing (count-only)
	if shopPtr == nil && !q.Scope.IsAdmin && len(q.Scope.AllowedShopIDs) > 0 {
		for _, sid := range q.Scope.AllowedShopIDs {
			id := sid
			invQ := inventory.AlertsListQuery{
				Platform: strings.TrimSpace(q.Platform), ShopID: &id,
				Page: 1, PageSize: 1, IncludeNormal: false, OnlyPublished: false,
			}
			if r, err := s.invCount(ctx, invQ, inventory.AlertTypeLowStock); err == nil {
				sum.LowStockSkus += r
			}
			if r, err := s.invCount(ctx, invQ, inventory.AlertTypeOutOfStock); err == nil {
				sum.OutOfStockSkus += r
			}
			if r, err := s.invCount(ctx, invQ, inventory.AlertTypePlatformStockMismatch); err == nil {
				sum.PlatformStockMismatchCount += r
			}
			if r, err := s.invCount(ctx, invQ, inventory.AlertTypeInventorySyncFailed); err == nil {
				sum.InventorySyncFailedCount += r
			}
		}
	} else {
		invQBase := inventory.AlertsListQuery{
			Platform:      strings.TrimSpace(q.Platform),
			ShopID:        shopPtr,
			Page:          1,
			PageSize:      1,
			IncludeNormal: false,
			OnlyPublished: false,
		}
		if r, err := s.invCount(ctx, invQBase, inventory.AlertTypeLowStock); err == nil {
			sum.LowStockSkus = r
		}
		if r, err := s.invCount(ctx, invQBase, inventory.AlertTypeOutOfStock); err == nil {
			sum.OutOfStockSkus = r
		}
		if r, err := s.invCount(ctx, invQBase, inventory.AlertTypePlatformStockMismatch); err == nil {
			sum.PlatformStockMismatchCount = r
		}
		if r, err := s.invCount(ctx, invQBase, inventory.AlertTypeInventorySyncFailed); err == nil {
			sum.InventorySyncFailedCount = r
		}
	}

	// Task center failure + alerts
	if s.TaskCenter != nil {
		p := taskcenter.ListFailureParams{
			Platform:        strings.TrimSpace(q.Platform),
			ShopID:          strings.TrimSpace(q.ShopID),
			IncludeResolved: false,
			IncludeMarked:   false,
			Start:           q.Start,
			End:             q.End,
			AllowedShopIDs:  q.Scope.AllowedShopIDs,
		}
		if su, err := s.TaskCenter.Summary(ctx, p); err == nil {
			sum.FailedTaskTotal = su.TotalFailed
			sum.FailedTasks = su.TotalFailed
		}
	}
	_ = s.DB.WithContext(ctx).Model(&taskcenter.TaskAlert{}).
		Where("status = ?", taskcenter.TaskAlertStatusOpen).
		Where("severity = ?", failureclassifier.SeverityCritical).
		Count(&sum.CriticalAlertCount).Error
	_ = s.DB.WithContext(ctx).Model(&taskcenter.TaskAlert{}).
		Where("status = ?", taskcenter.TaskAlertStatusOpen).
		Count(&sum.OpenAlertCount).Error

	if s.OrderExceptions != nil {
		ex, err := s.OrderExceptions.DashboardSummary(ctx, strings.TrimSpace(q.Platform), strings.TrimSpace(q.ShopID), q.Start, q.End)
		if err == nil {
			sum.OrderExceptionTotal = ex.TotalOpen
			sum.SKUUnmatchedOrderItems = ex.SKUUnmatched
			sum.InventoryDeductFailedOrders = ex.InsufficientStock + ex.InventoryDeductFailed
		}
	}

	// Publishable / blocked counts for todos
	publishableTx := s.publishableProductsTx(ctx, q)
	var publishableCount int64
	_ = publishableTx.Session(&gorm.Session{}).Count(&publishableCount).Error
	sum.Publishable = publishableCount

	// Image tasks (local DB only)
	imgPendingTx := s.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).
		Where("status IN ?", []string{imagetask.StatusPending, imagetask.StatusRunning, imagetask.StatusRetrying})
	if q.Start != nil {
		imgPendingTx = imgPendingTx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		imgPendingTx = imgPendingTx.Where("created_at <= ?", *q.End)
	}
	_ = imgPendingTx.Count(&sum.ImageTaskPending).Error

	imgFailedTx := s.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).Where("status = ?", imagetask.StatusFailed)
	if q.Start != nil {
		imgFailedTx = imgFailedTx.Where("updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		imgFailedTx = imgFailedTx.Where("updated_at <= ?", *q.End)
	}
	_ = imgFailedTx.Count(&sum.ImageTaskFailed).Error

	// Products with AI-processed images
	imgProdTx := s.DB.WithContext(ctx).Table("product_images AS pi").
		Joins("INNER JOIN products p ON p.id = pi.product_id").
		Where(`(pi.source = ? OR pi.image_type IN ? OR pi.source_task_id IS NOT NULL)`,
			product.ImageSourceAI, []string{product.ImageTypeAIGenerated, product.ImageTypeMarketing})
	if soft, _ := s.schemaFlags(ctx); soft {
		imgProdTx = imgProdTx.Where("p.deleted_at IS NULL")
	}
	imgProdTx = s.applyProductJoinScope(imgProdTx, q, "p")
	_ = imgProdTx.Distinct("pi.product_id").Count(&sum.ImageProcessedCount).Error

	// Collected products (have source URL or known collector source)
	collectedTx := s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where(`(TRIM(COALESCE(products.source_url,'')) <> '' OR products.source IN ?)`,
			[]string{"1688", "pinduoduo", "pdd", "taobao", "custom", "aliexpress"})
	_ = collectedTx.Count(&sum.CollectedProducts).Error

	// AI title / description completed
	aiTitleDoneTx := s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where("TRIM(COALESCE(products.ai_title,'')) <> ''")
	_ = aiTitleDoneTx.Count(&sum.AiTitleCompleted).Error

	aiDescDoneTx := s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where(s.aiDescDoneExpr(ctx))
	_ = aiDescDoneTx.Count(&sum.AiDescCompleted).Error

	aiTextDoneTx := s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where("TRIM(COALESCE(products.ai_title,'')) <> ''").
		Where(s.aiDescDoneExpr(ctx))
	_ = aiTextDoneTx.Count(&sum.AiTextCompleted).Error

	sum.ReadinessPassed = publishableCount

	// Collect failures
	collectFailTx := s.collectTaskScope(ctx, q).Where("status = ?", collect.StatusFailed)
	_ = collectFailTx.Count(&sum.CollectFailedCount).Error

	if s.ConfigStatus != nil {
		if cs, err := s.ConfigStatus.DashboardSummary(ctx); err == nil && cs != nil {
			sum.ConfigRiskCount = int64(cs.RiskCount)
		}
	}

	// Today new products
	todayStart := startOfDayUTC(time.Now().UTC())
	todayNewTx := s.productTimeScope(ctx, q)
	if q.Start != nil || q.End != nil {
		if q.Start != nil {
			todayNewTx = todayNewTx.Where("products.created_at >= ?", *q.Start)
		}
		if q.End != nil {
			todayNewTx = todayNewTx.Where("products.created_at <= ?", *q.End)
		}
	} else {
		todayNewTx = todayNewTx.Where("products.created_at >= ?", todayStart)
	}
	_ = todayNewTx.Count(&sum.TodayNewProducts).Error

	// Compact KPI aliases
	sum.DraftTotal = sum.DraftProducts + sum.ReadyProducts
	sum.MissingAiTitle = sum.MissingAiTitleCount
	sum.MissingAiDescription = sum.MissingAiDescriptionCount
	sum.ReadinessBlockedKPI = sum.ReadinessBlocked
	sum.Published = sum.PublishedProducts
	sum.InventoryAlerts = sum.LowStockSkus + sum.OutOfStockSkus
	sum.OrderExceptions = sum.OrderExceptionTotal

	out.Todos = buildTodoCards(sum, publishableCount)
	out.Funnel = buildFunnel(sum)
	out.Exceptions = s.buildExceptions(ctx, q, sum, shopPtr)
	out.Recent = s.buildRecent(ctx, q, shopPtr)

	return out, nil
}

func startOfDayUTC(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func (s *Service) applyProductJoinScope(tx *gorm.DB, q Query, alias string) *gorm.DB {
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where(alias+".source = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where(alias+".updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where(alias+".updated_at <= ?", *q.End)
	}
	return tx
}

func (s *Service) collectTaskScope(ctx context.Context, q Query) *gorm.DB {
	tx := s.DB.WithContext(ctx).Model(&collect.CollectTask{})
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where("updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("updated_at <= ?", *q.End)
	}
	return tx
}

func (s *Service) invCount(ctx context.Context, base inventory.AlertsListQuery, alertType string) (int64, error) {
	if s == nil || s.Inventory == nil {
		return 0, fmt.Errorf("no inventory service")
	}
	q := base
	q.AlertType = alertType
	res, err := s.Inventory.ListInventoryAlerts(ctx, q)
	if err != nil || res == nil {
		return 0, err
	}
	return res.Total, nil
}

func (s *Service) applyProductScope(tx *gorm.DB, q Query) *gorm.DB {
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where("products.source = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where("products.updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("products.updated_at <= ?", *q.End)
	}
	return tx
}

func (s *Service) publishTaskScope(ctx context.Context, q Query) *gorm.DB {
	tx := s.DB.WithContext(ctx).Model(&productpublish.ProductPublishTask{})
	if pl := strings.TrimSpace(q.Platform); pl != "" {
		tx = tx.Where("LOWER(platform) = ?", strings.ToLower(pl))
	}
	if v := strings.TrimSpace(q.ShopID); v != "" {
		tx = tx.Where("shop_id = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where("created_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("created_at <= ?", *q.End)
	}
	return q.Scope.applyShopColumn(tx, "shop_id")
}

func (s *Service) publicationScope(ctx context.Context, q Query) *gorm.DB {
	tx := s.DB.WithContext(ctx).Model(&productpublish.ProductPublication{})
	if pl := strings.TrimSpace(q.Platform); pl != "" {
		tx = tx.Where("LOWER(platform) = ?", strings.ToLower(pl))
	}
	if v := strings.TrimSpace(q.ShopID); v != "" {
		tx = tx.Where("shop_id = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where("updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("updated_at <= ?", *q.End)
	}
	return q.Scope.applyShopColumn(tx, "shop_id")
}

func (s *Service) customerConvScope(ctx context.Context, q Query) *gorm.DB {
	tx := s.DB.WithContext(ctx).Model(&customerchat.CustomerConversation{}).Where("customer_conversations.deleted_at IS NULL")
	if pl := strings.TrimSpace(q.Platform); pl != "" {
		tx = tx.Where("LOWER(customer_conversations.platform) = ?", strings.ToLower(pl))
	}
	if v := strings.TrimSpace(q.ShopID); v != "" {
		tx = tx.Where("customer_conversations.shop_id = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where("customer_conversations.updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("customer_conversations.updated_at <= ?", *q.End)
	}
	return q.Scope.applyShopColumn(tx, "customer_conversations.shop_id")
}

func (s *Service) suggestionPendingScope(ctx context.Context, q Query) *gorm.DB {
	tx := s.DB.WithContext(ctx).Model(&customerchat.CustomerReplySuggestion{}).
		Joins(`INNER JOIN customer_conversations c ON c.id = customer_reply_suggestions.conversation_id AND c.deleted_at IS NULL`).
		Where("customer_reply_suggestions.status IN ?", []string{customerchat.SuggestionGenerated, customerchat.SuggestionEdited})
	if pl := strings.TrimSpace(q.Platform); pl != "" {
		tx = tx.Where("LOWER(c.platform) = ?", strings.ToLower(pl))
	}
	if v := strings.TrimSpace(q.ShopID); v != "" {
		tx = tx.Where("c.shop_id = ?", v)
	}
	if q.Start != nil {
		tx = tx.Where("customer_reply_suggestions.updated_at >= ?", *q.Start)
	}
	if q.End != nil {
		tx = tx.Where("customer_reply_suggestions.updated_at <= ?", *q.End)
	}
	return tx
}

func (s *Service) readinessBlockedTx(ctx context.Context, q Query) *gorm.DB {
	tx := s.productTimeScope(ctx, q)
	return tx.Where("products.status <> ?", product.StatusArchived).
		Where(`(
			(TRIM(COALESCE(products.title,'')) = '' AND TRIM(COALESCE(products.original_title,'')) = '')
			OR TRIM(COALESCE(products.currency,'')) = ''
			OR NOT `+sqlMainImageExists+`
			OR NOT `+sqlSKUExists+`
			OR `+sqlInvalidPriceSKU+`
		)`, product.ImageTypeMain)
}

func (s *Service) readinessWarningTx(ctx context.Context, q Query) *gorm.DB {
	blocked := s.readinessBlockedTx(ctx, q).Select("products.id")
	descMissing := s.aiDescMissingExpr(ctx)
	return s.productTimeScope(ctx, q).
		Where("products.status NOT IN ?", []string{product.StatusArchived}).
		Where("products.id NOT IN (?)", blocked).
		Where(`(
			TRIM(COALESCE(products.ai_title,'')) = ''
			OR (`+descMissing+` AND LENGTH(TRIM(COALESCE(products.description,''))) < ?)
		)`, descShort)
}

func (s *Service) readinessReadyTx(ctx context.Context, q Query) *gorm.DB {
	blocked := s.readinessBlockedTx(ctx, q).Select("products.id")
	return s.productTimeScope(ctx, q).
		Where("products.status NOT IN ?", []string{product.StatusArchived}).
		Where("products.id NOT IN (?)", blocked).
		Where("TRIM(COALESCE(products.ai_title,'')) <> ''").
		Where(s.aiDescPresentExpr(ctx), descShort)
}

func (s *Service) missingAiTitleTx(ctx context.Context, q Query) *gorm.DB {
	return s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where("TRIM(COALESCE(products.ai_title,'')) = ''").
		Where("( TRIM(COALESCE(products.title,'')) <> '' OR TRIM(COALESCE(products.original_title,'')) <> '' )")
}

func (s *Service) missingAiDescriptionTx(ctx context.Context, q Query) *gorm.DB {
	descMissing := s.aiDescMissingExpr(ctx)
	return s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where(descMissing).
		Where("( TRIM(COALESCE(products.description,'')) = '' OR LENGTH(TRIM(COALESCE(products.description,''))) < ? )", descShort)
}

func (s *Service) publishableProductsTx(ctx context.Context, q Query) *gorm.DB {
	return s.productTimeScope(ctx, q).
		Where("products.status IN ?", []string{product.StatusDraft, product.StatusReady}).
		Where("( TRIM(COALESCE(products.title,'')) <> '' OR TRIM(COALESCE(products.original_title,'')) <> '' )").
		Where("TRIM(COALESCE(products.currency,'')) <> ''").
		Where(sqlMainImageExists, product.ImageTypeMain).
		Where(sqlSKUExists).
		Where("NOT " + sqlInvalidPriceSKU)
}

func buildTodoCards(sum *Summary, publishable int64) []TodoCard {
	return []TodoCard{
		todoCard("missing_ai_title", "待补 AI 标题", sum.MissingAiTitleCount, failureclassifier.SeverityMedium,
			"这些商品还没有 AI 标题", "/product/drafts?missingAiTitle=1"),
		todoCard("missing_ai_description", "待补 AI 描述", sum.MissingAiDescriptionCount, failureclassifier.SeverityMedium,
			"这些商品还没有 AI 描述", "/product/drafts?missingAiDescription=1"),
		todoCard("readiness_blocked", "发布检查未通过", sum.ReadinessBlocked, failureclassifier.SeverityHigh,
			"缺标题、主图、规格或价格等，需先补齐", "/product/drafts?readiness=blocked"),
		todoCard("inventory_alerts", "库存预警", sum.LowStockSkus+sum.OutOfStockSkus, failureclassifier.SeverityHigh,
			"低库存或缺货，建议尽快补货或调整", "/inventory/alerts"),
		todoCard("ai_image_failed", "AI 图片任务失败", sum.ImageTaskFailed, failureclassifier.SeverityHigh,
			"图片处理失败，可在任务页重试", "/ai/image-tasks?status=failed"),
		todoCard("collect_failed", "采集失败", sum.CollectFailedCount, failureclassifier.SeverityHigh,
			"商品链接采集未成功，可重试", "/collect/tasks?status=failed"),
		todoCard("publish_failed", "刊登失败", sum.PublishFailedTasks, failureclassifier.SeverityHigh,
			"刊登到平台时出错，请查看详情后重试", "/product/publish-tasks?status=failed"),
		todoCard("order_exceptions", "订单异常", sum.OrderExceptionTotal, failureclassifier.SeverityHigh,
			"含未匹配 SKU 等需人工处理的订单问题", "/orders/exceptions"),
		todoCard("customer_pending", "客服待回复", sum.CustomerPendingReplyCount, failureclassifier.SeverityHigh,
			"买家消息等待人工处理", "/customer/conversations?status=pending_reply"),
		todoCard("failed_tasks", "失败任务待处理", sum.FailedTaskTotal, failureclassifier.SeverityCritical,
			"统一失败任务中心有待处理项", "/ops/task-center/failures"),
	}
}

func todoCard(id, title string, count int64, severity, desc, link string) TodoCard {
	return TodoCard{
		ID:          id,
		Key:         id,
		Title:       title,
		Count:       count,
		Severity:    severity,
		Level:       severityToLevel(severity),
		Description: desc,
		Link:        link,
	}
}

func buildFunnel(sum *Summary) []FunnelStep {
	return []FunnelStep{
		{Key: "collected", Title: "采集商品", Count: sum.CollectedProducts, Link: "/collect/hub",
			Description: "已从链接采集并生成草稿的商品"},
		{Key: "draft", Title: "商品草稿", Count: sum.DraftTotal, Link: "/product/drafts",
			Description: "草稿与就绪状态的商品"},
		{Key: "ai_text", Title: "AI 标题 / 描述", Count: sum.AiTextCompleted, Link: "/product/drafts",
			Description: "已完成 AI 标题与描述的商品"},
		{Key: "ai_image", Title: "AI 图片处理", Count: sum.ImageProcessedCount, Link: "/ai/image-tasks",
			Description: "已有 AI 处理图片的商品"},
		{Key: "readiness_pass", Title: "发布检查通过", Count: sum.ReadinessPassed, Link: "/product/drafts?publishable=1",
			Description: "基础信息完备、可创建刊登任务"},
		{Key: "published", Title: "已刊登", Count: sum.Published, Link: "/product/drafts?status=published",
			Description: "已刊登到平台的商品"},
	}
}

func (s *Service) buildExceptions(ctx context.Context, q Query, sum *Summary, shopPtr *uuid.UUID) []ExceptionItem {
	var out []ExceptionItem

	// Collect failures
	{
		collectFailTx := s.collectTaskScope(ctx, q).Where("status = ?", collect.StatusFailed)
		last := scanOptionalMaxTime(collectFailTx)
		out = append(out, ExceptionItem{
			Key: "collect_failed", Title: "采集失败", Count: sum.CollectFailedCount, LastOccurred: last,
			Link:        "/collect/tasks?status=failed",
			Description: "商品链接采集未成功，可重试或检查登录状态",
		})
	}

	// AI text failures
	{
		tx := s.DB.WithContext(ctx).Model(&aitask.AITask{}).
			Where("status = ?", aitask.StatusFailed).
			Where("task_type IN ?", []string{"title_optimize", "product_description_generate"})
		if q.Start != nil {
			tx = tx.Where("updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			tx = tx.Where("updated_at <= ?", *q.End)
		}
		last := scanOptionalMaxTime(tx)
		out = append(out, ExceptionItem{
			Key: "ai_text_failed", Title: "AI 文本任务失败", Count: sum.AiTaskFailedCount, LastOccurred: last,
			Link:        "/ai/tasks?status=failed",
			Description: "标题优化或描述生成失败，可在 AI 任务页重试",
		})
	}

	// AI image failures
	{
		tx := s.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).Where("status = ?", imagetask.StatusFailed)
		if q.Start != nil {
			tx = tx.Where("updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			tx = tx.Where("updated_at <= ?", *q.End)
		}
		last := scanOptionalMaxTime(tx)
		out = append(out, ExceptionItem{
			Key: "ai_image_failed", Title: "AI 图片任务失败", Count: sum.ImageTaskFailed, LastOccurred: last,
			Link:        "/ai/image-tasks?status=failed",
			Description: "去水印、去背景等图片处理失败，可重试任务",
		})
	}

	// Publish failures
	{
		tx := s.publishTaskScope(ctx, q).Where("status = ?", productpublish.TaskFailed)
		last := scanOptionalMaxTime(tx)
		out = append(out, ExceptionItem{
			Key: "publish_failed", Title: "商品刊登失败", Count: sum.PublishFailedTasks, LastOccurred: last,
			Link:        "/product/publish-tasks?status=failed",
			Description: "刊登到平台时出错，请查看错误详情后重试",
		})
	}

	// Inventory sync failures
	{
		invTx := s.DB.WithContext(ctx).Model(&inventory.InventorySyncTask{}).Where("status = ?", inventory.StatusFailed)
		if pl := strings.TrimSpace(q.Platform); pl != "" {
			invTx = invTx.Where("LOWER(platform) = ?", strings.ToLower(pl))
		}
		if shopPtr != nil {
			invTx = invTx.Where("shop_id = ?", *shopPtr)
		}
		if q.Start != nil {
			invTx = invTx.Where("updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			invTx = invTx.Where("updated_at <= ?", *q.End)
		}
		last := scanOptionalMaxTime(invTx)
		out = append(out, ExceptionItem{
			Key: "inventory_sync_failed", Title: "库存同步失败", Count: sum.InventorySyncFailedCount, LastOccurred: last,
			Link:        "/inventory/sync-tasks?status=failed",
			Description: "平台库存同步未成功，可在同步任务页重试",
		})
	}

	// Order exceptions
	{
		var last *time.Time
		if s.OrderExceptions != nil {
			res, err := s.OrderExceptions.ListOrderExceptions(ctx, orderexception.ListOrderExceptionsRequest{
				Platform: strings.TrimSpace(q.Platform),
				ShopID:   strings.TrimSpace(q.ShopID),
				Start:    q.Start,
				End:      q.End,
				Page:     1,
				PageSize: 1,
			})
			if err == nil && res != nil && len(res.List) > 0 {
				t := res.List[0].UpdatedAt
				last = &t
			}
		}
		desc := fmt.Sprintf("含未匹配 SKU（%d 行）等需人工处理的订单问题", sum.SKUUnmatchedOrderItems)
		out = append(out, ExceptionItem{
			Key: "order_exceptions", Title: "订单异常 / SKU 未匹配", Count: sum.OrderExceptionTotal, LastOccurred: last,
			Link:        "/orders/exceptions",
			Description: desc,
		})
	}

	return out
}

func defaultQuickLinks() []QuickLink {
	return []QuickLink{
		{Title: "采集中心", Link: "/collect/hub", Description: "输入商品链接开始采集"},
		{Title: "商品草稿", Link: "/product/drafts", Description: "查看和编辑商品草稿"},
		{Title: "批量文案任务", Link: "/ai/text-batches", Description: "批量生成并复核标题与描述"},
		{Title: "AI 图片任务", Link: "/ai/image-tasks", Description: "去水印、营销图等图片处理"},
		{Title: "发布检查", Link: "/product/drafts?readiness=blocked", Description: "查看未通过发布检查的商品"},
		{Title: "商品刊登任务", Link: "/product/publish-tasks", Description: "管理刊登到平台的任务"},
		{Title: "库存预警", Link: "/inventory/alerts", Description: "低库存与缺货提醒"},
		{Title: "失败任务中心", Link: "/ops/task-center/failures", Description: "统一查看各类失败任务"},
		{Title: "订单异常工作台", Link: "/orders/exceptions", Description: "处理 SKU 未匹配等订单问题"},
		{Title: "设置 AI", Link: "/settings/ai", Description: "配置 AI 模型与 API"},
		{Title: "设置图片 AI", Link: "/settings/image", Description: "配置图片处理 Provider"},
		{Title: "设置存储", Link: "/settings/storage", Description: "配置图片与文件存储"},
	}
}

func (s *Service) buildRecent(ctx context.Context, q Query, shopPtr *uuid.UUID) RecentBuckets {
	var b RecentBuckets

	// Collected products
	{
		qb := s.DB.WithContext(ctx).Table("collect_tasks AS t").
			Select("t.id, t.result_product_id AS result_product_id, t.updated_at, t.source, p.title AS prod_title, t.status").
			Joins("INNER JOIN products p ON p.id = t.result_product_id").
			Where("t.status = ? AND t.result_product_id IS NOT NULL", collect.StatusSuccess)
		if soft, _ := s.schemaFlags(ctx); soft {
			qb = qb.Where("p.deleted_at IS NULL")
		}
		if v := strings.TrimSpace(q.Source); v != "" {
			qb = qb.Where("t.source = ?", v)
		}
		if q.Start != nil {
			qb = qb.Where("t.updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			qb = qb.Where("t.updated_at <= ?", *q.End)
		}
		var r2 []struct {
			ResultProduct uuid.UUID `gorm:"column:result_product_id"`
			UpdatedAt     time.Time `gorm:"column:updated_at"`
			Source        string    `gorm:"column:source"`
			ProdTitle     string    `gorm:"column:prod_title"`
			Status        string    `gorm:"column:status"`
		}
		_ = qb.Order("t.updated_at DESC").Limit(recentLimit).Scan(&r2).Error
		for _, r := range r2 {
			item := RecentItem{
				Type:       "collect",
				Title:      clip(r.ProdTitle, 120),
				Subtitle:   humanizeProductSource(r.Source),
				Status:     humanizeTaskStatus(r.Status),
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/product/drafts/%s", r.ResultProduct.String()),
			}
			b.CollectedProducts = append(b.CollectedProducts, item)
			b.Products = append(b.Products, item)
		}
	}

	// AI text tasks
	{
		var rows []aitask.AITask
		tx := s.DB.WithContext(ctx).Model(&aitask.AITask{}).
			Where("task_type IN ?", []string{"title_optimize", "product_description_generate"}).
			Order("updated_at DESC").Limit(recentLimit)
		if q.Start != nil {
			tx = tx.Where("updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			tx = tx.Where("updated_at <= ?", *q.End)
		}
		_ = tx.Find(&rows).Error
		for _, r := range rows {
			title := r.TaskType
			switch r.TaskType {
			case "title_optimize":
				title = "AI 标题优化"
			case "product_description_generate":
				title = "AI 描述生成"
			}
			link := "/ai/tasks"
			if r.ID != uuid.Nil {
				link = fmt.Sprintf("/ai/tasks?keyword=%s", r.ID.String())
			}
			item := RecentItem{
				Type:       "ai_task",
				Title:      title,
				Subtitle:   clip(r.ErrorMessage, 80),
				Status:     humanizeTaskStatus(r.Status),
				OccurredAt: r.UpdatedAt,
				Link:       link,
			}
			b.AiTasks = append(b.AiTasks, item)
		}
	}

	// AI batches
	{
		var rows []aioperationbatch.AIOperationBatch
		tx := s.DB.WithContext(ctx).Model(&aioperationbatch.AIOperationBatch{}).Where("deleted_at IS NULL").Order("updated_at DESC").Limit(recentLimit)
		if q.Start != nil {
			tx = tx.Where("updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			tx = tx.Where("updated_at <= ?", *q.End)
		}
		_ = tx.Find(&rows).Error
		for _, r := range rows {
			b.AiBatches = append(b.AiBatches, RecentItem{
				Type:       "ai_batch",
				Title:      r.BatchNo,
				Subtitle:   humanizeBatchOperationType(r.OperationType),
				Status:     humanizeTaskStatus(r.Status),
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/ai/batches?id=%s", r.ID.String()),
			})
		}
	}

	// Image tasks
	{
		var rows []imagetask.ImageTask
		tx := s.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).Order("updated_at DESC").Limit(recentLimit)
		if q.Start != nil {
			tx = tx.Where("updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			tx = tx.Where("updated_at <= ?", *q.End)
		}
		_ = tx.Find(&rows).Error
		for _, r := range rows {
			sub := humanizeImageTaskSubtitle(r.TaskType, r.Status, r.Output, r.ErrorMessage)
			b.ImageTasks = append(b.ImageTasks, RecentItem{
				Type:       "image_task",
				Title:      humanizeImageTaskType(r.TaskType),
				Subtitle:   sub,
				Status:     humanizeTaskStatus(r.Status),
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/ai/image-tasks?keyword=%s", r.ID.String()),
			})
		}
	}

	// Publish tasks
	{
		var rows []productpublish.ProductPublishTask
		tx := s.publishTaskScope(ctx, q).Order("updated_at DESC").Limit(recentLimit)
		_ = tx.Find(&rows).Error
		for _, r := range rows {
			sub := humanizeTaskStatus(r.Status)
			if r.Status == productpublish.TaskFailed {
				sub = clip(r.ErrorMessage, 100)
			}
			b.PublishTasks = append(b.PublishTasks, RecentItem{
				Type:       "product_publish",
				Title:      r.Platform + " 刊登",
				Subtitle:   sub,
				Status:     humanizeTaskStatus(r.Status),
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/product/publish-tasks?keyword=%s", r.ID.String()),
			})
		}
	}

	// Inventory alert samples (low stock)
	if s.Inventory != nil {
		res, err := s.Inventory.ListInventoryAlerts(ctx, inventory.AlertsListQuery{
			Platform:  strings.TrimSpace(q.Platform),
			ShopID:    shopPtr,
			AlertType: inventory.AlertTypeLowStock,
			Page:      1,
			PageSize:  recentLimit,
		})
		if err == nil && res != nil {
			for _, e := range res.Items {
				ts := time.Now().UTC()
				if e.LastInventoryChangeAt != nil {
					ts = *e.LastInventoryChangeAt
				}
				if e.LastSyncAt != nil && e.LastSyncAt.After(ts) {
					ts = *e.LastSyncAt
				}
				b.InventoryAlerts = append(b.InventoryAlerts, RecentItem{
					Type:       "inventory_alert",
					Title:      clip(e.ProductTitle, 80) + " · " + e.SKUCode,
					Subtitle:   "低库存",
					Status:     "预警",
					OccurredAt: ts,
					Link:       "/inventory/alerts",
				})
			}
		}
	}

	// Customer conversations
	{
		var rows []customerchat.CustomerConversation
		tx := s.customerConvScope(ctx, q).Order("updated_at DESC").Limit(recentLimit)
		_ = tx.Find(&rows).Error
		for _, r := range rows {
			b.CustomerConversations = append(b.CustomerConversations, RecentItem{
				Type:       "customer_conversation",
				Title:      clip(r.CustomerName, 64),
				Subtitle:   r.Platform,
				Status:     humanizeTaskStatus(r.Status),
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/customer/conversations/%s", r.ID.String()),
			})
		}
	}

	// Recent hard failures (publish + inventory sync + collect + ai + image)
	{
		var pubF []productpublish.ProductPublishTask
		tx := s.publishTaskScope(ctx, q).Where("status = ?", productpublish.TaskFailed).Order("updated_at DESC").Limit(recentLimit)
		_ = tx.Find(&pubF).Error
		for _, r := range pubF {
			b.FailedTasks = append(b.FailedTasks, RecentItem{
				Type:       "failed_publish",
				Title:      r.Platform + " · 刊登失败",
				Subtitle:   clip(r.ErrorMessage, 100),
				Status:     "失败",
				OccurredAt: r.UpdatedAt,
				Link:       "/product/publish-tasks?status=failed",
			})
		}
		var invF []inventory.InventorySyncTask
		invTx := s.DB.WithContext(ctx).Model(&inventory.InventorySyncTask{}).Where("status = ?", inventory.StatusFailed).Order("updated_at DESC").Limit(recentLimit)
		if pl := strings.TrimSpace(q.Platform); pl != "" {
			invTx = invTx.Where("LOWER(platform) = ?", strings.ToLower(pl))
		}
		if shopPtr != nil {
			invTx = invTx.Where("shop_id = ?", *shopPtr)
		}
		if q.Start != nil {
			invTx = invTx.Where("updated_at >= ?", *q.Start)
		}
		if q.End != nil {
			invTx = invTx.Where("updated_at <= ?", *q.End)
		}
		_ = invTx.Find(&invF).Error
		for _, r := range invF {
			b.FailedTasks = append(b.FailedTasks, RecentItem{
				Type:       "failed_inventory_sync",
				Title:      r.Platform + " · 库存同步失败",
				Subtitle:   clip(r.ErrorMessage, 100),
				Status:     "失败",
				OccurredAt: r.UpdatedAt,
				Link:       "/inventory/sync-tasks?status=failed",
			})
		}
		var collectF []collect.CollectTask
		cTx := s.collectTaskScope(ctx, q).Where("status = ?", collect.StatusFailed).Order("updated_at DESC").Limit(recentLimit)
		_ = cTx.Find(&collectF).Error
		for _, r := range collectF {
			code := collect.InferErrorCodeFromMessage(r.ErrorMessage)
			label := humanizeCollectorError(code)
			if label == "" {
				label = clip(r.ErrorMessage, 80)
			}
			b.FailedTasks = append(b.FailedTasks, RecentItem{
				Type:       "failed_collect",
				Title:      "采集失败",
				Subtitle:   label,
				Status:     "失败",
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/collect/tasks?keyword=%s", r.ID.String()),
			})
		}
	}

	// Open alerts
	{
		var rows []taskcenter.TaskAlert
		tx := s.DB.WithContext(ctx).Where("status = ?", taskcenter.TaskAlertStatusOpen).
			Order("last_seen_at DESC").Limit(recentLimit)
		if q.Start != nil {
			tx = tx.Where("last_seen_at >= ?", *q.Start)
		}
		if q.End != nil {
			tx = tx.Where("last_seen_at <= ?", *q.End)
		}
		_ = tx.Find(&rows).Error
		for _, r := range rows {
			b.Alerts = append(b.Alerts, RecentItem{
				Type:       "task_alert",
				Title:      clip(r.Title, 120),
				Subtitle:   r.TaskType,
				Status:     "开放",
				OccurredAt: r.LastSeenAt,
				Link:       "/ops/task-center/alerts",
			})
		}
	}

	return b
}

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
