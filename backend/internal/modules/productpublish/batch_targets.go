package productpublish

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/opslabels"
)

// BatchTargetsCheckRequest POST batch-targets/check body.
type BatchTargetsCheckRequest struct {
	ProductIDs   []string               `json:"productIds"`
	Targets      []PublishTargetRef     `json:"targets"`
	CommonConfig map[string]any         `json:"commonConfig,omitempty"`
	Overrides    PublishConfigOverrides `json:"overrides,omitempty"`
}

// BatchTargetsCheckSummary aggregates product × target matrix check.
type BatchTargetsCheckSummary struct {
	ProductCount        int `json:"productCount"`
	TargetCount         int `json:"targetCount"`
	TaskCount           int `json:"taskCount"`
	ReadyCount          int `json:"readyCount"`
	WarningCount        int `json:"warningCount"`
	BlockedCount        int `json:"blockedCount"`
	LocalDraftOnlyCount int `json:"localDraftOnlyCount"`
}

// BatchTargetCheckItem is one product × target check cell.
type BatchTargetCheckItem struct {
	ProductID     string               `json:"productId"`
	ProductTitle  string               `json:"productTitle"`
	TargetKey     string               `json:"targetKey"`
	Platform      string               `json:"platform"`
	PlatformLabel string               `json:"platformLabel"`
	ShopID        string               `json:"shopId,omitempty"`
	ShopName      string               `json:"shopName,omitempty"`
	Capability    string               `json:"capability"`
	Status        string               `json:"status"`
	StatusLabel   string               `json:"statusLabel"`
	CanCreate     bool                 `json:"canCreateDraft"`
	Issues        []PublishTargetIssue `json:"issues"`
}

// BatchTargetsCheckResponse POST batch-targets/check response.
type BatchTargetsCheckResponse struct {
	Summary BatchTargetsCheckSummary `json:"summary"`
	Items   []BatchTargetCheckItem   `json:"items"`
}

// BatchTargetsCreateDraftsRequest POST batch-targets/create-drafts body.
type BatchTargetsCreateDraftsRequest struct {
	ProductIDs      []string               `json:"productIds"`
	Targets         []PublishTargetRef     `json:"targets"`
	CommonConfig    map[string]any         `json:"commonConfig,omitempty"`
	Overrides       PublishConfigOverrides `json:"overrides,omitempty"`
	OnlyReady       bool                   `json:"onlyReady,omitempty"`
	IncludeWarnings bool                   `json:"includeWarnings,omitempty"`
	Force           bool                   `json:"force,omitempty"`
	Name            string                 `json:"name,omitempty"`
}

// BatchTargetsCreateDraftsResponse POST batch-targets/create-drafts response.
type BatchTargetsCreateDraftsResponse struct {
	BatchID      string                  `json:"batchId"`
	Status       string                  `json:"status"`
	StatusLabel  string                  `json:"statusLabel"`
	ProductCount int                     `json:"productCount"`
	TargetCount  int                     `json:"targetCount"`
	TaskCount    int                     `json:"taskCount"`
	SuccessCount int                     `json:"successCount"`
	FailedCount  int                     `json:"failedCount"`
	SkippedCount int                     `json:"skippedCount"`
	Items        []BatchTargetTaskResult `json:"items"`
}

// BatchTargetTaskResult is one sub-task outcome in a multi-product batch.
type BatchTargetTaskResult struct {
	ProductID         string `json:"productId"`
	ProductTitle      string `json:"productTitle"`
	TargetKey         string `json:"targetKey"`
	Platform          string `json:"platform"`
	PlatformLabel     string `json:"platformLabel"`
	ShopID            string `json:"shopId,omitempty"`
	ShopName          string `json:"shopName,omitempty"`
	TaskID            string `json:"taskId,omitempty"`
	PublicationID     string `json:"publicationId,omitempty"`
	Status            string `json:"status"`
	StatusLabel       string `json:"statusLabel"`
	Capability        string `json:"capability"`
	LocalDraftOnly    bool   `json:"localDraftOnly"`
	ErrorCode         string `json:"errorCode,omitempty"`
	ErrorMessage      string `json:"errorMessage,omitempty"`
	PlatformProductID string `json:"platformProductId,omitempty"`
}

// PublishBatchListItem is one row in batch list.
type PublishBatchListItem struct {
	ID           string     `json:"id"`
	BatchType    string     `json:"batchType"`
	Name         string     `json:"name,omitempty"`
	Status       string     `json:"status"`
	StatusLabel  string     `json:"statusLabel"`
	ProductCount int        `json:"productCount"`
	TargetCount  int        `json:"targetCount"`
	TaskCount    int        `json:"taskCount"`
	SuccessCount int        `json:"successCount"`
	FailedCount  int        `json:"failedCount"`
	SkippedCount int        `json:"skippedCount"`
	CreatedBy    string     `json:"createdBy,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	FinishedAt   *time.Time `json:"finishedAt,omitempty"`
}

// PublishBatchDetailDTO is GET batches/:id response.
type PublishBatchDetailDTO struct {
	PublishBatchListItem
	Items []BatchTargetTaskResult `json:"items"`
	Input map[string]any          `json:"input,omitempty"`
}

func (s *Service) parseBatchProductIDs(raw []string) ([]uuid.UUID, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("productIds required")
	}
	if len(raw) > s.batchMaxProducts() {
		return nil, batchLimitExceeded()
	}
	ids := make([]uuid.UUID, 0, len(raw))
	seen := map[uuid.UUID]struct{}{}
	for _, item := range raw {
		u, err := uuid.Parse(strings.TrimSpace(item))
		if err != nil || u == uuid.Nil {
			return nil, fmt.Errorf("invalid product id in list")
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		ids = append(ids, u)
	}
	return ids, nil
}

func (s *Service) validateBatchTargets(targets []PublishTargetRef) error {
	if len(targets) == 0 {
		return fmt.Errorf("targets required")
	}
	if len(targets) > s.batchMaxTargets() {
		return batchLimitExceeded()
	}
	return nil
}

func (s *Service) loadProductsForBatch(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*product.Product, error) {
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
		if strings.TrimSpace(strings.ToLower(p.Status)) == product.StatusArchived {
			continue
		}
		m[p.ID] = p
	}
	for _, id := range ids {
		if _, ok := m[id]; !ok {
			return nil, fmt.Errorf("product %s not found or not eligible", id.String())
		}
	}
	return m, nil
}

// CheckBatchTargets runs independent checks for each product × target (no side effects).
func (s *Service) CheckBatchTargets(ctx context.Context, req BatchTargetsCheckRequest) (*BatchTargetsCheckResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	productIDs, err := s.parseBatchProductIDs(req.ProductIDs)
	if err != nil {
		return nil, err
	}
	if err := s.validateBatchTargets(req.Targets); err != nil {
		return nil, err
	}
	if err := s.validateBatchTaskCount(len(productIDs), len(req.Targets)); err != nil {
		return nil, err
	}
	products, err := s.loadProductsForBatch(ctx, productIDs)
	if err != nil {
		return nil, err
	}

	items := make([]BatchTargetCheckItem, 0, len(productIDs)*len(req.Targets))
	var readyN, warnN, blockedN, localN int
	for _, pid := range productIDs {
		prod := products[pid]
		for _, t := range req.Targets {
			chk := s.checkOnePublishTarget(ctx, pid, t)
			item := BatchTargetCheckItem{
				ProductID:     pid.String(),
				ProductTitle:  strings.TrimSpace(prod.Title),
				TargetKey:     chk.TargetKey,
				Platform:      chk.Platform,
				PlatformLabel: chk.PlatformLabel,
				ShopID:        chk.ShopID,
				ShopName:      chk.ShopName,
				Capability:    chk.Capability,
				Status:        chk.Status,
				StatusLabel:   chk.StatusLabel,
				CanCreate:     chk.CanCreate,
				Issues:        chk.Issues,
			}
			items = append(items, item)
			switch chk.Status {
			case statusReady:
				readyN++
			case statusWarning:
				warnN++
			default:
				blockedN++
			}
			if chk.Capability == CapLocalDraftOnly {
				localN++
			}
		}
	}
	return &BatchTargetsCheckResponse{
		Summary: BatchTargetsCheckSummary{
			ProductCount:        len(productIDs),
			TargetCount:         len(req.Targets),
			TaskCount:           len(items),
			ReadyCount:          readyN,
			WarningCount:        warnN,
			BlockedCount:        blockedN,
			LocalDraftOnlyCount: localN,
		},
		Items: items,
	}, nil
}

func shouldCreateForCheck(chk PublishTargetCheckResult, onlyReady, includeWarnings bool) bool {
	if chk.Status == statusBlocked || !chk.CanCreate {
		return false
	}
	if onlyReady {
		return chk.Status == statusReady
	}
	if !includeWarnings && chk.Status == statusWarning {
		return false
	}
	return true
}

// CreateBatchTargetDrafts creates a multi-product batch with one sub-task per product × target.
func (s *Service) CreateBatchTargetDrafts(c *gin.Context, req BatchTargetsCreateDraftsRequest, adminID *uuid.UUID) (*BatchTargetsCreateDraftsResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	productIDs, err := s.parseBatchProductIDs(req.ProductIDs)
	if err != nil {
		return nil, err
	}
	if err := s.validateBatchTargets(req.Targets); err != nil {
		return nil, err
	}
	if err := s.validateBatchTaskCount(len(productIDs), len(req.Targets)); err != nil {
		return nil, err
	}
	if !req.IncludeWarnings && !req.OnlyReady {
		req.IncludeWarnings = true
	}
	products, err := s.loadProductsForBatch(ctx, productIDs)
	if err != nil {
		return nil, err
	}

	adminKey := ""
	if adminID != nil {
		adminKey = adminID.String()
	}
	idemKey := batchIdempotencyKey(adminKey, req.ProductIDs, req.Targets, req.CommonConfig, req.Overrides)
	var existing ProductPublishBatch
	if idemKey != "" {
		if err := s.DB.WithContext(ctx).Where("idempotency_key = ? AND status NOT IN ?", idemKey, []string{BatchFailed, BatchCancelled}).
			Order("created_at DESC").First(&existing).Error; err == nil {
			return s.batchCreateResponseFromExisting(ctx, &existing)
		}
	}

	checkRes, err := s.CheckBatchTargets(ctx, BatchTargetsCheckRequest{
		ProductIDs:   req.ProductIDs,
		Targets:      req.Targets,
		CommonConfig: req.CommonConfig,
		Overrides:    req.Overrides,
	})
	if err != nil {
		return nil, err
	}
	checkByKey := map[string]BatchTargetCheckItem{}
	for _, it := range checkRes.Items {
		checkByKey[it.ProductID+":"+it.TargetKey] = it
	}

	inRaw, _ := json.Marshal(req)
	batch := ProductPublishBatch{
		BatchType:      BatchTypeMultiProduct,
		Name:           strings.TrimSpace(req.Name),
		Status:         BatchRunning,
		ProductCount:   len(productIDs),
		TargetCount:    len(req.Targets),
		TaskCount:      len(productIDs) * len(req.Targets),
		IdempotencyKey: idemKey,
		Input:          datatypes.JSON(inRaw),
		CreatedBy:      adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&batch).Error; err != nil {
		if isDuplicateKeyError(err) && idemKey != "" {
			var dupBatch ProductPublishBatch
			if err2 := s.DB.WithContext(ctx).Where("idempotency_key = ? AND status NOT IN ?", idemKey, []string{BatchFailed, BatchCancelled}).
				Order("created_at DESC").First(&dupBatch).Error; err2 == nil {
				return s.batchCreateResponseFromExisting(ctx, &dupBatch)
			}
		}
		return nil, err
	}

	results := make([]BatchTargetTaskResult, 0, batch.TaskCount)
	var successN, failedN, skippedN int
	for _, pid := range productIDs {
		prod := products[pid]
		for _, t := range req.Targets {
			plat := strings.TrimSpace(strings.ToLower(t.Platform))
			var sid *uuid.UUID
			if t.ShopID != nil && strings.TrimSpace(*t.ShopID) != "" {
				if u, err := uuid.Parse(strings.TrimSpace(*t.ShopID)); err == nil {
					sid = &u
				}
			}
			key := publishTargetKey(plat, sid)
			cellKey := pid.String() + ":" + key
			chkItem := checkByKey[cellKey]
			chk := PublishTargetCheckResult{
				TargetKey:     chkItem.TargetKey,
				Platform:      chkItem.Platform,
				PlatformLabel: chkItem.PlatformLabel,
				ShopID:        chkItem.ShopID,
				ShopName:      chkItem.ShopName,
				Capability:    chkItem.Capability,
				Status:        chkItem.Status,
				StatusLabel:   chkItem.StatusLabel,
				CanCreate:     chkItem.CanCreate,
				Issues:        chkItem.Issues,
			}

			base := BatchTargetTaskResult{
				ProductID:     pid.String(),
				ProductTitle:  strings.TrimSpace(prod.Title),
				TargetKey:     key,
				Platform:      plat,
				PlatformLabel: opslabels.PlatformLabel(plat),
				ShopID:        shopIDString(sid),
				ShopName:      chk.ShopName,
				Capability:    chk.Capability,
			}

			if !shouldCreateForCheck(chk, req.OnlyReady, req.IncludeWarnings) {
				skippedN++
				base.Status = "skipped"
				base.StatusLabel = "已跳过"
				results = append(results, base)
				continue
			}

			eff := mergeEffectiveConfig(req.CommonConfig, req.Overrides, pid.String(), plat, shopIDString(sid))
			if dup, ok := s.findExistingSuccessfulTask(ctx, pid, plat, sid, eff); ok && dup != nil {
				successN++
				base.TaskID = dup.ID.String()
				base.Status = dup.Status
				base.StatusLabel = opslabels.StatusLabel(dup.Status)
				base.PlatformProductID = dup.PlatformProductID
				results = append(results, base)
				continue
			}

			var taskRes BatchTargetTaskResult
			switch chk.Capability {
			case CapRealDraftCreate:
				if sid == nil {
					failedN++
					base.Status = TaskFailed
					base.StatusLabel = opslabels.StatusLabel(TaskFailed)
					base.ErrorMessage = "缺少店铺"
					results = append(results, base)
				} else {
					single := s.createDouyinDraftForTarget(c, pid, *sid, batch.ID, adminID, req.Force, chk)
					taskRes = batchResultFromSingle(single, pid.String(), prod.Title)
					if taskRes.Status == TaskSuccess || taskRes.Status == TaskPending || taskRes.Status == TaskRunning {
						successN++
					} else {
						failedN++
					}
					results = append(results, taskRes)
				}
			default:
				taskRes = s.createLocalDraftForBatchTarget(ctx, pid, plat, sid, batch.ID, adminID, chk, eff)
				if taskRes.Status == TaskSuccess {
					successN++
				} else {
					failedN++
				}
				results = append(results, taskRes)
			}
		}
	}

	batchStatus, fin := finalizeBatchStatus(successN, failedN, skippedN)
	sumRaw, _ := json.Marshal(map[string]any{
		"productCount": len(productIDs), "targetCount": len(req.Targets), "taskCount": batch.TaskCount,
		"successCount": successN, "failedCount": failedN, "skippedCount": skippedN,
		"readyCount": checkRes.Summary.ReadyCount, "warningCount": checkRes.Summary.WarningCount, "blockedCount": checkRes.Summary.BlockedCount,
	})
	_ = s.DB.WithContext(ctx).Model(&ProductPublishBatch{}).Where("id = ?", batch.ID).Updates(map[string]any{
		"status": batchStatus, "success_count": successN, "failed_count": failedN, "skipped_count": skippedN,
		"ready_count": checkRes.Summary.ReadyCount, "warning_count": checkRes.Summary.WarningCount, "blocked_count": checkRes.Summary.BlockedCount,
		"summary": datatypes.JSON(sumRaw), "finished_at": &fin,
	}).Error

	s.writeBatchOpLog(c, batch.ID, batchStatus, successN, failedN, skippedN, "product.publish.batch.create")

	return &BatchTargetsCreateDraftsResponse{
		BatchID:      batch.ID.String(),
		Status:       batchStatus,
		StatusLabel:  opslabels.StatusLabel(batchStatus),
		ProductCount: len(productIDs),
		TargetCount:  len(req.Targets),
		TaskCount:    batch.TaskCount,
		SuccessCount: successN,
		FailedCount:  failedN,
		SkippedCount: skippedN,
		Items:        results,
	}, nil
}

func batchResultFromSingle(s PublishTargetTaskResult, productID, productTitle string) BatchTargetTaskResult {
	return BatchTargetTaskResult{
		ProductID:         productID,
		ProductTitle:      productTitle,
		TargetKey:         s.TargetKey,
		Platform:          s.Platform,
		PlatformLabel:     s.PlatformLabel,
		ShopID:            s.ShopID,
		ShopName:          s.ShopName,
		TaskID:            s.TaskID,
		PublicationID:     s.PublicationID,
		Status:            s.Status,
		StatusLabel:       s.StatusLabel,
		Capability:        s.Capability,
		LocalDraftOnly:    s.LocalDraftOnly,
		ErrorCode:         s.ErrorCode,
		ErrorMessage:      s.ErrorMessage,
		PlatformProductID: s.PlatformProductID,
	}
}

func (s *Service) createLocalDraftForBatchTarget(ctx context.Context, productID uuid.UUID, plat string, sid *uuid.UUID, batchID uuid.UUID, adminID *uuid.UUID, chk PublishTargetCheckResult, eff EffectivePublishConfig) BatchTargetTaskResult {
	single := s.createLocalDraftForTarget(ctx, productID, plat, sid, batchID, adminID, chk)
	out := batchResultFromSingle(single, productID.String(), "")
	if single.TaskID != "" {
		inSnap, _ := json.Marshal(map[string]any{
			"effectiveConfig": eff.Config,
			"configSources":   eff.ConfigSources,
			"idempotencyKey":  taskIdempotencyKey(productID.String(), plat, shopIDString(sid), eff),
		})
		_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", single.TaskID).
			Update("input", datatypes.JSON(inSnap)).Error
	}
	return out
}

func extractTaskIdempotencyKey(raw datatypes.JSON) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	if v, ok := m["idempotencyKey"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func (s *Service) findExistingSuccessfulTask(ctx context.Context, productID uuid.UUID, plat string, sid *uuid.UUID, eff EffectivePublishConfig) (*ProductPublishTask, bool) {
	if sid == nil {
		return nil, false
	}
	wantKey := taskIdempotencyKey(productID.String(), plat, shopIDString(sid), eff)
	var rows []ProductPublishTask
	q := s.DB.WithContext(ctx).Where("product_id = ? AND platform = ? AND shop_id = ? AND status = ?",
		productID, plat, *sid, TaskSuccess)
	if err := q.Order("created_at DESC").Find(&rows).Error; err != nil || len(rows) == 0 {
		return nil, false
	}
	for i := range rows {
		row := &rows[i]
		if row.TaskType != TaskTypeLocalDraftCreate && row.TaskType != TaskTypeDouyinDraftCreate {
			continue
		}
		storedKey := extractTaskIdempotencyKey(row.Input)
		if storedKey == "" {
			if len(eff.Config) == 0 {
				return row, true
			}
			continue
		}
		if storedKey == wantKey {
			return row, true
		}
	}
	return nil, false
}

func finalizeBatchStatus(successN, failedN, skippedN int) (string, time.Time) {
	fin := time.Now().UTC()
	if failedN > 0 && successN > 0 {
		return BatchPartialSuccess, fin
	}
	if failedN > 0 && successN == 0 {
		return BatchFailed, fin
	}
	return BatchSuccess, fin
}

func (s *Service) batchCreateResponseFromExisting(ctx context.Context, batch *ProductPublishBatch) (*BatchTargetsCreateDraftsResponse, error) {
	_, tasks, err := s.GetPublishBatch(ctx, batch.ID)
	if err != nil {
		return nil, err
	}
	items := s.tasksToBatchResults(ctx, tasks)
	return &BatchTargetsCreateDraftsResponse{
		BatchID:      batch.ID.String(),
		Status:       batch.Status,
		StatusLabel:  opslabels.StatusLabel(batch.Status),
		ProductCount: batch.ProductCount,
		TargetCount:  batch.TargetCount,
		TaskCount:    batch.TaskCount,
		SuccessCount: batch.SuccessCount,
		FailedCount:  batch.FailedCount,
		SkippedCount: batch.SkippedCount,
		Items:        items,
	}, nil
}

// ListPublishBatches returns paginated multi-product batches.
func (s *Service) ListPublishBatches(ctx context.Context, page, pageSize int) ([]PublishBatchListItem, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	q := s.DB.WithContext(ctx).Model(&ProductPublishBatch{}).Where("batch_type = ?", BatchTypeMultiProduct)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []ProductPublishBatch
	offset := (page - 1) * pageSize
	if err := q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]PublishBatchListItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, batchToListItem(r))
	}
	return out, total, nil
}

func batchToListItem(r ProductPublishBatch) PublishBatchListItem {
	createdBy := ""
	if r.CreatedBy != nil {
		createdBy = r.CreatedBy.String()
	}
	taskCount := r.TaskCount
	if taskCount == 0 {
		taskCount = r.ProductCount * r.TargetCount
	}
	return PublishBatchListItem{
		ID:           r.ID.String(),
		BatchType:    r.BatchType,
		Name:         r.Name,
		Status:       r.Status,
		StatusLabel:  opslabels.StatusLabel(r.Status),
		ProductCount: r.ProductCount,
		TargetCount:  r.TargetCount,
		TaskCount:    taskCount,
		SuccessCount: r.SuccessCount,
		FailedCount:  r.FailedCount,
		SkippedCount: r.SkippedCount,
		CreatedBy:    createdBy,
		CreatedAt:    r.CreatedAt,
		FinishedAt:   r.FinishedAt,
	}
}

// GetPublishBatchDetail returns batch summary with child task rows.
func (s *Service) GetPublishBatchDetail(ctx context.Context, batchID uuid.UUID, adminID *uuid.UUID) (*PublishBatchDetailDTO, error) {
	batch, tasks, err := s.GetPublishBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if err := s.assertBatchAccess(batch, adminID); err != nil {
		return nil, err
	}
	item := batchToListItem(*batch)
	var input map[string]any
	_ = json.Unmarshal(batch.Input, &input)
	return &PublishBatchDetailDTO{
		PublishBatchListItem: item,
		Items:                s.tasksToBatchResults(ctx, tasks),
		Input:                input,
	}, nil
}

func (s *Service) tasksToBatchResults(ctx context.Context, tasks []ProductPublishTask) []BatchTargetTaskResult {
	if len(tasks) == 0 {
		return nil
	}
	pids := make([]uuid.UUID, 0, len(tasks))
	for _, t := range tasks {
		pids = append(pids, t.ProductID)
	}
	titles := s.batchProductTitles(ctx, pids)
	out := make([]BatchTargetTaskResult, 0, len(tasks))
	for _, t := range tasks {
		capability := CapLocalDraftOnly
		if t.TaskType == TaskTypeDouyinDraftCreate {
			capability = CapRealDraftCreate
		}
		out = append(out, BatchTargetTaskResult{
			ProductID:         t.ProductID.String(),
			ProductTitle:      titles[t.ProductID],
			TargetKey:         t.TargetKey,
			Platform:          t.Platform,
			PlatformLabel:     opslabels.PlatformLabel(t.Platform),
			ShopID:            t.ShopID.String(),
			TaskID:            t.ID.String(),
			Status:            t.Status,
			StatusLabel:       opslabels.StatusLabel(t.Status),
			Capability:        capability,
			LocalDraftOnly:    t.TaskType == TaskTypeLocalDraftCreate,
			ErrorCode:         t.ErrorCode,
			ErrorMessage:      t.ErrorMessage,
			PlatformProductID: t.PlatformProductID,
		})
	}
	return out
}

func (s *Service) batchProductTitles(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	if s == nil || s.DB == nil || len(ids) == 0 {
		return out
	}
	uniq := make([]uuid.UUID, 0, len(ids))
	seen := map[uuid.UUID]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	var rows []struct {
		ID    uuid.UUID
		Title string
	}
	_ = s.DB.WithContext(ctx).Model(&product.Product{}).Select("id", "title").Where("id IN ?", uniq).Scan(&rows).Error
	for _, r := range rows {
		out[r.ID] = strings.TrimSpace(r.Title)
	}
	return out
}

// RetryFailedBatchTasks retries only failed sub-tasks in a batch.
func (s *Service) RetryFailedBatchTasks(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (*BatchTargetsCreateDraftsResponse, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	var batch ProductPublishBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return nil, err
	}
	if err := s.assertBatchAccess(&batch, adminID); err != nil {
		return nil, err
	}
	var in BatchTargetsCreateDraftsRequest
	_ = json.Unmarshal(batch.Input, &in)

	var failedTasks []ProductPublishTask
	if err := s.DB.WithContext(ctx).Where("batch_id = ? AND status = ?", batchID, TaskFailed).Find(&failedTasks).Error; err != nil {
		return nil, err
	}
	if len(failedTasks) == 0 {
		return nil, fmt.Errorf("没有可重试的失败任务")
	}

	results := make([]BatchTargetTaskResult, 0, len(failedTasks))
	var successN, failedN int
	for _, ft := range failedTasks {
		claimFin := time.Now().UTC()
		claim := s.DB.WithContext(ctx).Model(&ProductPublishTask{}).
			Where("id = ? AND status = ?", ft.ID, TaskFailed).
			Updates(map[string]any{
				"status":        TaskCancelled,
				"finished_at":   &claimFin,
				"error_message": "已由重试替代",
			})
		if claim.RowsAffected == 0 {
			continue
		}

		sid := ft.ShopID
		t := PublishTargetRef{Platform: ft.Platform, ShopID: ptrString(sid.String())}
		chk := s.checkOnePublishTarget(ctx, ft.ProductID, t)
		base := BatchTargetTaskResult{
			ProductID:     ft.ProductID.String(),
			TargetKey:     ft.TargetKey,
			Platform:      ft.Platform,
			PlatformLabel: opslabels.PlatformLabel(ft.Platform),
			ShopID:        sid.String(),
			ShopName:      chk.ShopName,
			Capability:    chk.Capability,
		}
		titles := s.batchProductTitles(ctx, []uuid.UUID{ft.ProductID})
		base.ProductTitle = titles[ft.ProductID]

		if chk.Status == statusBlocked {
			failedN++
			base.Status = TaskFailed
			base.StatusLabel = opslabels.StatusLabel(TaskFailed)
			if len(chk.Issues) > 0 {
				base.ErrorMessage = chk.Issues[0].Title
			}
			results = append(results, base)
			continue
		}

		eff := mergeEffectiveConfig(in.CommonConfig, in.Overrides, ft.ProductID.String(), ft.Platform, sid.String())
		if dup, ok := s.findExistingSuccessfulTask(ctx, ft.ProductID, ft.Platform, &sid, eff); ok && dup != nil {
			successN++
			base.TaskID = dup.ID.String()
			base.Status = dup.Status
			base.StatusLabel = opslabels.StatusLabel(dup.Status)
			results = append(results, base)
			continue
		}

		var taskRes BatchTargetTaskResult
		switch chk.Capability {
		case CapRealDraftCreate:
			single := s.createDouyinDraftForTarget(c, ft.ProductID, sid, batchID, adminID, in.Force, chk)
			taskRes = batchResultFromSingle(single, ft.ProductID.String(), base.ProductTitle)
		default:
			taskRes = s.createLocalDraftForBatchTarget(ctx, ft.ProductID, ft.Platform, &sid, batchID, adminID, chk, eff)
		}
		if taskRes.Status == TaskSuccess || taskRes.Status == TaskPending || taskRes.Status == TaskRunning {
			successN++
		} else {
			failedN++
		}
		results = append(results, taskRes)
	}

	var allTasks []ProductPublishTask
	_ = s.DB.WithContext(ctx).Where("batch_id = ?", batchID).Find(&allTasks).Error
	var totalSuccess, totalFailed, totalSkipped int
	for _, t := range allTasks {
		switch t.Status {
		case TaskSuccess, TaskPending, TaskRunning:
			totalSuccess++
		case TaskFailed:
			totalFailed++
		case TaskCancelled:
			totalSkipped++
		}
	}
	batchStatus, fin := finalizeBatchStatus(totalSuccess, totalFailed, totalSkipped)
	_ = s.DB.WithContext(ctx).Model(&ProductPublishBatch{}).Where("id = ?", batchID).Updates(map[string]any{
		"status": batchStatus, "success_count": totalSuccess, "failed_count": totalFailed,
		"skipped_count": totalSkipped, "finished_at": &fin,
	}).Error

	s.writeBatchOpLog(c, batchID, batchStatus, successN, failedN, 0, "product.publish.batch.retry_failed")

	return &BatchTargetsCreateDraftsResponse{
		BatchID:      batchID.String(),
		Status:       batchStatus,
		StatusLabel:  opslabels.StatusLabel(batchStatus),
		ProductCount: batch.ProductCount,
		TargetCount:  batch.TargetCount,
		TaskCount:    batch.TaskCount,
		SuccessCount: totalSuccess,
		FailedCount:  totalFailed,
		SkippedCount: totalSkipped,
		Items:        results,
	}, nil
}

// CancelPendingBatchTasks cancels pending/queued sub-tasks only.
func (s *Service) CancelPendingBatchTasks(c *gin.Context, batchID uuid.UUID, adminID *uuid.UUID) (*PublishBatchDetailDTO, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	var batch ProductPublishBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return nil, err
	}
	if err := s.assertBatchAccess(&batch, adminID); err != nil {
		return nil, err
	}
	fin := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&ProductPublishTask{}).
		Where("batch_id = ? AND status IN ?", batchID, []string{TaskPending}).
		Updates(map[string]any{"status": TaskCancelled, "finished_at": &fin, "error_message": "用户取消等待中的任务"})
	cancelled := res.RowsAffected

	var totalSuccess, totalFailed, totalSkipped int
	var tasks []ProductPublishTask
	_ = s.DB.WithContext(ctx).Where("batch_id = ?", batchID).Find(&tasks).Error
	for _, t := range tasks {
		switch t.Status {
		case TaskSuccess:
			totalSuccess++
		case TaskFailed:
			totalFailed++
		case TaskCancelled:
			totalSkipped++
		}
	}
	batchStatus := batch.Status
	if cancelled > 0 {
		batchStatus = BatchPartialSuccess
		if totalSuccess == 0 && totalFailed == 0 {
			batchStatus = BatchCancelled
		}
	}
	_ = s.DB.WithContext(ctx).Model(&ProductPublishBatch{}).Where("id = ?", batchID).Updates(map[string]any{
		"status": batchStatus, "success_count": totalSuccess, "failed_count": totalFailed,
		"skipped_count": totalSkipped, "finished_at": &fin,
	}).Error

	s.writeBatchOpLog(c, batchID, batchStatus, int(cancelled), 0, int(totalSkipped), "product.publish.batch.cancel_pending")

	return s.GetPublishBatchDetail(ctx, batchID, adminID)
}

func (s *Service) writeBatchOpLog(c *gin.Context, batchID uuid.UUID, status string, successN, failedN, skippedN int, action string) {
	if s == nil || s.OpLog == nil || c == nil {
		return
	}
	msg := fmt.Sprintf("batchId=%s status=%s success=%d failed=%d skipped=%d", batchID.String(), status, successN, failedN, skippedN)
	_ = s.OpLog.Write(c, operationlog.WriteOpts{
		Action:     action,
		Resource:   "product_publish_batch",
		ResourceID: batchID.String(),
		Status:     status,
		Message:    msg,
	})
}

func ptrString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := strings.TrimSpace(s)
	return &v
}
