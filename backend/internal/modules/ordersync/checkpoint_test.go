package ordersync

import (
	"encoding/json"
	"testing"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
)

func TestFailedPageNumbersFromOutput(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"pageErrors": []platformp.PageSyncError{{Page: 1, Error: "rate limited"}, {Page: 3, Error: "timeout"}},
	})
	pages := failedPageNumbers(pageErrorsFromOutput(datatypes.JSON(raw)))
	if len(pages) != 2 || pages[0] != 1 || pages[1] != 3 {
		t.Fatalf("unexpected pages: %v", pages)
	}
}

func TestMergeSyncOutputAccumulatesCounts(t *testing.T) {
	prev, _ := json.Marshal(map[string]any{
		"totalFetched":  10,
		"successPages":  1,
		"failedPages":   1,
		"pageErrors":    []platformp.PageSyncError{{Page: 1, Error: "err"}},
		"createdOrders": 5,
	})
	cur := map[string]any{
		"totalFetched":   8,
		"successPages":   1,
		"failedPages":    0,
		"pageErrors":     []platformp.PageSyncError{},
		"createdOrders":  3,
		"hasMore":        false,
		"retryPagesOnly": []int{1},
	}
	merged := mergeSyncOutput(datatypes.JSON(prev), cur)
	var m map[string]any
	_ = json.Unmarshal(merged, &m)
	if intFromAny(m["totalFetched"]) != 18 {
		t.Fatalf("expected totalFetched=18, got %v", m["totalFetched"])
	}
	if intFromAny(m["failedPages"]) != 0 {
		t.Fatalf("expected failedPages=0 after successful retry page, got %v", m["failedPages"])
	}
}

func TestResolveFinalSyncStatusHasMore(t *testing.T) {
	res := &platformp.SyncOrdersResult{HasMore: true}
	if resolveFinalSyncStatus(res, 0) != StatusPartialSuccess {
		t.Fatal("expected partial_success when hasMore")
	}
}

func TestBuildRetrySnapshotFromPartialTask(t *testing.T) {
	out, _ := json.Marshal(map[string]any{
		"pageErrors": []platformp.PageSyncError{{Page: 2, Error: "fail"}},
	})
	in, _ := json.Marshal(syncInputSnapshot{Mode: ModeIncremental, Limit: 50})
	task := OrderSyncTask{Status: StatusPartialSuccess, Input: datatypes.JSON(in), Output: datatypes.JSON(out)}
	snap, retryIn, ok, err := buildRetrySnapshotFromTask(&task)
	if err != nil || !ok {
		t.Fatalf("expected partial retry snapshot, err=%v ok=%v", err, ok)
	}
	if len(snap.RetryPagesOnly) != 1 || snap.RetryPagesOnly[0] != 2 {
		t.Fatalf("unexpected retry pages: %v", snap.RetryPagesOnly)
	}
	var parsed syncInputSnapshot
	_ = json.Unmarshal(retryIn, &parsed)
	if len(parsed.RetryPagesOnly) != 1 {
		t.Fatalf("retry input missing pages: %+v", parsed)
	}
}
