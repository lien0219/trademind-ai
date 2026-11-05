package aioperationbatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	applyModeTaskOnly    = "task_only"
	applyModeSaveAIField = "save_ai_field"
	applyTargetAIField   = "ai_field"
)

// Service orchestrates ai_operation_batches (bulk text / image orchestration).
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	Products *product.Service
	Image    *imagetask.Service
	OpLog    *operationlog.Service
}

func (s *Service) requireAudit() error {
	if s == nil || s.OpLog == nil || s.OpLog.DB == nil {
		return fmt.Errorf("ai batch audit unavailable")
	}
	return nil
}

// createBatchWithAudit persists the batch and its creation evidence together.
func (s *Service) createBatchWithAudit(c *gin.Context, batch *AIOperationBatch, opts operationlog.WriteOpts) error {
	if s == nil || s.DB == nil || c == nil || c.Request == nil {
		return fmt.Errorf("ai batch unavailable")
	}
	if err := s.requireAudit(); err != nil {
		return err
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		if strings.TrimSpace(opts.ResourceID) == "" {
			opts.ResourceID = batch.ID.String()
		}
		return s.OpLog.WriteTx(c, tx, opts)
	})
}

func (s *Service) updateBatchWithAudit(c *gin.Context, batchID uuid.UUID, updates map[string]any, opts operationlog.WriteOpts) error {
	if s == nil || s.DB == nil || c == nil || c.Request == nil {
		return fmt.Errorf("ai batch unavailable")
	}
	if err := s.requireAudit(); err != nil {
		return err
	}
	tenantID, err := contextTenantID(c.Request.Context())
	if err != nil {
		return err
	}
	return s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&AIOperationBatch{}).Where("id = ? AND tenant_id = ?", batchID, tenantID).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if strings.TrimSpace(opts.ResourceID) == "" {
			opts.ResourceID = batchID.String()
		}
		return s.OpLog.WriteTx(c, tx, opts)
	})
}

func (s *Service) writeBatchAudit(c *gin.Context, opts operationlog.WriteOpts) error {
	if err := s.requireAudit(); err != nil {
		return err
	}
	return s.OpLog.Write(c, opts)
}

// createImageTaskWithAudit keeps each image task and its creation evidence in
// one transaction. A task that cannot be audited must not become runnable.
func (s *Service) createImageTaskWithAudit(c *gin.Context, p imagetask.CreatePayload, opts operationlog.WriteOpts) (*imagetask.ImageTask, error) {
	if s == nil || s.DB == nil || s.Image == nil || c == nil || c.Request == nil {
		return nil, fmt.Errorf("ai image task unavailable")
	}
	if err := s.requireAudit(); err != nil {
		return nil, err
	}
	var row *imagetask.ImageTask
	err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var err error
		row, err = s.Image.CreateAndPersistTx(c.Request.Context(), tx, p)
		if err != nil {
			return err
		}
		if strings.TrimSpace(opts.ResourceID) == "" {
			opts.ResourceID = row.ID.String()
		}
		return s.OpLog.WriteTx(c, tx, opts)
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func requestTenantID(c *gin.Context) (int64, error) {
	return adminperm.TenantIDFromGin(c)
}

func contextTenantID(ctx context.Context) (int64, error) {
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return 0, err
	}
	return tenant.TenantID, nil
}

func truncateForLog(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func (s *Service) aiBatchEnabled(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return false
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(m["ai_batch_enabled"]), "true")
}

func (s *Service) aiBatchMaxSize(ctx context.Context) int {
	max := 100
	if s == nil || s.Settings == nil {
		return max
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return max
	}
	if v := strings.TrimSpace(m["ai_batch_max_size"]); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			return n
		}
	}
	return max
}

func (s *Service) aiBatchConcurrency(ctx context.Context) int {
	n := 2
	if s == nil || s.Settings == nil {
		return n
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return n
	}
	if v := strings.TrimSpace(m["ai_batch_concurrency"]); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x >= 1 && x <= 16 {
			return x
		}
	}
	return n
}

func (s *Service) aiBatchDefaultSaveAIField(ctx context.Context) bool {
	if s == nil || s.Settings == nil {
		return true
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "ai")
	if err != nil {
		return true
	}
	v := strings.TrimSpace(m["ai_batch_auto_save_ai_field"])
	if v == "" {
		return true
	}
	return strings.EqualFold(v, "true")
}

func (s *Service) nextBatchNo(ctx context.Context) (string, error) {
	if s == nil || s.DB == nil {
		return "", fmt.Errorf("ai batch: no db")
	}
	prefix := fmt.Sprintf("AI%s", time.Now().UTC().Format("20060102"))
	var last string
	err := s.DB.WithContext(ctx).Model(&AIOperationBatch{}).
		Select("batch_no").
		Where("batch_no LIKE ?", prefix+"%").
		Order("batch_no DESC").
		Limit(1).
		Scan(&last).Error
	if err != nil || strings.TrimSpace(last) == "" {
		return prefix + fmt.Sprintf("%04d", 1), nil
	}
	suf := strings.TrimPrefix(strings.TrimSpace(last), prefix)
	x, err := strconv.Atoi(suf)
	if err != nil || x < 1 {
		return prefix + fmt.Sprintf("%04d", 1), nil
	}
	return prefix + fmt.Sprintf("%04d", x+1), nil
}

func batchFiltersNarrow(f ProductFilters) bool {
	if strings.TrimSpace(f.Keyword) != "" {
		return true
	}
	if strings.TrimSpace(f.Status) != "" {
		return true
	}
	if strings.TrimSpace(f.Source) != "" {
		return true
	}
	if f.OnlyMissingAiTitle {
		return true
	}
	if f.OnlyMissingAiDescription {
		return true
	}
	if f.OnlyHasMainImage {
		return true
	}
	return false
}

func batchScopePresent(ids []uuid.UUID, f ProductFilters) bool {
	return len(ids) > 0 || batchFiltersNarrow(f)
}

func parseProductIDs(raw []string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		u, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid productIds")
		}
		out = append(out, u)
	}
	return out, nil
}

// resolveProductIDs resolves explicit ids or filter query; enforces scope + max.
func (s *Service) resolveProductIDs(ctx context.Context, tenantID int64, ids []uuid.UUID, f ProductFilters, maxN int, confirmAll bool) ([]uuid.UUID, error) {
	if !batchScopePresent(ids, f) && !confirmAll {
		return nil, fmt.Errorf("empty batch scope: provide productIds / filters or confirmAll=true")
	}
	if len(ids) > 0 {
		if len(ids) > maxN {
			return nil, fmt.Errorf("product count exceeds ai_batch_max_size (%d)", maxN)
		}
		var visible []uuid.UUID
		tx := s.DB.WithContext(ctx).Model(&product.Product{}).
			Where("tenant_id = ? AND id IN ?", tenantID, ids)
		tx = applyAIProductShopScope(tx, ctx, tenantID, "products.id")
		if err := tx.Pluck("id", &visible).Error; err != nil {
			return nil, err
		}
		if len(visible) != len(ids) {
			return nil, gorm.ErrRecordNotFound
		}
		return ids, nil
	}
	if !batchFiltersNarrow(f) && !confirmAll {
		return nil, fmt.Errorf("filters too broad: set confirmAll=true for full-table selection")
	}

	tx := s.DB.WithContext(ctx).Model(&product.Product{}).Select("id").Where("tenant_id = ?", tenantID)
	tx = applyAIProductShopScope(tx, ctx, tenantID, "products.id")
	if v := strings.TrimSpace(f.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(f.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if v := strings.TrimSpace(f.Keyword); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(title) LIKE ? OR LOWER(original_title) LIKE ?", pat, pat)
	}
	if f.OnlyMissingAiTitle {
		tx = tx.Where("(ai_title IS NULL OR ai_title = '')")
	}
	if f.OnlyMissingAiDescription {
		tx = tx.Where("(ai_description IS NULL OR ai_description = '')")
	}
	if f.OnlyHasMainImage {
		tx = tx.Where(`EXISTS (SELECT 1 FROM product_images pi WHERE pi.product_id = products.id AND pi.image_type = ?)`, product.ImageTypeMain)
	}

	var all []uuid.UUID
	if err := tx.Order("created_at DESC").Limit(maxN+1).Pluck("id", &all).Error; err != nil {
		return nil, err
	}
	if len(all) > maxN {
		return nil, fmt.Errorf("matched product count exceeds ai_batch_max_size (%d)", maxN)
	}
	return all, nil
}

func effectiveTextApplyMode(ctx context.Context, s *Service, explicit string) (bool, error) {
	explicit = strings.TrimSpace(strings.ToLower(explicit))
	if explicit == applyModeTaskOnly {
		return false, nil
	}
	if explicit == applyModeSaveAIField || explicit == "" {
		save := explicit == applyModeSaveAIField
		if explicit == "" {
			save = s.aiBatchDefaultSaveAIField(ctx)
		}
		return save, nil
	}
	return false, fmt.Errorf("invalid applyMode")
}

func summarizeTextInput(operationType string, idsCount int, f ProductFilters, opt ProductTextOptions, applyMode string) map[string]any {
	return map[string]any{
		"operationType": operationType,
		"productCount":  idsCount,
		"filters": map[string]any{
			"keyword":                  truncateForLog(f.Keyword, 120),
			"status":                   strings.TrimSpace(f.Status),
			"source":                   strings.TrimSpace(f.Source),
			"onlyMissingAiTitle":       f.OnlyMissingAiTitle,
			"onlyMissingAiDescription": f.OnlyMissingAiDescription,
		},
		"options": map[string]any{
			"language":  strings.TrimSpace(opt.Language),
			"platform":  strings.TrimSpace(opt.Platform),
			"maxLength": opt.MaxLength,
			"tone":      strings.TrimSpace(opt.Tone),
		},
		"applyMode": applyMode,
	}
}

func summarizeImageInput(operationType string, idsCount int, f ProductFilters, opt ProductImageOptions) map[string]any {
	return map[string]any{
		"operationType": operationType,
		"productCount":  idsCount,
		"filters": map[string]any{
			"keyword":          truncateForLog(f.Keyword, 120),
			"status":           strings.TrimSpace(f.Status),
			"source":           strings.TrimSpace(f.Source),
			"onlyHasMainImage": f.OnlyHasMainImage,
		},
		"options": map[string]any{
			"provider":        strings.TrimSpace(opt.Provider),
			"promptLen":       len(strings.TrimSpace(opt.Prompt)),
			"bgPromptLen":     len(strings.TrimSpace(opt.BackgroundPrompt)),
			"style":           truncateForLog(opt.Style, 80),
			"promptPreview":   truncateForLog(opt.Prompt, 80),
			"bgPromptPreview": truncateForLog(opt.BackgroundPrompt, 80),
		},
	}
}

// CreateProductTextBatch creates a batch and processes products with limited concurrency.
func (s *Service) CreateProductTextBatch(c *gin.Context, body CreateProductTextBatchBody, adminID *uuid.UUID) (*AIOperationBatch, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil || s.Products == nil {
		return nil, fmt.Errorf("ai batch unavailable")
	}
	if !s.aiBatchEnabled(ctx) {
		return nil, fmt.Errorf("bulk AI disabled (settings.ai.ai_batch_enabled=false)")
	}
	tenantID, err := requestTenantID(c)
	if err != nil {
		return nil, err
	}
	op := strings.TrimSpace(strings.ToLower(body.OperationType))
	switch op {
	case OperationTitleOptimize, OperationDescriptionGenerate:
	default:
		return nil, fmt.Errorf("invalid operationType")
	}
	maxN := s.aiBatchMaxSize(ctx)
	ids, err := parseProductIDs(body.ProductIDs)
	if err != nil {
		return nil, err
	}
	ids, err = s.resolveProductIDs(ctx, tenantID, ids, body.Filters, maxN, body.ConfirmAll)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no products matched")
	}
	saveField, err := effectiveTextApplyMode(ctx, s, body.ApplyMode)
	if err != nil {
		return nil, err
	}
	applyModeStr := applyModeTaskOnly
	if saveField {
		applyModeStr = applyModeSaveAIField
	}

	batchNo, err := s.nextBatchNo(ctx)
	if err != nil {
		return nil, err
	}
	inSum := summarizeTextInput(op, len(ids), body.Filters, body.Options, applyModeStr)
	inJSON, _ := json.Marshal(inSum)

	batch := &AIOperationBatch{
		TenantID:      tenantID,
		BatchNo:       batchNo,
		OperationType: op,
		Status:        StatusRunning,
		ProductCount:  len(ids),
		Input:         datatypes.JSON(inJSON),
		CreatedBy:     adminID,
	}
	now := time.Now().UTC()
	batch.StartedAt = &now
	if err := s.createBatchWithAudit(c, batch, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: "ai.batch.create",
		Resource: "ai_operation_batch", Status: "success",
		Message: fmt.Sprintf("batchNo=%s operationType=%s productCount=%d", batchNo, op, len(ids)),
	}); err != nil {
		return nil, err
	}
	batchID := batch.ID

	conc := s.aiBatchConcurrency(ctx)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	var mu sync.Mutex
	successN, failN, skipN := 0, 0, 0

	opt := body.Options
	titleBody := product.OptimizeTitleBody{
		Language:  opt.Language,
		Platform:  opt.Platform,
		MaxLength: opt.MaxLength,
		Tone:      opt.Tone,
	}
	descBody := product.GenerateDescriptionBody{
		Language: opt.Language,
		Platform: opt.Platform,
		Tone:     opt.Tone,
	}

	for _, pid := range ids {
		pid := pid
		if op == OperationTitleOptimize && body.Filters.OnlyMissingAiTitle {
			var has string
			if err := s.DB.WithContext(ctx).Model(&product.Product{}).
				Select("COALESCE(ai_title,'')").
				Where("id = ? AND tenant_id = ?", pid, tenantID).
				Scan(&has).Error; err != nil {
				mu.Lock()
				failN++
				mu.Unlock()
				continue
			}
			if strings.TrimSpace(has) != "" {
				mu.Lock()
				skipN++
				mu.Unlock()
				continue
			}
		}
		if op == OperationDescriptionGenerate && body.Filters.OnlyMissingAiDescription {
			var has string
			if err := s.DB.WithContext(ctx).Model(&product.Product{}).
				Select("COALESCE(ai_description,'')").
				Where("id = ? AND tenant_id = ?", pid, tenantID).
				Scan(&has).Error; err != nil {
				mu.Lock()
				failN++
				mu.Unlock()
				continue
			}
			if strings.TrimSpace(has) != "" {
				mu.Lock()
				skipN++
				mu.Unlock()
				continue
			}
		}

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			cp := c.Copy()
			var runErr error
			switch op {
			case OperationTitleOptimize:
				extra := &product.AITitleRunExtra{
					BatchID:         &batchID,
					BatchNo:         batchNo,
					SkipSingleOpLog: true,
					SaveAIField:     saveField,
				}
				_, runErr = s.Products.OptimizeTitleWithBatch(cp, pid, titleBody, adminID, extra)
			case OperationDescriptionGenerate:
				ex := &product.AIDescriptionRunExtra{
					BatchID:         &batchID,
					BatchNo:         batchNo,
					SkipSingleOpLog: true,
					SaveAIField:     saveField,
				}
				_, runErr = s.Products.GenerateDescriptionWithBatch(cp, pid, descBody, adminID, ex)
			}
			mu.Lock()
			defer mu.Unlock()
			if runErr != nil {
				failN++
			} else {
				successN++
			}
		}()
	}
	wg.Wait()

	taskCount, sErr := s.countAITasksForBatch(ctx, batchID)
	if sErr != nil {
		failedAt := time.Now().UTC()
		failureOutput, _ := json.Marshal(map[string]any{
			"successCount": successN,
			"failedCount":  failN,
			"skippedCount": skipN,
			"error":        "task count unavailable",
		})
		failureErr := s.updateBatchWithAudit(c, batchID, map[string]any{
			"success_count": successN,
			"failed_count":  failN,
			"skipped_count": skipN,
			"output":        datatypes.JSON(failureOutput),
			"status":        StatusFailed,
			"finished_at":   &failedAt,
		}, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: adminID, Action: "ai.batch.failed",
			Resource: "ai_operation_batch", Status: "failure",
			Message: fmt.Sprintf("batchNo=%s task count failed: %v", batchNo, sErr),
		})
		if failureErr != nil {
			return nil, fmt.Errorf("count ai tasks for batch: %w; persist failure: %v", sErr, failureErr)
		}
		return nil, fmt.Errorf("count ai tasks for batch: %w", sErr)
	}
	out := map[string]any{
		"successCount": successN,
		"failedCount":  failN,
		"skippedCount": skipN,
		"taskCount":    taskCount,
	}
	outJSON, _ := json.Marshal(out)
	fin := time.Now().UTC()
	st := deriveTextBatchStatus(successN, failN, len(ids)-skipN)
	updates := map[string]any{
		"task_count":    taskCount,
		"success_count": successN,
		"failed_count":  failN,
		"skipped_count": skipN,
		"output":        datatypes.JSON(outJSON),
		"status":        st,
		"finished_at":   &fin,
	}
	batchAction := "ai.batch.success"
	if failN > 0 && successN > 0 {
		batchAction = "ai.batch.partial_success"
	} else if successN == 0 && failN > 0 {
		batchAction = "ai.batch.failed"
	}
	if err := s.updateBatchWithAudit(c, batchID, updates, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: batchAction,
		Resource: "ai_operation_batch", Status: "success",
		Message: fmt.Sprintf("batchNo=%s success=%d failed=%d skipped=%d", batchNo, successN, failN, skipN),
	}); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, batchID)
}

func deriveTextBatchStatus(success, fail, attempted int) string {
	if attempted == 0 {
		return StatusSuccess
	}
	if fail == 0 {
		return StatusSuccess
	}
	if success == 0 {
		return StatusFailed
	}
	return StatusPartialSuccess
}

func (s *Service) countAITasksForBatch(ctx context.Context, batchID uuid.UUID) (int, error) {
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return 0, err
	}
	var n int64
	tx := s.DB.WithContext(ctx).Table("ai_tasks").
		Joins("JOIN ai_operation_batches b ON b.id = ai_tasks.batch_id").
		Where("ai_tasks.batch_id = ? AND b.tenant_id = ?", batchID, tenantID)
	tx = applyAIProductShopScope(tx, ctx, tenantID, "ai_tasks.product_id")
	err = tx.Count(&n).Error
	return int(n), err
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(x))
		return n
	default:
		return 0
	}
}

// countLatestAITasksByProductStatus counts each product's latest ai_task row in the batch (by created_at).
func (s *Service) countLatestAITasksByProductStatus(ctx context.Context, batchID uuid.UUID) (success int, failed int, err error) {
	if s == nil || s.DB == nil {
		return 0, 0, fmt.Errorf("ai batch: no db")
	}
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return 0, 0, err
	}
	type row struct {
		ProductID uuid.UUID `gorm:"column:product_id"`
		Status    string    `gorm:"column:status"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	var rows []row
	tx := s.DB.WithContext(ctx).Table("ai_tasks").
		Select("ai_tasks.product_id AS product_id, ai_tasks.status AS status, ai_tasks.created_at AS created_at").
		Joins("JOIN ai_operation_batches b ON b.id = ai_tasks.batch_id").
		Where("ai_tasks.batch_id = ? AND b.tenant_id = ? AND ai_tasks.product_id IS NOT NULL", batchID, tenantID)
	tx = applyAIProductShopScope(tx, ctx, tenantID, "ai_tasks.product_id")
	if err := tx.Order("ai_tasks.created_at DESC").Find(&rows).Error; err != nil {
		return 0, 0, err
	}
	seen := map[uuid.UUID]bool{}
	for _, r := range rows {
		if seen[r.ProductID] {
			continue
		}
		seen[r.ProductID] = true
		switch r.Status {
		case "success":
			success++
		case "failed":
			failed++
		}
	}
	return success, failed, nil
}

// CreateProductImagesBatch creates image_tasks for each product main image.
func (s *Service) CreateProductImagesBatch(c *gin.Context, body CreateProductImagesBatchBody, adminID *uuid.UUID) (*AIOperationBatch, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil || s.Image == nil {
		return nil, fmt.Errorf("ai batch unavailable")
	}
	if !s.aiBatchEnabled(ctx) {
		return nil, fmt.Errorf("bulk AI disabled (settings.ai.ai_batch_enabled=false)")
	}
	tenantID, err := requestTenantID(c)
	if err != nil {
		return nil, err
	}
	op := strings.TrimSpace(strings.ToLower(body.OperationType))
	var taskType string
	switch op {
	case OperationImageRemoveBackground:
		taskType = imagetask.TaskTypeRemoveBackground
	case OperationImageGenerateScene:
		taskType = imagetask.TaskTypeGenerateScene
	case OperationImageReplaceBackground:
		taskType = imagetask.TaskTypeReplaceBackground
	case OperationImageBatchGenerateMain:
		taskType = imagetask.TaskTypeBatchGenerateMain
	case OperationImageTranslateImageText:
		taskType = imagetask.TaskTypeTranslateImageText
	case OperationImageScore:
		taskType = imagetask.TaskTypeScoreImage
	case OperationImageSelectBestMain:
		taskType = imagetask.TaskTypeSelectBestMain
	default:
		return nil, fmt.Errorf("invalid operationType")
	}
	needsMainImage := taskType != imagetask.TaskTypeSelectBestMain
	maxN := s.aiBatchMaxSize(ctx)
	ids, err := parseProductIDs(body.ProductIDs)
	if err != nil {
		return nil, err
	}
	ids, err = s.resolveProductIDs(ctx, tenantID, ids, body.Filters, maxN, body.ConfirmAll)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no products matched")
	}

	batchNo, err := s.nextBatchNo(ctx)
	if err != nil {
		return nil, err
	}
	inSum := summarizeImageInput(op, len(ids), body.Filters, body.Options)
	inJSON, _ := json.Marshal(inSum)

	batch := &AIOperationBatch{
		TenantID:      tenantID,
		BatchNo:       batchNo,
		OperationType: op,
		Status:        StatusRunning,
		ProductCount:  len(ids),
		Input:         datatypes.JSON(inJSON),
		CreatedBy:     adminID,
	}
	now := time.Now().UTC()
	batch.StartedAt = &now
	if err := s.createBatchWithAudit(c, batch, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: "ai.batch.create",
		Resource: "ai_operation_batch", Status: "success",
		Message: fmt.Sprintf("batchNo=%s operationType=%s productCount=%d image", batchNo, op, len(ids)),
	}); err != nil {
		return nil, err
	}
	batchID := batch.ID

	opt := body.Options
	inPayload := map[string]any{
		"batchOperation": op,
		"style":          strings.TrimSpace(opt.Style),
	}
	if p := strings.TrimSpace(opt.Prompt); p != "" {
		inPayload["prompt"] = p
	}
	if p := strings.TrimSpace(opt.BackgroundPrompt); p != "" {
		inPayload["backgroundPrompt"] = p
	}
	inBytes, _ := json.Marshal(inPayload)

	createN, skipN, failN := 0, 0, 0
	for _, pid := range ids {
		var img product.ProductImage
		var srcID *uuid.UUID
		var srcURL string
		if needsMainImage {
			err := s.DB.WithContext(ctx).
				Where("product_id = ? AND image_type = ?", pid, product.ImageTypeMain).
				Order("sort_order ASC, created_at ASC").
				First(&img).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					skipN++
					continue
				}
				failN++
				continue
			}
			srcID = &img.ID
			srcURL = strings.TrimSpace(img.PublicURL)
			if srcURL == "" {
				srcURL = strings.TrimSpace(img.OriginURL)
			}
		}
		row, cerr := s.createImageTaskWithAudit(c, imagetask.CreatePayload{
			TaskType:       taskType,
			Provider:       opt.Provider,
			ProductID:      &pid,
			SourceImageID:  srcID,
			SourceImageURL: srcURL,
			Input:          datatypes.JSON(inBytes),
			CreatedBy:      adminID,
			BatchID:        &batchID,
			BatchNo:        batchNo,
		}, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: adminID, Action: "image.task.create", Resource: "image_task", Status: "success",
			Message: fmt.Sprintf("taskType=%s provider=%s productId=%s batchNo=%s", taskType, opt.Provider, pid.String(), batchNo),
		})
		if cerr != nil {
			failN++
			continue
		}
		if taskType == imagetask.TaskTypeTranslateImageText {
			// For translate image text, we do NOT enqueue to the general image:tasks queue.
			// We will process them via a dedicated background goroutine to enforce concurrency and delay.
		} else {
			if ferr := s.Image.FinalizeNewImageTask(ctx, c, row); ferr != nil {
				failN++
				continue
			}
		}
		createN++
	}

	if taskType == imagetask.TaskTypeTranslateImageText && len(ids) > 0 {
		asyncCtx, asyncErr := detachedTenantContext(ctx)
		if asyncErr != nil {
			return nil, asyncErr
		}
		go func() {
			if err := s.processTranslateImageBatchAsync(asyncCtx, batchID, adminID); err != nil {
				slog.ErrorContext(context.Background(), "ai_image_batch_async_failed", "batchId", batchID.String(), "error", err)
			}
		}()
	}

	batchAction := "ai.batch.success"
	if failN > 0 && createN > 0 {
		batchAction = "ai.batch.partial_success"
	} else if createN == 0 && failN > 0 {
		batchAction = "ai.batch.failed"
	}
	if err := s.reconcileImageBatchWithAudit(ctx, c, batchID, skipN, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: batchAction,
		Resource: "ai_operation_batch", ResourceID: batchID.String(), Status: "success",
		Message: fmt.Sprintf("batchNo=%s tasks=%d failed=%d skipped=%d", batchNo, createN, failN, skipN),
	}); err != nil {
		return nil, err
	}
	batch2, err := s.GetByID(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("load image batch after creation: %w", err)
	}
	return batch2, nil
}

func (s *Service) processTranslateImageBatchAsync(ctx context.Context, batchID uuid.UUID, adminID *uuid.UUID) error {
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return err
	}
	concurrency := 1
	delayMinMs := 1000
	delayMaxMs := 3000

	if s.Settings != nil {
		if m, err := s.Settings.PlainByGroup(ctx, 0, "image"); err == nil {
			if v := intFromAny(m["ocr_batch_concurrency"]); v > 0 {
				concurrency = v
			} else if v := intFromAny(m["translate_image_text_batch_concurrency"]); v > 0 {
				concurrency = v
			}
			if v := intFromAny(m["ocr_request_interval_ms"]); v >= 0 {
				delayMinMs = v
				delayMaxMs = v
			} else if v := intFromAny(m["translate_image_text_batch_delay_min_ms"]); v >= 0 {
				delayMinMs = v
			}
			if v := intFromAny(m["translate_image_text_batch_delay_max_ms"]); v > 0 {
				delayMaxMs = v
			}
		}
	}

	var tasks []imagetask.ImageTask
	if err := s.DB.WithContext(ctx).Where("batch_id = ? AND tenant_id = ? AND status = ?", batchID, tenantID, "pending").Find(&tasks).Error; err != nil {
		return fmt.Errorf("load pending image tasks: %w", err)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for i, task := range tasks {
		wg.Add(1)
		sem <- struct{}{}

		go func(taskID uuid.UUID) {
			defer wg.Done()
			defer func() { <-sem }()

			if err := s.Image.ProcessSyncAfterCreate(ctx, taskID, nil); err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("process image task %s: %w", taskID, err)
				}
				errMu.Unlock()
			}
		}(task.ID)

		if i < len(tasks)-1 && delayMaxMs > 0 {
			delay := delayMinMs
			if delayMaxMs > delayMinMs {
				// Simple pseudo-random delay without bringing in math/rand directly or just use average
				delay = delayMinMs + (delayMaxMs-delayMinMs)/2
			}
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	wg.Wait()
	reconcileErr := s.reconcileImageBatchWithAudit(ctx, nil, batchID, 0, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: "ai.batch.image_reconciled",
		Resource: "ai_operation_batch", ResourceID: batchID.String(), Status: "success",
		Message: "translated image tasks reconciled",
	})
	if firstErr != nil && reconcileErr != nil {
		return fmt.Errorf("process image task: %v; reconcile image batch: %w", firstErr, reconcileErr)
	}
	if firstErr != nil {
		return firstErr
	}
	if reconcileErr != nil {
		return fmt.Errorf("reconcile image batch: %w", reconcileErr)
	}
	return nil
}

func (s *Service) reconcileImageBatch(ctx context.Context, batchID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("no db")
	}
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return err
	}
	updates, err := s.buildImageBatchReconciliation(ctx, s.DB.WithContext(ctx), batchID, tenantID, 0)
	if err != nil {
		return err
	}
	result := s.DB.WithContext(ctx).Model(&AIOperationBatch{}).Where("id = ? AND tenant_id = ?", batchID, tenantID).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) reconcileImageBatchWithAudit(ctx context.Context, c *gin.Context, batchID uuid.UUID, skippedCount int, opts operationlog.WriteOpts) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("no db")
	}
	if err := s.requireAudit(); err != nil {
		return err
	}
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return err
	}
	if opts.TenantID == 0 {
		opts.TenantID = tenantID
	}
	if strings.TrimSpace(opts.ResourceID) == "" {
		opts.ResourceID = batchID.String()
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates, err := s.buildImageBatchReconciliation(ctx, tx.WithContext(ctx), batchID, tenantID, skippedCount)
		if err != nil {
			return err
		}
		result := tx.WithContext(ctx).Model(&AIOperationBatch{}).Where("id = ? AND tenant_id = ?", batchID, tenantID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if c != nil {
			return s.OpLog.WriteTx(c, tx, opts)
		}
		return s.OpLog.WriteTxContext(ctx, tx, opts)
	})
}

func (s *Service) buildImageBatchReconciliation(ctx context.Context, db *gorm.DB, batchID uuid.UUID, tenantID int64, skippedCount int) (map[string]any, error) {
	if db == nil {
		return nil, fmt.Errorf("no db")
	}
	type row struct {
		Status string
		N      int64
	}
	var rows []row
	if err := db.WithContext(ctx).Model(&imagetask.ImageTask{}).
		Select("status, COUNT(*) as n").
		Where("batch_id = ? AND tenant_id = ?", batchID, tenantID).
		Group("status").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	var pending, running, success, failed, cancelled int
	for _, r := range rows {
		switch strings.TrimSpace(r.Status) {
		case imagetask.StatusPending, imagetask.StatusRetrying:
			pending += int(r.N)
		case imagetask.StatusRunning:
			running += int(r.N)
		case imagetask.StatusSuccess:
			success += int(r.N)
		case imagetask.StatusFailed:
			failed += int(r.N)
		case imagetask.StatusCancelled:
			cancelled += int(r.N)
		}
	}
	taskCount := pending + running + success + failed + cancelled
	st := StatusRunning
	if pending == 0 && running == 0 {
		if failed == 0 {
			st = StatusSuccess
		} else if success == 0 {
			st = StatusFailed
		} else {
			st = StatusPartialSuccess
		}
	}
	out := map[string]any{
		"successCount": success,
		"failedCount":  failed,
		"skippedCount": skippedCount,
		"pendingCount": pending,
		"runningCount": running,
		"taskCount":    taskCount,
	}
	outJSON, _ := json.Marshal(out)
	updates := map[string]any{
		"task_count":    taskCount,
		"success_count": success,
		"failed_count":  failed,
		"skipped_count": skippedCount,
		"output":        datatypes.JSON(outJSON),
		"status":        st,
	}
	if st != StatusRunning {
		fin := time.Now().UTC()
		updates["finished_at"] = &fin
	}
	return updates, nil
}

func detachedTenantContext(ctx context.Context) (context.Context, error) {
	tenant, err := security.RequireTenantContext(ctx)
	if err != nil {
		return nil, err
	}
	return security.WithTenantContext(context.Background(), tenant), nil
}

// ListBatches returns paginated batches.
func (s *Service) ListBatches(c *gin.Context, q ListBatchesQuery) ([]AIOperationBatch, int64, error) {
	if s == nil || s.DB == nil {
		return nil, 0, fmt.Errorf("no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&AIOperationBatch{})
	tenantID, err := contextTenantID(c.Request.Context())
	if err != nil {
		return nil, 0, err
	}
	tx = tx.Where("tenant_id = ?", tenantID)
	tx = applyAIBatchShopScope(tx, c.Request.Context(), tenantID)
	if v := strings.TrimSpace(q.OperationType); v != "" {
		tx = tx.Where("operation_type = ?", v)
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if q.CreatedBy != nil {
		if u, err := uuid.Parse(strings.TrimSpace(*q.CreatedBy)); err == nil {
			tx = tx.Where("created_by = ?", u)
		}
	}
	if q.Start != nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(*q.Start)); err == nil {
			tx = tx.Where("created_at >= ?", t)
		}
	}
	if q.End != nil {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(*q.End)); err == nil {
			tx = tx.Where("created_at <= ?", t)
		}
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * ps
	var items []AIOperationBatch
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetByID returns a batch (reconciles image stats if applicable).
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*AIOperationBatch, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("no db")
	}
	var b AIOperationBatch
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return nil, err
	}
	query := s.DB.WithContext(ctx).Model(&AIOperationBatch{}).Where("id = ? AND tenant_id = ?", id, tenantID)
	query = applyAIBatchShopScope(query, ctx, tenantID)
	if err := query.First(&b).Error; err != nil {
		return nil, err
	}
	switch strings.TrimSpace(b.OperationType) {
	case OperationImageRemoveBackground, OperationImageGenerateScene, OperationImageReplaceBackground,
		OperationImageBatchGenerateMain, OperationImageTranslateImageText, OperationImageScore, OperationImageSelectBestMain:
		if err := s.reconcileImageBatch(ctx, id); err != nil {
			return nil, fmt.Errorf("reconcile image batch: %w", err)
		}
		query = s.DB.WithContext(ctx).Model(&AIOperationBatch{}).Where("id = ? AND tenant_id = ?", id, tenantID)
		query = applyAIBatchShopScope(query, ctx, tenantID)
		if err := query.First(&b).Error; err != nil {
			return nil, err
		}
	}
	return &b, nil
}

// operationTypeByID reads only the operation discriminator for authorization
// decisions. It deliberately does not reconcile image batches, because callers
// must complete the operation-specific permission check before triggering any
// state-changing reconciliation.
func (s *Service) operationTypeByID(ctx context.Context, id uuid.UUID) (string, error) {
	if s == nil || s.DB == nil {
		return "", fmt.Errorf("no db")
	}
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return "", err
	}
	var row struct {
		OperationType string `gorm:"column:operation_type"`
	}
	if err := s.DB.WithContext(ctx).Model(&AIOperationBatch{}).
		Select("operation_type").
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Scopes(func(tx *gorm.DB) *gorm.DB { return applyAIBatchShopScope(tx, ctx, tenantID) }).
		First(&row).Error; err != nil {
		return "", err
	}
	return strings.TrimSpace(row.OperationType), nil
}

// ListBatchAITasks returns ai_tasks for a text batch.
func (s *Service) ListBatchAITasks(c *gin.Context, batchID uuid.UUID, page, ps int) ([]map[string]any, int64, error) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	tenantID, err := contextTenantID(c.Request.Context())
	if err != nil {
		return nil, 0, err
	}
	tx := s.DB.WithContext(c.Request.Context()).Table("ai_tasks").
		Joins("JOIN ai_operation_batches b ON b.id = ai_tasks.batch_id").
		Where("ai_tasks.batch_id = ? AND b.tenant_id = ?", batchID, tenantID)
	tx = applyAIProductShopScope(tx, c.Request.Context(), tenantID, "ai_tasks.product_id")
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * ps
	type slim struct {
		ID           uuid.UUID `gorm:"column:id"`
		TaskType     string    `gorm:"column:task_type"`
		Status       string    `gorm:"column:status"`
		ProductID    *uuid.UUID
		ErrorMessage string
		CreatedAt    time.Time
	}
	var rows []slim
	rowsTx := s.DB.WithContext(c.Request.Context()).Table("ai_tasks").
		Select("ai_tasks.id", "ai_tasks.task_type", "ai_tasks.status", "ai_tasks.product_id", "ai_tasks.error_message", "ai_tasks.created_at").
		Joins("JOIN ai_operation_batches b ON b.id = ai_tasks.batch_id").
		Where("ai_tasks.batch_id = ? AND b.tenant_id = ?", batchID, tenantID)
	rowsTx = applyAIProductShopScope(rowsTx, c.Request.Context(), tenantID, "ai_tasks.product_id")
	if err := rowsTx.Order("ai_tasks.created_at DESC, ai_tasks.id DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":       r.ID,
			"taskType": r.TaskType,
			"status":   r.Status,
			"productId": func() string {
				if r.ProductID == nil {
					return ""
				}
				return r.ProductID.String()
			}(),
			"errorMessage": truncateForLog(r.ErrorMessage, 400),
			"createdAt":    r.CreatedAt,
		})
	}
	return out, total, nil
}

// ListBatchImageTasks returns image_tasks for an image batch.
func (s *Service) ListBatchImageTasks(c *gin.Context, batchID uuid.UUID, page, ps int) ([]imagetask.ImageTask, int64, error) {
	if s == nil || s.Image == nil {
		return nil, 0, fmt.Errorf("image service unavailable")
	}
	tenantID, err := contextTenantID(c.Request.Context())
	if err != nil {
		return nil, 0, err
	}
	q := imagetask.ListQuery{Page: page, PageSize: ps, BatchID: &batchID, TenantID: &tenantID}
	res, err := s.Image.List(c, q)
	if err != nil {
		return nil, 0, err
	}
	return res.Items, res.Total, nil
}

// RetryFailed retries failed child tasks for a batch.
func (s *Service) RetryFailed(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (*AIOperationBatch, error) {
	ctx := c.Request.Context()
	if s == nil || s.DB == nil || s.Products == nil || s.Image == nil {
		return nil, fmt.Errorf("ai batch unavailable")
	}
	if !s.aiBatchEnabled(ctx) {
		return nil, fmt.Errorf("bulk AI disabled")
	}
	b, err := s.GetByID(ctx, batchID)
	if err != nil {
		return nil, err
	}
	switch strings.TrimSpace(b.OperationType) {
	case OperationTitleOptimize, OperationDescriptionGenerate:
		return s.retryFailedText(c, b, adminID)
	case OperationImageRemoveBackground, OperationImageGenerateScene, OperationImageReplaceBackground,
		OperationImageBatchGenerateMain, OperationImageTranslateImageText, OperationImageScore, OperationImageSelectBestMain:
		return s.retryFailedImage(c, b, adminID)
	default:
		return nil, fmt.Errorf("unsupported operation for retry")
	}
}

func (s *Service) retryFailedText(c *gin.Context, b *AIOperationBatch, adminID *uuid.UUID) (*AIOperationBatch, error) {
	ctx := c.Request.Context()
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return nil, err
	}
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	opt := ProductTextOptions{}
	if o, ok := in["options"].(map[string]any); ok {
		opt.Language = fmt.Sprint(o["language"])
		opt.Platform = fmt.Sprint(o["platform"])
		opt.MaxLength = intFromAny(o["maxLength"])
		opt.Tone = fmt.Sprint(o["tone"])
	}
	applyMode := fmt.Sprint(in["applyMode"])
	saveField, _ := effectiveTextApplyMode(ctx, s, applyMode)

	taskType := "title_optimize"
	if b.OperationType == OperationDescriptionGenerate {
		taskType = "product_description_generate"
	}

	var failRows []uuid.UUID
	tx := s.DB.WithContext(ctx).Table("ai_tasks").
		Joins("JOIN ai_operation_batches b ON b.id = ai_tasks.batch_id").
		Distinct("product_id").
		Where("ai_tasks.batch_id = ? AND b.tenant_id = ? AND ai_tasks.status = ? AND ai_tasks.task_type = ?", b.ID, tenantID, "failed", taskType).
		Where("product_id IS NOT NULL")
	tx = applyAIProductShopScope(tx, ctx, tenantID, "ai_tasks.product_id")
	if err := tx.Pluck("product_id", &failRows).Error; err != nil {
		return nil, err
	}
	if len(failRows) == 0 {
		return s.GetByID(ctx, b.ID)
	}
	fails := failRows

	titleBody := product.OptimizeTitleBody{Language: opt.Language, Platform: opt.Platform, MaxLength: opt.MaxLength, Tone: opt.Tone}
	descBody := product.GenerateDescriptionBody{Language: opt.Language, Platform: opt.Platform, Tone: opt.Tone}
	conc := s.aiBatchConcurrency(ctx)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	for _, pid := range fails {
		pid := pid
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			cp := c.Copy()
			switch b.OperationType {
			case OperationTitleOptimize:
				_, _ = s.Products.OptimizeTitleWithBatch(cp, pid, titleBody, adminID, &product.AITitleRunExtra{
					BatchID: &b.ID, BatchNo: b.BatchNo, SkipSingleOpLog: true, SaveAIField: saveField,
				})
			case OperationDescriptionGenerate:
				_, _ = s.Products.GenerateDescriptionWithBatch(cp, pid, descBody, adminID, &product.AIDescriptionRunExtra{
					BatchID: &b.ID, BatchNo: b.BatchNo, SkipSingleOpLog: true, SaveAIField: saveField,
				})
			}
		}()
	}
	wg.Wait()

	successN, failN, err := s.countLatestAITasksByProductStatus(ctx, b.ID)
	if err != nil {
		return nil, fmt.Errorf("count latest AI task status: %w", err)
	}
	taskCount, err := s.countAITasksForBatch(ctx, b.ID)
	if err != nil {
		return nil, fmt.Errorf("count AI tasks for retry: %w", err)
	}
	out := map[string]any{"successCount": successN, "failedCount": failN, "taskCount": taskCount}
	outJSON, _ := json.Marshal(out)
	st := StatusSuccess
	if failN > 0 && successN > 0 {
		st = StatusPartialSuccess
	} else if successN == 0 && failN > 0 {
		st = StatusFailed
	}
	fin := time.Now().UTC()
	updates := map[string]any{
		"success_count": successN,
		"failed_count":  failN,
		"task_count":    taskCount,
		"output":        datatypes.JSON(outJSON),
		"status":        st,
		"finished_at":   &fin,
	}
	if err := s.updateBatchWithAudit(c, b.ID, updates, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: "ai.batch.retry_failed",
		Resource: "ai_operation_batch", Status: "success",
		Message: fmt.Sprintf("batchNo=%s retriedProducts=%d", b.BatchNo, len(fails)),
	}); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, b.ID)
}

func (s *Service) retryFailedImage(c *gin.Context, b *AIOperationBatch, adminID *uuid.UUID) (*AIOperationBatch, error) {
	ctx := c.Request.Context()
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return nil, err
	}
	var ids []uuid.UUID
	tx := s.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).
		Select("id").
		Where("batch_id = ? AND tenant_id = ? AND status = ?", b.ID, tenantID, imagetask.StatusFailed)
	tx = applyAIProductShopScope(tx, ctx, tenantID, "image_tasks.product_id")
	if err := tx.Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	if b.OperationType == OperationImageTranslateImageText {
		// Reset failed tasks manually to pending, then run async processor
		for _, id := range ids {
			update := s.DB.WithContext(ctx).Model(&imagetask.ImageTask{}).Where("id = ? AND tenant_id = ?", id, tenantID)
			update = applyAIProductShopScope(update, ctx, tenantID, "image_tasks.product_id")
			if err := update.Updates(map[string]any{
				"status":            imagetask.StatusPending,
				"retry_count":       0,
				"next_retry_at":     nil,
				"retry_enqueued_at": nil,
				"error_message":     "",
				"started_at":        nil,
				"finished_at":       nil,
				"result_url":        "",
				"result_file_id":    nil,
				"output":            nil,
			}).Error; err != nil {
				return nil, err
			}
		}
		asyncCtx, asyncErr := detachedTenantContext(ctx)
		if asyncErr != nil {
			return nil, asyncErr
		}
		go func() {
			if err := s.processTranslateImageBatchAsync(asyncCtx, b.ID, adminID); err != nil {
				slog.ErrorContext(context.Background(), "ai_image_batch_async_failed", "batchId", b.ID.String(), "error", err)
			}
		}()
	} else {
		for _, id := range ids {
			if err := s.Image.RetryEnqueue(c, id); err != nil {
				return nil, err
			}
		}
	}

	if err := s.reconcileImageBatchWithAudit(ctx, c, b.ID, 0, operationlog.WriteOpts{
		TenantID: tenantID, AdminUserID: adminID, Action: "ai.batch.retry_failed",
		Resource: "ai_operation_batch", ResourceID: b.ID.String(), Status: "success",
		Message: fmt.Sprintf("batchNo=%s imageTasks=%d", b.BatchNo, len(ids)),
	}); err != nil {
		return nil, err
	}
	return s.GetByID(ctx, b.ID)
}

// ApplyBatchResults writes successful task outputs to ai_title / ai_description only.
func (s *Service) ApplyBatchResults(c *gin.Context, batchID uuid.UUID, body ApplyBatchResultsBody, adminID *uuid.UUID) (int, error) {
	if c == nil || c.Request == nil {
		return 0, fmt.Errorf("ai batch unavailable")
	}
	ctx := c.Request.Context()
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("no db")
	}
	if strings.TrimSpace(body.Target) != applyTargetAIField {
		return 0, fmt.Errorf("unsupported target")
	}
	tenantID, err := contextTenantID(ctx)
	if err != nil {
		return 0, err
	}
	b, err := s.GetByID(ctx, batchID)
	if err != nil {
		return 0, err
	}
	var wantIDs []uuid.UUID
	if len(body.ProductIDs) > 0 {
		wantIDs, err = parseProductIDs(body.ProductIDs)
		if err != nil {
			return 0, err
		}
	}
	if err := s.requireAudit(); err != nil {
		return 0, err
	}

	applied := 0
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch b.OperationType {
		case OperationTitleOptimize:
			var tasks []struct {
				ID        uuid.UUID
				ProductID uuid.UUID
				Output    datatypes.JSON
				Status    string
			}
			query := tx.WithContext(ctx).Table("ai_tasks").
				Select("ai_tasks.id", "ai_tasks.product_id", "ai_tasks.output", "ai_tasks.status").
				Joins("JOIN ai_operation_batches b ON b.id = ai_tasks.batch_id").
				Where("ai_tasks.batch_id = ? AND b.tenant_id = ? AND ai_tasks.task_type = ? AND ai_tasks.status = ?", batchID, tenantID, "title_optimize", "success").
				Order("ai_tasks.created_at DESC")
			query = applyAIProductShopScope(query, ctx, tenantID, "ai_tasks.product_id")
			if err := query.Find(&tasks).Error; err != nil {
				return err
			}
			seen := map[uuid.UUID]bool{}
			for _, task := range tasks {
				if seen[task.ProductID] {
					continue
				}
				if len(wantIDs) > 0 && !containsUUID(wantIDs, task.ProductID) {
					continue
				}
				seen[task.ProductID] = true
				var output struct {
					OptimizedTitle string `json:"optimizedTitle"`
				}
				if err := json.Unmarshal(task.Output, &output); err != nil || strings.TrimSpace(output.OptimizedTitle) == "" {
					continue
				}
				productUpdate := tx.WithContext(ctx).Model(&product.Product{}).Where("id = ? AND tenant_id = ?", task.ProductID, tenantID)
				productUpdate = applyAIProductShopScope(productUpdate, ctx, tenantID, "products.id")
				res := productUpdate.Update("ai_title", strings.TrimSpace(output.OptimizedTitle))
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 1 {
					applied++
				}
			}
		case OperationDescriptionGenerate:
			var tasks []struct {
				ID        uuid.UUID
				ProductID uuid.UUID
				Output    datatypes.JSON
				Status    string
			}
			query := tx.WithContext(ctx).Table("ai_tasks").
				Select("ai_tasks.id", "ai_tasks.product_id", "ai_tasks.output", "ai_tasks.status").
				Joins("JOIN ai_operation_batches b ON b.id = ai_tasks.batch_id").
				Where("ai_tasks.batch_id = ? AND b.tenant_id = ? AND ai_tasks.task_type = ? AND ai_tasks.status = ?", batchID, tenantID, "product_description_generate", "success").
				Order("ai_tasks.created_at DESC")
			query = applyAIProductShopScope(query, ctx, tenantID, "ai_tasks.product_id")
			if err := query.Find(&tasks).Error; err != nil {
				return err
			}
			seen := map[uuid.UUID]bool{}
			for _, task := range tasks {
				if seen[task.ProductID] {
					continue
				}
				if len(wantIDs) > 0 && !containsUUID(wantIDs, task.ProductID) {
					continue
				}
				seen[task.ProductID] = true
				var output struct {
					Description string `json:"description"`
				}
				if err := json.Unmarshal(task.Output, &output); err != nil || strings.TrimSpace(output.Description) == "" {
					continue
				}
				productUpdate := tx.WithContext(ctx).Model(&product.Product{}).Where("id = ? AND tenant_id = ?", task.ProductID, tenantID)
				productUpdate = applyAIProductShopScope(productUpdate, ctx, tenantID, "products.id")
				res := productUpdate.Update("ai_description", strings.TrimSpace(output.Description))
				if res.Error != nil {
					return res.Error
				}
				if res.RowsAffected == 1 {
					applied++
				}
			}
		default:
			return fmt.Errorf("apply-results only for text batches")
		}

		return s.OpLog.WriteTx(c, tx, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: adminID, Action: "ai.batch.apply_results",
			Resource: "ai_operation_batch", ResourceID: batchID.String(), Status: "success",
			Message: fmt.Sprintf("batchNo=%s applied=%d", b.BatchNo, applied),
		})
	})
	if err != nil {
		return 0, err
	}
	return applied, nil
}

func containsUUID(ids []uuid.UUID, x uuid.UUID) bool {
	for _, u := range ids {
		if u == x {
			return true
		}
	}
	return false
}
