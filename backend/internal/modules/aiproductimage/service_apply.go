package aiproductimage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
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
	ctx := c.Request.Context()
	tenantID, hasTenant := optionalRequestTenantID(c)
	allowReplay := item.Status == ItemApplied && s.Idempotency != nil
	if item.Status == ItemApplied && !allowReplay {
		result.Status = ItemConflict
		result.StatusLabel = itemStatusLabel(ItemConflict)
		result.ErrorMessage = "该结果已应用，不能重复应用"
		return result
	}
	if !allowReplay && item.Status != ItemPendingReview && item.Status != ItemSuccess {
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
	targetVersion, beforeVersion := s.resolveImageTargetVersion(c, item)
	if !allowReplay {
		if err := s.verifyImageSnapshot(c, item); err != nil {
			_ = scopeOptional(s.DB.WithContext(ctx).Model(&AIProductImageItem{}), tenantID, scopeItemQuery).
				Where("id = ?", item.ID).Update("status", ItemConflict).Error
			result.Status = ItemConflict
			result.StatusLabel = itemStatusLabel(ItemConflict)
			result.ErrorCode = ErrCodeTargetVersionConflict
			result.ErrorMessage = ErrCodeTargetVersionConflict + ": " + ConflictUserMessage
			return result
		}
	}
	// Prefer frozen ImageUpdatedAt for the idempotency key so post-apply retries share the same key.
	if item.ImageUpdatedAt != nil {
		targetVersion = item.ImageUpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	slot := applySlot(item, mode)
	resultHash := strings.TrimSpace(item.ResultStorageKey)
	if resultHash == "" {
		resultHash = imageURLHash(strings.TrimSpace(item.ResultImageURL))
	} else {
		resultHash = imageURLHash(resultHash)
	}
	owner := applyIdempotencyOwner(c, "ai-image-apply")
	idemJob, replayRes, acqErr := s.acquireImageApply(ctx, owner, item.BatchID.String(), item.ID.String(), item.ProductID.String(), targetVersion, slot, mode, resultHash)
	if acqErr != nil {
		code := acqErr.Error()
		if code == idempotency.ErrCodeInProgress {
			result.Status = ItemProcessing
			result.StatusLabel = "处理中"
			result.ErrorCode = idempotency.ErrCodeInProgress
			result.ErrorMessage = idempotency.ErrCodeInProgress
			return result
		}
		if code == idempotency.ErrCodeKeyConflict {
			result.Status = ItemConflict
			result.StatusLabel = itemStatusLabel(ItemConflict)
			result.ErrorCode = idempotency.ErrCodeKeyConflict
			result.ErrorMessage = idempotency.ErrCodeKeyConflict
			return result
		}
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = code
		return result
	}
	if idemJob == nil && replayRes != nil {
		result.Status = ItemApplied
		result.StatusLabel = itemStatusLabel(ItemApplied)
		return result
	}
	if idemJob != nil {
		if hasTenant {
			ok, reconcileErr := s.ReconcileImageApply(ctx, idemJob, item, slot, *tenantID)
			if ok {
				result.Status = ItemApplied
				result.StatusLabel = itemStatusLabel(ItemApplied)
				return result
			}
			if reconcileErr != nil && strings.Contains(reconcileErr.Error(), product.CodeAIApplyReconciliationConflict) {
				result.Status = ItemConflict
				result.StatusLabel = itemStatusLabel(ItemConflict)
				result.ErrorCode = product.CodeAIApplyReconciliationConflict
				result.ErrorMessage = reconcileErr.Error()
				return result
			}
		} else {
			ok, reconcileErr := s.ReconcileImageApply(ctx, idemJob, item, slot)
			if ok {
				result.Status = ItemApplied
				result.StatusLabel = itemStatusLabel(ItemApplied)
				return result
			}
			if reconcileErr != nil && strings.Contains(reconcileErr.Error(), product.CodeAIApplyReconciliationConflict) {
				result.Status = ItemConflict
				result.StatusLabel = itemStatusLabel(ItemConflict)
				result.ErrorCode = product.CodeAIApplyReconciliationConflict
				result.ErrorMessage = reconcileErr.Error()
				return result
			}
		}
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
			s.failImageApply(ctx, idemJob, ErrCodeTargetVersionConflict, false)
			_ = scopeOptional(s.DB.WithContext(ctx).Model(&AIProductImageItem{}), tenantID, scopeItemQuery).
				Where("id = ?", item.ID).Update("status", ItemConflict).Error
			result.Status = ItemConflict
			result.StatusLabel = itemStatusLabel(ItemConflict)
			result.ErrorCode = ErrCodeTargetVersionConflict
			result.ErrorMessage = ErrCodeTargetVersionConflict + ": " + ConflictUserMessage
			return result
		}
		s.failImageApply(ctx, idemJob, "AI_IMAGE_APPLY_FAILED", true)
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
	appID := ""
	objectKey := ""
	if app != nil {
		updates["application_id"] = app.ID
		appID = app.ID.String()
	}
	_ = scopeOptional(s.DB.WithContext(ctx).Model(&AIProductImageItem{}), tenantID, scopeItemQuery).
		Where("id = ?", item.ID).Updates(updates).Error

	afterVersion, _ := s.resolveImageTargetVersion(c, item)
	if strings.TrimSpace(item.ResultStorageKey) != "" {
		objectKey = strings.TrimSpace(item.ResultStorageKey)
	}
	summary := map[string]string{
		"batchId":       item.BatchID.String(),
		"itemId":        item.ID.String(),
		"productId":     item.ProductID.String(),
		"beforeVersion": beforeVersion,
		"afterVersion":  afterVersion,
		"applicationId": appID,
		"slot":          slot,
		"objectKey":     objectKey,
	}
	_ = s.completeImageApply(ctx, idemJob, summary, appID)

	result.Status = ItemApplied
	result.StatusLabel = itemStatusLabel(ItemApplied)
	return result
}

func (s *Service) resolveImageTargetVersion(c *gin.Context, item *AIProductImageItem) (current string, expected string) {
	ctx := c.Request.Context()
	tenantID, _ := optionalRequestTenantID(c)
	if item.ImageID != nil {
		var img product.ProductImage
		if err := scopeOptional(s.DB.WithContext(ctx), tenantID, scopeImageQuery).
			Where("id = ?", *item.ImageID).First(&img).Error; err == nil {
			current = img.UpdatedAt.UTC().Format(time.RFC3339Nano)
			if item.ImageUpdatedAt != nil {
				expected = item.ImageUpdatedAt.UTC().Format(time.RFC3339Nano)
			} else {
				expected = current
			}
			return current, expected
		}
	}
	var p product.Product
	if err := scopeOptional(s.DB.WithContext(ctx), tenantID, scopeProductQuery).
		Where("id = ?", item.ProductID).First(&p).Error; err == nil {
		current = p.UpdatedAt.UTC().Format(time.RFC3339Nano)
		expected = current
	}
	return current, expected
}

func (s *Service) verifyImageSnapshot(c *gin.Context, item *AIProductImageItem) error {
	if item.ImageID == nil {
		return nil
	}
	var img product.ProductImage
	tenantID, _ := optionalRequestTenantID(c)
	if err := scopeOptional(s.DB.WithContext(c.Request.Context()), tenantID, scopeImageQuery).
		Where("id = ?", *item.ImageID).First(&img).Error; err != nil {
		return fmt.Errorf("content conflict: source image missing")
	}
	pubURL := strings.TrimSpace(img.PublicURL)
	if pubURL == "" {
		pubURL = strings.TrimSpace(img.OriginURL)
	}
	if item.SourceSnapshotHash != "" && item.SourceSnapshotHash != imageURLHash(pubURL) {
		return fmt.Errorf("content conflict: source image changed")
	}
	if item.ImageUpdatedAt != nil && img.UpdatedAt.After(item.ImageUpdatedAt.UTC().Add(time.Second)) {
		return fmt.Errorf("content conflict: source image updated")
	}
	return nil
}

func (s *Service) applyNewImage(c *gin.Context, item *AIProductImageItem, mode string, adminID *uuid.UUID) (*product.ProductImageApplication, error) {
	if s.Image == nil {
		return nil, fmt.Errorf("imagetask service unavailable")
	}
	ctx := c.Request.Context()
	tenantID, _ := optionalRequestTenantID(c)
	var prevBestMainID string
	if mode == ApplySetMain {
		var prevBest product.ProductImage
		if err := scopeOptional(s.DB.WithContext(ctx), tenantID, scopeImageQuery).
			Where("product_id = ? AND is_best_main = ?", item.ProductID, true).
			First(&prevBest).Error; err == nil {
			prevBestMainID = prevBest.ID.String()
		}
	}
	taskID := *item.ImageTaskID
	var task imagetask.ImageTask
	if err := scopeOptional(s.DB.WithContext(ctx), tenantID, scopeTaskQuery).
		Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	if task.ProductID == nil || *task.ProductID != item.ProductID {
		return nil, fmt.Errorf("image task product mismatch")
	}
	imgRow, err := s.Image.ApplyTaskResult(ctx, imagetask.ApplyItemOpts{
		ProductID: item.ProductID,
		TaskID:    taskID,
		ApplyMode: imagetaskApplyMode(mode),
		SetBest:   mode == ApplySetMain,
		AdminID:   adminID,
	})
	if err != nil {
		return nil, err
	}
	snapMap := map[string]any{"appliedImageId": imgRow.ID.String()}
	if prevBestMainID != "" {
		snapMap["previousBestMainId"] = prevBestMainID
		snapMap["previousIsBestMain"] = true
	}
	snap, _ := json.Marshal(snapMap)
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
	if err := s.DB.WithContext(ctx).Create(app).Error; err != nil {
		return nil, err
	}
	return app, nil
}

func (s *Service) applyReplaceImage(c *gin.Context, item *AIProductImageItem, adminID *uuid.UUID) (*product.ProductImageApplication, error) {
	if item.ImageID == nil || item.ImageTaskID == nil {
		return nil, fmt.Errorf("缺少原图信息")
	}
	ctx := c.Request.Context()
	tenantID, _ := optionalRequestTenantID(c)
	var orig product.ProductImage
	if err := scopeOptional(s.DB.WithContext(ctx), tenantID, scopeImageQuery).
		Where("id = ? AND product_id = ?", *item.ImageID, item.ProductID).First(&orig).Error; err != nil {
		return nil, err
	}
	prevSnap, _ := json.Marshal(orig)
	var task imagetask.ImageTask
	if err := scopeOptional(s.DB.WithContext(ctx), tenantID, scopeTaskQuery).
		Where("id = ?", *item.ImageTaskID).First(&task).Error; err != nil {
		return nil, err
	}
	if task.ProductID == nil || *task.ProductID != item.ProductID {
		return nil, fmt.Errorf("image task product mismatch")
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
		res := scopeOptional(tx.Model(&product.ProductImage{}), tenantID, scopeImageQuery).
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
	tenantID, err := requestTenantID(c)
	if err != nil {
		return nil, err
	}
	var item AIProductImageItem
	if err := scopeItemQuery(s.DB.WithContext(c.Request.Context()), tenantID).Where("id = ?", itemID).First(&item).Error; err != nil {
		return nil, err
	}
	if err := s.validateItemTenant(c.Request.Context(), &item, tenantID); err != nil {
		return nil, err
	}
	r := s.applyOneItem(c, &item, body.ApplyMode, adminID)
	s.refreshAppliedCount(c.Request.Context(), item.BatchID, tenantID)
	return &r, nil
}

func (s *Service) ApplySelected(c *gin.Context, batchID uuid.UUID, body ApplySelectedBody, adminID *uuid.UUID) (*ApplyResultSummary, error) {
	ctx := c.Request.Context()
	tenantID, err := requestTenantID(c)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetBatchByID(c, batchID); err != nil {
		return nil, err
	}
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
		if err := scopeItemQuery(s.DB.WithContext(ctx), tenantID).
			Where("id = ? AND batch_id = ?", id, batchID).First(&item).Error; err != nil {
			continue
		}
		if err := s.validateItemTenant(ctx, &item, tenantID); err != nil {
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
	s.refreshAppliedCount(ctx, batchID, tenantID)
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
	tenantID, err := requestTenantID(c)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetBatchByID(c, batchID); err != nil {
		return nil, err
	}
	var items []AIProductImageItem
	if err := scopeItemQuery(s.DB.WithContext(ctx), tenantID).
		Where("batch_id = ? AND status = ? AND application_id IS NOT NULL", batchID, ItemApplied).
		Find(&items).Error; err != nil {
		return nil, err
	}
	summary := &UndoAppliedSummary{Items: []ApplyItemResult{}}
	for _, item := range items {
		owner := applyIdempotencyOwner(c, "ai-image-undo")
		r := ApplyItemResult{ItemID: item.ID.String(), ProductID: item.ProductID.String()}
		if item.ApplicationID == nil {
			r.Status = "failed"
			r.StatusLabel = "失败"
			r.ErrorMessage = "缺少应用记录"
			summary.FailedCount++
			summary.Items = append(summary.Items, r)
			continue
		}
		appID := *item.ApplicationID
		var app product.ProductImageApplication
		if err := scopeApplicationQuery(s.DB.WithContext(ctx), tenantID).Where("id = ?", appID).First(&app).Error; err != nil {
			r.Status = "failed"
			r.StatusLabel = "失败"
			r.ErrorMessage = err.Error()
			summary.FailedCount++
			summary.Items = append(summary.Items, r)
			continue
		}
		targetVersion := ""
		var img product.ProductImage
		if err := scopeImageQuery(s.DB.WithContext(ctx), tenantID).Where("id = ?", app.ProductImageID).First(&img).Error; err == nil {
			targetVersion = img.UpdatedAt.UTC().Format(time.RFC3339Nano)
		} else {
			var p product.Product
			if err := scopeProductQuery(s.DB.WithContext(ctx), tenantID).Where("id = ?", item.ProductID).First(&p).Error; err == nil {
				targetVersion = p.UpdatedAt.UTC().Format(time.RFC3339Nano)
			}
		}
		idemJob, replayRes, acqErr := s.acquireImageUndo(ctx, owner, appID.String(), targetVersion)
		if acqErr != nil {
			code := acqErr.Error()
			if code == idempotency.ErrCodeInProgress {
				r.Status = ItemProcessing
				r.StatusLabel = "处理中"
				r.ErrorCode = idempotency.ErrCodeInProgress
				r.ErrorMessage = idempotency.ErrCodeInProgress
				summary.FailedCount++
			} else if code == idempotency.ErrCodeKeyConflict {
				r.Status = ItemConflict
				r.StatusLabel = itemStatusLabel(ItemConflict)
				r.ErrorCode = idempotency.ErrCodeKeyConflict
				r.ErrorMessage = idempotency.ErrCodeKeyConflict
				summary.ConflictCount++
			} else {
				r.Status = "failed"
				r.StatusLabel = "失败"
				r.ErrorMessage = code
				summary.FailedCount++
			}
			summary.Items = append(summary.Items, r)
			continue
		}
		if idemJob == nil && replayRes != nil {
			r.Status = "undone"
			r.StatusLabel = "已撤销"
			summary.SuccessCount++
			summary.Items = append(summary.Items, r)
			continue
		}
		if err := s.undoOneApplication(c, appID, adminID); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "conflict") {
				s.failImageUndo(ctx, idemJob, ErrCodeUndoVersionConflict, false)
				_ = scopeItemQuery(s.DB.WithContext(ctx).Model(&AIProductImageItem{}), tenantID).
					Where("id = ?", item.ID).Update("status", ItemConflict).Error
				r.Status = ItemConflict
				r.StatusLabel = itemStatusLabel(ItemConflict)
				r.ErrorCode = ErrCodeUndoVersionConflict
				r.ErrorMessage = ErrCodeUndoVersionConflict + ": 商品图片已变化，无法撤销"
				summary.ConflictCount++
			} else {
				s.failImageUndo(ctx, idemJob, "AI_IMAGE_UNDO_FAILED", true)
				r.Status = "failed"
				r.StatusLabel = "失败"
				r.ErrorMessage = msg
				summary.FailedCount++
			}
		} else {
			_ = s.completeImageUndo(ctx, idemJob, appID.String())
			_ = scopeItemQuery(s.DB.WithContext(ctx).Model(&AIProductImageItem{}), tenantID).
				Where("id = ?", item.ID).Updates(map[string]any{
				"status": ItemPendingReview, "application_id": nil, "applied_at": nil, "applied_by": nil, "apply_mode": "",
			}).Error
			r.Status = "undone"
			r.StatusLabel = "已撤销"
			summary.SuccessCount++
		}
		summary.Items = append(summary.Items, r)
	}
	s.refreshAppliedCount(ctx, batchID, tenantID)
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
	tenantID, _ := optionalRequestTenantID(c)
	var app product.ProductImageApplication
	if err := scopeOptional(s.DB.WithContext(ctx), tenantID, scopeApplicationQuery).
		Where("id = ? AND status = ?", appID, product.ImageApplyStatusApplied).First(&app).Error; err != nil {
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
			if err := scopeOptional(tx, tenantID, scopeImageQuery).
				Where("id = ?", app.ProductImageID).First(&cur).Error; err != nil {
				return err
			}
			res := scopeOptional(tx.Model(&product.ProductImage{}), tenantID, scopeImageQuery).
				Where("id = ? AND updated_at = ?", cur.ID, cur.UpdatedAt).Updates(map[string]any{
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
			if err := scopeOptional(tx, tenantID, scopeImageQuery).
				Where("id = ?", app.ProductImageID).First(&cur).Error; err != nil {
				return err
			}
			if cur.Source != product.ImageSourceAI {
				return fmt.Errorf("content conflict: image no longer AI-applied")
			}
			if err := scopeOptional(tx, tenantID, scopeImageQuery).Delete(&cur).Error; err != nil {
				return err
			}
			if app.ApplyMode == ApplySetMain {
				var snap map[string]any
				_ = json.Unmarshal(app.PreviousSnapshot, &snap)
				if prevID, ok := snap["previousBestMainId"].(string); ok && strings.TrimSpace(prevID) != "" {
					if pid, err := uuid.Parse(prevID); err == nil {
						_ = scopeOptional(tx.Model(&product.ProductImage{}), tenantID, scopeImageQuery).
							Where("id = ? AND product_id = ?", pid, app.ProductID).
							Update("is_best_main", true).Error
					}
				}
			}
		default:
			return fmt.Errorf("unsupported apply mode for undo")
		}
		res := scopeOptional(tx.Model(&product.ProductImageApplication{}), tenantID, scopeApplicationQuery).
			Where("id = ? AND status = ?", app.ID, product.ImageApplyStatusApplied).Updates(map[string]any{
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

func applyIdempotencyOwner(c *gin.Context, prefix string) string {
	if c != nil {
		if rid := strings.TrimSpace(c.GetString("requestId")); rid != "" {
			return rid
		}
	}
	return prefix + ":" + uuid.New().String()
}
