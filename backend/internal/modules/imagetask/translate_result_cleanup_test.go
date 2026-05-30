package imagetask

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func setupTranslateCleanupTestDB(t *testing.T) (*gorm.DB, *Service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ImageTask{}, &ImageTaskItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, &Service{DB: db}
}

type mockTranslateFileStore struct {
	mu          sync.Mutex
	deletedKeys []string
	deletedIDs  []uuid.UUID
}

func (m *mockTranslateFileStore) DeleteStorageObject(_ context.Context, objectKey string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedKeys = append(m.deletedKeys, objectKey)
	return nil
}

func (m *mockTranslateFileStore) DeleteRecordByID(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deletedIDs = append(m.deletedIDs, id)
	return nil
}

func TestHandleTranslateTaskFailureClearsOutput(t *testing.T) {
	db, svc := setupTranslateCleanupTestDB(t)
	sourceID := uuid.New()
	taskID := uuid.New()
	out := map[string]any{
		"resultUrl":      "https://example.com/out.webp",
		"storageKey":     "products/a/out.webp",
		"tempOutputPath": "products/a/temp.webp",
		"previewPath":    "products/a/preview.webp",
		"outputPath":     "products/a/out.webp",
	}
	outBytes, _ := json.Marshal(out)
	task := &ImageTask{
		TaskType:       TaskTypeTranslateImageText,
		Provider:       "local_render",
		Status:         StatusRunning,
		SourceImageID:  &sourceID,
		SourceImageURL: "https://example.com/src.jpg",
		ResultURL:      "https://example.com/out.webp",
		Output:         datatypes.JSON(outBytes),
	}
	task.ID = taskID
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	err := svc.handleTranslateTaskFailure(context.Background(), task, map[string]any{"targetLanguage": "en"},
		coordMappingUnsafeError())
	if err == nil {
		t.Fatal("expected error")
	}
	var got ImageTask
	if err := db.First(&got, "id = ?", taskID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.ResultURL != "" {
		t.Fatalf("result url should be cleared, got %q", got.ResultURL)
	}
	if len(got.Output) > 0 && string(got.Output) != "null" {
		t.Fatalf("output should be cleared, got %s", string(got.Output))
	}
}

func TestHandleTranslateTaskFailurePurgesArtifactFiles(t *testing.T) {
	db, svc := setupTranslateCleanupTestDB(t)
	mockFiles := &mockTranslateFileStore{}

	taskID := uuid.New()
	fileID := uuid.New()
	out := map[string]any{
		"tempOutputPath": "products/a/temp.webp",
		"previewPath":    "products/a/preview.webp",
		"outputPath":     "products/a/out.webp",
		"resultFileId":   fileID.String(),
	}
	outBytes, _ := json.Marshal(out)
	task := &ImageTask{
		TaskType: TaskTypeTranslateImageText, Provider: "local_render", Status: StatusRunning,
		SourceImageURL: "https://example.com/src.jpg",
		ResultURL:      "https://example.com/out.webp",
		ResultFileID:   &fileID,
		Output:         datatypes.JSON(outBytes),
	}
	task.ID = taskID
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	arts := extractTranslateArtifacts(task)
	purgeTranslateArtifactsWithStore(context.Background(), mockFiles, arts)
	_ = svc.clearTranslateTaskOutput(context.Background(), taskID, coordMappingUnsafeError().Error())

	mockFiles.mu.Lock()
	defer mockFiles.mu.Unlock()
	wantKeys := []string{"products/a/temp.webp", "products/a/preview.webp", "products/a/out.webp"}
	if len(mockFiles.deletedKeys) != len(wantKeys) {
		t.Fatalf("deleted keys = %v, want %v", mockFiles.deletedKeys, wantKeys)
	}
	for i, key := range wantKeys {
		if mockFiles.deletedKeys[i] != key {
			t.Fatalf("deleted keys = %v, want %v", mockFiles.deletedKeys, wantKeys)
		}
	}
	if len(mockFiles.deletedIDs) != 1 || mockFiles.deletedIDs[0] != fileID {
		t.Fatalf("deleted file ids = %v, want %v", mockFiles.deletedIDs, fileID)
	}
}

func TestPurgeTranslateArtifactsViaService(t *testing.T) {
	mockFiles := &mockTranslateFileStore{}
	svc := &Service{}
	arts := translateResultArtifacts{
		TempOutputPath: "temp.webp",
		PreviewPath:    "preview.webp",
		OutputPath:     "out.webp",
	}
	svc.purgeTranslateArtifacts(context.Background(), arts)
	if len(mockFiles.deletedKeys) != 0 {
		t.Fatalf("expected no deletes without Files, got %v", mockFiles.deletedKeys)
	}
	purgeTranslateArtifactsWithStore(context.Background(), mockFiles, arts)
	if len(mockFiles.deletedKeys) != 3 {
		t.Fatalf("deleted keys = %v", mockFiles.deletedKeys)
	}
}

func TestSupersedePriorTranslateResultsMarksObsolete(t *testing.T) {
	db, svc := setupTranslateCleanupTestDB(t)
	sourceID := uuid.New()
	oldID := uuid.New()
	newID := uuid.New()
	in := datatypes.JSON([]byte(`{"targetLanguage":"en","storageKey":"old.webp","resultUrl":"https://x/old.webp","outputPath":"old.webp","previewPath":"old.webp","tempOutputPath":"old.webp"}`))
	old := &ImageTask{
		TaskType: TaskTypeTranslateImageText, Provider: "local_render", Status: StatusLowQuality,
		SourceImageID: &sourceID, SourceImageURL: "https://example.com/src.jpg",
		ResultURL: "https://x/old.webp", Input: in, Output: in,
	}
	old.ID = oldID
	if err := db.Create(old).Error; err != nil {
		t.Fatalf("create old: %v", err)
	}
	current := &ImageTask{
		TaskType: TaskTypeTranslateImageText, Provider: "local_render", Status: StatusPending,
		SourceImageID: &sourceID, SourceImageURL: "https://example.com/src.jpg",
		Input: datatypes.JSON([]byte(`{"targetLanguage":"en"}`)),
	}
	current.ID = newID
	if err := svc.supersedePriorTranslateResults(context.Background(), current, "en", newID); err != nil {
		t.Fatalf("supersede: %v", err)
	}
	var reloaded ImageTask
	if err := db.First(&reloaded, "id = ?", oldID).Error; err != nil {
		t.Fatalf("reload old: %v", err)
	}
	if reloaded.Status != StatusObsolete {
		t.Fatalf("old status = %q, want obsolete", reloaded.Status)
	}
	if reloaded.ResultURL != "" {
		t.Fatalf("old result url should be cleared")
	}
}

func TestDiscardUnusableTranslateOutput(t *testing.T) {
	db, svc := setupTranslateCleanupTestDB(t)
	taskID := uuid.New()
	task := &ImageTask{
		TaskType: TaskTypeTranslateImageText, Provider: "local_render", Status: StatusLowQuality,
		ResultURL: "https://example.com/out.webp",
	}
	task.ID = taskID
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	arts := translateResultArtifacts{
		TempOutputPath: "temp.webp",
		PreviewPath:    "preview.webp",
		OutputPath:     "out.webp",
		ResultURL:      "https://example.com/out.webp",
	}
	svc.discardUnusableTranslateOutput(context.Background(), task, arts, StatusLowQuality)
	var got ImageTask
	if err := db.First(&got, "id = ?", taskID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.ResultURL != "" {
		t.Fatalf("result url should be empty")
	}
	var out map[string]any
	if err := json.Unmarshal(got.Output, &out); err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if out["resultUnavailable"] != true {
		t.Fatalf("output = %+v", out)
	}
}

func TestSupersedePriorTranslateResultsBeforeRetry(t *testing.T) {
	db, svc := setupTranslateCleanupTestDB(t)
	sourceID := uuid.New()
	oldID := uuid.New()
	retryID := uuid.New()
	in := datatypes.JSON([]byte(`{"targetLanguage":"en","outputPath":"retry-old.webp"}`))
	old := &ImageTask{
		TaskType: TaskTypeTranslateImageText, Provider: "local_render", Status: StatusFailed,
		SourceImageID: &sourceID, SourceImageURL: "https://example.com/src.jpg",
		Input: in, Output: in, ResultURL: "https://x/retry-old.webp",
	}
	old.ID = oldID
	if err := db.Create(old).Error; err != nil {
		t.Fatalf("create old: %v", err)
	}
	retry := &ImageTask{
		TaskType: TaskTypeTranslateImageText, Provider: "local_render", Status: StatusPending,
		SourceImageID: &sourceID, SourceImageURL: "https://example.com/src.jpg",
		Input: datatypes.JSON([]byte(`{"targetLanguage":"en"}`)),
	}
	retry.ID = retryID
	if err := svc.supersedePriorTranslateResults(context.Background(), retry, "en", retryID); err != nil {
		t.Fatalf("supersede before retry: %v", err)
	}
	var reloaded ImageTask
	if err := db.First(&reloaded, "id = ?", oldID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != StatusObsolete {
		t.Fatalf("status = %q, want obsolete", reloaded.Status)
	}
}
