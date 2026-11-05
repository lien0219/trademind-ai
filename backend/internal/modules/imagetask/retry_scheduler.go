package imagetask

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
)

func imageRetryDelaySeconds(retryCount, baseSec, capSec int) int {
	if retryCount < 1 {
		retryCount = 1
	}
	if baseSec < 1 {
		baseSec = 1
	}
	if capSec < baseSec {
		capSec = baseSec
	}
	mul := 1 << uint(retryCount-1)
	d := baseSec * mul
	if d > capSec {
		return capSec
	}
	return d
}

// StartImageRetryScheduler periodically moves due retrying tasks back onto the Redis list.
func StartImageRetryScheduler(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, interval time.Duration) {
	if svc == nil || !svc.QueueEnabled || !svc.AutoRetryEnabled || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				svc.flushImageDueRetries(ctx, log)
			}
		}
	}()
}

func (s *Service) flushImageDueRetries(ctx context.Context, log *slog.Logger) {
	if s == nil || s.DB == nil || !s.AutoRetryEnabled || !s.QueueEnabled {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	var due []ImageTask
	query := s.DB.WithContext(ctx).
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", StatusRetrying, now).
		Order("next_retry_at ASC").
		Limit(50)
	if s.DB.Migrator().HasTable("tenants") {
		var tenantCount int64
		if err := s.DB.WithContext(ctx).Table("tenants").Count(&tenantCount).Error; err != nil {
			return
		}
		if tenantCount > 0 {
			query = query.Where("EXISTS (SELECT 1 FROM tenants WHERE tenants.id = image_tasks.tenant_id AND tenants.status = ?)", "active")
		}
	}
	if err := query.Find(&due).Error; err != nil {
		return
	}
	for i := range due {
		s.tryEnqueueScheduledImageRetry(ctx, log, &due[i], now)
	}
}

func (s *Service) tryEnqueueScheduledImageRetry(ctx context.Context, log *slog.Logger, task *ImageTask, now time.Time) {
	if s == nil || task == nil || task.NextRetryAt == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if task.NextRetryAt.After(now) {
		return
	}
	if _, _, err := tasktenant.BeginWorker(ctx, s.DB, task.TenantID, uuid.Nil, "image_auto_retry"); err != nil {
		if log != nil {
			log.Warn("image_auto_retry_tenant_gate_rejected", "taskId", task.ID.String(), "tenantId", task.TenantID, "error", tasktenant.WrapError(err))
		}
		return
	}
	if task.ProductID != nil {
		productTenantID, err := tasktenant.ResolveProductTenant(ctx, s.DB, *task.ProductID)
		if err != nil {
			if log != nil {
				log.Warn("image_auto_retry_product_tenant_unavailable", "taskId", task.ID.String(), "tenantId", task.TenantID, "productId", task.ProductID.String(), "error", tasktenant.WrapError(err))
			}
			return
		}
		if err := tasktenant.EnsureResourceTenantMatch(task.TenantID, productTenantID); err != nil {
			if log != nil {
				log.Warn("image_auto_retry_product_tenant_mismatch", "taskId", task.ID.String(), "tenantId", task.TenantID, "productId", task.ProductID.String(), "error", tasktenant.WrapError(err))
			}
			return
		}
		if err := s.validateImageRetryProductShops(ctx, *task.ProductID, task.TenantID); err != nil {
			if log != nil {
				log.Warn("image_auto_retry_product_shop_scope_rejected", "taskId", task.ID.String(), "tenantId", task.TenantID, "productId", task.ProductID.String(), "error", tasktenant.WrapError(err))
			}
			return
		}
	}
	tid := task.ID
	savedNext := *task.NextRetryAt
	mark := now.UTC()

	res := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("id = ? AND status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", tid, StatusRetrying, now).
		Updates(map[string]any{
			"next_retry_at":     nil,
			"retry_enqueued_at": &mark,
			"locked_by":         nil,
			"locked_until":      nil,
		})
	if res.Error != nil || res.RowsAffected == 0 {
		return
	}

	if err := s.enqueueTask(ctx, tid, task.TaskType, task.Provider, task.CreatedBy, "auto-retry-scheduler"); err != nil {
		if log != nil {
			log.Warn("image_auto_retry_enqueue_failed", "taskId", tid.String(), "error", err)
		}
		_ = s.DB.WithContext(ctx).Model(&ImageTask{}).
			Where("id = ?", tid).
			Updates(map[string]any{
				"next_retry_at":     &savedNext,
				"retry_enqueued_at": nil,
			}).Error
		return
	}

	if s.OpLog != nil {
		msgLog := fmt.Sprintf("taskId=%s taskType=%s provider=%s retryCount=%d nextRetryAt=%s",
			tid.String(),
			task.TaskType,
			task.Provider,
			task.RetryCount,
			savedNext.Format(time.RFC3339),
		)
		var admin *uuid.UUID
		if task.CreatedBy != nil {
			admin = task.CreatedBy
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "image.task.auto_retry_enqueued",
			Resource:    "image_task",
			ResourceID:  tid.String(),
			Status:      "success",
			Message:     truncateRunes(msgLog, 2000),
		})
	}
}

func (s *Service) validateImageRetryProductShops(ctx context.Context, productID uuid.UUID, tenantID int64) error {
	var shopIDs []uuid.UUID
	if err := s.DB.WithContext(ctx).
		Table("product_platform_publish_configs").
		Where("product_id = ? AND shop_id IS NOT NULL", productID).
		Pluck("shop_id", &shopIDs).Error; err != nil {
		return err
	}
	var publicationShopIDs []uuid.UUID
	if err := s.DB.WithContext(ctx).
		Table("product_publications").
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Pluck("shop_id", &publicationShopIDs).Error; err != nil {
		return err
	}
	shopIDs = append(shopIDs, publicationShopIDs...)
	for _, shopID := range shopIDs {
		shopTenantID, err := tasktenant.ResolveShopTenant(ctx, s.DB, shopID)
		if err != nil {
			return err
		}
		if err := tasktenant.EnsureResourceTenantMatch(tenantID, shopTenantID); err != nil {
			return err
		}
	}
	return nil
}
