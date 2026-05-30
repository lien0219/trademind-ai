package imagetask

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type translateResultArtifacts struct {
	TempOutputPath string
	PreviewPath    string
	OutputPath     string
	ResultFileID   *uuid.UUID
	ResultURL      string
}

func translateTargetLanguageFromHints(hints map[string]any) string {
	source, target := resolveTranslateLanguages(hints)
	_ = source
	return strings.TrimSpace(target)
}

func extractTranslateArtifacts(task *ImageTask) translateResultArtifacts {
	out := translateResultArtifacts{}
	if task == nil {
		return out
	}
	out.ResultURL = strings.TrimSpace(task.ResultURL)
	if task.ResultFileID != nil {
		id := *task.ResultFileID
		out.ResultFileID = &id
	}
	if len(task.Output) == 0 {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(task.Output, &m); err != nil {
		return out
	}
	out.TempOutputPath = stringFromMap(m, "tempOutputPath")
	out.PreviewPath = stringFromMap(m, "previewPath")
	out.OutputPath = firstNonEmptyString(stringFromMap(m, "outputPath"), stringFromMap(m, "storageKey"))
	if out.OutputPath == "" {
		out.OutputPath = stringFromMap(m, "storageKey")
	}
	if out.ResultURL == "" {
		out.ResultURL = stringFromMap(m, "resultUrl")
	}
	if fid := strings.TrimSpace(stringFromMap(m, "resultFileId")); fid != "" {
		if u, err := uuid.Parse(fid); err == nil {
			out.ResultFileID = &u
		}
	}
	return out
}

type translateFileStore interface {
	DeleteStorageObject(ctx context.Context, objectKey string) error
	DeleteRecordByID(ctx context.Context, id uuid.UUID) error
}

func (s *Service) translateFiles() translateFileStore {
	if s == nil {
		return nil
	}
	return s.Files
}

func purgeTranslateArtifactsWithStore(ctx context.Context, store translateFileStore, arts translateResultArtifacts) {
	if store == nil {
		return
	}
	objectKey := strings.TrimSpace(arts.TempOutputPath)
	if objectKey != "" {
		_ = store.DeleteStorageObject(ctx, objectKey)
	}
	objectKey = strings.TrimSpace(arts.PreviewPath)
	if objectKey != "" {
		_ = store.DeleteStorageObject(ctx, objectKey)
	}
	objectKey = strings.TrimSpace(arts.OutputPath)
	if objectKey != "" {
		_ = store.DeleteStorageObject(ctx, objectKey)
	}
	if arts.ResultFileID != nil && *arts.ResultFileID != uuid.Nil {
		_ = store.DeleteRecordByID(ctx, *arts.ResultFileID)
	}
}

func (s *Service) deleteStorageObject(ctx context.Context, objectKey string) {
	objectKey = strings.TrimSpace(objectKey)
	if store := s.translateFiles(); store != nil && objectKey != "" {
		_ = store.DeleteStorageObject(ctx, objectKey)
	}
}

func (s *Service) deleteFileRecord(ctx context.Context, fileID *uuid.UUID) {
	if store := s.translateFiles(); store != nil && fileID != nil && *fileID != uuid.Nil {
		_ = store.DeleteRecordByID(ctx, *fileID)
	}
}

func (s *Service) purgeTranslateArtifacts(ctx context.Context, arts translateResultArtifacts) {
	purgeTranslateArtifactsWithStore(ctx, s.translateFiles(), arts)
}

func (s *Service) clearTranslateTaskOutput(ctx context.Context, taskID uuid.UUID, errMsg string) error {
	if s == nil || s.DB == nil {
		return nil
	}
	fin := time.Now().UTC()
	return s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", taskID).Updates(map[string]any{
		"status":         StatusFailed,
		"result_url":     "",
		"result_file_id": nil,
		"output":         nil,
		"result_count":   0,
		"error_message":  truncateRunes(errMsg, 8000),
		"finished_at":    &fin,
		"completed_at":   &fin,
	}).Error
}

func (s *Service) markTranslateTasksObsolete(ctx context.Context, taskIDs []uuid.UUID) error {
	if s == nil || s.DB == nil || len(taskIDs) == 0 {
		return nil
	}
	fin := time.Now().UTC()
	return s.DB.WithContext(ctx).Model(&ImageTask{}).
		Where("id IN ?", taskIDs).
		Updates(map[string]any{
			"status":         StatusObsolete,
			"result_url":     "",
			"result_file_id": nil,
			"output":         nil,
			"result_count":   0,
			"finished_at":    &fin,
			"completed_at":   &fin,
		}).Error
}

func (s *Service) supersedePriorTranslateResults(ctx context.Context, task *ImageTask, targetLang string, excludeTaskID uuid.UUID) error {
	if s == nil || s.DB == nil || task == nil || task.SourceImageID == nil {
		return nil
	}
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = "en"
	}
	var rows []ImageTask
	q := s.DB.WithContext(ctx).
		Where("task_type = ? AND source_image_id = ? AND id <> ?", TaskTypeTranslateImageText, task.SourceImageID, excludeTaskID).
		Where("status IN ?", []string{
			StatusFailed, StatusLowQuality, StatusFailedValidation, StatusNeedManualReview, StatusObsolete,
			StatusSuccess, StatusSuccessWithWarnings, StatusSuccessWithReview,
		})
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	var obsoleteIDs []uuid.UUID
	for _, row := range rows {
		hints := inputHints(row.Input)
		if strings.TrimSpace(translateTargetLanguageFromHints(hints)) != targetLang {
			continue
		}
		arts := extractTranslateArtifacts(&row)
		s.purgeTranslateArtifacts(ctx, arts)
		obsoleteIDs = append(obsoleteIDs, row.ID)
	}
	if len(obsoleteIDs) == 0 {
		return nil
	}
	return s.markTranslateTasksObsolete(ctx, obsoleteIDs)
}

func (s *Service) handleTranslateTaskFailure(ctx context.Context, task *ImageTask, hints map[string]any, runErr error) error {
	if s == nil || task == nil {
		return runErr
	}
	arts := extractTranslateArtifacts(task)
	s.purgeTranslateArtifacts(ctx, arts)
	msg := strings.TrimSpace(runErr.Error())
	_ = s.clearTranslateTaskOutput(ctx, task.ID, msg)
	s.logTranslateAudit(ctx, task, "ai_image.translate_text.failed", "failed",
		translateAuditMsg(task, map[string]any{"error": msg, "artifactsPurged": true}))
	return runErr
}

func (s *Service) discardUnusableTranslateOutput(ctx context.Context, task *ImageTask, arts translateResultArtifacts, reason string) {
	if s == nil || task == nil {
		return
	}
	s.purgeTranslateArtifacts(ctx, arts)
	out := map[string]any{
		"finalQualityStatus": reason,
		"resultUnavailable":  true,
		"discardReason":      reason,
	}
	outBytes, _ := json.Marshal(out)
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ImageTask{}).Where("id = ?", task.ID).Updates(map[string]any{
		"result_url":     "",
		"result_file_id": nil,
		"output":         outBytes,
		"result_count":   0,
		"finished_at":    &fin,
		"completed_at":   &fin,
	}).Error
	_ = s.DB.WithContext(ctx).Model(&ImageTaskItem{}).
		Where("task_id = ?", task.ID).
		Updates(map[string]any{
			"status":             ItemStatusFailed,
			"output_image_url":   "",
			"output_storage_key": "",
			"output_file_id":     nil,
			"error_message":      truncateRunes("输出质量不达标，结果已删除", 4000),
		}).Error
}

func translateArtifactsFromPersist(storageKey string, fileID *uuid.UUID, finalURL string) translateResultArtifacts {
	return translateResultArtifacts{
		TempOutputPath: storageKey,
		PreviewPath:    storageKey,
		OutputPath:     storageKey,
		ResultFileID:   fileID,
		ResultURL:      finalURL,
	}
}

func attachTranslateArtifactPaths(outObj map[string]any, storageKey, finalURL string) {
	if outObj == nil {
		return
	}
	sk := strings.TrimSpace(storageKey)
	outObj["tempOutputPath"] = sk
	outObj["previewPath"] = sk
	outObj["outputPath"] = sk
	outObj["resultUrl"] = finalURL
	outObj["storageKey"] = sk
}

func findPriorTranslateTasks(db *gorm.DB, sourceImageID uuid.UUID, targetLang string, exclude uuid.UUID) ([]ImageTask, error) {
	if db == nil || sourceImageID == uuid.Nil {
		return nil, nil
	}
	var rows []ImageTask
	err := db.Where("task_type = ? AND source_image_id = ? AND id <> ?", TaskTypeTranslateImageText, sourceImageID, exclude).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	var out []ImageTask
	for _, row := range rows {
		if strings.TrimSpace(translateTargetLanguageFromHints(inputHints(row.Input))) != strings.TrimSpace(targetLang) {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func formatMappedBoxLogs(meta translateCoordinateMeta) string {
	if len(meta.FinalMappedBoxes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(meta.FinalMappedBoxes))
	for _, b := range meta.FinalMappedBoxes {
		parts = append(parts, fmt.Sprintf("%s=%s", b.BlockID, b.Box))
	}
	return strings.Join(parts, "; ")
}
