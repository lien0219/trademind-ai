package taskcenter

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
)

func detailURL(taskType, id string) string {
	switch taskType {
	case TaskTypeCollect:
		return "/collect/tasks?id=" + url.QueryEscape(id)
	case TaskTypeImage:
		return "/ai/image-tasks?id=" + url.QueryEscape(id)
	case TaskTypeOrderSync:
		return "/orders/sync-tasks?id=" + url.QueryEscape(id)
	case TaskTypeCustomerMessageSync:
		return "/customer/message-sync-tasks?id=" + url.QueryEscape(id)
	case TaskTypeProductPublish:
		return "/product/publish-tasks?id=" + url.QueryEscape(id)
	case TaskTypeInventorySync:
		return "/inventory/sync-tasks?id=" + url.QueryEscape(id)
	default:
		return ""
	}
}

func retryActionFor(taskType string) string {
	switch taskType {
	case TaskTypeCollect:
		return "POST /api/v1/collect/tasks/:id/retry"
	case TaskTypeImage:
		return "POST /api/v1/image/tasks/:id/retry"
	case TaskTypeOrderSync:
		return "POST /api/v1/order-sync/tasks/:id/retry"
	case TaskTypeCustomerMessageSync:
		return "POST /api/v1/customer/message-sync/tasks/:id/retry"
	case TaskTypeProductPublish:
		return "POST /api/v1/product-publish/tasks/:id/retry"
	case TaskTypeInventorySync:
		return "POST /api/v1/inventory-sync/tasks/:id/retry"
	default:
		return ""
	}
}

func collectTitle(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return "采集任务"
	}
	if p, err := url.Parse(u); err == nil && p.Host != "" {
		return truncateRunes(p.Host+" · "+truncateRunes(pathTail(u), 80), 200)
	}
	return truncateRunes(u, 200)
}

func pathTail(u string) string {
	if i := strings.LastIndexAny(u, "/"); i >= 0 && i+1 < len(u) {
		return u[i+1:]
	}
	return u
}

func mapCollectTask(row *collect.CollectTask, productTitles map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	norm := normalizeFromLease(now, leaseFields{
		Status:      row.Status,
		LockedUntil: row.LockedUntil,
		LockedBy:    row.LockedBy,
		NextRetryAt: row.NextRetryAt,
		UpdatedAt:   row.UpdatedAt,
	})
	dto := UnifiedTaskDTO{
		ID:               row.ID.String(),
		TaskType:         TaskTypeCollect,
		SourceTable:      SourceTableCollectTasks,
		SourceID:         row.ID.String(),
		Platform:         strings.TrimSpace(row.Source),
		Title:            collectTitle(row.SourceURL),
		Status:           row.Status,
		NormalizedStatus: norm,
		Retryable:        norm == NormFailed,
		ErrorMessage:     truncateRunes(row.ErrorMessage, maxErrorMessageLen),
		ErrorCode:        collect.InferErrorCodeFromMessage(row.ErrorMessage),
		RetryCount:       row.RetryCount,
		MaxRetries:       row.MaxRetries,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		StartedAt:        row.StartedAt,
		FinishedAt:       row.FinishedAt,
		DetailURL:        detailURL(TaskTypeCollect, row.ID.String()),
		RetryAction:      retryActionFor(TaskTypeCollect),
		RawSummary:       truncateRunes("source="+row.Source+" "+row.SourceURL, maxRawSummaryLen),
		SortKey:          row.UpdatedAt,
	}
	if row.LockedBy != nil {
		dto.LockedBy = strings.TrimSpace(*row.LockedBy)
	}
	dto.LockedUntil = row.LockedUntil
	if row.ResultProductID != nil {
		pid := *row.ResultProductID
		dto.RelatedResourceType = "product"
		dto.RelatedResourceID = pid.String()
		if t := productTitles[pid]; t != "" {
			dto.RelatedResourceTitle = truncateRunes(t, 200)
		}
	} else {
		dto.RelatedResourceType = "collect_url"
		dto.RelatedResourceTitle = truncateRunes(row.SourceURL, 120)
	}
	applyMarks(&dto, TaskTypeCollect, row.ID.String(), marks)
	return dto
}

func mapImageTask(row *imagetask.ImageTask, productTitles map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	norm := normalizeFromLease(now, leaseFields{
		Status:      row.Status,
		LockedUntil: row.LockedUntil,
		LockedBy:    row.LockedBy,
		NextRetryAt: row.NextRetryAt,
		UpdatedAt:   row.UpdatedAt,
	})
	dto := UnifiedTaskDTO{
		ID:               row.ID.String(),
		TaskType:         TaskTypeImage,
		SourceTable:      SourceTableImageTasks,
		SourceID:         row.ID.String(),
		Title:            truncateRunes(row.TaskType+" · "+row.Provider, 240),
		Status:           row.Status,
		NormalizedStatus: norm,
		Retryable:        norm == NormFailed,
		ErrorMessage:     truncateRunes(row.ErrorMessage, maxErrorMessageLen),
		RetryCount:       row.RetryCount,
		MaxRetries:       row.MaxRetries,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		StartedAt:        row.StartedAt,
		FinishedAt:       row.FinishedAt,
		DetailURL:        detailURL(TaskTypeImage, row.ID.String()),
		RetryAction:      retryActionFor(TaskTypeImage),
		SortKey:          row.UpdatedAt,
	}
	if row.LockedBy != nil {
		dto.LockedBy = strings.TrimSpace(*row.LockedBy)
	}
	dto.LockedUntil = row.LockedUntil
	dto.RawSummary = truncateRunes("imageTaskType="+row.TaskType+" provider="+row.Provider, maxRawSummaryLen)
	if row.ProductID != nil {
		pid := *row.ProductID
		dto.RelatedResourceType = "product"
		dto.RelatedResourceID = pid.String()
		if t := productTitles[pid]; t != "" {
			dto.RelatedResourceTitle = truncateRunes(t, 200)
		}
	} else if row.SourceImageURL != "" {
		dto.RelatedResourceType = "image"
		dto.RelatedResourceTitle = truncateRunes(row.SourceImageURL, 120)
	}
	applyMarks(&dto, TaskTypeImage, row.ID.String(), marks)
	return dto
}

func mapOrderSyncTask(row *ordersync.OrderSyncTask, shopNames map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	norm := normalizeFromLease(now, leaseFields{
		Status:      row.Status,
		LockedUntil: row.LockedUntil,
		LockedBy:    row.LockedBy,
		NextRetryAt: nil,
		UpdatedAt:   row.UpdatedAt,
	})
	dto := UnifiedTaskDTO{
		ID:                   row.ID.String(),
		TaskType:             TaskTypeOrderSync,
		SourceTable:          SourceTableOrderSyncTasks,
		SourceID:             row.ID.String(),
		Title:                truncateRunes("订单同步 · "+row.Platform, 240),
		Platform:             row.Platform,
		ShopID:               row.ShopID.String(),
		ShopName:             truncateRunes(shopNames[row.ShopID], 255),
		RelatedResourceType:  "shop",
		RelatedResourceID:    row.ShopID.String(),
		RelatedResourceTitle: truncateRunes(shopNames[row.ShopID], 255),
		Status:               row.Status,
		NormalizedStatus:     norm,
		Retryable:            norm == NormFailed,
		ErrorMessage:         truncateRunes(row.ErrorMessage, maxErrorMessageLen),
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		StartedAt:            row.StartedAt,
		FinishedAt:           row.FinishedAt,
		DetailURL:            detailURL(TaskTypeOrderSync, row.ID.String()),
		RetryAction:          retryActionFor(TaskTypeOrderSync),
		RawSummary:           truncateRunes("mode="+row.Mode, maxRawSummaryLen),
		SortKey:              row.UpdatedAt,
	}
	if row.LockedBy != nil {
		dto.LockedBy = strings.TrimSpace(*row.LockedBy)
	}
	dto.LockedUntil = row.LockedUntil
	applyMarks(&dto, TaskTypeOrderSync, row.ID.String(), marks)
	return dto
}

func mapCustomerMessageSyncTask(row *customersync.CustomerMessageSyncTask, shopNames map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	norm := normalizeFromLease(now, leaseFields{
		Status:      row.Status,
		LockedUntil: row.LockedUntil,
		LockedBy:    row.LockedBy,
		NextRetryAt: nil,
		UpdatedAt:   row.UpdatedAt,
	})
	dto := UnifiedTaskDTO{
		ID:                   row.ID.String(),
		TaskType:             TaskTypeCustomerMessageSync,
		SourceTable:          SourceTableCustomerMessageSyncTasks,
		SourceID:             row.ID.String(),
		Title:                truncateRunes("客服消息同步 · "+row.Platform, 240),
		Platform:             row.Platform,
		ShopID:               row.ShopID.String(),
		ShopName:             truncateRunes(shopNames[row.ShopID], 255),
		RelatedResourceType:  "shop",
		RelatedResourceID:    row.ShopID.String(),
		RelatedResourceTitle: truncateRunes(shopNames[row.ShopID], 255),
		Status:               row.Status,
		NormalizedStatus:     norm,
		Retryable:            norm == NormFailed,
		ErrorMessage:         truncateRunes(row.ErrorMessage, maxErrorMessageLen),
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		StartedAt:            row.StartedAt,
		FinishedAt:           row.FinishedAt,
		DetailURL:            detailURL(TaskTypeCustomerMessageSync, row.ID.String()),
		RetryAction:          retryActionFor(TaskTypeCustomerMessageSync),
		RawSummary:           truncateRunes("mode="+row.Mode, maxRawSummaryLen),
		SortKey:              row.UpdatedAt,
	}
	if row.LockedBy != nil {
		dto.LockedBy = strings.TrimSpace(*row.LockedBy)
	}
	dto.LockedUntil = row.LockedUntil
	applyMarks(&dto, TaskTypeCustomerMessageSync, row.ID.String(), marks)
	return dto
}

func mapProductPublishTask(row *productpublish.ProductPublishTask, shopNames map[uuid.UUID]string, productTitles map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	norm := normalizeFromLease(now, leaseFields{
		Status:      row.Status,
		LockedUntil: row.LockedUntil,
		LockedBy:    row.LockedBy,
		NextRetryAt: nil,
		UpdatedAt:   row.UpdatedAt,
	})
	title := productTitles[row.ProductID]
	if title == "" {
		title = "商品刊登"
	}
	dto := UnifiedTaskDTO{
		ID:                   row.ID.String(),
		TaskType:             TaskTypeProductPublish,
		SourceTable:          SourceTableProductPublishTasks,
		SourceID:             row.ID.String(),
		Title:                truncateRunes("刊登 · "+row.Platform, 240),
		Platform:             row.Platform,
		ShopID:               row.ShopID.String(),
		ShopName:             truncateRunes(shopNames[row.ShopID], 255),
		RelatedResourceType:  "product",
		RelatedResourceID:    row.ProductID.String(),
		RelatedResourceTitle: truncateRunes(title, 255),
		Status:               row.Status,
		NormalizedStatus:     norm,
		Retryable:            norm == NormFailed,
		ErrorMessage:         truncateRunes(row.ErrorMessage, maxErrorMessageLen),
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
		StartedAt:            row.StartedAt,
		FinishedAt:           row.FinishedAt,
		DetailURL:            detailURL(TaskTypeProductPublish, row.ID.String()),
		RetryAction:          retryActionFor(TaskTypeProductPublish),
		RawSummary:           truncateRunes("mode="+row.Mode, maxRawSummaryLen),
		SortKey:              row.UpdatedAt,
	}
	if row.LockedBy != nil {
		dto.LockedBy = strings.TrimSpace(*row.LockedBy)
	}
	dto.LockedUntil = row.LockedUntil
	applyMarks(&dto, TaskTypeProductPublish, row.ID.String(), marks)
	return dto
}

func mapInventorySyncTask(row *inventory.InventorySyncTask, shopNames map[uuid.UUID]string, productTitles map[uuid.UUID]string, marks markSet, now time.Time) UnifiedTaskDTO {
	if row == nil {
		return UnifiedTaskDTO{}
	}
	norm := normalizeFromLease(now, leaseFields{
		Status:      row.Status,
		LockedUntil: row.LockedUntil,
		LockedBy:    row.LockedBy,
		NextRetryAt: nil,
		UpdatedAt:   row.UpdatedAt,
	})
	ptitle := productTitles[row.ProductID]
	dto := UnifiedTaskDTO{
		ID:               row.ID.String(),
		TaskType:         TaskTypeInventorySync,
		SourceTable:      SourceTableInventorySyncTasks,
		SourceID:         row.ID.String(),
		Title:            truncateRunes("库存同步 · "+row.Platform, 240),
		Platform:         row.Platform,
		ShopID:           row.ShopID.String(),
		ShopName:         truncateRunes(shopNames[row.ShopID], 255),
		Status:           row.Status,
		NormalizedStatus: norm,
		Retryable:        norm == NormFailed,
		ErrorMessage:     truncateRunes(row.ErrorMessage, maxErrorMessageLen),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
		StartedAt:        row.StartedAt,
		FinishedAt:       row.FinishedAt,
		DetailURL:        detailURL(TaskTypeInventorySync, row.ID.String()),
		RetryAction:      retryActionFor(TaskTypeInventorySync),
		RawSummary:       truncateRunes("targetStock="+strconv.Itoa(row.TargetStock), maxRawSummaryLen),
		SortKey:          row.UpdatedAt,
	}
	if row.LockedBy != nil {
		dto.LockedBy = strings.TrimSpace(*row.LockedBy)
	}
	dto.LockedUntil = row.LockedUntil
	dto.RelatedResourceType = "product_sku"
	if row.ProductSKUID != nil {
		dto.RelatedResourceID = row.ProductSKUID.String()
	} else {
		dto.RelatedResourceID = row.ProductID.String()
	}
	dto.RelatedResourceTitle = truncateRunes(ptitle, 255)
	applyMarks(&dto, TaskTypeInventorySync, row.ID.String(), marks)
	return dto
}
