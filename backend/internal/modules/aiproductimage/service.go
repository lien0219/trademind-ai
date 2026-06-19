package aiproductimage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/imagetask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/safedownload"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func detachedGinContext(c *gin.Context) *gin.Context {
	if c == nil || c.Request == nil {
		return c
	}
	cp := c.Copy()
	cp.Request = cp.Request.WithContext(context.Background())
	return cp
}

// Service orchestrates AI product image batch operations with human review.
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	Products *product.Service
	Image    *imagetask.Service
	OpLog    *operationlog.Service
}

func (s *Service) batchMaxProducts(ctx context.Context) int {
	return settingInt(ctx, s.Settings, "ai_image_batch_max_products", defaultMaxProducts, 500)
}

func (s *Service) batchMaxImages(ctx context.Context) int {
	return settingInt(ctx, s.Settings, "ai_image_batch_max_images", defaultMaxImages, 1000)
}

func (s *Service) batchConcurrency(ctx context.Context) int {
	return settingInt(ctx, s.Settings, "ai_image_batch_concurrency", 2, 16)
}

func settingInt(ctx context.Context, svc *settings.Service, key string, def, max int) int {
	if svc == nil {
		return def
	}
	m, err := svc.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return def
	}
	if v := strings.TrimSpace(m[key]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= max {
			return n
		}
	}
	return def
}

func (s *Service) configuredImageProvider(ctx context.Context) string {
	if s == nil || s.Settings == nil {
		return ""
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(strings.ToLower(m["image_task_default_provider"]))
	if v == "" {
		v = strings.TrimSpace(strings.ToLower(m["provider"]))
	}
	return v
}

func (s *Service) imageConfigured(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return false
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return false
	}
	v := strings.TrimSpace(strings.ToLower(m["image_task_default_provider"]))
	if v == "" {
		v = strings.TrimSpace(strings.ToLower(m["provider"]))
	}
	return v != "" && v != "noop"
}

func parseProductIDs(raw []string) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		u, err := uuid.Parse(item)
		if err != nil {
			return nil, fmt.Errorf("商品 ID 无效")
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("请至少选择一个商品")
	}
	return out, nil
}

func parseImageIDs(raw []string) ([]uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		u, err := uuid.Parse(item)
		if err != nil {
			return nil, fmt.Errorf("图片 ID 无效")
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out, nil
}

func imageURLHash(url string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(url)))
	return hex.EncodeToString(h[:])
}

func (s *Service) loadProducts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*product.Product, error) {
	var rows []product.Product
	if err := s.DB.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[uuid.UUID]*product.Product, len(rows))
	for i := range rows {
		p := &rows[i]
		if p.DeletedAt.Valid {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(p.Status), product.StatusArchived) {
			continue
		}
		m[p.ID] = p
	}
	for _, id := range ids {
		if _, ok := m[id]; !ok {
			return nil, fmt.Errorf("商品不存在或不可用")
		}
	}
	return m, nil
}

type imageSelection struct {
	ProductID uuid.UUID
	Image     product.ProductImage
}

func (s *Service) resolveImageSelections(ctx context.Context, productIDs, imageIDs []uuid.UUID) ([]imageSelection, error) {
	var images []product.ProductImage
	q := s.DB.WithContext(ctx).Where("product_id IN ?", productIDs)
	if len(imageIDs) > 0 {
		q = q.Where("id IN ?", imageIDs)
	}
	if err := q.Order("product_id ASC, sort_order ASC, created_at ASC").Find(&images).Error; err != nil {
		return nil, err
	}
	out := make([]imageSelection, 0, len(images))
	for _, img := range images {
		out = append(out, imageSelection{ProductID: img.ProductID, Image: img})
	}
	return out, nil
}

func (s *Service) validateImageURL(ctx context.Context, rawURL string) (accessible bool, issues []string) {
	url := strings.TrimSpace(rawURL)
	if url == "" {
		return false, []string{"图片缺少有效链接"}
	}
	if err := safedownload.ValidateURL(ctx, url); err != nil {
		return false, []string{"图片链接不安全或无效"}
	}
	res, err := safedownload.Download(ctx, url, safedownload.DefaultOptions())
	if err != nil {
		msg := safeDownloadUserMessage(err)
		return false, []string{msg}
	}
	if len(res.Data) > 10<<20 {
		return false, []string{"图片文件过大，请压缩后再处理。"}
	}
	return true, nil
}

func (s *Service) checkOneImage(p *product.Product, img product.ProductImage, op string, imageOK bool) CheckBatchItem {
	pubURL := strings.TrimSpace(img.PublicURL)
	if pubURL == "" {
		pubURL = strings.TrimSpace(img.OriginURL)
	}
	item := CheckBatchItem{
		ProductID:      img.ProductID.String(),
		ImageID:        img.ID.String(),
		ImageType:      img.ImageType,
		ImageTypeLabel: imageTypeLabel(img.ImageType),
		SourceImageURL: pubURL,
		OperationType:  op,
		OperationLabel: operationTypeLabel(op),
		Status:         "ready",
		StatusLabel:    checkStatusLabel("ready"),
		Issues:         []string{},
	}
	if p != nil {
		item.ProductTitle = strings.TrimSpace(p.Title)
	}
	if pubURL == "" {
		item.Status = "blocked"
		item.StatusLabel = checkStatusLabel("blocked")
		item.Issues = append(item.Issues, "图片缺少有效链接")
	}
	if op == OpSelectBestMain && img.ImageType != product.ImageTypeMain {
		item.Status = "warning"
		item.StatusLabel = checkStatusLabel("warning")
		item.Issues = append(item.Issues, "主图优选建议通常针对主图")
	}
	if !imageOK {
		item.Status = "blocked"
		item.StatusLabel = checkStatusLabel("blocked")
		item.Issues = append(item.Issues, "AI 图片服务未配置")
	}
	return item
}

// CheckBatch validates products and images before creating a batch.
func (s *Service) CheckBatch(ctx context.Context, req CheckBatchRequest) (*CheckBatchResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("服务不可用")
	}
	productIDs, err := parseProductIDs(req.ProductIDs)
	if err != nil {
		return nil, err
	}
	maxP := s.batchMaxProducts(ctx)
	if len(productIDs) > maxP {
		return nil, fmt.Errorf("本次选择的商品较多，请分批处理（最多 %d 个）", maxP)
	}
	imageIDs, err := parseImageIDs(req.ImageIDs)
	if err != nil {
		return nil, err
	}
	ops, err := normalizeOperationTypes(req.OperationTypes)
	if err != nil {
		return nil, err
	}
	products, err := s.loadProducts(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	selections, err := s.resolveImageSelections(ctx, productIDs, imageIDs)
	if err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return nil, fmt.Errorf("所选商品暂无可处理图片")
	}
	maxI := s.batchMaxImages(ctx)
	if len(selections)*len(ops) > maxI {
		return nil, fmt.Errorf("本次选择的图片较多，请分批处理（最多 %d 张）", maxI)
	}
	imageOK := s.imageConfigured(ctx)
	resp := &CheckBatchResponse{
		Summary: CheckBatchSummary{
			ProductCount: len(productIDs),
			ImageCount:   len(selections),
			ItemCount:    len(selections) * len(ops),
		},
		Items: make([]CheckBatchItem, 0, len(selections)*len(ops)),
	}
	seenProducts := map[uuid.UUID]struct{}{}
	for _, sel := range selections {
		seenProducts[sel.ProductID] = struct{}{}
		p := products[sel.ProductID]
		accessible, urlIssues := s.validateImageURL(ctx, sel.Image.PublicURL)
		if !accessible && strings.TrimSpace(sel.Image.OriginURL) != "" {
			accessible, urlIssues = s.validateImageURL(ctx, sel.Image.OriginURL)
		}
		for _, op := range ops {
			cell := s.checkOneImage(p, sel.Image, op, imageOK)
			if len(urlIssues) > 0 {
				if cell.Status == "ready" {
					cell.Status = "warning"
					cell.StatusLabel = checkStatusLabel("warning")
				}
				cell.Issues = append(cell.Issues, urlIssues...)
			}
			for _, w := range checkImageQualityWarnings(cell.SourceImageURL, accessible) {
				if cell.Status == "ready" {
					cell.Status = "warning"
					cell.StatusLabel = checkStatusLabel("warning")
				}
				cell.Issues = append(cell.Issues, w.Message)
			}
			resp.Items = append(resp.Items, cell)
			switch cell.Status {
			case "ready":
				resp.Summary.ReadyCount++
			case "warning":
				resp.Summary.WarningCount++
			case "blocked":
				resp.Summary.BlockedCount++
			}
		}
	}
	resp.Summary.ProductCount = len(seenProducts)
	return resp, nil
}

func buildIdempotencyKey(adminID *uuid.UUID, productIDs, imageIDs []uuid.UUID, ops []string, opts ImageGenerationOptions) string {
	ps := uuidStrings(productIDs)
	is := uuidStrings(imageIDs)
	sort.Strings(ps)
	sort.Strings(is)
	sort.Strings(ops)
	admin := ""
	if adminID != nil {
		admin = adminID.String()
	}
	optBytes, _ := json.Marshal(opts)
	raw := admin + "|p:" + strings.Join(ps, ",") + "|i:" + strings.Join(is, ",") + "|o:" + strings.Join(ops, ",") + "|" + string(optBytes)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func (s *Service) nextBatchNo(ctx context.Context) (string, error) {
	prefix := fmt.Sprintf("IMG%s", time.Now().UTC().Format("20060102"))
	var last string
	err := s.DB.WithContext(ctx).Model(&AIProductImageBatch{}).
		Select("batch_no").Where("batch_no LIKE ?", prefix+"%").
		Order("batch_no DESC").Limit(1).Scan(&last).Error
	if err != nil || strings.TrimSpace(last) == "" {
		return prefix + "0001", nil
	}
	suf := strings.TrimPrefix(strings.TrimSpace(last), prefix)
	x, err := strconv.Atoi(suf)
	if err != nil || x < 1 {
		return prefix + "0001", nil
	}
	return prefix + fmt.Sprintf("%04d", x+1), nil
}

func summarizeInput(ops []string, opts ImageGenerationOptions) map[string]any {
	return map[string]any{
		"batchType":      BatchTypeAIImage,
		"operationTypes": ops,
		"options": map[string]any{
			"language":         strings.TrimSpace(opts.Language),
			"backgroundStyle":  strings.TrimSpace(opts.BackgroundStyle),
			"keepSubject":      opts.KeepSubject,
			"keepBrandLogo":    opts.KeepBrandLogo,
			"skipFailedImages": opts.SkipFailedImages,
			"outputFormat":     strings.TrimSpace(opts.OutputFormat),
			"remarkLen":        len(strings.TrimSpace(opts.Remark)),
		},
	}
}

// CreateBatch creates items and runs generation asynchronously (never auto-applies).
func (s *Service) CreateBatch(c *gin.Context, req CreateBatchRequest, adminID *uuid.UUID) (*AIProductImageBatch, error) {
	ctx := c.Request.Context()
	if !s.imageConfigured(ctx) {
		return nil, fmt.Errorf("AI 图片服务未配置，请先在「设置 → 图片 AI」完成配置")
	}
	productIDs, err := parseProductIDs(req.ProductIDs)
	if err != nil {
		return nil, err
	}
	maxP := s.batchMaxProducts(ctx)
	if len(productIDs) > maxP {
		return nil, fmt.Errorf("本次选择的商品较多，请分批处理（最多 %d 个）", maxP)
	}
	imageIDs, err := parseImageIDs(req.ImageIDs)
	if err != nil {
		return nil, err
	}
	if len(imageIDs) == 0 {
		return nil, fmt.Errorf("请至少选择一张图片")
	}
	ops, err := normalizeOperationTypes(req.OperationTypes)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadProducts(ctx, productIDs); err != nil {
		return nil, err
	}
	selections, err := s.resolveImageSelections(ctx, productIDs, imageIDs)
	if err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return nil, fmt.Errorf("所选商品暂无可处理图片")
	}
	maxI := s.batchMaxImages(ctx)
	itemCount := len(selections) * len(ops)
	if itemCount > maxI {
		return nil, fmt.Errorf("本次选择的图片较多，请分批处理（最多 %d 张）", maxI)
	}
	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey == "" {
		idemKey = buildIdempotencyKey(adminID, productIDs, imageIDs, ops, req.Options)
	}
	var existing AIProductImageBatch
	if err := s.DB.WithContext(ctx).Where("idempotency_key = ?", idemKey).First(&existing).Error; err == nil {
		return &existing, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	batchNo, err := s.nextBatchNo(ctx)
	if err != nil {
		return nil, err
	}
	inJSON, _ := json.Marshal(summarizeInput(ops, req.Options))
	now := time.Now().UTC()
	uniqueProducts := map[uuid.UUID]struct{}{}
	for _, sel := range selections {
		uniqueProducts[sel.ProductID] = struct{}{}
	}
	batch := &AIProductImageBatch{
		BatchNo:        batchNo,
		BatchType:      BatchTypeAIImage,
		Status:         BatchRunning,
		ProductCount:   len(uniqueProducts),
		ImageCount:     len(selections),
		ItemCount:      itemCount,
		IdempotencyKey: idemKey,
		Input:          datatypes.JSON(inJSON),
		CreatedBy:      adminID,
		StartedAt:      &now,
	}
	if err := s.DB.WithContext(ctx).Create(batch).Error; err != nil {
		return nil, err
	}

	items := make([]AIProductImageItem, 0, itemCount)
	for _, sel := range selections {
		pubURL := strings.TrimSpace(sel.Image.PublicURL)
		if pubURL == "" {
			pubURL = strings.TrimSpace(sel.Image.OriginURL)
		}
		imgID := sel.Image.ID
		imgUpdated := sel.Image.UpdatedAt.UTC()
		hash := imageURLHash(pubURL)
		for _, op := range ops {
			items = append(items, AIProductImageItem{
				BatchID:            batch.ID,
				ProductID:          sel.ProductID,
				ImageID:            &imgID,
				ImageType:          sel.Image.ImageType,
				OperationType:      op,
				Status:             ItemPending,
				SourceImageURL:     pubURL,
				SourceSnapshotHash: hash,
				ImageUpdatedAt:     &imgUpdated,
			})
		}
	}
	if err := s.DB.WithContext(ctx).CreateInBatches(&items, 50).Error; err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.product_image.batch.create",
			Resource:    "ai_product_image_batch",
			ResourceID:  batch.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("batchNo=%s productCount=%d imageCount=%d itemCount=%d", batchNo, batch.ProductCount, batch.ImageCount, itemCount),
		})
	}

	go s.runGeneration(detachedGinContext(c), batch.ID, req.Options, adminID)
	return batch, nil
}

func (s *Service) runGeneration(c *gin.Context, batchID uuid.UUID, opts ImageGenerationOptions, adminID *uuid.UUID) {
	ctx := c.Request.Context()
	conc := s.batchConcurrency(ctx)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	var items []AIProductImageItem
	_ = s.DB.WithContext(ctx).Where("batch_id = ?", batchID).Find(&items).Error
	for _, item := range items {
		item := item
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOneItem(c, &item, opts, adminID)
		}()
	}
	wg.Wait()
	s.finalizeBatch(ctx, batchID)
}

func (s *Service) runOneItem(c *gin.Context, seed *AIProductImageItem, opts ImageGenerationOptions, adminID *uuid.UUID) {
	ctx := c.Request.Context()
	var item AIProductImageItem
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", seed.ID).Error; err != nil {
		return
	}
	if item.Status == ItemCancelled || item.Status == ItemApplied {
		return
	}
	_ = s.DB.WithContext(ctx).Model(&item).Update("status", ItemRunning).Error

	taskType := resolveGenerationTaskType(s.configuredImageProvider(ctx), item.OperationType)
	if taskType == "" {
		s.failItem(ctx, item.ID, "unsupported_operation", "不支持的处理类型")
		return
	}

	pubURL := strings.TrimSpace(item.SourceImageURL)
	inputHints := map[string]any{
		"imageType":       item.ImageType,
		"language":        strings.TrimSpace(opts.Language),
		"backgroundStyle": strings.TrimSpace(opts.BackgroundStyle),
		"keepSubject":     opts.KeepSubject,
		"keepBrandLogo":   opts.KeepBrandLogo,
		"outputFormat":    strings.TrimSpace(opts.OutputFormat),
		"batchReview":     true,
		"autoSave":        false,
		"autoSetMain":     false,
		"autoSetDetail":   false,
	}
	if taskType == imagetask.TaskTypeReplaceBackground {
		inputHints["background"] = strings.TrimSpace(opts.BackgroundStyle)
		if inputHints["background"] == "" {
			inputHints["background"] = "white"
		}
	}
	inputJSON, _ := json.Marshal(inputHints)

	pid := item.ProductID
	payload := imagetask.CreatePayload{
		TaskType:       taskType,
		ProductID:      &pid,
		SourceImageID:  item.ImageID,
		SourceImageURL: pubURL,
		Input:          datatypes.JSON(inputJSON),
		CreatedBy:      adminID,
		BatchID:        &item.BatchID,
	}

	task, err := s.Image.CreateAndPersist(ctx, payload)
	if err != nil {
		s.failItem(ctx, item.ID, "create_failed", truncateMsg(err.Error(), 500))
		return
	}
	if err := s.Image.FinalizeNewImageTask(ctx, c, task); err != nil {
		s.failItem(ctx, item.ID, "enqueue_failed", truncateMsg(err.Error(), 500))
		return
	}

	finished, runErr := s.waitForImageTask(ctx, task.ID, 5*time.Minute)
	if runErr != nil {
		s.failItem(ctx, item.ID, "generation_failed", truncateMsg(runErr.Error(), 500))
		return
	}

	updates := map[string]any{
		"status":        ItemPendingReview,
		"image_task_id": finished.ID,
	}
	warnings := []QualityWarning{}
	if item.OperationType == OpQualityCheck {
		updates["result_image_url"] = pubURL
		if len(finished.Output) > 0 {
			var out map[string]json.RawMessage
			if json.Unmarshal(finished.Output, &out) == nil {
				if sc, ok := out["score"]; ok {
					warnings = warningsFromScoreJSON(sc)
				}
			}
		}
	} else {
		resultURL := strings.TrimSpace(finished.ResultURL)
		storageKey := ""
		if len(finished.Output) > 0 {
			var out map[string]any
			if json.Unmarshal(finished.Output, &out) == nil {
				if sk, ok := out["storageKey"].(string); ok {
					storageKey = strings.TrimSpace(sk)
				}
			}
		}
		if resultURL == "" {
			s.failItem(ctx, item.ID, "no_result", "未生成有效结果图")
			return
		}
		updates["result_image_url"] = resultURL
		updates["result_storage_key"] = storageKey
	}
	if len(warnings) > 0 {
		warnJSON, _ := json.Marshal(warnings)
		updates["quality_warnings"] = datatypes.JSON(warnJSON)
	}
	if item.ImageID != nil {
		var img product.ProductImage
		if err := s.DB.WithContext(ctx).First(&img, "id = ?", *item.ImageID).Error; err == nil {
			curURL := strings.TrimSpace(img.PublicURL)
			if curURL == "" {
				curURL = strings.TrimSpace(img.OriginURL)
			}
			if curURL != "" {
				updates["source_image_url"] = curURL
				updates["source_snapshot_hash"] = imageURLHash(curURL)
			}
			t := img.UpdatedAt.UTC()
			updates["image_updated_at"] = &t
		}
	}
	_ = s.DB.WithContext(ctx).Model(&AIProductImageItem{}).Where("id = ?", item.ID).Updates(updates).Error
}

func (s *Service) waitForImageTask(ctx context.Context, taskID uuid.UUID, timeout time.Duration) (*imagetask.ImageTask, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var task imagetask.ImageTask
		if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
			return nil, err
		}
		switch task.Status {
		case imagetask.StatusSuccess, imagetask.StatusSuccessWithWarnings, imagetask.StatusSuccessWithReview, imagetask.StatusLowQuality:
			return &task, nil
		case imagetask.StatusFailed, imagetask.StatusFailedValidation, imagetask.StatusCancelled:
			msg := strings.TrimSpace(task.ErrorMessage)
			if msg == "" {
				msg = "图片处理失败"
			}
			return nil, fmt.Errorf("%s", msg)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("图片处理超时，请稍后在复核页重试")
}

func (s *Service) failItem(ctx context.Context, itemID uuid.UUID, code, msg string) {
	_ = s.DB.WithContext(ctx).Model(&AIProductImageItem{}).Where("id = ?", itemID).Updates(map[string]any{
		"status":        ItemFailed,
		"error_code":    code,
		"error_message": msg,
	}).Error
}

func truncateMsg(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (s *Service) finalizeBatch(ctx context.Context, batchID uuid.UUID) {
	type counts struct {
		Status string
		N      int64
	}
	var rows []counts
	_ = s.DB.WithContext(ctx).Model(&AIProductImageItem{}).
		Select("status, COUNT(*) as n").Where("batch_id = ?", batchID).
		Group("status").Find(&rows).Error
	var success, failed, pending, running, applied int
	for _, r := range rows {
		switch r.Status {
		case ItemPendingReview, ItemSuccess:
			success += int(r.N)
		case ItemFailed:
			failed += int(r.N)
		case ItemPending:
			pending += int(r.N)
		case ItemRunning:
			running += int(r.N)
		case ItemApplied:
			applied += int(r.N)
		}
	}
	st := BatchSuccess
	if failed > 0 && success > 0 {
		st = BatchPartialSuccess
	} else if success == 0 && failed > 0 {
		st = BatchFailed
	} else if pending > 0 || running > 0 {
		st = BatchRunning
	}
	out := map[string]any{"successCount": success, "failedCount": failed, "appliedCount": applied}
	outJSON, _ := json.Marshal(out)
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&AIProductImageBatch{}).Where("id = ?", batchID).Updates(map[string]any{
		"success_count": success,
		"failed_count":  failed,
		"applied_count": applied,
		"status":        st,
		"output":        datatypes.JSON(outJSON),
		"finished_at":   &fin,
	}).Error
}
