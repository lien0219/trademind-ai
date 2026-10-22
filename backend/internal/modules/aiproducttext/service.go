package aiproducttext

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
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	defaultBatchMax           = 100
	errAITextBatchInProgress  = "AI_TEXT_BATCH_IN_PROGRESS"
	errAITextBatchKeyConflict = "AI_TEXT_BATCH_KEY_CONFLICT"
)

// detachedGinContext returns a gin copy whose request context survives after the HTTP handler returns.
func detachedGinContext(c *gin.Context) *gin.Context {
	if c == nil || c.Request == nil {
		return c
	}
	cp := c.Copy()
	cp.Request = cp.Request.WithContext(context.Background())
	return cp
}

// Service orchestrates AI product text batch operations with human review.
type Service struct {
	DB          *gorm.DB
	Settings    *settings.Service
	Products    *product.Service
	OpLog       *operationlog.Service
	Idempotency *idempotency.Service
	Metrics     *metrics.Catalog
}

func (s *Service) batchMaxSize(ctx context.Context) int {
	max := defaultBatchMax
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

func (s *Service) batchConcurrency(ctx context.Context) int {
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

func (s *Service) aiConfigured() bool {
	return s != nil && s.Products != nil && s.Products.AIGateway != nil && s.Products.AITasks != nil
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

func (s *Service) checkOneProduct(p *product.Product, op string, aiOK bool) CheckBatchItem {
	item := CheckBatchItem{
		ProductID:      p.ID.String(),
		ProductTitle:   strings.TrimSpace(p.Title),
		OperationType:  op,
		OperationLabel: operationTypeLabel(op),
		Status:         "ready",
		StatusLabel:    checkStatusLabel("ready"),
		Issues:         []string{},
	}
	switch op {
	case OpTitle:
		item.CurrentContent = strings.TrimSpace(p.AITitle)
		if strings.TrimSpace(promptTitle(p)) == "" {
			item.Status = "blocked"
			item.StatusLabel = checkStatusLabel("blocked")
			item.Issues = append(item.Issues, "商品标题为空，建议先检查采集结果")
		}
		if strings.TrimSpace(p.AITitle) != "" && strings.TrimSpace(p.Title) != strings.TrimSpace(p.AITitle) {
			item.Status = "warning"
			item.StatusLabel = checkStatusLabel("warning")
			item.Issues = append(item.Issues, "商品已有 AI 标题，生成后需要人工确认")
		}
	case OpDescription:
		item.CurrentContent = strings.TrimSpace(p.AIDescription)
		if strings.TrimSpace(p.Description) == "" && strings.TrimSpace(promptTitle(p)) == "" {
			item.Status = "blocked"
			item.StatusLabel = checkStatusLabel("blocked")
			item.Issues = append(item.Issues, "商品缺少描述基础信息，建议先完善采集结果")
		}
		if strings.TrimSpace(p.AIDescription) != "" {
			item.Status = "warning"
			item.StatusLabel = checkStatusLabel("warning")
			item.Issues = append(item.Issues, "商品描述已有人工修改，生成后需要人工确认")
		}
	}
	if !aiOK {
		item.Status = "blocked"
		item.StatusLabel = checkStatusLabel("blocked")
		item.Issues = append(item.Issues, "AI 服务未配置")
	}
	return item
}

// CheckBatch validates products before creating a batch.
func (s *Service) CheckBatch(ctx context.Context, req CheckBatchRequest) (*CheckBatchResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("服务不可用")
	}
	ids, err := parseProductIDs(req.ProductIDs)
	if err != nil {
		return nil, err
	}
	maxN := s.batchMaxSize(ctx)
	if len(ids) > maxN {
		return nil, fmt.Errorf("批量数量超过上限（%d）", maxN)
	}
	ops, err := normalizeOperationTypes(req.OperationTypes)
	if err != nil {
		return nil, err
	}
	products, err := s.loadProducts(ctx, ids)
	if err != nil {
		return nil, err
	}
	aiOK := s.aiConfigured()
	resp := &CheckBatchResponse{
		Summary: CheckBatchSummary{
			ProductCount: len(ids),
			ItemCount:    len(ids) * len(ops),
		},
		Items: make([]CheckBatchItem, 0, len(ids)*len(ops)),
	}
	for _, id := range ids {
		p := products[id]
		for _, op := range ops {
			cell := s.checkOneProduct(p, op, aiOK)
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
	return resp, nil
}

func buildIdempotencyKey(adminID *uuid.UUID, ids []uuid.UUID, ops []string, opts TextGenerationOptions) string {
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = id.String()
	}
	sort.Strings(idStrs)
	admin := ""
	if adminID != nil {
		admin = adminID.String()
	}
	raw := admin + "|" + strings.Join(idStrs, ",") + "|" + strings.Join(ops, ",")
	optBytes, _ := json.Marshal(opts)
	raw += "|" + string(optBytes)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func (s *Service) nextBatchNo(ctx context.Context) (string, error) {
	prefix := fmt.Sprintf("AT%s", time.Now().UTC().Format("20060102"))
	var last string
	err := s.DB.WithContext(ctx).Model(&AIProductTextBatch{}).
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

func batchRequestHashPayload(req CreateBatchRequest, adminID *uuid.UUID, ids []uuid.UUID, ops []string) []byte {
	idStrs := make([]string, len(ids))
	for i, id := range ids {
		idStrs[i] = id.String()
	}
	sort.Strings(idStrs)
	payload := map[string]any{
		"productIds":     idStrs,
		"operationTypes": ops,
		"options":        req.Options,
	}
	if adminID != nil {
		payload["adminId"] = adminID.String()
	}
	if k := strings.TrimSpace(req.IdempotencyKey); k != "" {
		payload["idempotencyKey"] = k
	}
	b, _ := json.Marshal(payload)
	return b
}

type textBatchAcquire struct {
	RecordID uuid.UUID
	Owner    string
}

func (s *Service) acquireTextBatch(c *gin.Context, ctx context.Context, idemKey string, reqHash []byte) (*textBatchAcquire, *idempotency.AcquireResult, error) {
	if s == nil || s.Idempotency == nil || idemKey == "" {
		return nil, nil, nil
	}
	owner := "ai-text-batch-create"
	if c != nil {
		owner = idempotency.OwnerFromRequest(c.GetString("requestId"), owner)
	}
	res, err := s.Idempotency.Acquire(ctx, idempotency.ScopeAIText, idemKey, idempotency.HashRequest(reqHash), owner, idempotency.DefaultLease)
	decision, rec, _ := idempotency.Classify(res, err)
	switch decision {
	case idempotency.DecisionAlreadySucceeded:
		return nil, res, nil
	case idempotency.DecisionInProgress:
		return nil, res, fmt.Errorf("%s", errAITextBatchInProgress)
	case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
		return nil, res, fmt.Errorf("%s", errAITextBatchKeyConflict)
	case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
		if rec == nil && res != nil {
			rec = res.Record
		}
		if rec == nil {
			return nil, res, fmt.Errorf("idempotency: missing record")
		}
		return &textBatchAcquire{RecordID: rec.ID, Owner: owner}, res, nil
	default:
		return nil, res, err
	}
}

func (s *Service) completeTextBatchCreate(ctx context.Context, job *textBatchAcquire, batchID uuid.UUID) error {
	if s == nil || s.Idempotency == nil || job == nil {
		return nil
	}
	summary, _ := json.Marshal(map[string]string{"batchId": batchID.String()})
	return s.Idempotency.Complete(ctx, job.RecordID, job.Owner, idempotency.CompleteResult{
		ResponseCode:    "AI_TEXT_BATCH_CREATED",
		ResponseSummary: string(summary),
		ResourceType:    "ai_product_text_batch",
		ResourceID:      batchID.String(),
	})
}

func (s *Service) failTextBatchCreate(ctx context.Context, job *textBatchAcquire, code string, retryable bool) {
	if s == nil || s.Idempotency == nil || job == nil {
		return
	}
	_ = s.Idempotency.Fail(ctx, job.RecordID, job.Owner, code, retryable)
}

func (s *Service) resolveExistingTextBatch(ctx context.Context, res *idempotency.AcquireResult, idemKey string) (*AIProductTextBatch, error) {
	rid := ""
	if res != nil {
		rid = res.ResourceID
		if rid == "" && res.Record != nil {
			rid = res.Record.ResourceID
		}
	}
	if rid != "" {
		if bid, err := uuid.Parse(rid); err == nil {
			if batch, err := s.GetBatchByID(ctx, bid); err == nil {
				return batch, nil
			}
		}
	}
	var existing AIProductTextBatch
	if err := s.DB.WithContext(ctx).Where("idempotency_key = ?", idemKey).First(&existing).Error; err == nil {
		return &existing, nil
	}
	return nil, fmt.Errorf("%s", errAITextBatchInProgress)
}

func summarizeInput(ops []string, opts TextGenerationOptions) map[string]any {
	return map[string]any{
		"batchType":      BatchTypeAIText,
		"operationTypes": ops,
		"options": map[string]any{
			"language":           strings.TrimSpace(opts.Language),
			"platform":           strings.TrimSpace(opts.Platform),
			"tone":               strings.TrimSpace(opts.Tone),
			"maxLength":          opts.MaxLength,
			"titleStyle":         strings.TrimSpace(opts.TitleStyle),
			"highlightSelling":   opts.HighlightSelling,
			"keepBrandWords":     opts.KeepBrandWords,
			"keepSpecWords":      opts.KeepSpecWords,
			"removeCollectNoise": opts.RemoveCollectNoise,
			"descStyle":          strings.TrimSpace(opts.DescStyle),
			"descStructure":      strings.TrimSpace(opts.DescStructure),
			"highlightScenarios": opts.HighlightScenarios,
			"generateBullets":    opts.GenerateBullets,
			"keepOriginalParams": opts.KeepOriginalParams,
			"crossBorderReady":   opts.CrossBorderReady,
			"keywords":           opts.Keywords,
			"forbiddenWords":     opts.ForbiddenWords,
			"remarkLen":          len(strings.TrimSpace(opts.Remark)),
		},
	}
}

func sourceSnapshotForProduct(p *product.Product, op string) map[string]any {
	switch op {
	case OpTitle:
		return map[string]any{
			"title":       promptTitle(p),
			"aiTitle":     strings.TrimSpace(p.AITitle),
			"description": strings.TrimSpace(p.Description),
		}
	default:
		return map[string]any{
			"title":         strings.TrimSpace(p.Title),
			"description":   strings.TrimSpace(p.Description),
			"aiDescription": strings.TrimSpace(p.AIDescription),
		}
	}
}

func sourceHashForProduct(p *product.Product, op string) string {
	switch op {
	case OpTitle:
		return contentHash(promptTitle(p))
	default:
		return contentHash(p.Description)
	}
}

// CreateBatch creates items and runs generation asynchronously (never auto-applies).
func (s *Service) CreateBatch(c *gin.Context, req CreateBatchRequest, adminID *uuid.UUID) (*AIProductTextBatch, error) {
	ctx := c.Request.Context()
	if !s.aiConfigured() {
		s.ObserveAIText("unknown", "batch", "environment_blocked", "environment_blocked", "provider_missing", 0)
		return nil, fmt.Errorf("AI 服务未配置")
	}
	ids, err := parseProductIDs(req.ProductIDs)
	if err != nil {
		return nil, err
	}
	maxN := s.batchMaxSize(ctx)
	if len(ids) > maxN {
		return nil, fmt.Errorf("批量数量超过上限（%d）", maxN)
	}
	ops, err := normalizeOperationTypes(req.OperationTypes)
	if err != nil {
		return nil, err
	}
	if _, err := s.loadProducts(ctx, ids); err != nil {
		return nil, err
	}
	idemKey := strings.TrimSpace(req.IdempotencyKey)
	if idemKey == "" {
		idemKey = buildIdempotencyKey(adminID, ids, ops, req.Options)
	}
	reqHash := batchRequestHashPayload(req, adminID, ids, ops)
	idemJob, replayRes, acqErr := s.acquireTextBatch(c, ctx, idemKey, reqHash)
	if acqErr != nil {
		return nil, acqErr
	}
	if idemJob == nil && replayRes != nil {
		return s.resolveExistingTextBatch(ctx, replayRes, idemKey)
	}
	var existing AIProductTextBatch
	if err := s.DB.WithContext(ctx).Where("idempotency_key = ?", idemKey).First(&existing).Error; err == nil {
		return &existing, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	batchNo, err := s.nextBatchNo(ctx)
	if err != nil {
		s.failTextBatchCreate(ctx, idemJob, "AI_TEXT_BATCH_CREATE_FAILED", true)
		return nil, err
	}
	inJSON, _ := json.Marshal(summarizeInput(ops, req.Options))
	itemCount := len(ids) * len(ops)
	now := time.Now().UTC()
	batch := &AIProductTextBatch{
		BatchNo:        batchNo,
		BatchType:      BatchTypeAIText,
		Status:         BatchRunning,
		ProductCount:   len(ids),
		ItemCount:      itemCount,
		IdempotencyKey: idemKey,
		Input:          datatypes.JSON(inJSON),
		CreatedBy:      adminID,
		StartedAt:      &now,
	}
	if err := s.DB.WithContext(ctx).Create(batch).Error; err != nil {
		s.failTextBatchCreate(ctx, idemJob, "AI_TEXT_BATCH_CREATE_FAILED", true)
		s.ObserveAIText("internal", "batch_create", "batch", "failure", "database", 0)
		return nil, err
	}

	products, _ := s.loadProducts(ctx, ids)
	items := make([]AIProductTextItem, 0, itemCount)
	for _, id := range ids {
		p := products[id]
		for _, op := range ops {
			snap := sourceSnapshotForProduct(p, op)
			snapJSON, _ := json.Marshal(snap)
			hash := sourceHashForProduct(p, op)
			pu := p.UpdatedAt.UTC()
			items = append(items, AIProductTextItem{
				BatchID:            batch.ID,
				ProductID:          id,
				OperationType:      op,
				Status:             ItemPending,
				SourceSnapshot:     datatypes.JSON(snapJSON),
				SourceSnapshotHash: hash,
				ProductUpdatedAt:   &pu,
			})
		}
	}
	if err := s.DB.WithContext(ctx).CreateInBatches(&items, 50).Error; err != nil {
		s.failTextBatchCreate(ctx, idemJob, "AI_TEXT_BATCH_CREATE_FAILED", true)
		return nil, err
	}

	if err := s.completeTextBatchCreate(ctx, idemJob, batch.ID); err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.product_text.batch.create",
			Resource:    "ai_product_text_batch",
			ResourceID:  batch.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("batchNo=%s productCount=%d itemCount=%d", batchNo, len(ids), itemCount),
		})
	}

	s.ObserveAIText("internal", "batch_create", "batch", "success", "", 0)
	go s.runGeneration(detachedGinContext(c), batch.ID, ids, ops, req.Options, adminID)
	return batch, nil
}

func (s *Service) runGeneration(c *gin.Context, batchID uuid.UUID, ids []uuid.UUID, ops []string, opts TextGenerationOptions, adminID *uuid.UUID) {
	ctx := c.Request.Context()
	conc := s.batchConcurrency(ctx)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	titleBody := product.OptimizeTitleBody{
		Language:  opts.Language,
		Platform:  opts.Platform,
		MaxLength: opts.MaxLength,
		Tone:      opts.Tone,
	}
	descBody := product.GenerateDescriptionBody{
		Language: opts.Language,
		Platform: opts.Platform,
		Tone:     opts.Tone,
	}

	for _, pid := range ids {
		for _, op := range ops {
			pid, op := pid, op
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				cp := c.Copy()
				s.runOneItem(cp, batchID, pid, op, titleBody, descBody, opts, adminID)
			}()
		}
	}
	wg.Wait()
	s.finalizeBatch(ctx, batchID)
}

func (s *Service) runOneItem(c *gin.Context, batchID, productID uuid.UUID, op string, titleBody product.OptimizeTitleBody, descBody product.GenerateDescriptionBody, opts TextGenerationOptions, adminID *uuid.UUID) {
	ctx := c.Request.Context()
	start := time.Now()
	var item AIProductTextItem
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ? AND product_id = ? AND operation_type = ?", batchID, productID, op).
		First(&item).Error; err != nil {
		return
	}
	if item.Status == ItemCancelled || item.Status == ItemApplied {
		return
	}
	_ = s.DB.WithContext(ctx).Model(&item).Update("status", ItemRunning).Error

	var (
		taskID        string
		generatedText string
		runErr        error
	)
	switch op {
	case OpTitle:
		res, err := s.Products.OptimizeTitleWithBatch(c, productID, titleBody, adminID, &product.AITitleRunExtra{
			SkipSingleOpLog: true,
			SaveAIField:     false,
		})
		if err != nil {
			runErr = err
		} else if res != nil {
			taskID = res.TaskID
			generatedText = strings.TrimSpace(res.OptimizedTitle)
		}
	case OpDescription:
		res, err := s.Products.GenerateDescriptionWithBatch(c, productID, descBody, adminID, &product.AIDescriptionRunExtra{
			SkipSingleOpLog: true,
			SaveAIField:     false,
		})
		if err != nil {
			runErr = err
		} else if res != nil {
			taskID = res.TaskID
			generatedText = strings.TrimSpace(res.Description)
		}
	}

	updates := map[string]any{}
	if runErr != nil {
		updates["status"] = ItemFailed
		updates["error_message"] = truncateMsg(runErr.Error(), 500)
		updates["error_code"] = "generation_failed"
		if isAITextTimeout(runErr.Error()) {
			s.ObserveAIText("configured", op, "timeout", "timeout", "provider_timeout", time.Since(start))
		} else {
			s.ObserveAIText("configured", op, "failure", "failure", classifyAITextError(runErr.Error()), time.Since(start))
		}
	} else {
		s.ObserveAIText("configured", op, "request", "success", "", time.Since(start))
		var p product.Product
		_ = s.DB.WithContext(ctx).Select("title").First(&p, "id = ?", productID).Error
		warnings := checkTitleQuality(generatedText, opts, opts.ForbiddenWords)
		if op == OpDescription {
			warnings = checkDescriptionQuality(generatedText, p.Title, opts.ForbiddenWords)
		}
		warnJSON, _ := json.Marshal(warnings)
		updates["status"] = ItemPendingReview
		updates["generated_text"] = generatedText
		updates["quality_warnings"] = datatypes.JSON(warnJSON)
		if taskID != "" {
			if tid, err := uuid.Parse(taskID); err == nil {
				updates["ai_task_id"] = tid
			}
		}
	}
	_ = s.DB.WithContext(ctx).Model(&AIProductTextItem{}).Where("id = ? AND status = ?", item.ID, ItemRunning).Updates(updates).Error
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
	_ = s.DB.WithContext(ctx).Model(&AIProductTextItem{}).
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
	_ = s.DB.WithContext(ctx).Model(&AIProductTextBatch{}).Where("id = ?", batchID).Updates(map[string]any{
		"success_count": success,
		"failed_count":  failed,
		"applied_count": applied,
		"status":        st,
		"output":        datatypes.JSON(outJSON),
		"finished_at":   &fin,
	}).Error
}

func (s *Service) GetBatchByID(ctx context.Context, id uuid.UUID) (*AIProductTextBatch, error) {
	var b AIProductTextBatch
	if err := s.DB.WithContext(ctx).First(&b, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (s *Service) ListBatches(ctx context.Context, page, pageSize int) ([]BatchListItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	var total int64
	tx := s.DB.WithContext(ctx).Model(&AIProductTextBatch{})
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AIProductTextBatch
	offset := (page - 1) * pageSize
	if err := tx.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]BatchListItem, 0, len(rows))
	for _, b := range rows {
		out = append(out, s.toBatchListItem(&b))
	}
	return out, total, nil
}

func (s *Service) toBatchListItem(b *AIProductTextBatch) BatchListItem {
	ops := []string{}
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	if raw, ok := in["operationTypes"].([]any); ok {
		for _, v := range raw {
			ops = append(ops, fmt.Sprint(v))
		}
	}
	fin := (*string)(nil)
	if b.FinishedAt != nil {
		s := b.FinishedAt.UTC().Format(time.RFC3339)
		fin = &s
	}
	return BatchListItem{
		ID:             b.ID.String(),
		BatchNo:        b.BatchNo,
		Status:         b.Status,
		StatusLabel:    batchStatusLabel(b.Status),
		ProductCount:   b.ProductCount,
		ItemCount:      b.ItemCount,
		SuccessCount:   b.SuccessCount,
		FailedCount:    b.FailedCount,
		AppliedCount:   b.AppliedCount,
		OperationTypes: ops,
		CreatedAt:      b.CreatedAt.UTC().Format(time.RFC3339),
		FinishedAt:     fin,
	}
}

func (s *Service) buildItemDTO(ctx context.Context, item *AIProductTextItem, p *product.Product) ItemDetailDTO {
	dto := ItemDetailDTO{
		ID:                 item.ID.String(),
		ProductID:          item.ProductID.String(),
		OperationType:      item.OperationType,
		OperationLabel:     operationTypeLabel(item.OperationType),
		Status:             item.Status,
		StatusLabel:        itemStatusLabel(item.Status),
		GeneratedText:      item.GeneratedText,
		EditedText:         item.EditedText,
		SourceSnapshotHash: item.SourceSnapshotHash,
		ErrorMessage:       item.ErrorMessage,
	}
	if p != nil {
		dto.ProductTitle = strings.TrimSpace(p.Title)
		switch item.OperationType {
		case OpTitle:
			dto.CurrentContent = strings.TrimSpace(p.AITitle)
			if dto.CurrentContent == "" {
				dto.CurrentContent = strings.TrimSpace(p.Title)
			}
		case OpDescription:
			dto.CurrentContent = strings.TrimSpace(p.AIDescription)
			if dto.CurrentContent == "" {
				dto.CurrentContent = strings.TrimSpace(p.Description)
			}
		}
	}
	dto.PrepareApplyText = strings.TrimSpace(item.EditedText)
	if dto.PrepareApplyText == "" {
		dto.PrepareApplyText = strings.TrimSpace(item.GeneratedText)
	}
	if item.AITaskID != nil {
		dto.AITaskID = item.AITaskID.String()
	}
	if item.ProductUpdatedAt != nil {
		dto.ProductUpdatedAt = item.ProductUpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	if item.ApplicationID != nil {
		dto.ApplicationID = item.ApplicationID.String()
	}
	if item.AppliedAt != nil {
		s := item.AppliedAt.UTC().Format(time.RFC3339)
		dto.AppliedAt = &s
	}
	var warnings []QualityWarning
	_ = json.Unmarshal(item.QualityWarnings, &warnings)
	dto.QualityWarnings = warnings
	return dto
}

// GetBatchDetail returns batch with all items for review workspace.
func (s *Service) GetBatchDetail(ctx context.Context, id uuid.UUID, statusFilter string) (*BatchDetailDTO, error) {
	b, err := s.GetBatchByID(ctx, id)
	if err != nil {
		return nil, err
	}
	q := s.DB.WithContext(ctx).Where("batch_id = ?", id)
	if sf := strings.TrimSpace(statusFilter); sf != "" && sf != "all" {
		q = q.Where("status = ?", sf)
	}
	var items []AIProductTextItem
	if err := q.Order("created_at ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	pids := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		pids = append(pids, it.ProductID)
	}
	products := map[uuid.UUID]*product.Product{}
	if len(pids) > 0 {
		var rows []product.Product
		_ = s.DB.WithContext(ctx).Where("id IN ?", pids).Find(&rows).Error
		for i := range rows {
			products[rows[i].ID] = &rows[i]
		}
	}
	dtoItems := make([]ItemDetailDTO, 0, len(items))
	for i := range items {
		dtoItems = append(dtoItems, s.buildItemDTO(ctx, &items[i], products[items[i].ProductID]))
	}
	var in, out map[string]any
	_ = json.Unmarshal(b.Input, &in)
	_ = json.Unmarshal(b.Output, &out)
	detail := &BatchDetailDTO{
		BatchListItem: s.toBatchListItem(b),
		Items:         dtoItems,
		Input:         in,
		Output:        out,
	}
	return detail, nil
}

// RetryFailed retries failed items and orphaned pending/running items (e.g. after server restart).
func (s *Service) RetryFailed(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (*AIProductTextBatch, error) {
	ctx := c.Request.Context()
	b, err := s.GetBatchByID(ctx, batchID)
	if err != nil {
		return nil, err
	}
	var retryItems []AIProductTextItem
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ? AND status IN ?", batchID, []string{ItemFailed, ItemPending, ItemRunning}).
		Find(&retryItems).Error; err != nil {
		return nil, err
	}
	if len(retryItems) == 0 {
		return b, nil
	}
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	opts := TextGenerationOptions{}
	if o, ok := in["options"].(map[string]any); ok {
		opts.Language = fmt.Sprint(o["language"])
		opts.Platform = fmt.Sprint(o["platform"])
		opts.Tone = fmt.Sprint(o["tone"])
		opts.MaxLength = intFromAny(o["maxLength"])
		if fw, ok := o["forbiddenWords"].([]any); ok {
			for _, w := range fw {
				opts.ForbiddenWords = append(opts.ForbiddenWords, fmt.Sprint(w))
			}
		}
	}
	titleBody := product.OptimizeTitleBody{Language: opts.Language, Platform: opts.Platform, MaxLength: opts.MaxLength, Tone: opts.Tone}
	descBody := product.GenerateDescriptionBody{Language: opts.Language, Platform: opts.Platform, Tone: opts.Tone}

	_ = s.DB.WithContext(ctx).Model(&AIProductTextBatch{}).Where("id = ?", batchID).Updates(map[string]any{
		"status": BatchRunning, "finished_at": nil,
	}).Error

	conc := s.batchConcurrency(ctx)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup
	for _, item := range retryItems {
		item := item
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			cp := c.Copy()
			s.runOneItem(cp, batchID, item.ProductID, item.OperationType, titleBody, descBody, opts, adminID)
		}()
	}
	wg.Wait()
	s.finalizeBatch(ctx, batchID)

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "ai.product_text.batch.retry_failed",
			Resource:    "ai_product_text_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("batchNo=%s retried=%d", b.BatchNo, len(retryItems)),
		})
	}
	return b, nil
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	default:
		return 0
	}
}

// CancelPending cancels pending items without affecting success items.
func (s *Service) CancelPending(ctx context.Context, batchID uuid.UUID) (int, error) {
	res := s.DB.WithContext(ctx).Model(&AIProductTextItem{}).
		Where("batch_id = ? AND status = ?", batchID, ItemPending).
		Update("status", ItemCancelled)
	if res.Error != nil {
		return 0, res.Error
	}
	s.finalizeBatch(ctx, batchID)
	return int(res.RowsAffected), nil
}

// UpdateEditedText saves user-edited text before apply.
func (s *Service) UpdateEditedText(ctx context.Context, itemID uuid.UUID, text string) error {
	var item AIProductTextItem
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		return err
	}
	if item.Status != ItemPendingReview && item.Status != ItemSuccess {
		return fmt.Errorf("当前状态不可编辑")
	}
	return s.DB.WithContext(ctx).Model(&item).Update("edited_text", strings.TrimSpace(text)).Error
}

// RejectItem marks item as rejected.
func (s *Service) RejectItem(ctx context.Context, itemID uuid.UUID) error {
	var item AIProductTextItem
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		return err
	}
	if item.Status != ItemPendingReview && item.Status != ItemSuccess {
		return fmt.Errorf("当前状态不可放弃")
	}
	return s.DB.WithContext(ctx).Model(&item).Update("status", ItemRejected).Error
}

// RegenerateItem re-runs AI for one item.
func (s *Service) RegenerateItem(c *gin.Context, itemID uuid.UUID, adminID *uuid.UUID) (*ItemDetailDTO, error) {
	ctx := c.Request.Context()
	var item AIProductTextItem
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", itemID).Error; err != nil {
		return nil, err
	}
	b, err := s.GetBatchByID(ctx, item.BatchID)
	if err != nil {
		return nil, err
	}
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	opts := TextGenerationOptions{}
	if o, ok := in["options"].(map[string]any); ok {
		opts.Language = fmt.Sprint(o["language"])
		opts.Platform = fmt.Sprint(o["platform"])
		opts.Tone = fmt.Sprint(o["tone"])
		opts.MaxLength = intFromAny(o["maxLength"])
	}
	titleBody := product.OptimizeTitleBody{Language: opts.Language, Platform: opts.Platform, MaxLength: opts.MaxLength, Tone: opts.Tone}
	descBody := product.GenerateDescriptionBody{Language: opts.Language, Platform: opts.Platform, Tone: opts.Tone}
	s.runOneItem(c, item.BatchID, item.ProductID, item.OperationType, titleBody, descBody, opts, adminID)
	var refreshed AIProductTextItem
	_ = s.DB.WithContext(ctx).First(&refreshed, "id = ?", itemID).Error
	var p product.Product
	_ = s.DB.WithContext(ctx).First(&p, "id = ?", refreshed.ProductID).Error
	dto := s.buildItemDTO(ctx, &refreshed, &p)
	return &dto, nil
}

func (s *Service) applyOneItem(c *gin.Context, item *AIProductTextItem, text string, adminID *uuid.UUID) ApplyItemResult {
	result := ApplyItemResult{
		ItemID:    item.ID.String(),
		ProductID: item.ProductID.String(),
	}
	ctx := c.Request.Context()
	allowReplay := item.Status == ItemApplied && s.Idempotency != nil
	if item.Status == ItemApplied && !allowReplay {
		result.Status = ItemConflict
		result.StatusLabel = itemStatusLabel(ItemConflict)
		result.ErrorMessage = "该结果已应用，不能重复应用"
		return result
	}
	if item.Status == ItemConflict {
		result.Status = ItemConflict
		result.StatusLabel = itemStatusLabel(ItemConflict)
		result.ErrorMessage = "该结果存在冲突，不能继续应用"
		return result
	}
	if !allowReplay && item.Status != ItemPendingReview && item.Status != ItemSuccess {
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = "当前状态不可应用"
		return result
	}
	applyText := strings.TrimSpace(text)
	if applyText == "" {
		applyText = strings.TrimSpace(item.EditedText)
	}
	if applyText == "" {
		applyText = strings.TrimSpace(item.GeneratedText)
	}
	if applyText == "" {
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = "没有可应用的文案"
		return result
	}
	if item.AITaskID == nil {
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = "缺少 AI 任务关联"
		return result
	}

	var prod product.Product
	if err := s.DB.WithContext(ctx).First(&prod, "id = ?", item.ProductID).Error; err != nil {
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = err.Error()
		return result
	}
	currentVersion := prod.UpdatedAt.UTC().Format(time.RFC3339Nano)
	targetVersion := currentVersion
	if item.ProductUpdatedAt != nil {
		targetVersion = item.ProductUpdatedAt.UTC().Format(time.RFC3339Nano)
		if !allowReplay && !textVersionMatches(*item.ProductUpdatedAt, prod.UpdatedAt) {
			// A concurrent loser must not downgrade an item already applied by a winner.
			_ = s.DB.WithContext(ctx).Model(&AIProductTextItem{}).
				Where("id = ? AND status IN ?", item.ID, []string{ItemPendingReview, ItemSuccess}).
				Update("status", ItemConflict).Error
			s.ObserveAIText("internal", item.OperationType, "apply_conflict", "failure", ErrCodeTargetVersionConflict, 0)
			result.Status = ItemConflict
			result.StatusLabel = itemStatusLabel(ItemConflict)
			result.ErrorCode = ErrCodeTargetVersionConflict
			result.ErrorMessage = ErrCodeTargetVersionConflict + ": " + ConflictUserMessage
			return result
		}
	}

	resultHash := contentHash(applyText)
	owner := applyIdempotencyOwner(c, "ai-text-apply")
	idemJob, replayRes, acqErr := s.acquireTextApply(ctx, owner, item.BatchID.String(), item.ID.String(), item.ProductID.String(), targetVersion, item.OperationType, resultHash)
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
	if idemJob != nil {
		ok, reconcileErr := s.ReconcileTextApply(ctx, idemJob, item)
		if ok {
			result.Status = ItemApplied
			result.StatusLabel = itemStatusLabel(ItemApplied)
			s.ObserveAIText("internal", item.OperationType, "reconcile", "success", "", 0)
			return result
		}
		if reconcileErr != nil && strings.Contains(reconcileErr.Error(), product.CodeAIApplyReconciliationConflict) {
			s.ObserveAIText("internal", item.OperationType, "reconcile", "failure", "conflict", 0)
			result.Status = ItemConflict
			result.StatusLabel = itemStatusLabel(ItemConflict)
			result.ErrorCode = product.CodeAIApplyReconciliationConflict
			result.ErrorMessage = reconcileErr.Error()
			return result
		}
	}
	if idemJob == nil && replayRes != nil {
		var refreshed AIProductTextItem
		_ = s.DB.WithContext(ctx).First(&refreshed, "id = ?", item.ID).Error
		result.Status = ItemApplied
		result.StatusLabel = itemStatusLabel(ItemApplied)
		if refreshed.Status == ItemApplied {
			return result
		}
		// Replay of a completed apply even if item row is stale.
		result.Status = ItemApplied
		result.StatusLabel = itemStatusLabel(ItemApplied)
		return result
	}

	expectedAt := currentVersion
	var err error
	switch item.OperationType {
	case OpTitle:
		_, err = s.Products.ApplyAITitle(c, item.ProductID, product.ApplyAITitleBody{
			AITitle:            applyText,
			TaskID:             item.AITaskID.String(),
			ExpectedUpdatedAt:  expectedAt,
			SourceSnapshotHash: item.SourceSnapshotHash,
		}, adminID)
	case OpDescription:
		_, err = s.Products.ApplyAIDescription(c, item.ProductID, product.ApplyAIDescriptionBody{
			AIDescription:      applyText,
			TaskID:             item.AITaskID.String(),
			ExpectedUpdatedAt:  expectedAt,
			SourceSnapshotHash: item.SourceSnapshotHash,
		}, adminID)
	default:
		err = fmt.Errorf("unsupported operation type")
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "content conflict") {
			s.failTextApply(ctx, idemJob, ErrCodeTargetVersionConflict, false)
			// A concurrent loser must not downgrade an item already applied by a winner.
			_ = s.DB.WithContext(ctx).Model(&AIProductTextItem{}).
				Where("id = ? AND status IN ?", item.ID, []string{ItemPendingReview, ItemSuccess}).
				Update("status", ItemConflict).Error
			s.ObserveAIText("internal", item.OperationType, "apply_conflict", "failure", ErrCodeTargetVersionConflict, 0)
			result.Status = ItemConflict
			result.StatusLabel = itemStatusLabel(ItemConflict)
			result.ErrorCode = ErrCodeTargetVersionConflict
			result.ErrorMessage = ErrCodeTargetVersionConflict + ": " + ConflictUserMessage
			return result
		}
		s.failTextApply(ctx, idemJob, "AI_TEXT_APPLY_FAILED", true)
		s.ObserveAIText("internal", item.OperationType, "apply", "failure", "apply_failed", 0)
		result.Status = "failed"
		result.StatusLabel = "失败"
		result.ErrorMessage = msg
		return result
	}
	var app product.ProductAIContentApplication
	_ = s.DB.WithContext(ctx).
		Where("product_id = ? AND ai_task_id = ? AND status = ?", item.ProductID, item.AITaskID, product.AIContentApplyStatusApplied).
		Order("applied_at DESC").First(&app).Error
	now := time.Now().UTC()
	updates := map[string]any{
		"status":     ItemApplied,
		"applied_at": &now,
		"applied_by": adminID,
	}
	appID := ""
	if app.ID != uuid.Nil {
		updates["application_id"] = app.ID
		appID = app.ID.String()
	}
	_ = s.DB.WithContext(ctx).Model(item).Updates(updates).Error

	var after product.Product
	_ = s.DB.WithContext(ctx).Select("updated_at").First(&after, "id = ?", item.ProductID).Error
	summary := map[string]string{
		"batchId":       item.BatchID.String(),
		"itemId":        item.ID.String(),
		"productId":     item.ProductID.String(),
		"beforeVersion": targetVersion,
		"afterVersion":  after.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"applicationId": appID,
	}
	_ = s.completeTextApply(ctx, idemJob, summary, appID)

	result.Status = ItemApplied
	result.StatusLabel = itemStatusLabel(ItemApplied)
	s.ObserveAIText("internal", item.OperationType, "apply", "success", "", 0)
	return result
}

func textVersionMatches(expected, current time.Time) bool {
	// Same truncated second: rejects post-apply / post-edit bumps while tolerating DB rounding.
	return expected.UTC().Truncate(time.Second).Equal(current.UTC().Truncate(time.Second))
}

func applyIdempotencyOwner(c *gin.Context, prefix string) string {
	if c != nil {
		if rid := strings.TrimSpace(c.GetString("requestId")); rid != "" {
			return rid
		}
	}
	return prefix + ":" + uuid.New().String()
}

// ApplyItem applies one reviewed item with conflict protection.
func (s *Service) ApplyItem(c *gin.Context, itemID uuid.UUID, body ApplyItemBody, adminID *uuid.UUID) (*ApplyItemResult, error) {
	var item AIProductTextItem
	if err := s.DB.WithContext(c.Request.Context()).First(&item, "id = ?", itemID).Error; err != nil {
		return nil, err
	}
	r := s.applyOneItem(c, &item, body.Text, adminID)
	s.refreshAppliedCount(c.Request.Context(), item.BatchID)
	return &r, nil
}

// ApplySelected applies multiple reviewed items; partial success allowed.
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
		var item AIProductTextItem
		if err := s.DB.WithContext(ctx).Where("id = ? AND batch_id = ?", id, batchID).First(&item).Error; err != nil {
			continue
		}
		r := s.applyOneItem(c, &item, "", adminID)
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
			Action:      "ai.product_text.batch.apply_selected",
			Resource:    "ai_product_text_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("success=%d conflict=%d failed=%d", summary.SuccessCount, summary.ConflictCount, summary.FailedCount),
		})
	}
	return summary, nil
}

func (s *Service) refreshAppliedCount(ctx context.Context, batchID uuid.UUID) {
	var n int64
	_ = s.DB.WithContext(ctx).Model(&AIProductTextItem{}).
		Where("batch_id = ? AND status = ?", batchID, ItemApplied).Count(&n).Error
	_ = s.DB.WithContext(ctx).Model(&AIProductTextBatch{}).Where("id = ?", batchID).Update("applied_count", n).Error
}

// UndoApplied undoes applied items from this batch only.
func (s *Service) UndoApplied(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (*UndoAppliedSummary, error) {
	ctx := c.Request.Context()
	var items []AIProductTextItem
	if err := s.DB.WithContext(ctx).
		Where("batch_id = ? AND status = ? AND application_id IS NOT NULL", batchID, ItemApplied).
		Find(&items).Error; err != nil {
		return nil, err
	}
	summary := &UndoAppliedSummary{Items: []ApplyItemResult{}}
	for _, item := range items {
		owner := applyIdempotencyOwner(c, "ai-text-undo")
		fieldType := product.AIContentFieldTitle
		if item.OperationType == OpDescription {
			fieldType = product.AIContentFieldDescription
		}
		appID := ""
		if item.ApplicationID != nil {
			appID = item.ApplicationID.String()
		}
		r := ApplyItemResult{
			ItemID:    item.ID.String(),
			ProductID: item.ProductID.String(),
		}
		if appID == "" {
			r.Status = "failed"
			r.StatusLabel = "失败"
			r.ErrorMessage = "缺少应用记录"
			summary.FailedCount++
			summary.Items = append(summary.Items, r)
			continue
		}

		var prod product.Product
		if err := s.DB.WithContext(ctx).First(&prod, "id = ?", item.ProductID).Error; err != nil {
			r.Status = "failed"
			r.StatusLabel = "失败"
			r.ErrorMessage = err.Error()
			summary.FailedCount++
			summary.Items = append(summary.Items, r)
			continue
		}
		var app product.ProductAIContentApplication
		if err := s.DB.WithContext(ctx).First(&app, "id = ?", *item.ApplicationID).Error; err != nil {
			r.Status = "failed"
			r.StatusLabel = "失败"
			r.ErrorMessage = err.Error()
			summary.FailedCount++
			summary.Items = append(summary.Items, r)
			continue
		}
		curVal := ""
		switch fieldType {
		case product.AIContentFieldTitle:
			curVal = strings.TrimSpace(prod.AITitle)
		case product.AIContentFieldDescription:
			curVal = strings.TrimSpace(prod.AIDescription)
		}
		if curVal != strings.TrimSpace(app.AppliedValue) {
			_ = s.DB.WithContext(ctx).Model(&item).Update("status", ItemConflict).Error
			r.Status = ItemConflict
			r.StatusLabel = itemStatusLabel(ItemConflict)
			r.ErrorCode = ErrCodeUndoVersionConflict
			r.ErrorMessage = ErrCodeUndoVersionConflict + ": 商品内容已变化，无法撤销"
			summary.ConflictCount++
			summary.Items = append(summary.Items, r)
			continue
		}

		targetVersion := prod.UpdatedAt.UTC().Format(time.RFC3339Nano)
		idemJob, replayRes, acqErr := s.acquireTextUndo(ctx, owner, appID, targetVersion)
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

		_, err := s.Products.UndoAIContent(c, item.ProductID, fieldType, product.UndoAIContentBody{
			ApplicationID: appID,
			// Empty ExpectedUpdatedAt: UndoAIContent already verifies AppliedValue match.
			ExpectedUpdatedAt: "",
		}, adminID)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "content conflict") || strings.Contains(msg, "AI field changed") {
				s.failTextUndo(ctx, idemJob, ErrCodeUndoVersionConflict, false)
				_ = s.DB.WithContext(ctx).Model(&item).Update("status", ItemConflict).Error
				r.Status = ItemConflict
				r.StatusLabel = itemStatusLabel(ItemConflict)
				r.ErrorCode = ErrCodeUndoVersionConflict
				r.ErrorMessage = ErrCodeUndoVersionConflict + ": 商品内容已变化，无法撤销"
				summary.ConflictCount++
			} else {
				s.failTextUndo(ctx, idemJob, "AI_TEXT_UNDO_FAILED", true)
				r.Status = "failed"
				r.StatusLabel = "失败"
				r.ErrorMessage = msg
				summary.FailedCount++
			}
		} else {
			_ = s.completeTextUndo(ctx, idemJob, appID)
			_ = s.DB.WithContext(ctx).Model(&item).Updates(map[string]any{
				"status": ItemPendingReview, "application_id": nil, "applied_at": nil, "applied_by": nil,
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
			Action:      "ai.product_text.batch.undo_applied",
			Resource:    "ai_product_text_batch",
			ResourceID:  batchID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("success=%d conflict=%d failed=%d", summary.SuccessCount, summary.ConflictCount, summary.FailedCount),
		})
	}
	return summary, nil
}

// ListFailedItemsForTaskCenter returns failed items for failure hub (limited).
func (s *Service) ListFailedItemsForTaskCenter(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var items []AIProductTextItem
	if err := s.DB.WithContext(ctx).Where("status = ?", ItemFailed).
		Order("updated_at DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"id":            it.ID.String(),
			"batchId":       it.BatchID.String(),
			"productId":     it.ProductID.String(),
			"operationType": it.OperationType,
			"errorMessage":  truncateMsg(it.ErrorMessage, 200),
			"detailUrl":     "/product/ai-text-batches/" + it.BatchID.String(),
		})
	}
	return out, nil
}

// EnsureAITaskLinked validates ai task belongs to product (for tests).
func (s *Service) EnsureAITaskLinked(ctx context.Context, taskID, productID uuid.UUID) error {
	if s == nil || s.Products == nil || s.Products.AITasks == nil {
		return nil
	}
	tk, err := s.Products.AITasks.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if tk.ProductID == nil || *tk.ProductID != productID {
		return fmt.Errorf("task does not belong to product")
	}
	if !strings.EqualFold(tk.Status, aitask.StatusSuccess) {
		return fmt.Errorf("AI result is not ready")
	}
	return nil
}
