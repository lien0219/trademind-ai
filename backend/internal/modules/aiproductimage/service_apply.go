package aiproductimage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) applyOneItem(c *gin.Context, item *AIProductImageItem, applyMode string, adminID *uuid.UUID) ApplyItemResult {
	result := ApplyItemResult{
		ItemID:    item.ID.String(),
		ProductID: item.ProductID.String(),
	}
	if item.Status == ItemApplied {
		result.Status = ItemConflict
		result.StatusLabel = itemStatusLabel(ItemConflict)
		result.ErrorMessage = "该结果已应用，不能重复应用"
		return result
	}
	if item.Status != ItemPendingReview && item.Status != ItemSuccess {
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = "当前状态不可应用"
		return result
	}
	mode := strings.TrimSpace(applyMode)
	if mode == "" {
		mode = ApplySaveToGallery
	}
	if item.ImageTaskID == nil {
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = "缺少图片任务关联"
		return result
	}
	if err := s.verifyImageSnapshot(c, item); err != nil {
		_ = s.DB.WithContext(c.Request.Context()).Model(item).Update("status", ItemConflict).Error
		result.Status = ItemConflict
		result.StatusLabel = itemStatusLabel(ItemConflict)
		result.ErrorMessage = ConflictUserMessage
		return result
	}

	var app *product.ProductImageApplication
	var applyErr error
	switch mode {
	case ApplyReplaceImage:
		app, applyErr = s.applyReplaceImage(c, item, adminID)
	case ApplySetMain, ApplyAddDetail, ApplySaveToGallery:
		app, applyErr = s.applyNewImage(c, item, mode, adminID)
	default:
		applyErr = fmt.Errorf("不支持的应用方式")
	}
	if applyErr != nil {
		msg := applyErr.Error()
		if strings.Contains(msg, "content conflict") || strings.Contains(msg, "conflict") {
			_ = s.DB.WithContext(c.Request.Context()).Model(item).Update("status", ItemConflict).Error
			result.Status = ItemConflict
			result.StatusLabel = itemStatusLabel(ItemConflict)
			result.ErrorMessage = ConflictUserMessage
			return result
		}
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = msg
		return result
	}
	now := time.Now().UTC()
	updates := map[string]any{
		"status":     ItemApplied,
		"apply_mode": mode,
		"applied_at": &now,
		"applied_by": adminID,
	}
	if app != nil {
		updates["application_id"] = app.ID
	}
	_ = s.DB.WithContext(c.Request.Context()).Model(item).Updates(updates).Error
	result.Status = ItemApplied
	result.StatusLabel = itemStatusLabel(ItemApplied)
	return result
}

func (s *Service) verifyImageSnapshot(c *gin.Context, item *AIProductImageItem) error {
	if item.ImageID == nil {
		return nil
	}
	var img product.ProductImage
	if err := s.DB.WithContext(c.Request.Context()).First(&img, "id = ?", *item.ImageID).Error; err != nil {
		return fmt.Errorf("content conflict: source image missing")
	}
	pubURL := strings.TrimSpace(img.PublicURL)
	if pubURL == "" {
		pubURL = strings.TrimSpace(img.OriginURL)
	}
	if item.SourceSnapshotHash != "" && item.SourceSnapshotHash != imageURLHash(pubURL) {
		return fmt.Errorf("content conflict: source image changed")
	}
	if item.ImageUpdatedAt != nil && img.UpdatedAt.After(*item.ImageUpdatedAt) {
		return fmt.Errorf("content conflict: source image updated")
	}
	return nil
}

func (s *Service) applyNewImage(c *gin.Context, item *AIProductImageItem, mode string, adminID *uuid.UUID) (*product.ProductImageApplication, error) {
	if s.Image == nil {
		return nil, fmt.Errorf("imagetask service unavailable")
	}
	taskID := *item.ImageTaskID
	imgRow, err := s.Image.ApplyTaskResult(c.Request.Context(), imagetask.ApplyItemOpts{
		ProductID: item.ProductID,
		TaskID:    taskID,
		ApplyMode: imagetaskApplyMode(mode),
		SetBest:   mode == ApplySetMain,
		AdminID:   adminID,
	})
	if err != nil {
		return nil, err
	}
	snap, _ := json.Marshal(map[string]any{"appliedImageId": imgRow.ID.String()})
	app := &product.ProductImageApplication{
		ProductID:          item.ProductID,
		ProductImageID:     imgRow.ID,
		ApplyMode:          mode,
		ImageTaskID:        item.ImageTaskID,
		BatchItemID:        &item.ID,
		PreviousSnapshot:   datatypes.JSON(snap),
		SourceSnapshotHash: item.SourceSnapshotHash,
		AppliedBy:          adminID,
		AppliedAt:          time.Now().UTC(),
		Status:             product.ImageApplyStatusApplied,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(app).Error; err != nil {
		return nil, err
	}
	return app, nil
}

func (s *Service) applyReplaceImage(c *gin.Context, item *AIProductImageItem, adminID *uuid.UUID) (*product.ProductImageApplication, error) {
	if item.ImageID == nil || item.ImageTaskID == nil {
		return nil, fmt.Errorf("缺少原图信息")
	}
	ctx := c.Request.Context()
	var orig product.ProductImage
	if err := s.DB.WithContext(ctx).First(&orig, "id = ? AND product_id = ?", *item.ImageID, item.ProductID).Error; err != nil {
		return nil, err
	}
	prevSnap, _ := json.Marshal(orig)
	var task imagetask.ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", *item.ImageTaskID).Error; err != nil {
		return nil, err
	}
	resultURL := strings.TrimSpace(item.ResultImageURL)
	if resultURL == "" {
		resultURL = strings.TrimSpace(task.ResultURL)
	}
	if resultURL == "" {
		return nil, fmt.Errorf("没有可应用的结果图")
	}
	storageKey := strings.TrimSpace(item.ResultStorageKey)
	now := time.Now().UTC()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&product.ProductImage{}).
			Where("id = ? AND updated_at = ?", orig.ID, orig.UpdatedAt).
			Updates(map[string]any{
				"public_url":     resultURL,
				"storage_key":    storageKey,
				"object_key":     storageKey,
				"source":         product.ImageSourceAI,
				"source_task_id": item.ImageTaskID,
				"updated_at":     now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("content conflict: image changed while applying")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	app := &product.ProductImageApplication{
		ProductID:          item.ProductID,
		ProductImageID:     orig.ID,
		ApplyMode:          ApplyReplaceImage,
		ImageTaskID:        item.ImageTaskID,
		BatchItemID:        &item.ID,
		PreviousSnapshot:   datatypes.JSON(prevSnap),
		SourceSnapshotHash: item.SourceSnapshotHash,
		AppliedBy:          adminID,
		AppliedAt:          now,
		Status:             product.ImageApplyStatusApplied,
	}
	if err := s.DB.WithContext(ctx).Create(app).Error; err != nil {
		return nil, err
	}
	return app, nil
}

func (s *Service) ApplyItem(c *gin.Context, itemID uuid.UUID, body ApplyItemBody, adminID *uuid.UUID) (*ApplyItemResult, error) {
	var item AIProductImageItem
	if err := s.DB.WithContext(c.Request.Context()).First(&item, "id = ?", itemID).Error; err != nil {
		return nil, err
	}
	r := s.applyOneItem(c, &item, body.ApplyMode, adminID)
	s.refreshAppliedCount(c.Request.Context(), item.BatchID)
	return &r, nil
}

func (s *Service) ApplySelected(c *gin.Context, batchID uuid.UUID, body ApplySelectedBody, adminID *uuid.UUID) (*ApplyResultSummary, error) {
	ctx := c.Request.Context()
	if len(body.ItemIDs) == 0 {
		return nil, fmt.Errorf("请选择要应用的结果")
	}
	summary := &ApplyResultSummary{Items: []ApplyItemResult{}}
	for _, raw := range body.ItemIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		var item AIProductImageItem
		if err := s.DB.WithContext(ctx).Where("id = ? AND batch_id = ?", id, batchID).First(&item).Error; err != nil {
			continue
		}
		mode := body.ApplyMode
		if mode == "" && item.ApplyMode != "" {
			mode = item.ApplyMode
		}
		r := s.applyOneItem(c, &item, mode, adminID)
		summary.Items = append(summary.Items, r)
		switch r.Status {
		case ItemApplied:
			summary.SuccessCount++
		case ItemConflict:
			summary.ConflictCount++
		default:
			summary.FailedCount++
		}
	}
	s.refreshAppliedCount(ctx, batchID)
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.product_image.batch.apply_selected",
			Resource:    "ai_product_image_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("success=%d conflict=%d failed=%d", summary.SuccessCount, summary.ConflictCount, summary.FailedCount),
		})
	}
	return summary, nil
}

func (s *Service) UndoApplied(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (*UndoAppliedSummary, error) {
	ctx := c.Request.Context()
	var items []AIProductImageItem
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ? AND status = ? AND application_id IS NOT NULL", batchID, ItemApplied).
		Find(&items).Error; err != nil {
		return nil, err
	}
	summary := &UndoAppliedSummary{Items: []ApplyItemResult{}}
	for _, item := range items {
		r := ApplyItemResult{ItemID: item.ID.String(), ProductID: item.ProductID.String()}
		if item.ApplicationID == nil {
			r.Status = "failed"
			r.StatusLabel = "失败"
			r.ErrorMessage = "缺少应用记录"
			summary.FailedCount++
			summary.Items = append(summary.Items, r)
			continue
		}
		if err := s.undoOneApplication(c, *item.ApplicationID, adminID); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "conflict") {
				_ = s.DB.WithContext(ctx).Model(&item).Update("status", ItemConflict).Error
				r.Status = ItemConflict
				r.StatusLabel = itemStatusLabel(ItemConflict)
				r.ErrorMessage = "商品图片已变化，无法撤销"
				summary.ConflictCount++
			} else {
				r.Status = "failed"
				r.StatusLabel = "失败"
				r.ErrorMessage = msg
				summary.FailedCount++
			}
		} else {
			_ = s.DB.WithContext(ctx).Model(&item).Updates(map[string]any{
				"status": ItemPendingReview, "application_id": nil, "applied_at": nil, "applied_by": nil, "apply_mode": "",
			}).Error
			r.Status = "undone"
			r.StatusLabel = "已撤销"
			summary.SuccessCount++
		}
		summary.Items = append(summary.Items, r)
	}
	s.refreshAppliedCount(ctx, batchID)
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.product_image.batch.undo_applied",
			Resource:    "ai_product_image_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("success=%d conflict=%d failed=%d", summary.SuccessCount, summary.ConflictCount, summary.FailedCount),
		})
	}
	return summary, nil
}

func (s *Service) undoOneApplication(c *gin.Context, appID uuid.UUID, adminID *uuid.UUID) error {
	ctx := c.Request.Context()
	var app product.ProductImageApplication
	if err := s.DB.WithContext(ctx).First(&app, "id = ? AND status = ?", appID, product.ImageApplyStatusApplied).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch app.ApplyMode {
		case ApplyReplaceImage:
			var prev product.ProductImage
			if err := json.Unmarshal(app.PreviousSnapshot, &prev); err != nil {
				return fmt.Errorf("undo snapshot invalid")
			}
			var cur product.ProductImage
			if err := tx.First(&cur, "id = ?", app.ProductImageID).Error; err != nil {
				return err
			}
			res := tx.Model(&product.ProductImage{}).Where("id = ? AND updated_at = ?", cur.ID, cur.UpdatedAt).Updates(map[string]any{
				"public_url":     prev.PublicURL,
				"origin_url":     prev.OriginURL,
				"storage_key":    prev.StorageKey,
				"object_key":     prev.ObjectKey,
				"source":         prev.Source,
				"source_task_id": prev.SourceTaskID,
				"updated_at":     now,
			})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("content conflict: image changed while undoing")
			}
		case ApplySetMain, ApplyAddDetail, ApplySaveToGallery:
			var cur product.ProductImage
			if err := tx.First(&cur, "id = ?", app.ProductImageID).Error; err != nil {
				return err
			}
			if cur.Source != product.ImageSourceAI {
				return fmt.Errorf("content conflict: image no longer AI-applied")
			}
			if err := tx.Delete(&cur).Error; err != nil {
				return err
			}
			if app.ApplyMode == ApplySetMain {
				var snap map[string]any
				_ = json.Unmarshal(app.PreviousSnapshot, &snap)
			}
		default:
			return fmt.Errorf("unsupported apply mode for undo")
		}
		res := tx.Model(&product.ProductImageApplication{}).Where("id = ? AND status = ?", app.ID, product.ImageApplyStatusApplied).Updates(map[string]any{
			"status":    product.ImageApplyStatusUndone,
			"undone_by": adminID,
			"undone_at": &now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return fmt.Errorf("content conflict: application already undone")
		}
		return nil
	})
}
