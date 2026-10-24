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
	tenantID, scoped, err := imageTaskTenantScope(ctx)
	if err != nil {
		return err
	}
	tt := strings.TrimSpace(strings.ToLower(task.TaskType))

	productTitle := ""
	if task.ProductID != nil {
		var p product.Product
		query := s.DB.WithContext(ctx).Model(&product.Product{}).Select("title").Where("id = ?", *task.ProductID)
		query = scopeProductQuery(query, tenantID, scoped)
		if err := query.First(&p).Error; err == nil {
			productTitle = strings.TrimSpace(p.Title)
		} else if scoped {
			return err
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
		if task.SourceImageID != nil {
			v := score.OverallScore
			query := s.DB.WithContext(ctx).Model(&product.ProductImage{}).
				Where("id = ?", *task.SourceImageID)
			query = scopeProductImageQuery(query, tenantID, scoped)
			_ = query.Update("score", v).Error
		}
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
		imageQuery := s.DB.WithContext(ctx).Model(&product.ProductImage{}).
			Where("product_id = ?", *task.ProductID)
		imageQuery = scopeProductImageQuery(imageQuery, tenantID, scoped)
		if err := imageQuery.
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
				ProductID:      task.ProductID,
				SourceImageID:  ptrUUID(r.img.ID),
				SourceImageURL: strings.TrimSpace(r.img.PublicURL),
				ScoreJSON:      raw,
				Status:         ItemStatusSuccess,
				IsSelectedBest: i == bestIdx,
			}
			_ = s.DB.WithContext(ctx).Create(item).Error
			imageUpdate := s.DB.WithContext(ctx).Model(&product.ProductImage{}).
				Where("id = ?", r.img.ID)
			imageUpdate = scopeProductImageQuery(imageUpdate, tenantID, scoped)
			_ = imageUpdate.Updates(map[string]any{"score": r.score.OverallScore}).Error
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
			clearBest := s.DB.WithContext(ctx).Model(&product.ProductImage{}).
				Where("product_id = ?", *task.ProductID)
			clearBest = scopeProductImageQuery(clearBest, tenantID, scoped)
			_ = clearBest.Update("is_best_main", false).Error
			setBest := s.DB.WithContext(ctx).Model(&product.ProductImage{}).
				Where("id = ? AND product_id = ?", best.img.ID, *task.ProductID)
			setBest = scopeProductImageQuery(setBest, tenantID, scoped)
			_ = setBest.Updates(map[string]any{"is_best_main": true, "image_type": product.ImageTypeMain, "sort_order": 0}).Error
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
		provName := strings.TrimSpace(strings.ToLower(task.Provider))
		if provName == "openai_image" || provName == "dashscope_image" {
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
		if provName == "comfyui" {
			return prov.ReplaceBackground(ctx, imgprov.ReplaceBackgroundRequest{
				ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
			})
		}
		return nil, imgprov.UnsupportedTaskError(task.Provider, task.TaskType)
	}

	switch tt {
	case TaskTypeGenerateMarketing, TaskTypeGenerateMainImage, TaskTypeBatchGenerateMain:
		hints = prepareGenerationHints(task, hints)
		provName := strings.TrimSpace(strings.ToLower(task.Provider))
		if src != "" && (provName == "openai_image" || provName == "comfyui" || provName == "dashscope_image") {
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
