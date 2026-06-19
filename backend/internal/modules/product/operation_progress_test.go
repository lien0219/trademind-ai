package product

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func newOperationProgressTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&Product{},
		&ProductImage{},
		&ProductSKU{},
		&ProductAIContentApplication{},
		&operationImageTaskProbe{},
		&aitask.AITask{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOperationProgressCalculatesReadyFromContent(t *testing.T) {
	db := newOperationProgressTestDB(t)
	price := 19.9
	p := Product{
		Source:      "manual",
		Title:       "Portable coffee grinder",
		Description: "A compact hand coffee grinder with stainless steel burrs for travel and home brewing.",
		Currency:    "USD",
		Status:      StatusDraft,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductImage{ProductID: p.ID, ImageType: ImageTypeMain, PublicURL: "https://example.com/main.jpg"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductSKU{ProductID: p.ID, SKUName: "Default", Price: &price, Attrs: datatypes.JSON([]byte(`{"color":"black"}`))}).Error; err != nil {
		t.Fatal(err)
	}

	readinessCalls := 0
	svc := &Service{DB: db, Readiness: func(context.Context, OperationReadinessRequest) (*OperationReadinessResult, error) {
		readinessCalls++
		return &OperationReadinessResult{Status: "passed", Result: "passed", CanPublish: true}, nil
	}}

	progress, err := svc.GetOperationProgress(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if readinessCalls != 1 {
		t.Fatalf("expected one readonly readiness call, got %d", readinessCalls)
	}
	if !progress.PublishReady {
		t.Fatalf("expected publish ready, got %#v", progress)
	}
	if progress.CurrentStep != OperationStepReady {
		t.Fatalf("expected ready step, got %s", progress.CurrentStep)
	}
	if progress.CompletionPercent != 100 {
		t.Fatalf("expected 100%% completion, got %d", progress.CompletionPercent)
	}
}

func TestOperationProgressPrioritizesFailedIssues(t *testing.T) {
	db := newOperationProgressTestDB(t)
	p := Product{
		Source:      "manual",
		Title:       "Portable coffee grinder",
		Description: "A compact hand coffee grinder with stainless steel burrs for travel and home brewing.",
		Currency:    "USD",
		Status:      StatusDraft,
		RawData:     datatypes.JSON([]byte(`{"raw":{"qualityWarnings":["source page had missing details"]}}`)),
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db}
	progress, err := svc.GetOperationProgress(context.Background(), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if progress.CurrentStep != OperationStepCollectReview {
		t.Fatalf("expected collect review current step, got %s", progress.CurrentStep)
	}
	if len(progress.Blockers) == 0 || progress.Blockers[0].Severity != "failed" {
		t.Fatalf("expected failed blocker first, got %#v", progress.Blockers)
	}
	if len(progress.Warnings) == 0 {
		t.Fatalf("expected collect warning to be preserved")
	}
}

func TestAIContentApplyRejectsConflictAndUndoProtectsManualChange(t *testing.T) {
	db := newOperationProgressTestDB(t)
	p := Product{
		Source:      "manual",
		Title:       "Portable coffee grinder",
		Description: "A compact hand coffee grinder with stainless steel burrs for travel and home brewing.",
		Currency:    "USD",
		Status:      StatusDraft,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	task := aitask.AITask{
		TaskType:  "product_title_optimize",
		Status:    aitask.StatusSuccess,
		ProductID: &p.ID,
		Input:     datatypes.JSON(mustJSON(t, map[string]any{"sourceSnapshotHash": productContentHash(productPromptTitle(&p))})),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db, AITasks: &aitask.Service{DB: db}}
	c := testGinContext()

	_, err := svc.ApplyAITitle(c, p.ID, ApplyAITitleBody{
		AITitle:           "AI portable coffee grinder title",
		TaskID:            task.ID.String(),
		ExpectedUpdatedAt: p.UpdatedAt.Add(-2 * time.Second).Format(time.RFC3339Nano),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "content conflict") {
		t.Fatalf("expected stale apply conflict, got %v", err)
	}

	detail, err := svc.ApplyAITitle(c, p.ID, ApplyAITitleBody{
		AITitle:           "AI portable coffee grinder title",
		TaskID:            task.ID.String(),
		ExpectedUpdatedAt: p.UpdatedAt.Format(time.RFC3339Nano),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if detail.AITitle != "AI portable coffee grinder title" {
		t.Fatalf("expected AI title applied, got %q", detail.AITitle)
	}
	if err := db.Model(&Product{}).Where("id = ?", p.ID).Update("ai_title", "manual edit after apply").Error; err != nil {
		t.Fatal(err)
	}
	_, err = svc.UndoAIContent(c, p.ID, AIContentFieldTitle, UndoAIContentBody{}, nil)
	if err == nil || !strings.Contains(err.Error(), "content conflict") {
		t.Fatalf("expected undo conflict after manual edit, got %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
