package taskcenter

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"gorm.io/gorm"
)

func (s *Service) listOneType(ctx context.Context, taskType string, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	switch taskType {
	case TaskTypeCollect:
		return s.listCollect(ctx, p, now, fetchLimit)
	case TaskTypeImage:
		return s.listImage(ctx, p, now, fetchLimit)
	case TaskTypeOrderSync:
		return s.listOrderSync(ctx, p, now, fetchLimit)
	case TaskTypeCustomerMessageSync:
		return s.listCustomerSync(ctx, p, now, fetchLimit)
	case TaskTypeProductPublish:
		return s.listProductPublish(ctx, p, now, fetchLimit)
	case TaskTypeInventorySync:
		return s.listInventorySync(ctx, p, now, fetchLimit)
	case TaskTypeAIText:
		return s.listAIProductText(ctx, p, now, fetchLimit)
	default:
		return nil, gorm.ErrRecordNotFound
	}
}

func likePat(kw string) string {
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return ""
	}
	return "%" + kw + "%"
}

func (s *Service) listCollect(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&collect.CollectTask{})
	q = failureRowFilter(s.applyTimeRange(q, p), now, p.IncludeResolved, true)
	q = s.applyMarkFilters(q, TaskTypeCollect, "collect_tasks.id::text", p)
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where("(source_url ILIKE ? OR COALESCE(error_message,'') ILIKE ? OR CAST(id AS TEXT) ILIKE ?)", lk, lk, lk)
	}

	var rows []collect.CollectTask
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	prodIDs := make([]uuid.UUID, 0)
	for i := range rows {
		if rows[i].ResultProductID != nil {
			prodIDs = append(prodIDs, *rows[i].ResultProductID)
		}
	}
	titles := s.batchProductTitles(ctx, prodIDs)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeCollect, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, mapCollectTask(&rows[i], titles, ms, now))
	}
	return out, nil
}

func (s *Service) listImage(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&imagetask.ImageTask{})
	q = failureRowFilter(s.applyTimeRange(q, p), now, p.IncludeResolved, true)
	q = s.applyMarkFilters(q, TaskTypeImage, "image_tasks.id::text", p)
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where("(COALESCE(error_message,'') ILIKE ? OR task_type ILIKE ? OR provider ILIKE ? OR CAST(id AS TEXT) ILIKE ?)", lk, lk, lk, lk)
	}

	var rows []imagetask.ImageTask
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	prodIDs := make([]uuid.UUID, 0)
	for i := range rows {
		if rows[i].ProductID != nil {
			prodIDs = append(prodIDs, *rows[i].ProductID)
		}
	}
	titles := s.batchProductTitles(ctx, prodIDs)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeImage, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, mapImageTask(&rows[i], titles, ms, now))
	}
	return out, nil
}

func (s *Service) listOrderSync(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&ordersync.OrderSyncTask{})
	q = failureRowFilter(s.applyTimeRange(q, p), now, p.IncludeResolved, false)
	q = s.applyMarkFilters(q, TaskTypeOrderSync, "order_sync_tasks.id::text", p)
	if sid := strings.TrimSpace(p.ShopID); sid != "" {
		if u, err := uuid.Parse(sid); err == nil {
			q = q.Where("shop_id = ?", u)
		}
	}
	if pl := strings.TrimSpace(p.Platform); pl != "" {
		q = q.Where("LOWER(platform) = LOWER(?)", pl)
	}
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where("(COALESCE(error_message,'') ILIKE ? OR CAST(id AS TEXT) ILIKE ? OR platform ILIKE ?)", lk, lk, lk)
	}

	var rows []ordersync.OrderSyncTask
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	shopIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		shopIDs = append(shopIDs, rows[i].ShopID)
	}
	names := s.batchShopNames(ctx, shopIDs)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeOrderSync, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, mapOrderSyncTask(&rows[i], names, ms, now))
	}
	return out, nil
}

func (s *Service) listCustomerSync(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&customersync.CustomerMessageSyncTask{})
	q = failureRowFilter(s.applyTimeRange(q, p), now, p.IncludeResolved, false)
	q = s.applyMarkFilters(q, TaskTypeCustomerMessageSync, "customer_message_sync_tasks.id::text", p)
	if sid := strings.TrimSpace(p.ShopID); sid != "" {
		if u, err := uuid.Parse(sid); err == nil {
			q = q.Where("shop_id = ?", u)
		}
	}
	if pl := strings.TrimSpace(p.Platform); pl != "" {
		q = q.Where("LOWER(platform) = LOWER(?)", pl)
	}
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where("(COALESCE(error_message,'') ILIKE ? OR CAST(id AS TEXT) ILIKE ? OR platform ILIKE ?)", lk, lk, lk)
	}

	var rows []customersync.CustomerMessageSyncTask
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	shopIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		shopIDs = append(shopIDs, rows[i].ShopID)
	}
	names := s.batchShopNames(ctx, shopIDs)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeCustomerMessageSync, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, mapCustomerMessageSyncTask(&rows[i], names, ms, now))
	}
	return out, nil
}

func (s *Service) listProductPublish(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&productpublish.ProductPublishTask{})
	q = failureRowFilter(s.applyTimeRange(q, p), now, p.IncludeResolved, false)
	q = s.applyMarkFilters(q, TaskTypeProductPublish, "product_publish_tasks.id::text", p)
	if sid := strings.TrimSpace(p.ShopID); sid != "" {
		if u, err := uuid.Parse(sid); err == nil {
			q = q.Where("shop_id = ?", u)
		}
	}
	if pl := strings.TrimSpace(p.Platform); pl != "" {
		q = q.Where("LOWER(platform) = LOWER(?)", pl)
	}
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where("(COALESCE(error_message,'') ILIKE ? OR CAST(id AS TEXT) ILIKE ? OR platform ILIKE ? OR CAST(product_id AS TEXT) ILIKE ?)", lk, lk, lk, lk)
	}

	var rows []productpublish.ProductPublishTask
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	shopIDs := make([]uuid.UUID, 0, len(rows))
	prodIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		shopIDs = append(shopIDs, rows[i].ShopID)
		prodIDs = append(prodIDs, rows[i].ProductID)
	}
	names := s.batchShopNames(ctx, shopIDs)
	titles := s.batchProductTitles(ctx, prodIDs)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeProductPublish, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, mapProductPublishTask(&rows[i], names, titles, ms, now))
	}
	return out, nil
}

func (s *Service) listInventorySync(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&inventory.InventorySyncTask{})
	q = failureRowFilter(s.applyTimeRange(q, p), now, p.IncludeResolved, false)
	q = s.applyMarkFilters(q, TaskTypeInventorySync, "inventory_sync_tasks.id::text", p)
	if sid := strings.TrimSpace(p.ShopID); sid != "" {
		if u, err := uuid.Parse(sid); err == nil {
			q = q.Where("shop_id = ?", u)
		}
	}
	if pl := strings.TrimSpace(p.Platform); pl != "" {
		q = q.Where("LOWER(platform) = LOWER(?)", pl)
	}
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where("(COALESCE(error_message,'') ILIKE ? OR CAST(id AS TEXT) ILIKE ? OR platform ILIKE ? OR CAST(product_id AS TEXT) ILIKE ? OR CAST(publication_sku_id AS TEXT) ILIKE ?)", lk, lk, lk, lk, lk)
	}

	var rows []inventory.InventorySyncTask
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	shopIDs := make([]uuid.UUID, 0, len(rows))
	prodIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		shopIDs = append(shopIDs, rows[i].ShopID)
		prodIDs = append(prodIDs, rows[i].ProductID)
	}
	names := s.batchShopNames(ctx, shopIDs)
	titles := s.batchProductTitles(ctx, prodIDs)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeInventorySync, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		out = append(out, mapInventorySyncTask(&rows[i], names, titles, ms, now))
	}
	return out, nil
}

func (s *Service) listAIProductText(ctx context.Context, p ListFailureParams, now time.Time, fetchLimit int) ([]UnifiedTaskDTO, error) {
	q := s.DB.WithContext(ctx).Model(&aiproducttext.AIProductTextItem{})
	q = aiTextFailureRowFilter(s.applyTimeRange(q, p), p.IncludeResolved)
	q = s.applyMarkFilters(q, TaskTypeAIText, "ai_product_text_items.id::text", p)
	if lk := likePat(p.Keyword); lk != "" {
		q = q.Where(`(
			COALESCE(error_message,'') ILIKE ?
			OR CAST(id AS TEXT) ILIKE ?
			OR CAST(batch_id AS TEXT) ILIKE ?
			OR CAST(product_id AS TEXT) ILIKE ?
			OR operation_type ILIKE ?
		)`, lk, lk, lk, lk, lk)
	}
	if fc := strings.TrimSpace(p.FailureCategory); fc != "" {
		switch fc {
		case CategoryAITextGenerationFailed:
			q = q.Where("status = ?", aiproducttext.ItemFailed)
		case CategoryAITextApplyConflict:
			q = q.Where("status = ?", aiproducttext.ItemConflict)
		case CategoryAITextQualityWarning:
			q = q.Where("status IN ?", []string{aiproducttext.ItemPendingReview, aiproducttext.ItemSuccess}).
				Where("quality_warnings IS NOT NULL AND TRIM(quality_warnings::text) NOT IN ('null', '[]', '')")
		}
	}

	var rows []aiproducttext.AIProductTextItem
	if err := q.Order("updated_at DESC").Limit(fetchLimit).Find(&rows).Error; err != nil {
		return nil, err
	}
	prodIDs := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		prodIDs = append(prodIDs, rows[i].ProductID)
	}
	titles := s.batchProductTitles(ctx, prodIDs)
	ids := make([]string, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID.String()
	}
	ms, err := s.fetchMarks(ctx, TaskTypeAIText, ids)
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTaskDTO, 0, len(rows))
	for i := range rows {
		dto := mapAIProductTextItem(&rows[i], titles, ms, now)
		if fc := strings.TrimSpace(p.FailureCategory); fc != "" && !strings.EqualFold(fc, dto.FailureCategory) {
			continue
		}
		out = append(out, dto)
	}
	return out, nil
}
