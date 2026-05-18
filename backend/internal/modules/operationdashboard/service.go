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
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"gorm.io/gorm"
)

const (
	recentLimit = 5
	descShort   = 60
)

// Service builds read-only product operations dashboard aggregates.
type Service struct {
	DB         *gorm.DB
	Inventory  *inventory.Service
	TaskCenter *taskcenter.Service
}

// GetProductOperationDashboard returns dashboard data (local DB only; no side effects).
func (s *Service) GetProductOperationDashboard(ctx context.Context, q Query) (*ProductOperationsDTO, error) {
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

	titleOrDesc := s.DB.WithContext(ctx).Model(&product.Product{}).Where("products.deleted_at IS NULL")
	titleOrDesc = s.applyProductScope(titleOrDesc, q)
	titleOrDesc = titleOrDesc.Where("products.status <> ?", product.StatusArchived).
		Where(`(
			(TRIM(COALESCE(products.ai_title,'')) = '' AND (TRIM(COALESCE(products.title,'')) <> '' OR TRIM(COALESCE(products.original_title,'')) <> ''))
			OR
			(TRIM(COALESCE(products.ai_description,'')) = '' AND (TRIM(COALESCE(products.description,'')) = '' OR LENGTH(TRIM(COALESCE(products.description,''))) < ?))
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

	// Task center failure + alerts
	if s.TaskCenter != nil {
		p := taskcenter.ListFailureParams{
			Platform:        strings.TrimSpace(q.Platform),
			ShopID:          strings.TrimSpace(q.ShopID),
			IncludeResolved: false,
			IncludeMarked:   false,
			Start:           q.Start,
			End:             q.End,
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

	// Publishable / blocked counts for todos
	publishableTx := s.publishableProductsTx(ctx, q)
	var publishableCount int64
	_ = publishableTx.Session(&gorm.Session{}).Count(&publishableCount).Error

	out.Todos = buildTodoCards(sum, publishableCount)
	out.Recent = s.buildRecent(ctx, q, shopPtr)

	return out, nil
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

func (s *Service) productTimeScope(ctx context.Context, q Query) *gorm.DB {
	tx := s.DB.WithContext(ctx).Model(&product.Product{}).Where("products.deleted_at IS NULL")
	return s.applyProductScope(tx, q)
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
	return tx
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
	return tx
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
	return tx
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
			OR NOT EXISTS (SELECT 1 FROM product_images pi WHERE pi.product_id = products.id AND pi.image_type = ? AND pi.deleted_at IS NULL)
			OR NOT EXISTS (SELECT 1 FROM product_skus ps0 WHERE ps0.product_id = products.id AND ps0.deleted_at IS NULL)
			OR EXISTS (SELECT 1 FROM product_skus ps1 WHERE ps1.product_id = products.id AND ps1.deleted_at IS NULL AND (ps1.price IS NULL OR ps1.price <= 0))
		)`, product.ImageTypeMain)
}

func (s *Service) readinessWarningTx(ctx context.Context, q Query) *gorm.DB {
	blocked := s.readinessBlockedTx(ctx, q).Select("products.id")
	return s.productTimeScope(ctx, q).
		Where("products.status NOT IN ?", []string{product.StatusArchived}).
		Where("products.id NOT IN (?)", blocked).
		Where(`(
			TRIM(COALESCE(products.ai_title,'')) = ''
			OR (TRIM(COALESCE(products.ai_description,'')) = '' AND LENGTH(TRIM(COALESCE(products.description,''))) < ?)
		)`, descShort)
}

func (s *Service) readinessReadyTx(ctx context.Context, q Query) *gorm.DB {
	blocked := s.readinessBlockedTx(ctx, q).Select("products.id")
	return s.productTimeScope(ctx, q).
		Where("products.status NOT IN ?", []string{product.StatusArchived}).
		Where("products.id NOT IN (?)", blocked).
		Where("TRIM(COALESCE(products.ai_title,'')) <> ''").
		Where("(TRIM(COALESCE(products.ai_description,'')) <> '' OR LENGTH(TRIM(COALESCE(products.description,''))) >= ?)", descShort)
}

func (s *Service) missingAiTitleTx(ctx context.Context, q Query) *gorm.DB {
	return s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where("TRIM(COALESCE(products.ai_title,'')) = ''").
		Where("( TRIM(COALESCE(products.title,'')) <> '' OR TRIM(COALESCE(products.original_title,'')) <> '' )")
}

func (s *Service) missingAiDescriptionTx(ctx context.Context, q Query) *gorm.DB {
	return s.productTimeScope(ctx, q).
		Where("products.status <> ?", product.StatusArchived).
		Where("TRIM(COALESCE(products.ai_description,'')) = ''").
		Where("( TRIM(COALESCE(products.description,'')) = '' OR LENGTH(TRIM(COALESCE(products.description,''))) < ? )", descShort)
}

func (s *Service) publishableProductsTx(ctx context.Context, q Query) *gorm.DB {
	return s.productTimeScope(ctx, q).
		Where("products.status IN ?", []string{product.StatusDraft, product.StatusReady}).
		Where("( TRIM(COALESCE(products.title,'')) <> '' OR TRIM(COALESCE(products.original_title,'')) <> '' )").
		Where("TRIM(COALESCE(products.currency,'')) <> ''").
		Where(`EXISTS (
			SELECT 1 FROM product_images pi
			WHERE pi.product_id = products.id AND pi.image_type = ? AND pi.deleted_at IS NULL
		)`, product.ImageTypeMain).
		Where(`EXISTS (
			SELECT 1 FROM product_skus ps
			WHERE ps.product_id = products.id AND ps.deleted_at IS NULL
		)`).
		Where(`NOT EXISTS (
			SELECT 1 FROM product_skus ps2
			WHERE ps2.product_id = products.id AND ps2.deleted_at IS NULL AND (ps2.price IS NULL OR ps2.price <= 0)
		)`)
}

func buildTodoCards(sum *Summary, publishable int64) []TodoCard {
	return []TodoCard{
		{ID: "missing_ai_title", Title: "待 AI 标题优化", Count: sum.MissingAiTitleCount, Severity: failureclassifier.SeverityMedium,
			Description: "已有标题但尚未生成 AI 标题", Link: "/product/drafts?missingAiTitle=1"},
		{ID: "missing_ai_description", Title: "待 AI 描述生成", Count: sum.MissingAiDescriptionCount, Severity: failureclassifier.SeverityMedium,
			Description: "描述缺失或过短且尚无 AI 描述", Link: "/product/drafts?missingAiDescription=1"},
		{ID: "readiness_blocked", Title: "发布检查不通过（近似）", Count: sum.ReadinessBlocked, Severity: failureclassifier.SeverityHigh,
			Description: "缺标题/主图/SKU/有效价格/币种等", Link: "/product/drafts?readiness=blocked"},
		{ID: "publishable", Title: "待刊登商品（基础完备）", Count: publishable, Severity: failureclassifier.SeverityLow,
			Description: "草稿/就绪且具备主图、SKU 与有效价格", Link: "/product/drafts?publishable=1"},
		{ID: "publish_failed", Title: "刊登失败任务", Count: sum.PublishFailedTasks, Severity: failureclassifier.SeverityHigh,
			Description: "商品刊登任务处于失败态", Link: "/product/publish-tasks?status=failed"},
		{ID: "inventory_alerts", Title: "库存预警 SKU", Count: sum.LowStockSkus + sum.OutOfStockSkus, Severity: failureclassifier.SeverityHigh,
			Description: "低库存或缺货（按预警策略）", Link: "/inventory/alerts"},
		{ID: "inventory_sync_failed", Title: "库存同步失败", Count: sum.InventorySyncFailedCount, Severity: failureclassifier.SeverityHigh,
			Description: "最近同步任务失败（刊登映射维度）", Link: "/inventory/sync-tasks?status=failed"},
		{ID: "customer_pending", Title: "客服待回复", Count: sum.CustomerPendingConversations, Severity: failureclassifier.SeverityMedium,
			Description: "会话处于开放或待回复状态", Link: "/customer/conversations?status=open"},
		{ID: "failures", Title: "失败任务（汇总）", Count: sum.FailedTaskTotal, Severity: failureclassifier.SeverityHigh,
			Description: "统一失败任务中心统计", Link: "/task-center/failures"},
	}
}

func defaultQuickLinks() []QuickLink {
	return []QuickLink{
		{Title: "批量 AI 商品运营", Link: "/ai/batches"},
		{Title: "商品草稿（批量发布检查）", Link: "/product/drafts"},
		{Title: "商品刊登任务", Link: "/product/publish-tasks"},
		{Title: "库存预警", Link: "/inventory/alerts"},
		{Title: "库存同步任务", Link: "/inventory/sync-tasks"},
		{Title: "失败任务中心", Link: "/task-center/failures"},
		{Title: "客服会话", Link: "/customer/conversations"},
	}
}

func (s *Service) buildRecent(ctx context.Context, q Query, shopPtr *uuid.UUID) RecentBuckets {
	var b RecentBuckets

	// Collected products
	{
		qb := s.DB.WithContext(ctx).Table("collect_tasks AS t").
			Select("t.id, t.result_product_id AS result_product_id, t.updated_at, t.source, p.title AS prod_title").
			Joins("INNER JOIN products p ON p.id = t.result_product_id AND p.deleted_at IS NULL").
			Where("t.status = ? AND t.result_product_id IS NOT NULL", collect.StatusSuccess)
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
		}
		_ = qb.Order("t.updated_at DESC").Limit(recentLimit).Scan(&r2).Error
		for _, r := range r2 {
			b.CollectedProducts = append(b.CollectedProducts, RecentItem{
				Type:       "collect",
				Title:      clip(r.ProdTitle, 80),
				Subtitle:   r.Source,
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/product/drafts/%s", r.ResultProduct.String()),
			})
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
				Subtitle:   r.OperationType + " · " + r.Status,
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/ai/batches?id=%s", r.ID.String()),
			})
		}
	}

	// Publish tasks
	{
		var rows []productpublish.ProductPublishTask
		tx := s.publishTaskScope(ctx, q).Order("updated_at DESC").Limit(recentLimit)
		_ = tx.Find(&rows).Error
		for _, r := range rows {
			st := clip(r.ErrorMessage, 120)
			b.PublishTasks = append(b.PublishTasks, RecentItem{
				Type:       "product_publish",
				Title:      r.Platform + " · " + r.Status,
				Subtitle:   st,
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
					Subtitle:   strings.Join(e.AlertTypes, ","),
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
				Subtitle:   r.Platform + " · " + r.Status,
				OccurredAt: r.UpdatedAt,
				Link:       fmt.Sprintf("/customer/conversations/%s", r.ID.String()),
			})
		}
	}

	// Recent hard failures (publish + inventory sync)
	{
		var pubF []productpublish.ProductPublishTask
		tx := s.publishTaskScope(ctx, q).Where("status = ?", productpublish.TaskFailed).Order("updated_at DESC").Limit(recentLimit)
		_ = tx.Find(&pubF).Error
		for _, r := range pubF {
			b.FailedTasks = append(b.FailedTasks, RecentItem{
				Type:       "failed_publish",
				Title:      r.Platform + " · 刊登失败",
				Subtitle:   clip(r.ErrorMessage, 100),
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
				OccurredAt: r.UpdatedAt,
				Link:       "/inventory/sync-tasks?status=failed",
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
				Subtitle:   r.Severity + " · " + r.TaskType,
				OccurredAt: r.LastSeenAt,
				Link:       "/task-center/alerts",
			})
		}
	}

	return b
}

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
