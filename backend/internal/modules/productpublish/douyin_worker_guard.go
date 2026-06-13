package productpublish

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	douyinmetrics "github.com/trademind-ai/trademind/backend/internal/metrics/douyin"
	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
	"gorm.io/datatypes"
)

func (s *Service) guardDouyinWorker(ctx context.Context, taskID uuid.UUID, shopID uuid.UUID, feature string, isScheduled bool, createdBy *uuid.UUID) error {
	if ge := platformdouyin.GuardWorkerWithShop(ctx, shopID.String(), feature, true, isScheduled); ge != nil {
		douyinmetrics.RecordRuntimeBlockedTask()
		return s.blockDouyinTask(ctx, taskID, ge, createdBy)
	}
	return nil
}

func (s *Service) blockDouyinTask(ctx context.Context, taskID uuid.UUID, ge *platformdouyin.Error, createdBy *uuid.UUID) error {
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
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":         TaskCancelled,
			"publish_status": StatusPubFailed,
			"error_code":     ge.Code,
			"error_message":  ge.Message,
			"retryable":      false,
			"finished_at":    &fin,
			"output":         datatypes.JSON(out),
			"locked_by":      nil,
			"locked_until":   nil,
			"updated_at":     fin,
		}).Error
	return ge
}

func (s *Service) markDouyinStale(ctx context.Context, taskID uuid.UUID, code, recoveryStatus string, createdBy *uuid.UUID) {
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
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":         TaskFailed,
			"publish_status": StatusPubFailed,
			"error_code":     code,
			"error_message":  meta.UserMessage,
			"retryable":      true,
			"finished_at":    &fin,
			"output":         datatypes.JSON(out),
			"locked_by":      nil,
			"locked_until":   nil,
			"updated_at":     fin,
		}).Error
}

func (s *Service) touchDouyinTaskProgress(ctx context.Context, taskID uuid.UUID, patch map[string]any) {
	if s == nil || s.DB == nil {
		return
	}
	patch["updated_at"] = time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).Updates(patch).Error
}

func parseTaskOutputMap(raw datatypes.JSON) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	_ = json.Unmarshal(raw, &out)
	return out
}

func mergeTaskOutput(existing datatypes.JSON, patch map[string]any) datatypes.JSON {
	base := parseTaskOutputMap(existing)
	for k, v := range patch {
		base[k] = v
	}
	b, _ := json.Marshal(base)
	return datatypes.JSON(b)
}

// RecoverDouyinDraftStale attempts product.detail recovery for result_unknown tasks.
func (s *Service) RecoverDouyinDraftStale(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.DB == nil || s.Shops == nil {
		return nil
	}
	var task ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if task.Platform != "douyin_shop" || task.Status == TaskSuccess || task.Status == TaskCancelled {
		return nil
	}
	if err := s.guardDouyinWorker(ctx, taskID, task.ShopID, platformdouyin.FeatureProductDraft, false, task.CreatedBy); err != nil {
		return err
	}
	client, _, err := s.Shops.DouyinClientForShopContext(ctx, task.ShopID, task.CreatedBy)
	if err != nil {
		return err
	}
	res, recovered, recErr := tryRecoverDouyinDraftFromPlatform(ctx, client, task.ShopID.String(), task.ProductID.String())
	if recErr != nil {
		return recErr
	}
	if !recovered || res == nil {
		s.markDouyinStale(ctx, taskID, platformdouyin.CodeDouyinTaskRecoveryRequired, platformdouyin.RecoveryRequired, task.CreatedBy)
		douyinmetrics.RecordRecoveryFailed()
		return nil
	}
	snap, ok := parseDouyinDraftSnapshot(task.Input)
	if !ok {
		return nil
	}
	buildRes, err := BuildDouyinProductPayload(ctx, s.DB, task.ProductID, snap.ConfigID)
	if err != nil {
		return err
	}
	douyinmetrics.RecordRecoverySuccess()
	return s.completeDouyinDraftSuccess(ctx, &task, taskID, snap, buildRes, res)
}
