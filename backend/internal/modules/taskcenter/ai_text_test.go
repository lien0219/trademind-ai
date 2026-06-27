package taskcenter

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/aiproducttext"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

func TestAITextFailureCategory(t *testing.T) {
	cases := []struct {
		status string
		code   string
		want   string
	}{
		{aiproducttext.ItemFailed, "", CategoryAITextGenerationFailed},
		{aiproducttext.ItemConflict, "", CategoryAITextApplyConflict},
		{aiproducttext.ItemFailed, "apply_failed", CategoryAITextApplyFailed},
		{aiproducttext.ItemFailed, "undo_failed", CategoryAITextUndoFailed},
	}
	for _, tc := range cases {
		item := &aiproducttext.AIProductTextItem{Status: tc.status, ErrorCode: tc.code}
		if got := aiTextFailureCategory(item); got != tc.want {
			t.Fatalf("status=%s code=%s: got %q want %q", tc.status, tc.code, got, tc.want)
		}
	}
}

func TestAITextFailureCategoryQualityWarning(t *testing.T) {
	item := &aiproducttext.AIProductTextItem{
		Status:          aiproducttext.ItemPendingReview,
		QualityWarnings: datatypes.JSON(`[{"code":"title_too_short","message":"标题可能过短"}]`),
	}
	if got := aiTextFailureCategory(item); got != CategoryAITextQualityWarning {
		t.Fatalf("got %q want %q", got, CategoryAITextQualityWarning)
	}
}

func TestAITextDetailURLIncludesItemId(t *testing.T) {
	batchID := uuid.New().String()
	itemID := uuid.New().String()
	url := aiTextDetailURL(batchID, itemID)
	if url == "" || !strings.Contains(url, batchID) || !strings.Contains(url, "itemId=") || !strings.Contains(url, itemID) {
		t.Fatalf("unexpected detail url: %q", url)
	}
}

func TestMapAIProductTextItemConflictMessage(t *testing.T) {
	batchID := uuid.New()
	productID := uuid.New()
	itemID := uuid.New()
	row := &aiproducttext.AIProductTextItem{
		Base:          model.Base{ID: itemID},
		BatchID:       batchID,
		ProductID:     productID,
		OperationType: aiproducttext.OpTitle,
		Status:        aiproducttext.ItemConflict,
	}
	dto := mapAIProductTextItem(row, map[uuid.UUID]string{productID: "测试商品"}, markSet{}, time.Now().UTC())
	if dto.FailureCategory != CategoryAITextApplyConflict {
		t.Fatalf("category: %s", dto.FailureCategory)
	}
	if dto.ErrorMessage != aiproducttext.ConflictUserMessage {
		t.Fatalf("message: %q", dto.ErrorMessage)
	}
	if dto.DetailURL == "" {
		t.Fatal("expected detail url")
	}
	if dto.Retryable {
		t.Fatal("conflict should not be retryable")
	}
}
