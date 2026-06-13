package ordersync

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
)

func (s *Service) guardDouyinOrderWorker(ctx context.Context, taskID uuid.UUID, task *OrderSyncTask) error {
	if task == nil || task.Platform != "douyin_shop" {
		return nil
	}
	isScheduled := strings.TrimSpace(task.Mode) != ModeManual
	if ge := platformdouyin.GuardWorkerWithShop(ctx, task.ShopID.String(), platformdouyin.FeatureOrderSync, true, isScheduled); ge != nil {
		douyinmetrics.RecordRuntimeBlockedTask()
		return s.blockDouyinOrderTask(ctx, taskID, ge, task)
	}
	return nil
}

func (s *Service) blockDouyinOrderTask(ctx context.Context, taskID uuid.UUID, ge *platformdouyin.Error, task *OrderSyncTask) error {
	if s == nil || s.DB == nil || ge == nil {
		return ge
	}
	fin := time.Now().UTC()
	out := platformdouyin.MarshalRecoveryOutput(nil, platformdouyin.TaskRecoveryMeta{
		RecoveryStatus: platformdouyin.RecoverySkipped,
		LastErrorCode:  ge.Code,
		UserMessage:    ge.Message,
		TechnicalCode:  ge.Code,
	})
	_ = s.DB.WithContext(ctx).Model(&OrderSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusCancelled,
			"error_message": ge.Message,
			"finished_at":   &fin,
			"output":        datatypes.JSON(out),
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
	return ge
}

func (s *Service) markDouyinOrderStale(ctx context.Context, taskID uuid.UUID, code, recoveryStatus string) {
	if s == nil || s.DB == nil {
		return
	}
	douyinmetrics.RecordStaleTask()
	fin := time.Now().UTC()
	meta := platformdouyin.TaskRecoveryMeta{
		RecoveryStatus: recoveryStatus,
		LastErrorCode:  code,
		UserMessage:    platformdouyin.UserMessageForRecovery(recoveryStatus),
		TechnicalCode:  code,
	}
	out := platformdouyin.MarshalRecoveryOutput(nil, meta)
	_ = s.DB.WithContext(ctx).Model(&OrderSyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_message": meta.UserMessage,
			"finished_at":   &fin,
			"output":        datatypes.JSON(out),
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
}

func (s *Service) touchOrderSyncProgress(ctx context.Context, taskID uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	_ = s.DB.WithContext(ctx).Model(&OrderSyncTask{}).Where("id = ?", taskID).
		Update("updated_at", time.Now().UTC()).Error
}
