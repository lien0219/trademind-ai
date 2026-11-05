package taskcenter

import (
	"container/heap"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/pkg/pagination"
	"gorm.io/gorm"
)

type sourceFetchResult struct {
	rows    []UnifiedTaskDTO
	cursor  TaskSourceCursor
	fetched int
	hasMore bool
}

type sourceBuffer struct {
	sourceID string
	priority int
	rows     []UnifiedTaskDTO
	idx      int
	cursor   TaskSourceCursor
	fetched  int
	hasMore  bool
}

type mergeHeapItem struct {
	dto      UnifiedTaskDTO
	sourceID string
	priority int
}

type mergeHeap []*mergeHeapItem

func (h mergeHeap) Len() int { return len(h) }

func (h mergeHeap) Less(i, j int) bool {
	return compareMergeItems(h[i], h[j]) > 0
}

func (h mergeHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *mergeHeap) Push(x any) {
	*h = append(*h, x.(*mergeHeapItem))
}

func (h *mergeHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func compareMergeItems(a, b *mergeHeapItem) int {
	if a == nil || b == nil {
		return 0
	}
	if a.dto.SortKey.After(b.dto.SortKey) {
		return 1
	}
	if a.dto.SortKey.Before(b.dto.SortKey) {
		return -1
	}
	if a.priority < b.priority {
		return 1
	}
	if a.priority > b.priority {
		return -1
	}
	ai := strings.TrimSpace(a.dto.ID)
	bi := strings.TrimSpace(b.dto.ID)
	if ai > bi {
		return 1
	}
	if ai < bi {
		return -1
	}
	return 0
}

func applySourceKeysetQuery(q *gorm.DB, cur TaskSourceCursor) (*gorm.DB, error) {
	if cur.Exhausted {
		return nil, nil
	}
	payload := applyTaskSourceKeyset(cur)
	if strings.TrimSpace(payload.SortValue) == "" {
		return q, nil
	}
	tie := strings.TrimSpace(payload.TieID)
	if tie == "" {
		return nil, pagination.ErrCursorSignatureInvalid
	}
	sortVal := strings.TrimSpace(payload.SortValue)
	if _, err := time.Parse(time.RFC3339Nano, sortVal); err == nil {
		// Use the original string with an explicit timestamptz cast so PostgreSQL keeps
		// microsecond precision; binding time.Time truncates to milliseconds in the driver.
		return q.Where("(updated_at < ?::timestamptz OR (updated_at = ?::timestamptz AND id < ?))", sortVal, sortVal, tie), nil
	}
	return q.Where("(updated_at < ? OR (updated_at = ? AND id < ?))", sortVal, sortVal, tie), nil
}

func (s *Service) listOneTypeKeyset(ctx context.Context, taskType string, p ListFailureParams, now time.Time, fetchLimit int, cur TaskSourceCursor) (sourceFetchResult, error) {
	var zero sourceFetchResult
	if cur.Exhausted {
		cur.SourceID = taskType
		return sourceFetchResult{cursor: cur}, nil
	}
	if fetchLimit < 1 {
		fetchLimit = 1
	}
	rows, err := s.listOneType(ctx, taskType, p, now, fetchLimit, cur)
	if err != nil {
		return zero, err
	}
	out := sourceFetchResult{
		rows:    rows,
		fetched: len(rows),
		hasMore: len(rows) >= fetchLimit,
		cursor:  TaskSourceCursor{SourceID: taskType, Exhausted: len(rows) < fetchLimit},
	}
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		out.cursor.SortTime = last.SortKey.UTC().Format(time.RFC3339Nano)
		out.cursor.ID = last.ID
	}
	return out, nil
}

func (s *Service) mergeTaskSources(ctx context.Context, p ListFailureParams, mergeCur *TaskMergeCursorPayload, limit int) ([]UnifiedTaskDTO, map[string]TaskSourceCursor, bool, error) {
	types := taskTypesFor(p)
	if len(types) == 0 {
		return nil, nil, false, fmt.Errorf("%w: invalid taskType", errTaskInvalidRequest)
	}
	if limit < 1 {
		limit = pagination.DefaultLimit
	}
	if limit > pagination.MaxLimit {
		limit = pagination.MaxLimit
	}
	fetchLimit := limit + 1
	now := time.Now().UTC()
	sourceCursors := initSourceCursors(types, mergeCur)
	buffers := make(map[string]*sourceBuffer, len(types))

	for _, tt := range types {
		cur := sourceCursors[tt]
		res, err := s.listOneTypeKeyset(ctx, tt, p, now, fetchLimit, cur)
		if err != nil {
			return nil, nil, false, err
		}
		sourceCursors[tt] = res.cursor
		if len(res.rows) == 0 {
			if res.cursor.Exhausted {
				sourceCursors[tt] = res.cursor
			}
			continue
		}
		buffers[tt] = &sourceBuffer{
			sourceID: tt,
			priority: sourcePriority(tt),
			rows:     res.rows,
			cursor:   res.cursor,
			fetched:  res.fetched,
			hasMore:  res.hasMore,
		}
	}

	h := make(mergeHeap, 0, len(buffers))
	heap.Init(&h)
	for src, buf := range buffers {
		if buf.idx < len(buf.rows) {
			heap.Push(&h, &mergeHeapItem{
				dto:      buf.rows[buf.idx],
				sourceID: src,
				priority: buf.priority,
			})
		}
	}

	seen := map[string]struct{}{}
	out := make([]UnifiedTaskDTO, 0, limit+1)
	consumed := map[string]int{}
	lastOut := map[string]UnifiedTaskDTO{}
	mergeHasMore := false

	for h.Len() > 0 {
		if len(out) >= limit {
			mergeHasMore = true
			break
		}
		item := heap.Pop(&h).(*mergeHeapItem)
		buf := buffers[item.sourceID]
		if buf == nil {
			continue
		}
		dedupeKey := item.dto.TaskType + "|" + item.dto.SourceID
		if _, dup := seen[dedupeKey]; !dup {
			seen[dedupeKey] = struct{}{}
			dto := item.dto
			applyClassification(&dto)
			if passesUnifiedFilters(dto, p) {
				out = append(out, dto)
				lastOut[item.sourceID] = dto
			}
		}
		consumed[item.sourceID]++
		buf.idx++
		if buf.idx < len(buf.rows) {
			heap.Push(&h, &mergeHeapItem{
				dto:      buf.rows[buf.idx],
				sourceID: item.sourceID,
				priority: buf.priority,
			})
		}
	}
	if !mergeHasMore {
		for _, buf := range buffers {
			if buf != nil && (buf.hasMore || buf.idx < len(buf.rows)) {
				mergeHasMore = true
				break
			}
		}
	}

	for src, dto := range lastOut {
		buf := buffers[src]
		exhausted := buf == nil || (!buf.hasMore && buf.idx >= len(buf.rows))
		sourceCursors[src] = TaskSourceCursor{
			SourceID:  src,
			SortTime:  dto.SortKey.UTC().Format(time.RFC3339Nano),
			ID:        dto.ID,
			Exhausted: exhausted,
		}
	}
	for src, n := range consumed {
		if _, ok := lastOut[src]; ok {
			continue
		}
		buf := buffers[src]
		if buf == nil || n == 0 {
			continue
		}
		idx := buf.idx - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(buf.rows) {
			idx = len(buf.rows) - 1
		}
		last := buf.rows[idx]
		cur := TaskSourceCursor{
			SourceID:  src,
			SortTime:  last.SortKey.UTC().Format(time.RFC3339Nano),
			ID:        last.ID,
			Exhausted: !buf.hasMore && buf.idx >= len(buf.rows),
		}
		sourceCursors[src] = cur
	}

	hasMore := mergeHasMore
	return out, sourceCursors, hasMore, nil
}
