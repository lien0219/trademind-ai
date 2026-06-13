package ordersync

import (
	"encoding/json"
	"fmt"
	"strings"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
)

func pageErrorsFromOutput(raw datatypes.JSON) []platformp.PageSyncError {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	rawPE, ok := m["pageErrors"]
	if !ok || rawPE == nil {
		return nil
	}
	b, err := json.Marshal(rawPE)
	if err != nil {
		return nil
	}
	var out []platformp.PageSyncError
	_ = json.Unmarshal(b, &out)
	return out
}

func failedPageNumbers(pageErrors []platformp.PageSyncError) []int {
	if len(pageErrors) == 0 {
		return nil
	}
	out := make([]int, 0, len(pageErrors))
	seen := map[int]struct{}{}
	for _, pe := range pageErrors {
		if pe.Page < 0 {
			continue
		}
		if _, ok := seen[pe.Page]; ok {
			continue
		}
		seen[pe.Page] = struct{}{}
		out = append(out, pe.Page)
	}
	return out
}

func mergeSyncOutput(previous datatypes.JSON, current map[string]any) datatypes.JSON {
	prev := map[string]any{}
	if len(previous) > 0 {
		_ = json.Unmarshal(previous, &prev)
	}
	merged := map[string]any{}
	for k, v := range prev {
		merged[k] = v
	}
	for k, v := range current {
		switch k {
		case "totalFetched", "createdOrders", "updatedOrders", "matchedItems", "unmatchedItems", "deductedStockItems", "receivedOrders", "upsertSuccess", "successPages":
			merged[k] = addNumeric(prev[k], v)
		case "pageErrors":
			// handled below with retryPagesOnly
		case "failedPages":
			// recomputed after merge from pageErrors
		case "retryPagesOnly":
			merged[k] = v
		default:
			merged[k] = v
		}
	}
	if rp, ok := current["retryPagesOnly"]; ok {
		merged["pageErrors"] = mergePageErrorsAfterRetry(prev["pageErrors"], current["pageErrors"], rp)
	} else {
		merged["pageErrors"] = mergePageErrors(prev["pageErrors"], current["pageErrors"])
	}
	if pe, ok := merged["pageErrors"]; ok && pe != nil {
		merged["failedPages"] = len(pageErrorsSlice(pe))
	} else if v, ok := current["failedPages"]; ok && prev["pageErrors"] == nil {
		merged["failedPages"] = intFromAny(v)
	}
	merged["retryMerged"] = true
	b, _ := json.Marshal(merged)
	return datatypes.JSON(b)
}

func mergePageErrors(a, b any) []platformp.PageSyncError {
	var left, right []platformp.PageSyncError
	if a != nil {
		ba, _ := json.Marshal(a)
		_ = json.Unmarshal(ba, &left)
	}
	if b != nil {
		bb, _ := json.Marshal(b)
		_ = json.Unmarshal(bb, &right)
	}
	byPage := map[int]platformp.PageSyncError{}
	for _, pe := range left {
		byPage[pe.Page] = pe
	}
	for _, pe := range right {
		if strings.TrimSpace(pe.Error) == "" {
			delete(byPage, pe.Page)
			continue
		}
		byPage[pe.Page] = pe
	}
	out := make([]platformp.PageSyncError, 0, len(byPage))
	for _, pe := range byPage {
		out = append(out, pe)
	}
	return out
}

func mergePageErrorsAfterRetry(prev, cur, retryPages any) []platformp.PageSyncError {
	byPage := map[int]platformp.PageSyncError{}
	for _, pe := range pageErrorsSlice(prev) {
		byPage[pe.Page] = pe
	}
	for _, p := range intSliceFromAny(retryPages) {
		delete(byPage, p)
	}
	for _, pe := range pageErrorsSlice(cur) {
		if strings.TrimSpace(pe.Error) == "" {
			delete(byPage, pe.Page)
			continue
		}
		byPage[pe.Page] = pe
	}
	out := make([]platformp.PageSyncError, 0, len(byPage))
	for _, pe := range byPage {
		out = append(out, pe)
	}
	return out
}

func intSliceFromAny(v any) []int {
	switch x := v.(type) {
	case []int:
		return x
	case []any:
		out := make([]int, 0, len(x))
		for _, item := range x {
			out = append(out, intFromAny(item))
		}
		return out
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var out []int
		_ = json.Unmarshal(b, &out)
		return out
	}
}

func pageErrorsSlice(v any) []platformp.PageSyncError {
	var out []platformp.PageSyncError
	if v == nil {
		return out
	}
	b, err := json.Marshal(v)
	if err != nil {
		return out
	}
	_ = json.Unmarshal(b, &out)
	return out
}

func addNumeric(a, b any) int {
	return intFromAny(a) + intFromAny(b)
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	default:
		return 0
	}
}

func resolveFinalSyncStatus(res *platformp.SyncOrdersResult, upsertFailed int) string {
	if res == nil {
		return StatusFailed
	}
	if res.FailedPages > 0 || upsertFailed > 0 {
		return StatusPartialSuccess
	}
	if res.HasMore {
		return StatusPartialSuccess
	}
	if len(res.PageErrors) > 0 {
		return StatusPartialSuccess
	}
	return StatusSuccess
}

func buildRetrySnapshotFromTask(task *OrderSyncTask) (syncInputSnapshot, datatypes.JSON, bool, error) {
	snap := snapFromJSON(task.Input)
	if task == nil {
		return snap, nil, false, fmt.Errorf("task nil")
	}
	st := strings.TrimSpace(task.Status)
	if st != StatusPartialSuccess && st != StatusFailed {
		return snap, task.Input, false, nil
	}
	pages := failedPageNumbers(pageErrorsFromOutput(task.Output))
	if len(pages) == 0 {
		return snap, task.Input, false, nil
	}
	snap.RetryPagesOnly = pages
	b, err := json.Marshal(snap)
	if err != nil {
		return snap, nil, false, err
	}
	return snap, datatypes.JSON(b), true, nil
}
