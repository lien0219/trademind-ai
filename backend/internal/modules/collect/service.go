package collect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/trademind-ai/trademind/backend/internal/modules/collectbrowserprofile"
	"github.com/trademind-ai/trademind/backend/internal/modules/collectrule"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/tenantconfig"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasklease"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
)

// Service orchestrates collect tasks and persists results via product drafts.
type Service struct {
	DB                      *gorm.DB
	Products                *product.Service
	Rules                   *collectrule.Service
	Profiles                *collectbrowserprofile.Service
	OpLog                   *operationlog.Service
	Client                  *CollectorClient
	Redis                   *rdb.Client
	QueueName               string
	QueueEnabled            bool
	AllowLegacyTenantZero   bool
	BatchMaxURLs            int
	CollectorTimeoutSeconds int

	AutoRetryEnabled  bool
	MaxAutoRetries    int
	RetryBaseDelaySec int
	RetryMaxDelaySec  int

	// 1688 batch throttling (env defaults; settings group collector overrides at runtime).
	Source1688BatchConcurrency int
	Source1688BatchDelayMinMs  int
	Source1688BatchDelayMaxMs  int
	BatchRetryOnBlocked        bool
	BatchRetryOnTimeout        bool
	Source1688BatchMaxRetries  int

	Settings *settings.Service

	// TaskLeaseTimeoutSeconds is Redis/DB lease for multi-instance workers (from COLLECT_TASK_TIMEOUT_SECONDS).
	TaskLeaseTimeoutSeconds int
}

func (s *Service) acceptsTenantID(tenantID int64) bool {
	return tenantID > 0 || (tenantID == 0 && s != nil && s.AllowLegacyTenantZero)
}

func clampCollectPage(page, ps int) (int, int) {
	if page < 1 {
		page = 1
	}
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	return page, ps
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

type normalizedProduct struct {
	Source            string            `json:"source"`
	SourceURL         string            `json:"sourceUrl"`
	Title             string            `json:"title"`
	Currency          string            `json:"currency"`
	MainDescription   string            `json:"mainDescription"`
	MainImages        []string          `json:"mainImages"`
	DescriptionImages []string          `json:"descriptionImages"`
	Attributes        json.RawMessage   `json:"attributes"`
	SKUs              []json.RawMessage `json:"skus"`
	Raw               json.RawMessage   `json:"raw"`
}

func parseNormalized(b json.RawMessage) (*normalizedProduct, error) {
	var n normalizedProduct
	if err := json.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (n *normalizedProduct) importParams(fullJSON json.RawMessage) product.ImportDraftParams {
	if n == nil {
		return product.ImportDraftParams{FullNormalizedJSON: fullJSON}
	}
	var skus []product.ImportSKUParams
	for _, raw := range n.SKUs {
		line, err := product.BuildImportSKU(raw)
		if err != nil {
			continue
		}
		skus = append(skus, line)
	}
	return product.ImportDraftParams{
		Source:             strings.TrimSpace(n.Source),
		SourceURL:          strings.TrimSpace(n.SourceURL),
		Title:              strings.TrimSpace(n.Title),
		Currency:           strings.TrimSpace(n.Currency),
		Description:        strings.TrimSpace(n.MainDescription),
		MainImages:         n.MainImages,
		DescriptionImages:  n.DescriptionImages,
		SKUs:               skus,
		FullNormalizedJSON: fullJSON,
	}
}

func (s *Service) importDraftAndFinishTask(
	ctx context.Context,
	task *CollectTask,
	workerID string,
	claim *tasklease.ClaimResult,
	params product.ImportDraftParams,
	rawResult json.RawMessage,
) (*product.Product, time.Time, error) {
	if s == nil || s.DB == nil || s.Products == nil || task == nil || claim == nil {
		return nil, time.Time{}, fmt.Errorf("collect import transaction unavailable")
	}
	finishedAt := time.Now().UTC()
	var created *product.Product
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owned CollectTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ? AND status = ? AND locked_by = ? AND execution_id = ? AND lock_version = ? AND locked_until > ?",
				task.ID, task.TenantID, StatusRunning, workerID, claim.ExecutionID.String(), claim.LeaseVersion, finishedAt).
			First(&owned).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tasklease.ErrLeaseLost
			}
			return err
		}
		var err error
		created, err = s.Products.ImportDraftInTransaction(ctx, tx, task.CreatedBy, params)
		if err != nil {
			return err
		}
		res := tx.Model(&CollectTask{}).
			Where("id = ? AND tenant_id = ? AND status = ? AND locked_by = ? AND execution_id = ? AND lock_version = ?",
				task.ID, task.TenantID, StatusRunning, workerID, claim.ExecutionID.String(), claim.LeaseVersion).
			Updates(map[string]interface{}{
				"status":            StatusSuccess,
				"result_product_id": created.ID,
				"raw_result":        datatypes.JSON(rawResult),
				"error_message":     "",
				"finished_at":       &finishedAt,
				"next_retry_at":     nil,
				"retry_enqueued_at": nil,
				"retry_count":       0,
				"locked_by":         nil,
				"locked_until":      nil,
				"updated_at":        finishedAt,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return tasklease.ErrLeaseLost
		}
		return nil
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	return created, finishedAt, nil
}

func (s *Service) failTask(ctx context.Context, task *CollectTask, fromStatus, msg string, payload map[string]any, workerID string, claim *tasklease.ClaimResult) {
	if s == nil || s.DB == nil || task == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	fs := strings.TrimSpace(fromStatus)
	if fs == "" {
		fs = StatusRunning
	}
	msg = truncateRunes(strings.TrimSpace(msg), 8000)
	fin := time.Now().UTC()
	tid := task.ID
	updates := map[string]interface{}{
		"status":            StatusFailed,
		"error_message":     msg,
		"finished_at":       &fin,
		"next_retry_at":     nil,
		"retry_enqueued_at": nil,
		"locked_by":         nil,
		"locked_until":      nil,
		"updated_at":        fin,
	}
	if claim != nil && strings.TrimSpace(workerID) != "" {
		if err := s.finishCollectTask(ctx, tid, workerID, claim, updates); err != nil {
			slog.Warn("collect_fail_lease_lost", "taskId", tid.String(), "error", err.Error())
			return
		}
	} else {
		_ = s.DB.WithContext(ctx).Model(&CollectTask{}).Where("id = ?", tid).Updates(updates).Error
	}

	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:    EventTaskFailed,
		FromStatus:   fs,
		ToStatus:     StatusFailed,
		Message:      "collect job failed",
		ErrorMessage: msg,
		PayloadMap:   payload,
		MaxRetries:   s.effectiveMaxRetries(task),
		RetryCount:   task.RetryCount,
	})

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.failed",
			Resource:    "collect_task",
			ResourceID:  tid.String(),
			Status:      "failed",
			Message:     truncateRunes(msg, 2000),
		})
		if task.BatchID != nil && isTaobaoTmallCollectSource(task.Source) {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: task.CreatedBy,
				Action:      "collect.taobao_tmall.batch.task_failed",
				Resource:    "collect_task",
				ResourceID:  tid.String(),
				Status:      "failed",
				Message:     truncateRunes(fmt.Sprintf("batchId=%s %s", task.BatchID.String(), msg), 2000),
			})
		}
	}
	if task.BatchID != nil {
		s.maybePauseTaobaoTmallBatchOnAuthFailure(ctx, task, payload)
		s.reconcileCollectBatchWithTerminalLog(ctx, task.BatchID)
	}
}

func (s *Service) failTaskRetryExhausted(ctx context.Context, task *CollectTask, msg string, payload map[string]any, workerID string, claim *tasklease.ClaimResult) {
	if s == nil || s.DB == nil || task == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	msg = truncateRunes(strings.TrimSpace(msg), 8000)
	fin := time.Now().UTC()
	tid := task.ID
	updates := map[string]interface{}{
		"status":            StatusFailed,
		"error_message":     msg,
		"finished_at":       &fin,
		"next_retry_at":     nil,
		"retry_enqueued_at": nil,
		"locked_by":         nil,
		"locked_until":      nil,
		"updated_at":        fin,
	}
	if claim != nil && strings.TrimSpace(workerID) != "" {
		if err := s.finishCollectTask(ctx, tid, workerID, claim, updates); err != nil {
			slog.Warn("collect_retry_exhausted_lease_lost", "taskId", tid.String(), "error", err.Error())
			return
		}
	} else {
		_ = s.DB.WithContext(ctx).Model(&CollectTask{}).Where("id = ?", tid).Updates(updates).Error
	}

	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:    EventTaskRetryExhausted,
		FromStatus:   StatusRunning,
		ToStatus:     StatusFailed,
		Message:      "auto-retry exhausted",
		ErrorMessage: msg,
		RetryCount:   task.RetryCount,
		MaxRetries:   s.effectiveMaxRetries(task),
		PayloadMap:   payload,
	})

	if s.OpLog != nil {
		logMsg := fmt.Sprintf("taskId=%s retryCount=%d", tid.String(), task.RetryCount)
		if task.BatchID != nil {
			logMsg += fmt.Sprintf(" batchId=%s", task.BatchID.String())
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: task.CreatedBy,
			Action:      "collect.task.retry_exhausted",
			Resource:    "collect_task",
			ResourceID:  tid.String(),
			Status:      "failed",
			Message:     truncateRunes(logMsg+" "+msg, 2000),
		})
	}
	if task.BatchID != nil {
		s.maybePauseTaobaoTmallBatchOnAuthFailure(ctx, task, payload)
		s.reconcileCollectBatchWithTerminalLog(ctx, task.BatchID)
	}
}

// RunCollectJob executes one task from the queue (Collector → product draft).
func (s *Service) RunCollectJob(parent context.Context, taskID uuid.UUID, workerID string) (claimed bool, err error) {
	if s == nil || s.DB == nil || s.Client == nil || s.Products == nil {
		return false, fmt.Errorf("collect: worker dependencies unavailable")
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}

	defer func() {
		if r := recover(); r != nil {
			s.handleCollectPanic(ctx, taskID, workerID, r)
			claimed = true
			err = nil
		}
	}()

	var peek CollectTask
	if err := s.DB.WithContext(ctx).First(&peek, "id = ?", taskID).Error; err != nil {
		return false, err
	}
	prevStatus := peek.Status

	lease := s.collectLeaseTTL()
	task, claim, ok, err := s.tryClaimCollectTask(ctx, taskID, workerID, lease)
	if err != nil || !ok {
		return false, err
	}
	claimed = true

	stopRenew := s.startCollectLeaseRenewal(ctx, taskID, workerID, claim, lease)
	defer stopRenew()

	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:  EventWorkerLeaseAcquired,
		FromStatus: prevStatus,
		ToStatus:   StatusRunning,
		Message:    "lease acquired",
		PayloadMap: map[string]any{"workerId": workerID},
	})
	s.RecordTaskEvent(ctx, task, TaskEventInput{
		EventType:  EventTaskRunning,
		FromStatus: prevStatus,
		ToStatus:   StatusRunning,
		Message:    "worker claimed task",
	})
	s.reconcileCollectBatch(ctx, task.BatchID)

	releaseGate, preflightErr := s.runBatchCollectPreflight(ctx, task)
	if preflightErr != nil {
		s.handleCollectJobError(ctx, task, preflightErr, workerID, claim)
		return true, nil
	}
	if releaseGate != nil {
		defer releaseGate()
	}

	var collectorOpts map[string]any
	if isPinduoduoCollectSource(task.Source) {
		useProfile := isPifaPinduoduoURL(task.SourceURL)
		if len(task.RequestOptions) > 0 {
			var snap struct {
				ProfileKey        string `json:"profileKey,omitempty"`
				UseBrowserProfile bool   `json:"useBrowserProfile"`
			}
			if err := json.Unmarshal(task.RequestOptions, &snap); err == nil {
				if snap.UseBrowserProfile {
					useProfile = true
				}
			}
		}
		collectorOpts = mergeJSONIntoCollectorOpts(collectorOpts, s.buildPinduoduoRequestOptions(ctx, task.SourceURL, useProfile))
	}
	if isTaobaoTmallCollectSource(task.Source) {
		useProfile := true
		if len(task.RequestOptions) > 0 {
			var snap struct {
				UseBrowserProfile bool `json:"useBrowserProfile"`
			}
			if err := json.Unmarshal(task.RequestOptions, &snap); err == nil && snap.UseBrowserProfile {
				useProfile = true
			}
		}
		collectorOpts = mergeJSONIntoCollectorOpts(collectorOpts, s.buildTaobaoTmallRequestOptions(ctx, task.SourceURL, useProfile))
	}
	if strings.EqualFold(strings.TrimSpace(task.Source), "custom") {
		var snap struct {
			RuleID            string          `json:"ruleId"`
			RuleName          string          `json:"ruleName"`
			Domain            string          `json:"domain"`
			MatchPattern      string          `json:"matchPattern"`
			Rule              json.RawMessage `json:"rule"`
			ProfileID         string          `json:"profileId,omitempty"`
			ProfileKey        string          `json:"profileKey,omitempty"`
			UseBrowserProfile bool            `json:"useBrowserProfile"`
		}
		if len(task.RequestOptions) == 0 {
			s.failTask(ctx, task, prevStatus, "missing custom rule snapshot", nil, workerID, claim)
			return true, nil
		}
		if err := json.Unmarshal(task.RequestOptions, &snap); err != nil || len(snap.Rule) == 0 {
			s.failTask(ctx, task, prevStatus, "invalid custom rule snapshot", nil, workerID, claim)
			return true, nil
		}
		var ruleObj any
		if err := json.Unmarshal(snap.Rule, &ruleObj); err != nil {
			s.failTask(ctx, task, prevStatus, "invalid embedded rule json", nil, workerID, claim)
			return true, nil
		}
		collectorOpts = map[string]any{
			"ruleId":       snap.RuleID,
			"ruleName":     snap.RuleName,
			"domain":       snap.Domain,
			"matchPattern": snap.MatchPattern,
			"rule":         ruleObj,
		}
		if snap.UseBrowserProfile && strings.TrimSpace(snap.ProfileKey) != "" {
			if s.Profiles == nil || strings.TrimSpace(snap.ProfileID) == "" {
				s.failTask(ctx, task, prevStatus, "browser profile reference unavailable", nil, workerID, claim)
				return true, nil
			}
			profileID, err := uuid.Parse(strings.TrimSpace(snap.ProfileID))
			if err != nil {
				s.failTask(ctx, task, prevStatus, "invalid browser profile reference", nil, workerID, claim)
				return true, nil
			}
			if err := s.Profiles.EnrichCollectorOptions(ctx, task.TenantID, collectorOpts, &profileID, true, task.SourceURL); err != nil {
				s.failTask(ctx, task, prevStatus, "browser profile unavailable", nil, workerID, claim)
				return true, nil
			}
		}
	}
	if task.BatchID != nil {
		if collectorOpts == nil {
			collectorOpts = map[string]any{}
		}
		collectorOpts["batchMode"] = true
	}

	collectTimeout := s.collectorHTTPTimeoutForTask(ctx, task, collectorOpts)
	outcome, err := s.Client.CollectWithTimeout(ctx, task.Source, task.SourceURL, collectorOpts, collectTimeout)
	if err != nil {
		s.handleCollectJobError(ctx, task, err, workerID, claim)
		return true, nil
	}

	norm, err := parseNormalized(outcome.ProductJSON)
	if err != nil {
		s.handleCollectJobError(ctx, task, fmt.Errorf("parse normalized product: %w", err), workerID, claim)
		return true, nil
	}
	if strings.EqualFold(strings.TrimSpace(task.Source), "1688") && len(norm.MainImages) == 0 {
		s.handleCollectJobError(ctx, task, &CollectorRejectedError{
			Code:    "PARSE_FAILED",
			Message: "missing main images after collect",
		}, workerID, claim)
		return true, nil
	}

	params := norm.importParams(outcome.ProductJSON)
	if strings.EqualFold(strings.TrimSpace(task.Source), "custom") {
		params, outcome.ProductJSON = normalizeCustomImport(task.Source, norm, outcome.ProductJSON)
	}
	if strings.EqualFold(strings.TrimSpace(task.Source), "pinduoduo") || strings.EqualFold(strings.TrimSpace(task.Source), "pdd") {
		params, outcome.ProductJSON = normalizePinduoduoImport(task.Source, norm, outcome.ProductJSON)
	}
	if isTaobaoTmallCollectSource(task.Source) {
		params, outcome.ProductJSON = normalizeTaobaoTmallImport(task.Source, norm, outcome.ProductJSON)
	}
	created, _, err := s.importDraftAndFinishTask(ctx, task, workerID, claim, params, outcome.ProductJSON)
	if err != nil {
		if errors.Is(err, tasklease.ErrLeaseLost) {
			slog.Warn("collect_import_lease_lost", "taskId", taskID.String(), "error", err.Error())
			return true, nil
		}
		s.handleCollectJobError(ctx, task, err, workerID, claim)
		return true, nil
	}

	pid := created.ID
	var refreshed CollectTask
	if err := s.DB.WithContext(ctx).First(&refreshed, "id = ?", taskID).Error; err != nil {
		return true, nil
	}
	s.reconcileCollectBatchWithTerminalLog(ctx, refreshed.BatchID)

	s.RecordTaskEvent(ctx, &refreshed, TaskEventInput{
		EventType:  EventTaskSuccess,
		FromStatus: StatusRunning,
		ToStatus:   StatusSuccess,
		Message:    "draft imported from collector response",
		RetryCount: refreshed.RetryCount,
		MaxRetries: refreshed.MaxRetries,
		PayloadMap: map[string]any{"productId": pid.String()},
	})

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: refreshed.CreatedBy,
			Action:      "product.create",
			Resource:    "product",
			ResourceID:  pid.String(),
			Status:      "success",
			Message:     "draft imported from collect",
		})
		action := "collect.task.success"
		msg := fmt.Sprintf("product_id=%s", pid.String())
		if isPinduoduoCollectSource(refreshed.Source) {
			action = "collect.pinduoduo.single"
			warnN := 0
			var wrap struct {
				Raw struct {
					Warnings []string `json:"warnings"`
				} `json:"raw"`
			}
			_ = json.Unmarshal(outcome.ProductJSON, &wrap)
			warnN = len(wrap.Raw.Warnings)
			if warnN > 0 {
				_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
					AdminUserID: refreshed.CreatedBy,
					Action:      "collect.pinduoduo.parse_warning",
					Resource:    "collect_task",
					ResourceID:  taskID.String(),
					Status:      "success",
					Message:     fmt.Sprintf("source=pinduoduo warningCount=%d productId=%s", warnN, pid.String()),
				})
			}
			msg = fmt.Sprintf("source=pinduoduo success productId=%s warningCount=%d", pid.String(), warnN)
		} else if isTaobaoTmallCollectSource(refreshed.Source) {
			action = "collect.taobao_tmall.single"
			warnN := 0
			var wrap struct {
				Raw struct {
					QualityWarnings []string `json:"qualityWarnings"`
					Warnings        []string `json:"warnings"`
				} `json:"raw"`
			}
			_ = json.Unmarshal(outcome.ProductJSON, &wrap)
			warnN = len(wrap.Raw.QualityWarnings) + len(wrap.Raw.Warnings)
			if warnN > 0 {
				_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
					AdminUserID: refreshed.CreatedBy,
					Action:      "collect.taobao_tmall.parse_warning",
					Resource:    "collect_task",
					ResourceID:  taskID.String(),
					Status:      "success",
					Message:     fmt.Sprintf("source=taobao_tmall warningCount=%d productId=%s", warnN, pid.String()),
				})
			}
			msg = fmt.Sprintf("source=taobao_tmall success productId=%s warningCount=%d", pid.String(), warnN)
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: refreshed.CreatedBy,
			Action:      action,
			Resource:    "collect_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     msg,
		})
		if refreshed.BatchID != nil && isTaobaoTmallCollectSource(refreshed.Source) {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: refreshed.CreatedBy,
				Action:      "collect.taobao_tmall.batch.task_success",
				Resource:    "collect_task",
				ResourceID:  taskID.String(),
				Status:      "success",
				Message:     fmt.Sprintf("batchId=%s productId=%s", refreshed.BatchID.String(), pid.String()),
			})
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: refreshed.CreatedBy,
				Action:      "collect.taobao_tmall.batch.draft_created",
				Resource:    "product",
				ResourceID:  pid.String(),
				Status:      "success",
				Message:     fmt.Sprintf("batchId=%s taskId=%s", refreshed.BatchID.String(), taskID.String()),
			})
		}
	}
	return true, nil
}

func parseOptionalRuleID(p *string) (*uuid.UUID, error) {
	if p == nil {
		return nil, nil
	}
	t := strings.TrimSpace(*p)
	if t == "" {
		return nil, nil
	}
	u, err := uuid.Parse(t)
	if err != nil || u == uuid.Nil {
		return nil, fmt.Errorf("invalid ruleId")
	}
	return &u, nil
}

func requestIDFromGin(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if v, ok := c.Get(ctxkey.TraceID); ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// CreateTaskAsync validates input, persists a pending task, enqueues, returns immediately.
func (s *Service) CreateTaskAsync(c *gin.Context, body CreateTaskBody, adminID *uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if !s.QueueEnabled {
		return zero, ErrCollectQueueDisabled
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil || !s.acceptsTenantID(tenantID) {
		return zero, security.ErrTenantContextMissing
	}
	source := strings.TrimSpace(body.Source)
	if tenantID > 0 {
		policy, _, err := tenantconfig.ReadCollect(c.Request.Context(), s.DB, tenantID)
		if err != nil { return zero, fmt.Errorf("tenant collect policy unavailable: %w", err) }
		if err := tenantconfig.CheckCollectSource(policy, source); err != nil { return zero, err }
	}
	url := strings.TrimSpace(body.URL)
	if source == "" || url == "" {
		return zero, fmt.Errorf("source and url are required")
	}
	if !looksLikeCollectURL(url) {
		return zero, ErrCollectURLNeedsHTTPScheme
	}
	if err := s.ValidateSourceForCollect(c.Request.Context(), source, false); err != nil {
		return zero, err
	}
	if isTaobaoTmallCollectSource(source) {
		if err := validateTaobaoTmallCollectURL(url); err != nil {
			return zero, err
		}
	}
	if strings.EqualFold(strings.TrimSpace(source), "custom") {
		if err := s.checkCustomCollectURLConflict(c.Request.Context(), url); err != nil {
			return zero, err
		}
	}

	var reqOpts datatypes.JSON
	if strings.EqualFold(strings.TrimSpace(source), "custom") {
		if s.Rules == nil {
			return zero, fmt.Errorf("custom collect rules unavailable")
		}
		explicitID, err := parseOptionalRuleID(body.RuleID)
		if err != nil {
			return zero, err
		}
		rule, err := s.Rules.ResolveEnabledRuleForCustom(c.Request.Context(), url, explicitID)
		if err != nil {
			return zero, err
		}
		blob, err := s.Rules.BuildTaskPayload(rule)
		if err != nil {
			return zero, err
		}
		if s.Profiles != nil && body.UseBrowserProfile && body.ProfileID != nil {
			pid, err := uuid.Parse(strings.TrimSpace(*body.ProfileID))
			if err != nil {
				return zero, fmt.Errorf("invalid profileId")
			}
			blob, err = s.Profiles.MergeIntoRequestOptions(c.Request.Context(), tenantID, blob, &pid, true, url)
			if err != nil {
				return zero, err
			}
		}
		reqOpts = blob
	} else if isPinduoduoCollectSource(source) {
		reqOpts = s.buildPinduoduoRequestOptions(c.Request.Context(), url, body.UseBrowserProfile)
	} else if isTaobaoTmallCollectSource(source) {
		reqOpts = s.buildTaobaoTmallRequestOptions(c.Request.Context(), url, true)
	}

	maxRetries := s.defaultMaxRetriesForNewTask()
	if isTaobaoTmallCollectSource(source) {
		maxRetries = s.taobaoTmallMaxRetries(c.Request.Context())
	}

	task := &CollectTask{
		TenantID:       tenantID,
		Source:         source,
		SourceURL:      url,
		Status:         StatusPending,
		MaxRetries:     maxRetries,
		CreatedBy:      adminID,
		RequestOptions: reqOpts,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(task).Error; err != nil {
		return zero, err
	}

	s.RecordTaskEvent(c.Request.Context(), task, TaskEventInput{
		EventType:  EventTaskCreated,
		ToStatus:   StatusPending,
		Message:    "collect task persisted",
		MaxRetries: task.MaxRetries,
	})

	reqID := requestIDFromGin(c)
	if err := s.enqueueTask(c.Request.Context(), task.ID, task.Source, task.SourceURL, adminID, reqID); err != nil {
		_ = s.DB.WithContext(c.Request.Context()).Where("task_id = ?", task.ID).Delete(&CollectTaskEvent{}).Error
		_ = s.DB.WithContext(c.Request.Context()).Unscoped().Where("id = ?", task.ID).Delete(&CollectTask{}).Error
		return zero, err
	}

	s.RecordTaskEvent(c.Request.Context(), task, TaskEventInput{
		EventType:  EventTaskEnqueued,
		ToStatus:   StatusPending,
		Message:    "queued to Redis",
		MaxRetries: task.MaxRetries,
		PayloadMap: func() map[string]any {
			if strings.TrimSpace(reqID) != "" {
				return map[string]any{"requestId": reqID}
			}
			return nil
		}(),
	})

	if s.OpLog != nil {
		action := "collect.task.create"
		msg := "task submitted to queue"
		if isPinduoduoCollectSource(source) {
			action = "collect.pinduoduo.single"
			msg = "source=pinduoduo taskId=" + task.ID.String()
		} else if isTaobaoTmallCollectSource(source) {
			action = "collect.taobao_tmall.single"
			msg = "source=taobao_tmall taskId=" + task.ID.String()
		}
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     action,
			Resource:   "collect_task",
			ResourceID: task.ID.String(),
			Status:     "success",
			Message:    msg,
		})
	}
	return s.GetDTO(c, task.ID)
}

// RetryAsync re-queues a failed task.
func (s *Service) RetryAsync(c *gin.Context, id uuid.UUID, adminID *uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	if !s.QueueEnabled {
		return zero, ErrCollectQueueDisabled
	}

	var task CollectTask
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return zero, err
	}
	if err := repository.FindByID(c.Request.Context(), s.DB, &task, tenantID, id); err != nil {
		return zero, err
	}
	if task.Status != StatusFailed {
		return zero, fmt.Errorf("only failed tasks can be retried")
	}

	retryAt := time.Now().UTC()
	update := s.DB.WithContext(c.Request.Context()).Model(&CollectTask{}).
		Where("id = ? AND tenant_id = ? AND status = ?", id, tenantID, StatusFailed).
		Updates(map[string]interface{}{
			"status":            StatusRetrying,
			"error_message":     "",
			"finished_at":       nil,
			"result_product_id": nil,
			"raw_result":        datatypes.JSON(nil),
			"retry_count":       0,
			"next_retry_at":     nil,
			"retry_enqueued_at": nil,
			"locked_by":         nil,
			"locked_until":      nil,
			"updated_at":        retryAt,
		})
	if update.Error != nil {
		return zero, update.Error
	}
	if update.RowsAffected != 1 {
		return zero, fmt.Errorf("only failed tasks can be retried")
	}

	if err := s.DB.WithContext(c.Request.Context()).First(&task, "id = ? AND tenant_id = ?", id, tenantID).Error; err != nil {
		return zero, err
	}

	reqID := requestIDFromGin(c)
	if err := s.enqueueTask(c.Request.Context(), task.ID, task.Source, task.SourceURL, task.CreatedBy, reqID); err != nil {
		fin := time.Now().UTC()
		_ = s.DB.WithContext(c.Request.Context()).Model(&CollectTask{}).
			Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(map[string]interface{}{
				"status":        StatusFailed,
				"error_message": ErrRedisQueueUnavailable.Error(),
				"finished_at":   &fin,
				"updated_at":    fin,
			}).Error
		var bumped CollectTask
		if er := s.DB.WithContext(c.Request.Context()).First(&bumped, "id = ? AND tenant_id = ?", id, tenantID).Error; er == nil {
			s.RecordTaskEvent(c.Request.Context(), &bumped, TaskEventInput{
				EventType:    EventTaskFailed,
				FromStatus:   StatusRetrying,
				ToStatus:     StatusFailed,
				Message:      "enqueue after manual retry failed",
				ErrorMessage: ErrRedisQueueUnavailable.Error(),
			})
		}
		s.reconcileCollectBatch(c.Request.Context(), task.BatchID)
		return zero, err
	}

	s.RecordTaskEvent(c.Request.Context(), &task, TaskEventInput{
		EventType:  EventTaskManualRetry,
		FromStatus: StatusFailed,
		ToStatus:   StatusRetrying,
		Message:    "manual retry re-queued",
		RetryCount: task.RetryCount,
		MaxRetries: task.MaxRetries,
	})

	s.reconcileCollectBatch(c.Request.Context(), task.BatchID)

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "collect.task.retry",
			Resource:    "collect_task",
			ResourceID:  id.String(),
			Status:      "success",
			Message:     "task re-queued",
		})
	}
	return s.GetDTO(c, id)
}

// GetDTO returns one task by id.
func (s *Service) GetDTO(c *gin.Context, id uuid.UUID) (TaskDTO, error) {
	var zero TaskDTO
	if s == nil || s.DB == nil {
		return zero, fmt.Errorf("collect: no db")
	}
	var t CollectTask
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return zero, err
	}
	if err := repository.FindByID(c.Request.Context(), s.DB, &t, tenantID, id); err != nil {
		return zero, err
	}
	return s.enrichTaskDTO(c.Request.Context(), &t), nil
}

// List paginates tasks with filters.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("collect: no db")
	}
	page, ps := clampCollectPage(q.Page, q.PageSize)

	tx := s.DB.WithContext(c.Request.Context()).Model(&CollectTask{})
	var err error
	if tx, _, err = adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(q.Status); v != "" {
		tx = tx.Where("status = ?", v)
	}
	if v := strings.TrimSpace(q.Source); v != "" {
		tx = tx.Where("source = ?", v)
	}
	if bid := strings.TrimSpace(q.BatchID); bid != "" {
		tx = tx.Where("batch_id = ?", bid)
	}
	if v := strings.TrimSpace(q.Keyword); v != "" {
		pat := "%" + strings.ToLower(v) + "%"
		tx = tx.Where("LOWER(source_url) LIKE ?", pat)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page - 1) * ps
	var rows []CollectTask
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		items = append(items, s.enrichTaskDTO(c.Request.Context(), &rows[i]))
	}

	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}

	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}
