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
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service orchestrates image_tasks.
type Service struct {
	DB       *gorm.DB
	OpLog    *operationlog.Service
	Settings *settings.Service
	Files    *files.Service
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
	imgID, srcURL, err := s.ResolveSource(ctx, p.SourceImageID, p.SourceImageURL)
	if err != nil {
		return nil, err
	}
	effectiveProv, err := s.resolveImageProvider(ctx, p.Provider)
	if err != nil {
		return nil, err
	}
	switch effectiveProv {
	case "noop", "removebg":
	default:
		return nil, fmt.Errorf("unsupported image provider %q", effectiveProv)
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

// RunSync executes provider logic for a task row and updates status (JWT handlers pass *gin.Context for operation logs).
func (s *Service) RunSync(c *gin.Context, taskID uuid.UUID, logRetry bool) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("imagetask: no db")
	}
	ctx := c.Request.Context()

	var task ImageTask
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}
	if logRetry {
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				Action:     "image.task.retry",
				Resource:   "image_task",
				ResourceID: taskID.String(),
				Status:     "success",
				Message: fmt.Sprintf("taskType=%s provider=%s productId=%s",
					task.TaskType, task.Provider, ptrUUIDStr(task.ProductID)),
			})
		}
	}

	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"status":        StatusRunning,
		"started_at":    &now,
		"finished_at":   nil,
		"error_message": "",
	}).Error; err != nil {
		return err
	}
	if err := s.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return err
	}

	prov, err := imgprov.NewForTask(ctx, task.Provider, s.Settings)
	if err != nil {
		return s.fail(c, &task, err.Error())
	}

	src := strings.TrimSpace(task.SourceImageURL)
	hints := inputHints(task.Input)
	timeout := imageOperationTimeout(ctx, s.Settings)
	pctx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		pctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	res, runErr := s.dispatch(pctx, prov, &task, src, hints)
	if runErr != nil {
		return s.fail(c, &task, runErr.Error())
	}
	if res == nil {
		return s.fail(c, &task, "provider returned empty result")
	}

	finalURL := strings.TrimSpace(res.PublicURL)
	var finalFID *uuid.UUID
	if res.FileID != nil {
		finalFID = res.FileID
	}

	if len(res.RawPayload) > 0 {
		if s.Files == nil {
			return s.fail(c, &task, "files service unavailable for storing pipeline output")
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
		objKey := fmt.Sprintf("image-tasks/%s/%s-removebg-%s.png", day, task.ID.String(), suffix)
		fr, saveErr := s.Files.SaveProcessed(ctx, files.SaveProcessedOpts{
			OriginalName: fmt.Sprintf("removebg-%s.png", task.ID.String()),
			ObjectKey:    objKey,
			Data:         res.RawPayload,
			ContentType:  ct,
			CreatedBy:    task.CreatedBy,
		})
		if saveErr != nil {
			return s.fail(c, &task, saveErr.Error())
		}
		finalURL = strings.TrimSpace(fr.PublicURL)
		idCopy := fr.ID
		finalFID = &idCopy
	}

	if finalURL == "" {
		return s.fail(c, &task, "provider returned empty result")
	}

	outObj := map[string]any{
		"resultUrl": finalURL,
		"provider":  task.Provider,
	}
	if finalFID != nil {
		outObj["resultFileId"] = finalFID.String()
	}
	if ct := strings.TrimSpace(res.PayloadContentType); ct != "" {
		outObj["contentType"] = ct
	} else if len(res.RawPayload) > 0 {
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
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "image.task.success",
			Resource:   "image_task",
			ResourceID: taskID.String(),
			Status:     "success",
			Message: fmt.Sprintf("taskType=%s provider=%s productId=%s",
				task.TaskType, task.Provider, ptrUUIDStr(task.ProductID)),
		})
	}
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

func (s *Service) fail(c *gin.Context, task *ImageTask, msg string) error {
	ctx := c.Request.Context()
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"status":        StatusFailed,
		"error_message": msg,
		"finished_at":   &fin,
	}).Error
	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			Action:     "image.task.failed",
			Resource:   "image_task",
			ResourceID: task.ID.String(),
			Status:     "failed",
			Message: fmt.Sprintf("taskType=%s provider=%s productId=%s err=%s",
				task.TaskType, task.Provider, ptrUUIDStr(task.ProductID), truncateMsg(msg, 300)),
		})
	}
	return errors.New(msg)
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

// RetryFailed resets a failed task and runs again.
func (s *Service) RetryFailed(c *gin.Context, id uuid.UUID) error {
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
	return s.RunSync(c, id, true)
}
