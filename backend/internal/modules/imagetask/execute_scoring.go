package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

func (s *Service) executeScoringTask(ctx context.Context, task *ImageTask, hints map[string]any) error {
	if s == nil || task == nil {
		return fmt.Errorf("imagetask: invalid scoring task")
	}
	tt := strings.TrimSpace(strings.ToLower(task.TaskType))

	productTitle := ""
	if task.ProductID != nil {
		var p product.Product
		if err := s.DB.WithContext(ctx).Select("title").First(&p, "id = ?", *task.ProductID).Error; err == nil {
			productTitle = strings.TrimSpace(p.Title)
		}
	}

	switch tt {
	case TaskTypeScoreImage:
		src := strings.TrimSpace(task.SourceImageURL)
		if src == "" {
			return fmt.Errorf("source image required for score_image")
		}
		imgType := stringFromMap(hints, "imageType")
		if imgType == "" {
			imgType = "main"
		}
		score, err := s.scoreImageURL(ctx, src, imgType, productTitle)
		if err != nil {
			return err
		}
		raw, _ := scoreJSONFromScore(score)
		outObj := map[string]any{
			"provider": task.Provider,
			"taskType": task.TaskType,
			"score":    json.RawMessage(raw),
		}
		return s.finalizeTaskSuccess(ctx, nil, task, "", nil, "", outObj, raw, false)

	case TaskTypeSelectBestMain:
		if task.ProductID == nil {
			return fmt.Errorf("productId required for select_best_main")
		}
		var images []product.ProductImage
		if err := s.DB.WithContext(ctx).
			Where("product_id = ?", *task.ProductID).
			Order("sort_order ASC, created_at ASC").
			Find(&images).Error; err != nil {
			return err
		}
		if len(images) == 0 {
			return fmt.Errorf("product has no images to score")
		}
		mode := selectModeFromHints(hints)
		type ranked struct {
			img    product.ProductImage
			score  ImageScore
			weight float64
		}
		var rankedList []ranked
		for _, img := range images {
			url := strings.TrimSpace(img.PublicURL)
			if url == "" {
				url = strings.TrimSpace(img.OriginURL)
			}
			if url == "" {
				continue
			}
			sc, err := s.scoreImageURL(ctx, url, img.ImageType, productTitle)
			if err != nil {
				continue
			}
			rankedList = append(rankedList, ranked{img: img, score: sc, weight: weightedOverall(sc)})
		}
		if len(rankedList) == 0 {
			return fmt.Errorf("no scorable images found")
		}
		bestIdx := 0
		for i := 1; i < len(rankedList); i++ {
			if rankedList[i].weight > rankedList[bestIdx].weight {
				bestIdx = i
			}
		}
		best := rankedList[bestIdx]
		bestRaw, _ := scoreJSONFromScore(best.score)

		// Persist item rows with scores
		for i, r := range rankedList {
			raw, _ := scoreJSONFromScore(r.score)
			item := &ImageTaskItem{
				TaskID:         task.ID,
				SourceImageID:  ptrUUID(r.img.ID),
				SourceImageURL: strings.TrimSpace(r.img.PublicURL),
				ScoreJSON:      raw,
				Status:         ItemStatusSuccess,
				IsSelectedBest: i == bestIdx,
			}
			_ = s.DB.WithContext(ctx).Create(item).Error
			_ = s.DB.WithContext(ctx).Model(&product.ProductImage{}).
				Where("id = ?", r.img.ID).
				Updates(map[string]any{"score": r.score.OverallScore}).Error
		}

		outObj := map[string]any{
			"provider":       task.Provider,
			"taskType":       task.TaskType,
			"selectMode":     mode,
			"recommendedId":  best.img.ID.String(),
			"recommendedUrl": strings.TrimSpace(best.img.PublicURL),
			"score":          json.RawMessage(bestRaw),
		}
		if err := s.finalizeTaskSuccess(ctx, nil, task, strings.TrimSpace(best.img.PublicURL), nil, strings.TrimSpace(best.img.StorageKey), outObj, bestRaw, true); err != nil {
			return err
		}
		if mode == "auto_set" {
			_ = s.DB.WithContext(ctx).Model(&product.ProductImage{}).
				Where("product_id = ?", *task.ProductID).
				Update("is_best_main", false).Error
			_ = s.DB.WithContext(ctx).Model(&product.ProductImage{}).
				Where("id = ? AND product_id = ?", best.img.ID, *task.ProductID).
				Updates(map[string]any{"is_best_main": true, "image_type": product.ImageTypeMain, "sort_order": 0}).Error
		}
		return nil
	default:
		return fmt.Errorf("unsupported scoring task %q", task.TaskType)
	}
}

func (s *Service) runProviderForTask(ctx context.Context, prov imgprov.Provider, task *ImageTask, src string, hints map[string]any) (*imgprov.ImageResult, error) {
	tt := strings.TrimSpace(strings.ToLower(task.TaskType))

	if IsCleanupTaskType(tt) {
		hints = prepareCleanupHints(task, hints)
		if strings.EqualFold(strings.TrimSpace(task.Provider), "openai_image") {
			rb, err := s.resolveOpenAIEditSource(ctx, task)
			if err != nil {
				return nil, err
			}
			if rb.File != nil {
				defer rb.File.Close()
			}
			return prov.ReplaceBackground(ctx, imgprov.ReplaceBackgroundRequest{
				ImageRequest: imgprov.ImageRequest{
					SourceURL:         rb.PublicURL,
					SourceFile:        rb.File,
					SourceFilename:    rb.Filename,
					SourceContentType: rb.ContentType,
					Input:             hints,
				},
			})
		}
		if strings.EqualFold(strings.TrimSpace(task.Provider), "comfyui") {
			return prov.ReplaceBackground(ctx, imgprov.ReplaceBackgroundRequest{
				ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
			})
		}
		return prov.Enhance(ctx, imgprov.ImageRequest{SourceURL: src, Input: hints})
	}

	switch tt {
	case TaskTypeGenerateMarketing, TaskTypeGenerateMainImage, TaskTypeBatchGenerateMain:
		hints = prepareGenerationHints(task, hints)
		if src != "" && (strings.EqualFold(task.Provider, "openai_image") || strings.EqualFold(task.Provider, "comfyui")) {
			rb, err := s.resolveOpenAIEditSource(ctx, task)
			if err == nil {
				if rb.File != nil {
					defer rb.File.Close()
				}
				return prov.ReplaceBackground(ctx, imgprov.ReplaceBackgroundRequest{
					ImageRequest: imgprov.ImageRequest{
						SourceURL:         rb.PublicURL,
						SourceFile:        rb.File,
						SourceFilename:    rb.Filename,
						SourceContentType: rb.ContentType,
						Input:             hints,
					},
				})
			}
		}
		return prov.GenerateScene(ctx, imgprov.GenerateSceneRequest{
			ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
			Scene:        stringFromMap(hints, "scene"),
		})
	default:
		return s.dispatch(ctx, prov, task, src, hints)
	}
}
