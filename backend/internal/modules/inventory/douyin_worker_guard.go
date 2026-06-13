package inventory

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
)

func (s *Service) guardDouyinInventoryWorker(ctx context.Context, taskID uuid.UUID, task *InventorySyncTask) error {
	if task == nil || strings.TrimSpace(strings.ToLower(task.Platform)) != "douyin_shop" {
		return nil
	}
	isScheduled := strings.TrimSpace(task.Mode) != ModeManual
	if ge := platformdouyin.GuardWorkerWithShop(ctx, task.ShopID.String(), platformdouyin.FeatureInventorySync, true, isScheduled); ge != nil {
		douyinmetrics.RecordRuntimeBlockedTask()
		return s.blockDouyinInventoryTask(ctx, taskID, ge, task)
	}
	return nil
}

func (s *Service) blockDouyinInventoryTask(ctx context.Context, taskID uuid.UUID, ge *platformdouyin.Error, task *InventorySyncTask) error {
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
	_ = s.DB.WithContext(ctx).Model(&InventorySyncTask{}).Where("id = ?", taskID).
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

func (s *Service) markDouyinInventoryStale(ctx context.Context, taskID uuid.UUID, code, recoveryStatus string) {
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
	_ = s.DB.WithContext(ctx).Model(&InventorySyncTask{}).Where("id = ?", taskID).
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

func (s *Service) touchInventoryProgress(ctx context.Context, taskID uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	_ = s.DB.WithContext(ctx).Model(&InventorySyncTask{}).Where("id = ?", taskID).
		Update("updated_at", time.Now().UTC()).Error
}
