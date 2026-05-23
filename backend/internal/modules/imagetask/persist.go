package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
)

// persistProviderResult uploads bytes to configured storage; never returns provider temp URLs as final.
func (s *Service) persistProviderResult(ctx context.Context, task *ImageTask, res *imgprov.ImageResult, hints map[string]any) (finalURL string, fileID *uuid.UUID, storageKey string, err error) {
	if res == nil {
		return "", nil, "", fmt.Errorf("empty provider result")
	}
	if s.Files == nil {
		return "", nil, "", fmt.Errorf("files service unavailable")
	}

	data := res.RawPayload
	ct := strings.TrimSpace(res.PayloadContentType)
	if len(data) == 0 {
		u := strings.TrimSpace(res.PublicURL)
		if u == "" {
			return "", nil, "", fmt.Errorf("provider returned empty result")
		}
		data, ct, err = downloadResultBytes(ctx, u)
		if err != nil {
			return "", nil, "", err
		}
	}
	if ct == "" {
		ct = "image/webp"
	}
	if !strings.HasPrefix(strings.ToLower(ct), "image/") {
		ct = "image/webp"
	}

	objKey := BuildAIImageObjectKey(task.ProductID, task.TaskType)
	origName := fmt.Sprintf("%s-%s.webp", task.TaskType, task.ID.String())

	fr, saveErr := s.Files.SaveProcessed(ctx, files.SaveProcessedOpts{
		OriginalName: origName,
		ObjectKey:    objKey,
		Data:         data,
		ContentType:  ct,
		CreatedBy:    task.CreatedBy,
	})
	if saveErr != nil {
		return "", nil, "", saveErr
	}
	idCopy := fr.ID
	return strings.TrimSpace(fr.PublicURL), &idCopy, strings.TrimSpace(fr.ObjectKey), nil
}

func downloadResultBytes(ctx context.Context, rawURL string) ([]byte, string, error) {
	u := strings.TrimSpace(rawURL)
	if u == "" {
		return nil, "", fmt.Errorf("empty url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	cli := &http.Client{Timeout: 90 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download result: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("download result HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(slurp)))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 30<<20))
	if err != nil {
		return nil, "", err
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	return data, ct, nil
}

func (s *Service) upsertPrimaryTaskItem(ctx context.Context, task *ImageTask, finalURL, storageKey string, fileID *uuid.UUID, scoreJSON []byte, selectedBest bool) error {
	if s == nil || s.DB == nil || task == nil {
		return nil
	}
	var existing ImageTaskItem
	err := s.DB.WithContext(ctx).Where("task_id = ?", task.ID).Order("created_at ASC").First(&existing).Error
	if err != nil {
		item := &ImageTaskItem{
			TaskID:           task.ID,
			ProductID:        task.ProductID,
			SourceImageID:    task.SourceImageID,
			SourceImageURL:   task.SourceImageURL,
			OutputImageURL:   finalURL,
			OutputStorageKey: storageKey,
			OutputFileID:     fileID,
			Status:           ItemStatusSuccess,
			IsSelectedBest:   selectedBest,
		}
		if len(scoreJSON) > 0 {
			item.ScoreJSON = scoreJSON
		}
		return s.DB.WithContext(ctx).Create(item).Error
	}
	updates := map[string]any{
		"output_image_url":   finalURL,
		"output_storage_key": storageKey,
		"output_file_id":     fileID,
		"status":             ItemStatusSuccess,
		"error_message":      "",
		"is_selected_best":   selectedBest,
	}
	if len(scoreJSON) > 0 {
		updates["score_json"] = scoreJSON
	}
	return s.DB.WithContext(ctx).Model(&ImageTaskItem{}).Where("id = ?", existing.ID).Updates(updates).Error
}

func (s *Service) finalizeTaskSuccess(ctx context.Context, _ interface{}, task *ImageTask, finalURL string, fileID *uuid.UUID, storageKey string, outObj map[string]any, scoreJSON []byte, selectedBest bool) error {
	if outObj == nil {
		outObj = map[string]any{}
	}
	outObj["resultUrl"] = finalURL
	outObj["storageKey"] = storageKey
	if fileID != nil {
		outObj["resultFileId"] = fileID.String()
	}
	if len(scoreJSON) > 0 {
		outObj["score"] = json.RawMessage(scoreJSON)
	}
	outBytes, _ := json.Marshal(outObj)
	fin := time.Now().UTC()
	updates := map[string]any{
		"status":            StatusSuccess,
		"output":            outBytes,
		"result_url":        finalURL,
		"error_message":     "",
		"finished_at":       &fin,
		"completed_at":      &fin,
		"result_count":      1,
		"retry_count":       0,
		"next_retry_at":     nil,
		"retry_enqueued_at": nil,
		"locked_by":         nil,
		"locked_until":      nil,
	}
	if fileID != nil {
		updates["result_file_id"] = fileID
	}
	if err := s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", task.ID).Updates(updates).Error; err != nil {
		return err
	}
	_ = s.upsertPrimaryTaskItem(ctx, task, finalURL, storageKey, fileID, scoreJSON, selectedBest)
	hints := inputHints(task.Input)
	s.maybeAutoApply(ctx, task, hints)
	return nil
}

func extractPromptFields(p CreatePayload) (prompt, neg, inputMode string) {
	h := inputHints(p.Input)
	prompt = stringFromMap(h, "prompt")
	neg = stringFromMap(h, "negativePrompt")
	inputMode = stringFromMap(h, "inputMode")
	if prompt == "" {
		prompt = stringFromMap(h, "assembled_prompt")
	}
	return prompt, neg, inputMode
}

func (s *Service) createTaskItemPending(ctx context.Context, taskID uuid.UUID, productID *uuid.UUID, srcID *uuid.UUID, srcURL string) (*uuid.UUID, error) {
	item := &ImageTaskItem{
		TaskID:         taskID,
		ProductID:      productID,
		SourceImageID:  srcID,
		SourceImageURL: srcURL,
		Status:         ItemStatusPending,
	}
	if err := s.DB.WithContext(ctx).Create(item).Error; err != nil {
		return nil, err
	}
	id := item.ID
	return &id, nil
}

func (s *Service) markTaskItemFailed(ctx context.Context, taskID uuid.UUID, msg string) {
	_ = s.DB.WithContext(ctx).Model(&ImageTaskItem{}).
		Where("task_id = ? AND status IN ?", taskID, []string{ItemStatusPending, ItemStatusRunning}).
		Updates(map[string]any{"status": ItemStatusFailed, "error_message": truncateRunes(msg, 4000)}).Error
}

// resolveOpenAIEditSource resolves bytes for openai edit operations.
func (s *Service) resolveOpenAIEditSource(ctx context.Context, task *ImageTask) (removeBGSource, error) {
	return s.resolveOpenAIReplaceBackgroundSource(ctx, task)
}

func ptrUUID(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}
