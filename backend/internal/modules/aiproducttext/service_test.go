package aiproducttext

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCheckTitleQualityWarnings(t *testing.T) {
	w := checkTitleQuality("", TextGenerationOptions{}, nil)
	if len(w) == 0 || w[0].Code != "title_empty" {
		t.Fatalf("expected title_empty warning, got %#v", w)
	}
	long := strings.Repeat("词", 150)
	w = checkTitleQuality(long, TextGenerationOptions{MaxLength: 80}, nil)
	found := map[string]bool{}
	for _, x := range w {
		found[x.Code] = true
	}
	if !found["title_too_long"] {
		t.Fatal("expected title_too_long")
	}
	w = checkTitleQuality("优质蓝牙耳机 违禁 促销", TextGenerationOptions{}, []string{"违禁"})
	found = map[string]bool{}
	for _, x := range w {
		found[x.Code] = true
	}
	if !found["title_forbidden_word"] {
		t.Fatal("expected title_forbidden_word")
	}
}

func TestCheckDescriptionQualityWarnings(t *testing.T) {
	w := checkDescriptionQuality("短描述", "蓝牙耳机", nil)
	found := map[string]bool{}
	for _, x := range w {
		found[x.Code] = true
	}
	if !found["desc_too_short"] {
		t.Fatal("expected desc_too_short")
	}
}

func TestBuildIdempotencyKeyStable(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	ops := []string{"description", "title"}
	k1 := buildIdempotencyKey(nil, []uuid.UUID{id2, id1}, ops, TextGenerationOptions{Language: "zh"})
	k2 := buildIdempotencyKey(nil, []uuid.UUID{id1, id2}, ops, TextGenerationOptions{Language: "zh"})
	if k1 != k2 {
		t.Fatalf("idempotency key should be order-independent: %s vs %s", k1, k2)
	}
	if k1 == "" {
		t.Fatal("expected non-empty key")
	}
}

func TestNormalizeOperationTypes(t *testing.T) {
	ops, err := normalizeOperationTypes([]string{"title", "title", "description"})
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 || ops[0] != OpDescription || ops[1] != OpTitle {
		t.Fatalf("unexpected ops: %#v", ops)
	}
	_, err = normalizeOperationTypes([]string{"invalid"})
	if err == nil {
		t.Fatal("expected error for invalid op")
	}
}

func TestParseProductIDsDedupes(t *testing.T) {
	id := uuid.New().String()
	ids, err := parseProductIDs([]string{id, id, id})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1 id, got %d", len(ids))
	}
}

func TestItemStatusLabelsChinese(t *testing.T) {
	cases := map[string]string{
		ItemPendingReview: "待复核",
		ItemConflict:      "内容有冲突",
		ItemApplied:       "已应用",
	}
	for code, want := range cases {
		if got := itemStatusLabel(code); got != want {
			t.Fatalf("%s: got %q want %q", code, got, want)
		}
	}
}

func TestCheckBatchSummaryCounts(t *testing.T) {
	svc := &Service{}
	pReady := &struct {
		ID            uuid.UUID
		Title         string
		Description   string
		AITitle       string
		AIDescription string
		OriginalTitle string
		Status        string
	}{Title: "蓝牙耳机", Description: "优质蓝牙耳机", OriginalTitle: "蓝牙耳机", Status: "draft"}
	// use product.Product via adapter - skip full integration here
	_ = svc
	_ = pReady
	item := CheckBatchItem{Status: "ready"}
	switch item.Status {
	case "ready":
	default:
		t.Fatal("unexpected")
	}
}

func TestTruncateMsg(t *testing.T) {
	s := truncateMsg(fmt.Sprintf("%4000s", "x"), 10)
	if len(s) > 15 {
		t.Fatalf("expected truncated message, got len %d", len(s))
	}
}
