package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/datatypes"
)

// GetTaskItem loads one task item by id.
func (s *Service) GetTaskItem(ctx context.Context, itemID uuid.UUID) (*ImageTaskItem, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
	}
	var item ImageTaskItem
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// ApplyItemByID saves a successful task item output to product_images.
func (s *Service) ApplyItemByID(ctx context.Context, itemID uuid.UUID, productID uuid.UUID, applyMode string, setBest bool, adminID *uuid.UUID) (*product.ProductImage, error) {
	item, err := s.GetTaskItem(ctx, itemID)
	if err != nil {
		return nil, err
	}
	return s.ApplyTaskResult(ctx, ApplyItemOpts{
		ProductID: productID,
		TaskID:    item.TaskID,
		ItemID:    &itemID,
		ApplyMode: applyMode,
		SetBest:   setBest,
		AdminID:   adminID,
	})
}

// SetItemAsMain saves item result as main image and marks is_best_main.
func (s *Service) SetItemAsMain(ctx context.Context, itemID uuid.UUID, productID uuid.UUID, adminID *uuid.UUID) (*product.ProductImage, error) {
	return s.ApplyItemByID(ctx, itemID, productID, "main", true, adminID)
}

// ScoreImageRequest is input for synchronous image scoring.
type ScoreImageRequest struct {
	ProductID      *uuid.UUID
	SourceImageID  *uuid.UUID
	SourceImageURL string
	ImageType      string
}

// ScoreImageHTTP scores one image synchronously (no task row required).
func (s *Service) ScoreImageHTTP(ctx context.Context, req ScoreImageRequest) (ImageScore, error) {
	if s == nil {
		return ImageScore{}, fmt.Errorf("imagetask: unavailable")
	}
	imageURL := strings.TrimSpace(req.SourceImageURL)
	imageType := strings.TrimSpace(req.ImageType)
	if imageType == "" {
		imageType = "main"
	}
	productTitle := ""
	if req.ProductID != nil && s.DB != nil {
		var p product.Product
		if err := s.DB.WithContext(ctx).Select("title").First(&p, "id = ?", *req.ProductID).Error; err == nil {
			productTitle = strings.TrimSpace(p.Title)
		}
	}
	if imageURL == "" && req.SourceImageID != nil && s.DB != nil {
		var im product.ProductImage
		if err := s.DB.WithContext(ctx).First(&im, "id = ?", *req.SourceImageID).Error; err == nil {
			imageURL = strings.TrimSpace(im.PublicURL)
			if imageURL == "" {
				imageURL = strings.TrimSpace(im.OriginURL)
			}
			if imageType == "main" && strings.TrimSpace(im.ImageType) != "" {
				imageType = strings.TrimSpace(im.ImageType)
			}
			if req.ProductID == nil {
				pid := im.ProductID
				req.ProductID = &pid
			}
		}
	}
	if imageURL == "" {
		return ImageScore{}, fmt.Errorf("sourceImageUrl or sourceImageId required")
	}
	score, err := s.scoreImageURL(ctx, imageURL, imageType, productTitle)
	if err != nil {
		return ImageScore{}, err
	}
	if req.SourceImageID != nil && s.DB != nil {
		v := score.OverallScore
		_ = s.DB.WithContext(ctx).Model(&product.ProductImage{}).
			Where("id = ?", *req.SourceImageID).
			Update("score", v).Error
	}
	return score, nil
}

// CreateSelectBestMainTask creates and optionally executes select_best_main for a product.
func (s *Service) CreateSelectBestMainTask(ctx context.Context, productID uuid.UUID, mode string, createdBy *uuid.UUID) (*ImageTask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
	}
	mode = normalizeSelectMode(mode)
	in := map[string]any{"selectMode": mode}
	raw, _ := json.Marshal(in)
	row, err := s.CreateAndPersist(ctx, CreatePayload{
		TaskType:  TaskTypeSelectBestMain,
		ProductID: &productID,
		Input:     datatypes.JSON(raw),
		CreatedBy: createdBy,
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func normalizeSelectMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "score_only", "recommend", "auto_set":
		return strings.TrimSpace(strings.ToLower(mode))
	default:
		return "recommend"
	}
}
