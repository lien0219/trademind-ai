package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

// ApplyItemOpts controls saving a task item result to product_images.
type ApplyItemOpts struct {
	ProductID uuid.UUID
	TaskID    uuid.UUID
	ItemID    *uuid.UUID
	ApplyMode string // main | detail | marketing | ai_generated
	SetBest   bool
	AdminID   *uuid.UUID
}

// ApplyTaskResult saves a successful task output to the product image library.
func (s *Service) ApplyTaskResult(ctx context.Context, opts ApplyItemOpts) (*product.ProductImage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
	}
	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", opts.TaskID).Error; err != nil {
		return nil, err
	}
	if task.Status != StatusSuccess {
		return nil, fmt.Errorf("only successful tasks can be applied")
	}
	if task.ProductID == nil || *task.ProductID != opts.ProductID {
		return nil, fmt.Errorf("productId mismatch")
	}

	imgType := imageTypeFromApplyMode(opts.ApplyMode)
	var (
		pubURL     string
		storageKey string
		fileID     *uuid.UUID
		originID   *uuid.UUID
		srcURL     string
		score      *float64
	)

	if opts.ItemID != nil && *opts.ItemID != uuid.Nil {
		var item ImageTaskItem
		if err := s.DB.WithContext(ctx).First(&item, "id = ? AND task_id = ?", *opts.ItemID, opts.TaskID).Error; err != nil {
			return nil, err
		}
		if item.Status != ItemStatusSuccess {
			return nil, fmt.Errorf("task item not successful")
		}
		pubURL = strings.TrimSpace(item.OutputImageURL)
		storageKey = strings.TrimSpace(item.OutputStorageKey)
		fileID = item.OutputFileID
		originID = item.SourceImageID
		srcURL = strings.TrimSpace(item.SourceImageURL)
		if sc, err := parseScoreJSON(item.ScoreJSON); err == nil && sc != nil {
			v := sc.OverallScore
			score = &v
		}
	} else {
		pubURL = strings.TrimSpace(task.ResultURL)
		storageKey = ""
		fileID = task.ResultFileID
		originID = task.SourceImageID
		srcURL = strings.TrimSpace(task.SourceImageURL)
		if len(task.Output) > 0 {
			var out map[string]any
			if json.Unmarshal(task.Output, &out) == nil {
				if sk, ok := out["storageKey"].(string); ok {
					storageKey = strings.TrimSpace(sk)
				}
			}
		}
	}
	if pubURL == "" {
		return nil, fmt.Errorf("no result image to apply")
	}
	if storageKey == "" && fileID != nil && s.Files != nil {
		var fr files.FileRecord
		if err := s.DB.WithContext(ctx).Where("id = ?", *fileID).First(&fr).Error; err == nil {
			storageKey = strings.TrimSpace(fr.ObjectKey)
		}
	}

	var maxOrder int
	_ = s.DB.WithContext(ctx).Model(&product.ProductImage{}).
		Where("product_id = ?", opts.ProductID).
		Select("COALESCE(MAX(sort_order), -1)").
		Scan(&maxOrder).Error

	row := &product.ProductImage{
		ProductID:       opts.ProductID,
		ImageType:       imgType,
		Source:          product.ImageSourceAI,
		SourceTaskID:    &opts.TaskID,
		OriginalImageID: originID,
		OriginURL:       srcURL,
		ObjectKey:       storageKey,
		StorageKey:      storageKey,
		PublicURL:       pubURL,
		Score:           score,
		SortOrder:       maxOrder + 1,
	}
	if opts.SetBest || imgType == product.ImageTypeMain {
		row.IsBestMain = opts.SetBest || imgType == product.ImageTypeMain
	}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if row.IsBestMain {
			if err := tx.Model(&product.ProductImage{}).
				Where("product_id = ?", opts.ProductID).
				Update("is_best_main", false).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		if imgType == product.ImageTypeMain {
			if err := tx.Model(&product.ProductImage{}).
				Where("id = ? AND product_id = ?", row.ID, opts.ProductID).
				Update("sort_order", 0).Error; err != nil {
				return err
			}
			row.SortOrder = 0
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = fileID
	return row, nil
}

// ApplyTaskResultHTTP wraps ApplyTaskResult for handlers.
func (s *Service) ApplyTaskResultHTTP(c *gin.Context, productID, taskID uuid.UUID, itemID *uuid.UUID, applyMode string, setBest bool, adminID *uuid.UUID) (*product.ProductImage, error) {
	return s.ApplyTaskResult(c.Request.Context(), ApplyItemOpts{
		ProductID: productID,
		TaskID:    taskID,
		ItemID:    itemID,
		ApplyMode: applyMode,
		SetBest:   setBest,
		AdminID:   adminID,
	})
}

// maybeAutoApply runs after task success when input hints request auto save / set main.
func (s *Service) maybeAutoApply(ctx context.Context, task *ImageTask, hints map[string]any) {
	if s == nil || task == nil || task.ProductID == nil {
		return
	}
	if !autoSaveFromHints(hints) && !autoSetMainFromHints(hints) && !autoSetDetailFromHints(hints) {
		return
	}
	mode := "ai_generated"
	setBest := false
	if autoSetMainFromHints(hints) {
		mode = "main"
		setBest = true
	} else if autoSetDetailFromHints(hints) {
		mode = "detail"
	}
	_, _ = s.ApplyTaskResult(ctx, ApplyItemOpts{
		ProductID: *task.ProductID,
		TaskID:    task.ID,
		ApplyMode: mode,
		SetBest:   setBest,
		AdminID:   task.CreatedBy,
	})
}
