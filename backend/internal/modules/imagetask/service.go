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
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service orchestrates image_tasks.
type Service struct {
	DB       *gorm.DB
	OpLog    *operationlog.Service
	Settings *settings.Service
	Files    *files.Service
	Redis    *rdb.Client

	QueueEnabled bool
	QueueName    string

	// TaskTimeoutMax caps provider call context (0 = follow settings only).
	TaskTimeoutMax time.Duration
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

func (s *Service) imageOpTimeout(ctx context.Context) time.Duration {
	t := imageOperationTimeout(ctx, s.Settings)
	if s != nil && s.TaskTimeoutMax > 0 && t > s.TaskTimeoutMax {
		t = s.TaskTimeoutMax
	}
	return t
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

// AllowsGenerateSceneNoSource gates text-only ecommerce scene generation (OpenAI Image only).
func (s *Service) AllowsGenerateSceneNoSource(ctx context.Context, explicitProvider string) bool {
	if strings.TrimSpace(strings.ToLower(explicitProvider)) == "openai_image" {
		return true
	}
	if s == nil || s.Settings == nil {
		return false
	}
	m, err := s.Settings.PlainByGroup(ctx, 0, "image")
	if err != nil {
		return false
	}
	return strings.TrimSpace(strings.ToLower(m["provider"])) == "openai_image"
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

// ResolveSource looks up source_image_id in files or product_images, or uses explicit URL.
func (s *Service) ResolveSource(ctx context.Context, sourceImageID *uuid.UUID, sourceURL string) (imageID *uuid.UUID, resolvedURL string, err error) {
	urlStr := strings.TrimSpace(sourceURL)
	if sourceImageID != nil && *sourceImageID != uuid.Nil {
		sid := *sourceImageID
		var fr files.FileRecord
		if err := s.DB.WithContext(ctx).First(&fr, "id = ?", sid).Error; err == nil {
			u := strings.TrimSpace(fr.PublicURL)
			if u == "" {
				return nil, "", fmt.Errorf("file has no public url")
			}
			return &sid, u, nil
		}
		var pi product.ProductImage
		if err := s.DB.WithContext(ctx).First(&pi, "id = ?", sid).Error; err == nil {
			u := strings.TrimSpace(pi.PublicURL)
			if u == "" {
				u = strings.TrimSpace(pi.OriginURL)
			}
			if u == "" {
				return nil, "", fmt.Errorf("product image has no url")
			}
			return &sid, u, nil
		}
		if urlStr != "" {
			return nil, urlStr, nil
		}
		return nil, "", fmt.Errorf("sourceImageId not found")
	}
	if urlStr != "" {
		return nil, urlStr, nil
	}
	return nil, "", fmt.Errorf("sourceImageId or sourceImageUrl required")
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
	switch effectiveProv {
	case "noop", "removebg", "openai_image":
	default:
		return nil, fmt.Errorf("unsupported image provider %q", effectiveProv)
	}

	hasSource := (p.SourceImageID != nil && *p.SourceImageID != uuid.Nil) || strings.TrimSpace(p.SourceImageURL) != ""

	var imgID *uuid.UUID
	var srcURL string
	if hasSource {
		var rsErr error
		imgID, srcURL, rsErr = s.ResolveSource(ctx, p.SourceImageID, p.SourceImageURL)
		if rsErr != nil {
			return nil, rsErr
		}
	} else if p.TaskType == TaskTypeGenerateScene && effectiveProv == "openai_image" {
		imgID, srcURL = nil, ""
	} else {
		return nil, fmt.Errorf("sourceImageId or sourceImageUrl required")
	}

	row := &ImageTask{
		TaskType:       p.TaskType,
		Provider:       effectiveProv,
		Status:         StatusPending,
		ProductID:      p.ProductID,
		SourceImageID:  imgID,
		SourceImageURL: srcURL,
		Input:          p.Input,
		CreatedBy:      p.CreatedBy,
	}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) deleteTask(ctx context.Context, id uuid.UUID) {
	if s == nil || s.DB == nil {
		return
	}
	_ = s.DB.WithContext(ctx).Unscoped().Delete(&ImageTask{}, "id = ?", id).Error
}

// ProcessQueuedTask is invoked by the image worker: CAS pending → running, then run provider.
func (s *Service) ProcessQueuedTask(ctx context.Context, taskID uuid.UUID) error {
	return s.executeTask(ctx, taskID, nil)
}

// ProcessSyncAfterCreate runs a pending task inline (IMAGE_QUEUE_ENABLED=false development mode).
func (s *Service) ProcessSyncAfterCreate(ctx context.Context, taskID uuid.UUID, httpCtx *gin.Context) error {
	return s.executeTask(ctx, taskID, httpCtx)
}

func (s *Service) executeTask(ctx context.Context, taskID uuid.UUID, httpCtx *gin.Context) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}

	task, claimed, err := s.claimPendingTask(ctx, taskID)
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}

	src := strings.TrimSpace(task.SourceImageURL)
	hints := inputHints(task.Input)
	if task.TaskType == TaskTypeGenerateScene {
		hints = s.prepareGenerateSceneHints(ctx, task, hints)
	}
	timeout := s.imageOpTimeout(ctx)
	pctx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		pctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	provName := task.Provider
	if task.TaskType == TaskTypeRemoveBackground {
		provName = "removebg"
	}

	prov, err := imgprov.NewForTask(ctx, provName, s.Settings)
	if err != nil {
		return s.fail(ctx, httpCtx, task, err.Error())
	}

	res, runErr := s.dispatch(pctx, prov, task, src, hints)
	if runErr != nil {
		return s.fail(ctx, httpCtx, task, runErr.Error())
	}
	if res == nil {
		return s.fail(ctx, httpCtx, task, "provider returned empty result")
	}

	finalURL := strings.TrimSpace(res.PublicURL)
	var finalFID *uuid.UUID
	if res.FileID != nil {
		finalFID = res.FileID
	}

	if len(res.RawPayload) > 0 {
		if s.Files == nil {
			return s.fail(ctx, httpCtx, task, "files service unavailable for storing pipeline output")
		}
		ct := strings.TrimSpace(res.PayloadContentType)
		if ct == "" {
			ct = "image/png"
		}
		day := time.Now().UTC().Format("20060102")
		suffix := strings.ReplaceAll(uuid.New().String(), "-", "")
		if len(suffix) > 12 {
			suffix = suffix[:12]
		}
		objTag := processedObjectTag(task.Provider)
		objKey := fmt.Sprintf("image-tasks/%s/%s-%s-%s.png", day, task.ID.String(), objTag, suffix)
		fr, saveErr := s.Files.SaveProcessed(ctx, files.SaveProcessedOpts{
			OriginalName: fmt.Sprintf("%s-%s.png", objTag, task.ID.String()),
			ObjectKey:    objKey,
			Data:         res.RawPayload,
			ContentType:  ct,
			CreatedBy:    task.CreatedBy,
		})
		if saveErr != nil {
			return s.fail(ctx, httpCtx, task, saveErr.Error())
		}
		finalURL = strings.TrimSpace(fr.PublicURL)
		idCopy := fr.ID
		finalFID = &idCopy
	}

	if finalURL == "" {
		return s.fail(ctx, httpCtx, task, "provider returned empty result")
	}

	outObj := map[string]any{
		"resultUrl": finalURL,
		"provider":  task.Provider,
	}
	modelOut := ""
	if res.Meta != nil {
		if mv, ok := res.Meta["model"].(string); ok {
			modelOut = strings.TrimSpace(mv)
			if modelOut != "" {
				outObj["model"] = modelOut
			}
		}
	}
	if finalFID != nil {
		outObj["resultFileId"] = finalFID.String()
	}
	if ct := strings.TrimSpace(res.PayloadContentType); ct != "" {
		outObj["contentType"] = ct
	} else if len(res.Meta) > 0 {
		if cv, ok := res.Meta["contentType"].(string); ok && strings.TrimSpace(cv) != "" {
			outObj["contentType"] = strings.TrimSpace(cv)
		}
	}
	if _, has := outObj["contentType"]; !has && len(res.RawPayload) > 0 {
		outObj["contentType"] = "image/png"
	}
	if len(res.Meta) > 0 {
		outObj["meta"] = res.Meta
	}
	outBytes, _ := json.Marshal(outObj)
	fin := time.Now().UTC()
	updates := map[string]any{
		"status":        StatusSuccess,
		"output":        datatypes.JSON(outBytes),
		"result_url":    finalURL,
		"error_message": "",
		"finished_at":   &fin,
	}
	if finalFID != nil {
		updates["result_file_id"] = finalFID
	} else {
		updates["result_file_id"] = nil
	}
	if err := s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", taskID).Updates(updates).Error; err != nil {
		return err
	}
	s.logSuccess(ctx, httpCtx, taskID, task, modelOut)
	return nil
}

func (s *Service) claimPendingTask(ctx context.Context, taskID uuid.UUID) (*ImageTask, bool, error) {
	now := time.Now().UTC()
	res := s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("id = ? AND status = ?", taskID, StatusPending).
		Updates(map[string]any{
			"status":        StatusRunning,
			"started_at":    &now,
			"finished_at":   nil,
			"error_message": "",
		})
	if res.Error != nil {
		return nil, false, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, false, nil
	}
	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, false, err
	}
	return &task, true, nil
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
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":        StatusFailed,
		"error_message": msg,
		"finished_at":   &fin,
	}).Error
	s.logFailed(ctx, httpCtx, task, msg)
	return errors.New(msg)
}

func (s *Service) logSuccess(ctx context.Context, httpCtx *gin.Context, taskID uuid.UUID, task *ImageTask, model string) {
	if s.OpLog == nil {
		return
	}
	msg := fmt.Sprintf("taskType=%s provider=%s model=%s productId=%s",
		task.TaskType, task.Provider, strings.TrimSpace(model), ptrUUIDStr(task.ProductID))
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

	updates := map[string]any{
		"status":         StatusPending,
		"error_message":  "",
		"started_at":     nil,
		"finished_at":    nil,
		"result_url":     "",
		"result_file_id": nil,
		"output":         nil,
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
