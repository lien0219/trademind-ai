package taskcenter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/modules/customersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/productpublish"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"gorm.io/gorm"
)

func adminFromGin(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func (s *Service) unifiedOne(ctx context.Context, taskType string, id uuid.UUID, now time.Time) (UnifiedTaskDTO, error) {
	var zero UnifiedTaskDTO
	ms, err := s.fetchMarks(ctx, taskType, []string{id.String()})
	if err != nil {
		return zero, err
	}
	switch taskType {
	case TaskTypeCollect:
		var row collect.CollectTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
			return zero, err
		}
		pids := productIDsFromCollect(&row)
		titles := s.batchProductTitles(ctx, pids)
		return mapCollectTask(&row, titles, ms, now), nil
	case TaskTypeImage:
		var row imagetask.ImageTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
			return zero, err
		}
		var pids []uuid.UUID
		if row.ProductID != nil {
			pids = append(pids, *row.ProductID)
		}
		titles := s.batchProductTitles(ctx, pids)
		return mapImageTask(&row, titles, ms, now), nil
	case TaskTypeOrderSync:
		var row ordersync.OrderSyncTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
			return zero, err
		}
		names := s.batchShopNames(ctx, []uuid.UUID{row.ShopID})
		return mapOrderSyncTask(&row, names, ms, now), nil
	case TaskTypeCustomerMessageSync:
		var row customersync.CustomerMessageSyncTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
			return zero, err
		}
		names := s.batchShopNames(ctx, []uuid.UUID{row.ShopID})
		return mapCustomerMessageSyncTask(&row, names, ms, now), nil
	case TaskTypeProductPublish:
		var row productpublish.ProductPublishTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
			return zero, err
		}
		names := s.batchShopNames(ctx, []uuid.UUID{row.ShopID})
		titles := s.batchProductTitles(ctx, []uuid.UUID{row.ProductID})
		return mapProductPublishTask(&row, names, titles, ms, now), nil
	case TaskTypeInventorySync:
		var row inventory.InventorySyncTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
			return zero, err
		}
		names := s.batchShopNames(ctx, []uuid.UUID{row.ShopID})
		titles := s.batchProductTitles(ctx, []uuid.UUID{row.ProductID})
		return mapInventorySyncTask(&row, names, titles, ms, now), nil
	default:
		return zero, fmt.Errorf("unknown task type")
	}
}

func productIDsFromCollect(row *collect.CollectTask) []uuid.UUID {
	if row != nil && row.ResultProductID != nil {
		return []uuid.UUID{*row.ResultProductID}
	}
	return nil
}

// GetFailureDetail returns one row with optional type-specific summaries (no secrets / no big JSON).
func (s *Service) GetFailureDetail(c *gin.Context, taskTypeRaw string, id uuid.UUID) (*FailureDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("taskcenter: no db")
	}
	taskType, err := parseTaskType(taskTypeRaw)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	base, err := s.unifiedOne(c.Request.Context(), taskType, id, now)
	if err != nil {
		return nil, err
	}
	if err := s.ClassifyOne(c.Request.Context(), &base); err != nil {
		return nil, err
	}
	out := &FailureDetailDTO{UnifiedTaskDTO: base, Extra: map[string]any{}}
	ctx := c.Request.Context()

	switch taskType {
	case TaskTypeCollect:
		var row collect.CollectTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err == nil {
			out.Extra["sourceUrl"] = truncateRunes(row.SourceURL, 512)
			urlType, accessStatus, suggested := collectFailureContextExtras(row.SourceURL, row.ErrorMessage, base.FailureCategory, base.SuggestedAction)
			if urlType != "" {
				out.Extra["urlTypeLabel"] = urlType
			}
			if accessStatus != "" {
				out.Extra["accessStatusLabel"] = accessStatus
			}
			if suggested != "" {
				out.Extra["suggestedActionDetail"] = suggested
			}
		}
		var events []collect.CollectTaskEvent
		_ = s.DB.WithContext(ctx).
			Model(&collect.CollectTaskEvent{}).
			Where("task_id = ?", id).
			Order("created_at DESC").
			Limit(detailEventsCollectLimit).
			Find(&events).Error
		evs := make([]map[string]string, 0, len(events))
		for i := range events {
			evs = append(evs, map[string]string{
				"eventType":    truncateRunes(events[i].EventType, 96),
				"message":      truncateRunes(events[i].Message, 240),
				"errorMessage": truncateRunes(events[i].ErrorMessage, 240),
				"createdAt":    events[i].CreatedAt.UTC().Format(time.RFC3339),
			})
		}
		out.Extra["collectEvents"] = evs
	case TaskTypeImage:
		var row imagetask.ImageTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err == nil {
			out.Extra["imageTaskType"] = row.TaskType
			out.Extra["provider"] = row.Provider
			out.Extra["retryCount"] = row.RetryCount
			if row.NextRetryAt != nil {
				out.Extra["nextRetryAt"] = row.NextRetryAt.UTC().Format(time.RFC3339)
			}
		}
	case TaskTypeOrderSync:
		var row ordersync.OrderSyncTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err == nil {
			out.Extra["mode"] = row.Mode
			out.Extra["successCount"] = row.SuccessCount
			out.Extra["failedCount"] = row.FailedCount
			mergeRecoveryExtra(out.Extra, row.Output)
		}
	case TaskTypeCustomerMessageSync:
		var row customersync.CustomerMessageSyncTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err == nil {
			out.Extra["mode"] = row.Mode
			out.Extra["totalCount"] = row.TotalCount
			out.Extra["failedCount"] = row.FailedCount
		}
	case TaskTypeProductPublish:
		var row productpublish.ProductPublishTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err == nil {
			out.Extra["productId"] = row.ProductID.String()
			out.Extra["platform"] = row.Platform
			mergeRecoveryExtra(out.Extra, row.Output)
			var pub productpublish.ProductPublication
			if err := s.DB.WithContext(ctx).Where("publish_task_id = ?", id).Take(&pub).Error; err == nil {
				out.Extra["externalProductId"] = truncateRunes(pub.ExternalProductID, 256)
			}
		}
	case TaskTypeInventorySync:
		var row inventory.InventorySyncTask
		if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err == nil {
			out.Extra["targetStock"] = row.TargetStock
			out.Extra["productId"] = row.ProductID.String()
			mergeRecoveryExtra(out.Extra, row.Output)
			if row.ProductSKUID != nil {
				out.Extra["productSkuId"] = row.ProductSKUID.String()
			}
		}
	}
	return out, nil
}

func (s *Service) deleteFailureMarks(ctx context.Context, taskType, sourceID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	return s.DB.WithContext(ctx).
		Where("task_type = ? AND source_id = ?", taskType, sourceID).
		Delete(&TaskFailureMark{}).Error
}

func (s *Service) upsertFailureMark(ctx context.Context, taskType, sourceID, sourceTable, markType, remark string, admin *uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	remark = truncateRunes(strings.TrimSpace(remark), 2000)
	now := time.Now().UTC()
	var cur TaskFailureMark
	err := s.DB.WithContext(ctx).
		Where("task_type = ? AND source_id = ? AND mark_type = ?", taskType, sourceID, markType).
		First(&cur).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		m := TaskFailureMark{
			TaskType:    taskType,
			SourceID:    sourceID,
			SourceTable: sourceTable,
			MarkType:    markType,
			Remark:      remark,
			CreatedBy:   admin,
		}
		m.CreatedAt = now
		m.UpdatedAt = now
		return s.DB.WithContext(ctx).Create(&m).Error
	}
	if err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Model(&TaskFailureMark{}).
		Where("id = ?", cur.ID).
		Updates(map[string]any{"remark": remark, "updated_at": now, "created_by": admin}).Error
}

// RetryFailure dispatches retry to owning module then clears marks and writes audit.
func (s *Service) RetryFailure(c *gin.Context, taskTypeRaw string, id uuid.UUID) error {
	if s == nil {
		return fmt.Errorf("taskcenter: unavailable")
	}
	taskType, err := parseTaskType(taskTypeRaw)
	if err != nil {
		return err
	}
	admin := adminFromGin(c)
	base, err := s.unifiedOne(c.Request.Context(), taskType, id, time.Now().UTC())
	if err != nil {
		return err
	}
	if !base.Retryable {
		return fmt.Errorf("task not retryable in current status")
	}
	var execErr error
	switch taskType {
	case TaskTypeCollect:
		if s.Collect == nil {
			return fmt.Errorf("collect service unavailable")
		}
		if !s.Collect.QueueEnabled {
			return fmt.Errorf("collect queue disabled; use original retry on collect page")
		}
		_, execErr = s.Collect.RetryAsync(c, id, admin)
	case TaskTypeImage:
		if s.Image == nil {
			return fmt.Errorf("image service unavailable")
		}
		execErr = s.Image.RetryEnqueue(c, id)
	case TaskTypeOrderSync:
		if s.OrderSync == nil {
			return fmt.Errorf("order sync unavailable")
		}
		_, execErr = s.OrderSync.RetryFailed(c, id, admin)
	case TaskTypeCustomerMessageSync:
		if s.CustomerSync == nil {
			return fmt.Errorf("customer message sync unavailable")
		}
		_, execErr = s.CustomerSync.RetryFailed(c, id, admin)
	case TaskTypeProductPublish:
		if s.ProductPublish == nil {
			return fmt.Errorf("product publish unavailable")
		}
		_, execErr = s.ProductPublish.RetryFailed(c, id, admin)
	case TaskTypeInventorySync:
		if s.Inventory == nil {
			return fmt.Errorf("inventory sync unavailable")
		}
		_, execErr = s.Inventory.RetryFailed(c, id, admin)
	default:
		return fmt.Errorf("unsupported task type for retry")
	}
	if execErr != nil {
		return execErr
	}
	if err := s.deleteFailureMarks(c.Request.Context(), taskType, id.String()); err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.retry",
			Resource:   "task_center",
			ResourceID: taskType + "/" + id.String(),
			Status:     "success",
			Message:    truncateRunes(fmt.Sprintf("taskType=%s id=%s", taskType, id.String()), 2000),
		})
	}
	return nil
}

// BatchRetryFailure retries multiple tasks; isolated errors per row.
func (s *Service) BatchRetryFailure(c *gin.Context, req BatchRetryRequest) BatchRetryResponse {
	out := BatchRetryResponse{Results: make([]BatchRetryOneResult, 0, len(req.Items))}
	if len(req.Items) > maxBatchItems {
		for _, it := range req.Items {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: it.TaskType, ID: it.ID, OK: false, Error: fmt.Sprintf("batch exceeds %d", maxBatchItems)})
			out.FailedCount++
		}
		return out
	}
	for _, it := range req.Items {
		id, err := uuid.Parse(strings.TrimSpace(it.ID))
		if err != nil {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: it.TaskType, ID: it.ID, OK: false, Error: "invalid id"})
			out.FailedCount++
			continue
		}
		tt, perr := parseTaskType(it.TaskType)
		if perr != nil {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: it.TaskType, ID: it.ID, OK: false, Error: "invalid taskType"})
			out.FailedCount++
			continue
		}
		if err := s.RetryFailure(c, tt, id); err != nil {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: tt, ID: id.String(), OK: false, Error: err.Error()})
			out.FailedCount++
			continue
		}
		out.Results = append(out.Results, BatchRetryOneResult{TaskType: tt, ID: id.String(), OK: true})
		out.SuccessCount++
	}
	if s.OpLog != nil {
		sum := truncateRunes("success="+strconv.Itoa(out.SuccessCount)+" failed="+strconv.Itoa(out.FailedCount), 2000)
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.batch_retry",
			Resource:   "task_center",
			ResourceID: "batch",
			Status:     "success",
			Message:    sum,
		})
	}
	return out
}

func sourceTableForType(taskType string) string {
	switch taskType {
	case TaskTypeCollect:
		return SourceTableCollectTasks
	case TaskTypeImage:
		return SourceTableImageTasks
	case TaskTypeOrderSync:
		return SourceTableOrderSyncTasks
	case TaskTypeCustomerMessageSync:
		return SourceTableCustomerMessageSyncTasks
	case TaskTypeProductPublish:
		return SourceTableProductPublishTasks
	case TaskTypeInventorySync:
		return SourceTableInventorySyncTasks
	default:
		return "unknown"
	}
}

// IgnoreFailure records an ignored mark (does not change source task).
func (s *Service) IgnoreFailure(c *gin.Context, taskType string, id uuid.UUID, remark string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	tt, err := parseTaskType(taskType)
	if err != nil {
		return err
	}
	if _, err := s.unifiedOne(c.Request.Context(), tt, id, time.Now().UTC()); err != nil {
		return err
	}
	src := sourceTableForType(tt)
	if err := s.upsertFailureMark(c.Request.Context(), tt, id.String(), src, MarkIgnored, remark, adminFromGin(c)); err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.ignore",
			Resource:   "task_center",
			ResourceID: tt + "/" + id.String(),
			Status:     "success",
			Message:    truncateRunes("markType="+MarkIgnored, 2000),
		})
	}
	return nil
}

// HandleFailure records handled mark (does not change source task).
func (s *Service) HandleFailure(c *gin.Context, taskType string, id uuid.UUID, remark string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	tt, err := parseTaskType(taskType)
	if err != nil {
		return err
	}
	if _, err := s.unifiedOne(c.Request.Context(), tt, id, time.Now().UTC()); err != nil {
		return err
	}
	src := sourceTableForType(tt)
	if err := s.upsertFailureMark(c.Request.Context(), tt, id.String(), src, MarkHandled, remark, adminFromGin(c)); err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.handle",
			Resource:   "task_center",
			ResourceID: tt + "/" + id.String(),
			Status:     "success",
			Message:    truncateRunes("markType="+MarkHandled, 2000),
		})
	}
	return nil
}

// UnmarkFailure removes ignored/handled marks for a row.
func (s *Service) UnmarkFailure(c *gin.Context, taskType string, id uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	tt, err := parseTaskType(taskType)
	if err != nil {
		return err
	}
	if _, err := s.unifiedOne(c.Request.Context(), tt, id, time.Now().UTC()); err != nil {
		return err
	}
	if err := s.deleteFailureMarks(c.Request.Context(), tt, id.String()); err != nil {
		return err
	}
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "task_center.unmark",
			Resource:   "task_center",
			ResourceID: tt + "/" + id.String(),
			Status:     "success",
			Message:    "cleared failure marks",
		})
	}
	return nil
}

// BatchIgnoreFailures applies ignore marks in bulk (single aggregated audit row).
func (s *Service) BatchIgnoreFailures(c *gin.Context, req BatchMarkRequest) BatchRetryResponse {
	return s.batchApplyMark(c, req, MarkIgnored, "task_center.batch_ignore")
}

// BatchHandleFailures applies handled marks in bulk (single aggregated audit row).
func (s *Service) BatchHandleFailures(c *gin.Context, req BatchMarkRequest) BatchRetryResponse {
	return s.batchApplyMark(c, req, MarkHandled, "task_center.batch_handle")
}

func (s *Service) batchApplyMark(c *gin.Context, req BatchMarkRequest, markType, auditAction string) BatchRetryResponse {
	out := BatchRetryResponse{Results: make([]BatchRetryOneResult, 0, len(req.Items))}
	if len(req.Items) > maxBatchItems {
		for _, it := range req.Items {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: it.TaskType, ID: it.ID, OK: false, Error: fmt.Sprintf("batch exceeds %d", maxBatchItems)})
			out.FailedCount++
		}
		return out
	}
	admin := adminFromGin(c)
	ctx := c.Request.Context()
	now := time.Now().UTC()
	for _, it := range req.Items {
		id, err := uuid.Parse(strings.TrimSpace(it.ID))
		if err != nil {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: it.TaskType, ID: it.ID, OK: false, Error: "invalid id"})
			out.FailedCount++
			continue
		}
		tt, perr := parseTaskType(it.TaskType)
		if perr != nil {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: it.TaskType, ID: it.ID, OK: false, Error: "invalid taskType"})
			out.FailedCount++
			continue
		}
		if _, err := s.unifiedOne(ctx, tt, id, now); err != nil {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: tt, ID: id.String(), OK: false, Error: "not found"})
			out.FailedCount++
			continue
		}
		src := sourceTableForType(tt)
		if err := s.upsertFailureMark(ctx, tt, id.String(), src, markType, req.Remark, admin); err != nil {
			out.Results = append(out.Results, BatchRetryOneResult{TaskType: tt, ID: id.String(), OK: false, Error: err.Error()})
			out.FailedCount++
			continue
		}
		out.Results = append(out.Results, BatchRetryOneResult{TaskType: tt, ID: id.String(), OK: true})
		out.SuccessCount++
	}
	if s.OpLog != nil {
		sum := truncateRunes("success="+strconv.Itoa(out.SuccessCount)+" failed="+strconv.Itoa(out.FailedCount)+" mark="+markType, 2000)
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     auditAction,
			Resource:   "task_center",
			ResourceID: "batch",
			Status:     "success",
			Message:    sum,
		})
	}
	return out
}
