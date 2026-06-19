package product

import (
	"context"
	"encoding/json"
	"fmt"
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
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(4)
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

func TestAIContentApplyRejectsFailedTaskAndStaleSnapshot(t *testing.T) {
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
	successTask := aitask.AITask{
		TaskType:  "product_title_optimize",
		Status:    aitask.StatusSuccess,
		ProductID: &p.ID,
		Input:     datatypes.JSON(mustJSON(t, map[string]any{"sourceSnapshotHash": productContentHash(productPromptTitle(&p))})),
	}
	failedTask := aitask.AITask{
		TaskType:  "product_title_optimize",
		Status:    aitask.StatusFailed,
		ProductID: &p.ID,
		Input:     datatypes.JSON(mustJSON(t, map[string]any{"sourceSnapshotHash": productContentHash(productPromptTitle(&p))})),
	}
	if err := db.Create(&successTask).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&failedTask).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: db, AITasks: &aitask.Service{DB: db}}
	c := testGinContext()

	if _, err := svc.ApplyAITitle(c, p.ID, ApplyAITitleBody{
		AITitle:           "should not apply from failed task",
		TaskID:            failedTask.ID.String(),
		ExpectedUpdatedAt: p.UpdatedAt.Format(time.RFC3339Nano),
	}, nil); err == nil || !strings.Contains(strings.ToLower(err.Error()), "not ready") {
		t.Fatalf("expected failed task to be rejected, got %v", err)
	}

	stale := p
	if err := db.Model(&Product{}).Where("id = ?", p.ID).Updates(map[string]any{
		"title":      "Portable coffee grinder manual update",
		"updated_at": p.UpdatedAt.Add(2 * time.Second),
	}).Error; err != nil {
		t.Fatal(err)
	}
	err := svc.applyAIContent(c, &stale, AIContentFieldTitle, "AI title after stale page", successTask.ID, stale.UpdatedAt.Format(time.RFC3339Nano), "", nil)
	if err == nil || !strings.Contains(err.Error(), "content conflict") {
		t.Fatalf("expected stale snapshot conflict, got %v", err)
	}
}

func TestListOperationStepFiltersReuseEffectiveTitleDescriptionAndImageRules(t *testing.T) {
	db := newOperationProgressTestDB(t)
	c := testGinContext()
	svc := &Service{DB: db}
	price := 19.9

	createProduct := func(title, originalTitle, description, aiDescription string) Product {
		p := Product{
			Source:        "manual",
			Title:         title,
			OriginalTitle: originalTitle,
			Description:   description,
			AIDescription: aiDescription,
			Currency:      "USD",
			Status:        StatusDraft,
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		return p
	}

	titleOK := createProduct("", "Portable coffee grinder", "desc", "")
	descOK := createProduct("Title ok", "", "", "Long AI description that should satisfy the operation progress filter.")
	imageOK := createProduct("Image ok", "", "Long description with enough words for the filter to pass.", "")

	if err := db.Create(&ProductImage{ProductID: imageOK.ID, ImageType: ImageTypeDetail, PublicURL: "https://example.com/detail.jpg"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductSKU{ProductID: titleOK.ID, SKUName: "Default", Price: &price}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductSKU{ProductID: descOK.ID, SKUName: "Default", Price: &price}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductSKU{ProductID: imageOK.ID, SKUName: "Default", Price: &price}).Error; err != nil {
		t.Fatal(err)
	}

	if err := db.Create(&ProductImage{ProductID: titleOK.ID, ImageType: ImageTypeMain, PublicURL: "https://example.com/main.jpg"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ProductImage{ProductID: descOK.ID, ImageType: ImageTypeMain, PublicURL: "https://example.com/main2.jpg"}).Error; err != nil {
		t.Fatal(err)
	}

	res, err := svc.List(c, ListQuery{Page: 1, PageSize: 50, OperationStep: string(OperationStepTitle)})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range res.Items {
		if item.ID == titleOK.ID {
			t.Fatalf("product with valid original title should not remain in title step filter")
		}
	}

	res, err = svc.List(c, ListQuery{Page: 1, PageSize: 50, OperationStep: string(OperationStepDescription)})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range res.Items {
		if item.ID == descOK.ID {
			t.Fatalf("product with valid AI description should not remain in description step filter")
		}
	}

	res, err = svc.List(c, ListQuery{Page: 1, PageSize: 50, OperationStep: string(OperationStepImages)})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range res.Items {
		if item.ID == imageOK.ID {
			t.Fatalf("product with usable non-SKU image should not remain in images step filter")
		}
	}
}

func TestListAttachOperationProgressUsesFixedBatchQueries(t *testing.T) {
	db := newOperationProgressTestDB(t)
	c := testGinContext()
	svc := &Service{DB: db}
	price := 12.5

	for i := 0; i < 25; i++ {
		p := Product{
			Source:      "manual",
			Title:       fmt.Sprintf("Batch progress product %d", i),
			Description: "Enough description text for list operation progress summary attachment.",
			Currency:    "CNY",
			Status:      StatusDraft,
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&ProductImage{ProductID: p.ID, ImageType: ImageTypeMain, PublicURL: fmt.Sprintf("https://example.com/%d.jpg", i)}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&ProductSKU{ProductID: p.ID, SKUName: "Default", Price: &price}).Error; err != nil {
			t.Fatal(err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)

	var queryCount int
	db.Callback().Query().Before("gorm:query").Register("count_queries", func(tx *gorm.DB) {
		queryCount++
	})

	res, err := svc.List(c, ListQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 25 {
		t.Fatalf("expected 25 items, got %d", len(res.Items))
	}
	for _, item := range res.Items {
		if item.OperationProgress == nil {
			t.Fatalf("expected operation progress summary for %s", item.ID)
		}
	}
	// products + images + skus + image_tasks (batch, not per-row)
	if queryCount > 8 {
		t.Fatalf("expected bounded batch queries, got %d SQL queries for 25-row list", queryCount)
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
