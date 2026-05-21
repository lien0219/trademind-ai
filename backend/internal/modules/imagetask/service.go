package imagetask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service orchestrates image_tasks.
type Service struct {
	DB        *gorm.DB
	OpLog     *operationlog.Service
	Settings  *settings.Service
	Files     *files.Service
	Redis     *rdb.Client
	AIGateway *aigate.Gateway

	QueueEnabled bool
	QueueName    string

	// TaskTimeoutMax caps provider call context (0 = follow settings only).
	TaskTimeoutMax time.Duration

	AutoRetryEnabled  bool
	MaxAutoRetries    int
	RetryBaseDelaySec int
	RetryMaxDelaySec  int
}

// CreatePayload is the normalized create input.
type CreatePayload struct {
	TaskType       string
	Provider       string
	ProductID      *uuid.UUID
	SourceImageID  *uuid.UUID
	SourceImageURL string
	Input          datatypes.JSON
	CreatedBy      *uuid.UUID
	BatchID        *uuid.UUID
	BatchNo        string
}

func imageOperationTimeout(ctx context.Context, svc *settings.Service) time.Duration {
	def := 60 * time.Second
	if svc == nil {
		return def
	}
	m, err := svc.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return def
	}
	raw := strings.TrimSpace(m["timeout_sec"])
	if raw == "" {
		return def
	}
	sec, err := strconv.Atoi(raw)
	if err != nil || sec < 5 {
		return def
	}
	if sec > 600 {
		sec = 600
	}
	return time.Duration(sec) * time.Second
}

func (s *Service) comfyUIExecutionBudget(ctx context.Context) time.Duration {
	if s == nil || s.Settings == nil {
		return 0
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return 0
	}
	maxPoll := comfyIntSetting(m["comfyui_max_poll_seconds"], 180, 5, 7200)
	httpSec := comfyIntSetting(m["comfyui_timeout_sec"], 180, 5, 3600)
	return time.Duration(maxPoll)*time.Second + time.Duration(httpSec)*time.Second + 45*time.Second
}

func (s *Service) effectiveMaxRetries(task *ImageTask) int {
	if task != nil && task.MaxRetries > 0 {
		return task.MaxRetries
	}
	if s != nil && s.MaxAutoRetries > 0 {
		return s.MaxAutoRetries
	}
	return 2
}

func (s *Service) defaultMaxRetriesForNewTask() int {
	return s.effectiveMaxRetries(nil)
}

func (s *Service) effectiveRetryBaseSec() int {
	if s != nil && s.RetryBaseDelaySec > 0 {
		return s.RetryBaseDelaySec
	}
	return 30
}

func (s *Service) effectiveRetryMaxSec() int {
	if s != nil && s.RetryMaxDelaySec > 0 {
		return s.RetryMaxDelaySec
	}
	return 300
}

func comfyIntSetting(raw string, def, minV, maxV int) int {
	s := strings.TrimSpace(raw)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < minV {
		return def
	}
	if n > maxV {
		return maxV
	}
	return n
}

func (s *Service) resolveImageProvider(ctx context.Context, explicit string) (string, error) {
	v := strings.TrimSpace(strings.ToLower(explicit))
	if v != "" {
		return v, nil
	}
	if s.Settings == nil {
		return "noop", nil
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return "", err
	}
	v = strings.TrimSpace(strings.ToLower(m["provider"]))
	if v == "" {
		v = "noop"
	}
	return v, nil
}

// AllowsGenerateSceneNoSource gates text-only ecommerce scene generation.
func (s *Service) AllowsGenerateSceneNoSource(ctx context.Context, explicitProvider string) bool {
	ex := strings.TrimSpace(strings.ToLower(explicitProvider))
	if ex != "" {
		return imgprov.ProviderSupportsGenerateSceneNoSource(ex)
	}
	if s == nil || s.Settings == nil {
		return false
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return false
	}
	p := strings.TrimSpace(strings.ToLower(m["provider"]))
	return imgprov.ProviderSupportsGenerateSceneNoSource(p)
}

func inputHints(raw datatypes.JSON) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil
	}
	return m
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	default:
		return 0
	}
}

func stringFromMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

// CreateAndPersist inserts a pending task row (without running the provider).
func (s *Service) CreateAndPersist(ctx context.Context, p CreatePayload) (*ImageTask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
	}
	if !isValidTaskType(p.TaskType) {
		return nil, fmt.Errorf("invalid taskType")
	}
	effectiveProv, err := s.resolveImageProvider(ctx, p.Provider)
	if err != nil {
		return nil, err
	}
	if p.TaskType == TaskTypeRemoveBackground {
		effectiveProv = "removebg"
	}
	if IsScoringTaskType(p.TaskType) {
		if ep, err := s.resolveImageProvider(ctx, p.Provider); err == nil && ep != "" {
			effectiveProv = ep
		} else {
			effectiveProv = "noop"
		}
	}
	if !IsScoringTaskType(p.TaskType) {
		if !imgprov.IsRunnableProvider(effectiveProv) {
			return nil, imgprov.UnsupportedTaskError(effectiveProv, p.TaskType)
		}
		if !imgprov.SupportsTask(effectiveProv, p.TaskType) {
			return nil, imgprov.UnsupportedTaskError(effectiveProv, p.TaskType)
		}
	}
	if s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		if err := imgprov.ValidateSettingsForProvider(effectiveProv, m); err != nil {
			return nil, err
		}
	}

	hasSource := (p.SourceImageID != nil && *p.SourceImageID != uuid.Nil) || strings.TrimSpace(p.SourceImageURL) != ""

	if !hasSource && p.TaskType == TaskTypeGenerateScene && imgprov.ProviderSupportsGenerateSceneNoSource(effectiveProv) {
		// ok
	} else if !hasSource && p.TaskType == TaskTypeSelectBestMain {
		// scores all product images
	} else if !hasSource && !RequiresSourceImage(p.TaskType) && IsGenerationTaskType(p.TaskType) {
		// text-only generation
	} else if !hasSource {
		return nil, fmt.Errorf("sourceImageId or sourceImageUrl required")
	}

	prompt, neg, inputMode := extractPromptFields(p)

	var imgID *uuid.UUID
	var srcURL string
	if hasSource {
		var rsErr error
		imgID, srcURL, rsErr = s.ResolveSource(ctx, p.SourceImageID, p.SourceImageURL)
		if rsErr != nil {
			return nil, rsErr
		}
	}

	row := &ImageTask{
		TaskType:       p.TaskType,
		Provider:       effectiveProv,
		Status:         StatusPending,
		ProductID:      p.ProductID,
		SourceImageID:  imgID,
		SourceImageURL: srcURL,
		InputMode:      inputMode,
		Prompt:         prompt,
		NegativePrompt: neg,
		OptionsJSON:    p.Input,
		Input:          p.Input,
		CreatedBy:      p.CreatedBy,
		MaxRetries:     s.defaultMaxRetriesForNewTask(),
		BatchID:        p.BatchID,
		BatchNo:        strings.TrimSpace(p.BatchNo),
	}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

// FinalizeNewImageTask enqueues a pending task or runs it inline (same contract as HTTP POST /image/tasks).
func (s *Service) FinalizeNewImageTask(ctx context.Context, c *gin.Context, row *ImageTask) error {
	if s == nil || row == nil {
		return fmt.Errorf("imagetask: invalid finalize")
	}
	if s.QueueEnabled {
		var rid string
		if c != nil {
			if tid, ok := c.Get(ctxkey.TraceID); ok {
				if str, ok := tid.(string); ok {
					rid = str
				}
			}
		}
		if err := s.enqueueTask(ctx, row.ID, row.TaskType, row.Provider, row.CreatedBy, rid); err != nil {
			s.deleteTask(ctx, row.ID)
			return err
		}
		return nil
	}
	return s.ProcessSyncAfterCreate(ctx, row.ID, c)
}

func (s *Service) deleteTask(ctx context.Context, id uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	_ = s.DB.WithContext(ctx).Unscoped().Delete(&ImageTask{}, "id = ?", id).Error
}

// ProcessQueuedTask is invoked by the image worker: CAS pending → running, then run provider.
func (s *Service) ProcessQueuedTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	return s.executeTask(ctx, taskID, nil, workerID)
}

// ProcessSyncAfterCreate runs a pending task inline (IMAGE_QUEUE_ENABLED=false development mode).
func (s *Service) ProcessSyncAfterCreate(ctx context.Context, taskID uuid.UUID, httpCtx *gin.Context) error {
	return s.executeTask(ctx, taskID, httpCtx, worker.GenerateInlineWorkerID(worker.TypeImage))
}

func (s *Service) executeTask(ctx context.Context, taskID uuid.UUID, httpCtx *gin.Context, workerID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}

	defer func() {
		if r := recover(); r != nil {
			s.handleImagePanic(ctx, httpCtx, taskID, workerID, r)
		}
	}()

	var peek ImageTask
	if err := s.DB.WithContext(ctx).First(&peek, "id = ?", taskID).Error; err != nil {
		return err
	}
	lease := s.computeExecutionTimeout(ctx, &peek)

	task, claimed, err := s.tryClaimImageTask(ctx, taskID, workerID, lease)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	stopRen := s.startImageLeaseRenewal(ctx, taskID, workerID, lease)
	defer stopRen()

	src := strings.TrimSpace(task.SourceImageURL)
	hints := inputHints(task.Input)
	if task.TaskType == TaskTypeGenerateScene || IsGenerationTaskType(task.TaskType) {
		hints = s.prepareGenerateSceneHints(ctx, task, hints)
	}
	if task.TaskType == TaskTypeGenerateMarketing || task.TaskType == TaskTypeGenerateMainImage || task.TaskType == TaskTypeBatchGenerateMain {
		hints = prepareGenerationHints(task, hints)
	}
	if IsCleanupTaskType(task.TaskType) {
		hints = prepareCleanupHints(task, hints)
	}
	if task.TaskType == TaskTypeReplaceBackground &&
		(strings.EqualFold(strings.TrimSpace(task.Provider), "comfyui") ||
			strings.EqualFold(strings.TrimSpace(task.Provider), "openai_image")) {
		hints = s.prepareReplaceBackgroundHints(ctx, task, hints)
	}
	timeout := s.computeExecutionTimeout(ctx, task)
	pctx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		pctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	if IsScoringTaskType(task.TaskType) {
		if err := s.executeScoringTask(pctx, task, hints); err != nil {
			return s.fail(ctx, httpCtx, task, err.Error())
		}
		return nil
	}

	provName := task.Provider
	if task.TaskType == TaskTypeRemoveBackground {
		provName = "removebg"
	}

	prov, err := imgprov.NewForTask(ctx, provName, s.Settings)
	if err != nil {
		return s.fail(ctx, httpCtx, task, err.Error())
	}

	res, runErr := func() (*imgprov.ImageResult, error) {
		if task.TaskType == TaskTypeRemoveBackground {
			rb, err := s.resolveRemoveBGSource(pctx, task)
			if err != nil {
				return nil, err
			}
			if rb.File != nil {
				defer rb.File.Close()
			}
			imgReq := imgprov.ImageRequest{
				SourceURL:         rb.PublicURL,
				SourceFile:        rb.File,
				SourceFilename:    rb.Filename,
				SourceContentType: rb.ContentType,
				Input:             hints,
			}
			return prov.RemoveBackground(pctx, imgReq)
		}
		if task.TaskType == TaskTypeReplaceBackground && strings.EqualFold(strings.TrimSpace(task.Provider), "openai_image") {
			rb, err := s.resolveOpenAIReplaceBackgroundSource(pctx, task)
			if err != nil {
				return nil, err
			}
			if rb.File != nil {
				defer rb.File.Close()
			}
			return prov.ReplaceBackground(pctx, imgprov.ReplaceBackgroundRequest{
				ImageRequest: imgprov.ImageRequest{
					SourceURL:         rb.PublicURL,
					SourceFile:        rb.File,
					SourceFilename:    rb.Filename,
					SourceContentType: rb.ContentType,
					Input:             hints,
				},
				Background: stringFromMap(hints, "background"),
			})
		}
		return s.runProviderForTask(pctx, prov, task, src, hints)
	}()
	if runErr != nil {
		return s.fail(ctx, httpCtx, task, runErr.Error())
	}
	if res == nil {
		return s.fail(ctx, httpCtx, task, "provider returned empty result")
	}

	finalURL, finalFID, storageKey, persistErr := s.persistProviderResult(pctx, task, res, hints)
	if persistErr != nil {
		return s.fail(ctx, httpCtx, task, persistErr.Error())
	}

	outObj := map[string]any{
		"resultUrl":  finalURL,
		"storageKey": storageKey,
		"provider":   task.Provider,
		"taskType":   task.TaskType,
	}
	modelOut := ""
	promptIDOut := ""
	if res.Meta != nil {
		if mv, ok := res.Meta["model"].(string); ok {
			modelOut = strings.TrimSpace(mv)
			if modelOut != "" {
				outObj["model"] = modelOut
			}
		}
		if pv, ok := res.Meta["promptId"].(string); ok {
			promptIDOut = strings.TrimSpace(pv)
			if promptIDOut != "" {
				outObj["promptId"] = promptIDOut
			}
		}
		if wv, ok := res.Meta["workflow"].(string); ok {
			outObj["workflow"] = wv
		}
	}
	if finalFID != nil {
		outObj["resultFileId"] = finalFID.String()
	}
	if ct := strings.TrimSpace(res.PayloadContentType); ct != "" {
		outObj["contentType"] = ct
	}
	if err := s.finalizeTaskSuccess(ctx, httpCtx, task, finalURL, finalFID, storageKey, outObj, nil, false); err != nil {
		return err
	}
	s.logSuccess(ctx, httpCtx, task.ID, task, modelOut, promptIDOut)
	return nil
}

func (s *Service) dispatch(ctx context.Context, prov imgprov.Provider, task *ImageTask, src string, hints map[string]any) (*imgprov.ImageResult, error) {
	switch task.TaskType {
	case TaskTypeRemoveBackground:
		return prov.RemoveBackground(ctx, imgprov.ImageRequest{SourceURL: src, Input: hints})
	case TaskTypeReplaceBackground:
		return prov.ReplaceBackground(ctx, imgprov.ReplaceBackgroundRequest{
			ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
			Background:   stringFromMap(hints, "background"),
		})
	case TaskTypeGenerateScene:
		return prov.GenerateScene(ctx, imgprov.GenerateSceneRequest{
			ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
			Scene:        stringFromMap(hints, "scene"),
		})
	case TaskTypeResize:
		return prov.Resize(ctx, imgprov.ResizeRequest{
			SourceURL: src,
			Width:     intFromAny(hints["width"]),
			Height:    intFromAny(hints["height"]),
			Input:     hints,
		})
	case TaskTypeEnhance:
		return prov.Enhance(ctx, imgprov.ImageRequest{SourceURL: src, Input: hints})
	case TaskTypeTranslateImage:
		return prov.TranslateImage(ctx, imgprov.TranslateImageRequest{
			ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
			TargetLang:   stringFromMap(hints, "targetLang"),
		})
	case TaskTypePosterGenerate:
		return prov.PosterGenerate(ctx, imgprov.PosterGenerateRequest{
			ImageRequest: imgprov.ImageRequest{SourceURL: src, Input: hints},
			Title:        stringFromMap(hints, "title"),
		})
	default:
		return nil, fmt.Errorf("unknown task type %q", task.TaskType)
	}
}

func (s *Service) fail(ctx context.Context, httpCtx *gin.Context, task *ImageTask, msg string) error {
	return s.handleImageTaskFailure(ctx, httpCtx, task, errors.New(msg))
}

func (s *Service) handleImageTaskFailure(ctx context.Context, httpCtx *gin.Context, task *ImageTask, runErr error) error {
	msg := strings.TrimSpace(runErr.Error())
	msg = redactSensitiveErr(msg)
	msg = truncateRunes(msg, 8000)

	if httpCtx != nil || s == nil || !s.AutoRetryEnabled || !s.QueueEnabled {
		return s.finalizeImageFailed(ctx, httpCtx, task, msg, false)
	}
	if !IsRetryableImageTaskError(runErr) {
		return s.finalizeImageFailed(ctx, httpCtx, task, msg, false)
	}
	maxR := s.effectiveMaxRetries(task)
	if task.RetryCount >= maxR {
		return s.finalizeImageFailed(ctx, httpCtx, task, msg, true)
	}
	s.scheduleImageAutoRetry(ctx, task, msg)
	return errors.New(msg)
}

func (s *Service) scheduleImageAutoRetry(ctx context.Context, task *ImageTask, msg string) {
	if s == nil || s.DB == nil || task == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now().UTC()
	newRC := task.RetryCount + 1
	delaySec := imageRetryDelaySeconds(newRC, s.effectiveRetryBaseSec(), s.effectiveRetryMaxSec())
	lm := strings.ToLower(msg)
	if strings.Contains(lm, "429") || strings.Contains(lm, "rate limit") || strings.Contains(lm, "too many requests") {
		if delaySec < s.effectiveRetryMaxSec() {
			delaySec *= 2
			if delaySec > s.effectiveRetryMaxSec() {
				delaySec = s.effectiveRetryMaxSec()
			}
		}
	}
	next := now.Add(time.Duration(delaySec) * time.Second)

	_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":            StatusRetrying,
		"retry_count":       newRC,
		"next_retry_at":     &next,
		"error_message":     msg,
		"finished_at":       nil,
		"retry_enqueued_at": nil,
		"locked_by":         nil,
		"locked_until":      nil,
	}).Error

	maxR := s.effectiveMaxRetries(task)
	if s.OpLog != nil {
		logMsg := fmt.Sprintf("taskType=%s provider=%s retryCount=%d maxRetries=%d nextRetryAt=%s productId=%s",
			task.TaskType, task.Provider, newRC, maxR, next.Format(time.RFC3339), ptrUUIDStr(task.ProductID))
		var admin *uuid.UUID
		if task.CreatedBy != nil {
			admin = task.CreatedBy
		}
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "image.task.auto_retry_scheduled",
			Resource:    "image_task",
			ResourceID:  task.ID.String(),
			Status:      "success",
			Message:     truncateRunes(logMsg, 2000),
		})
	}
}

func (s *Service) finalizeImageFailed(ctx context.Context, httpCtx *gin.Context, task *ImageTask, msg string, exhausted bool) error {
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":            StatusFailed,
		"error_message":     msg,
		"finished_at":       &fin,
		"next_retry_at":     nil,
		"retry_enqueued_at": nil,
		"locked_by":         nil,
		"locked_until":      nil,
	}).Error
	if exhausted {
		s.logRetryExhausted(ctx, httpCtx, task, msg)
	} else {
		s.logFailed(ctx, httpCtx, task, msg)
	}
	return errors.New(msg)
}

func (s *Service) logRetryExhausted(ctx context.Context, httpCtx *gin.Context, task *ImageTask, msg string) {
	if s.OpLog == nil {
		return
	}
	maxR := s.effectiveMaxRetries(task)
	opts := operationlog.WriteOpts{
		Action:     "image.task.retry_exhausted",
		Resource:   "image_task",
		ResourceID: task.ID.String(),
		Status:     "failed",
		Message: fmt.Sprintf("taskType=%s provider=%s retryCount=%d maxRetries=%d productId=%s err=%s",
			task.TaskType, task.Provider, task.RetryCount, maxR, ptrUUIDStr(task.ProductID), truncateMsg(msg, 300)),
	}
	if httpCtx != nil {
		_ = s.OpLog.Write(httpCtx, opts)
		return
	}
	var admin *uuid.UUID
	if task.CreatedBy != nil {
		admin = task.CreatedBy
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: admin,
		Action:      opts.Action,
		Resource:    opts.Resource,
		ResourceID:  opts.ResourceID,
		Status:      opts.Status,
		Message:     opts.Message,
	})
}

func (s *Service) logSuccess(ctx context.Context, httpCtx *gin.Context, taskID uuid.UUID, task *ImageTask, model string, promptID string) {
	if s.OpLog == nil {
		return
	}
	msg := fmt.Sprintf("taskType=%s provider=%s model=%s productId=%s",
		task.TaskType, task.Provider, strings.TrimSpace(model), ptrUUIDStr(task.ProductID))
	if pid := strings.TrimSpace(promptID); pid != "" {
		msg += " promptId=" + pid
	}
	opts := operationlog.WriteOpts{
		Action:     "image.task.success",
		Resource:   "image_task",
		ResourceID: taskID.String(),
		Status:     "success",
		Message:    msg,
	}
	if httpCtx != nil {
		_ = s.OpLog.Write(httpCtx, opts)
		return
	}
	var admin *uuid.UUID
	if task.CreatedBy != nil {
		admin = task.CreatedBy
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: admin,
		Action:      opts.Action,
		Resource:    opts.Resource,
		ResourceID:  opts.ResourceID,
		Status:      opts.Status,
		Message:     opts.Message,
	})
}

func (s *Service) logFailed(ctx context.Context, httpCtx *gin.Context, task *ImageTask, msg string) {
	if s.OpLog == nil {
		return
	}
	opts := operationlog.WriteOpts{
		Action:     "image.task.failed",
		Resource:   "image_task",
		ResourceID: task.ID.String(),
		Status:     "failed",
		Message: fmt.Sprintf("taskType=%s provider=%s productId=%s err=%s",
			task.TaskType, task.Provider, ptrUUIDStr(task.ProductID), truncateMsg(msg, 300)),
	}
	if httpCtx != nil {
		_ = s.OpLog.Write(httpCtx, opts)
		return
	}
	var admin *uuid.UUID
	if task.CreatedBy != nil {
		admin = task.CreatedBy
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		AdminUserID: admin,
		Action:      opts.Action,
		Resource:    opts.Resource,
		ResourceID:  opts.ResourceID,
		Status:      opts.Status,
		Message:     opts.Message,
	})
}

func truncateMsg(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func ptrUUIDStr(p *uuid.UUID) string {
	if p == nil {
		return ""
	}
	return p.String()
}

// GetByID loads one task with all columns.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*ImageTask, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("imagetask: no db")
	}
	var row ImageTask
	if err := s.DB.WithContext(ctx).First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// RetryEnqueue resets a failed task to pending and enqueues it (async), or runs once if queue disabled.
func (s *Service) RetryEnqueue(c *gin.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	ctx := c.Request.Context()

	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return err
	}
	if task.Status != StatusFailed {
		return fmt.Errorf("only failed tasks can be retried")
	}

	maxR := s.defaultMaxRetriesForNewTask()
	updates := map[string]any{
		"status":            StatusPending,
		"retry_count":       0,
		"max_retries":       maxR,
		"next_retry_at":     nil,
		"retry_enqueued_at": nil,
		"error_message":     "",
		"started_at":        nil,
		"finished_at":       nil,
		"result_url":        "",
		"result_file_id":    nil,
		"output":            nil,
		"locked_by":         nil,
		"locked_until":      nil,
	}
	res := s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ? AND status = ?", id, StatusFailed).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("only failed tasks can be retried")
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "image.task.retry",
			Resource:   "image_task",
			ResourceID: id.String(),
			Status:     "success",
			Message: fmt.Sprintf("taskType=%s provider=%s productId=%s",
				task.TaskType, task.Provider, ptrUUIDStr(task.ProductID)),
		})
	}

	if s.QueueEnabled {
		reqStr, _ := c.Get(ctxkey.TraceID)
		var rid string
		if s, ok := reqStr.(string); ok {
			rid = s
		}
		if err := s.enqueueTask(ctx, id, task.TaskType, task.Provider, task.CreatedBy, rid); err != nil {
			_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", id).Updates(map[string]any{
				"status":        StatusFailed,
				"error_message": "retry enqueue failed: " + err.Error(),
				"finished_at":   time.Now().UTC(),
				"locked_by":     nil,
				"locked_until":  nil,
			}).Error
			return err
		}
		return nil
	}

	if err := s.ProcessSyncAfterCreate(ctx, id, c); err != nil {
		return err
	}
	return nil
}
