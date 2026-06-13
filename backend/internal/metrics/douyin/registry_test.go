package douyin

import (
	"testing"
	"time"
)

func TestRecordAPIRequestSuccessAndFailure(t *testing.T) {
	r := NewRegistry()
	RecordAPIRequestFrom(r, "product.addV2", "production", 120*time.Millisecond, APIRequestOutcome{Success: true})
	RecordAPIRequestFrom(r, "product.addV2", "production", 80*time.Millisecond, APIRequestOutcome{
		ErrorCode:   "DOUYIN_RATE_LIMITED",
		Retryable:   true,
		RateLimited: true,
	})

	if got := r.sumCounter24h(MetricAPIRequestsTotal, nil); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
	if got := r.sumCounter24h(MetricAPISuccessTotal, nil); got != 1 {
		t.Fatalf("success = %d, want 1", got)
	}
	if got := r.sumCounter24h(MetricAPIFailedTotal, nil); got != 1 {
		t.Fatalf("failed = %d, want 1", got)
	}
	if got := r.sumCounter24h(MetricAPIRateLimitedTotal, nil); got != 1 {
		t.Fatalf("rate limited = %d, want 1", got)
	}
	sum := Summary24hFrom(r)
	if sum.APISuccessRate != 50 {
		t.Fatalf("success rate = %v, want 50", sum.APISuccessRate)
	}
	if sum.APIDurationAvgMs <= 0 {
		t.Fatalf("expected avg duration > 0")
	}
}

func TestRecordAPIRequestDoesNotPanicOnNilRegistry(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	RecordAPIRequest("token.refresh", "sandbox", time.Millisecond, APIRequestOutcome{ErrorCode: "UNKNOWN_DOUYIN_ERROR"})
}

func TestBusinessMetricsOncePerOutcome(t *testing.T) {
	r := NewRegistry()
	RecordProductDraftCreateFrom(r, true)
	RecordProductDraftCreateFrom(r, true)
	if got := r.sumCounter24h(MetricProductDraftCreateTotal, nil); got != 2 {
		t.Fatalf("draft create total = %d", got)
	}
	RecordOrderSyncOutcomeFrom(r, 10, 2, 3, true, 1, 4)
	sum := Summary24hFrom(r)
	if sum.OrderPartialSuccessTotal != 1 || sum.OrderFetchedTotal != 10 {
		t.Fatalf("unexpected order summary: %+v", sum)
	}
}

func TestSummary24hPrunesOldEvents(t *testing.T) {
	r := NewRegistry()
	r.mu.Lock()
	r.counterLog = append(r.counterLog, counterEvent{
		at: time.Now().UTC().Add(-25 * time.Hour), name: MetricAPIRequestsTotal, labels: "_|_|_|_|production", delta: 99,
	})
	r.mu.Unlock()
	r.incCounter(MetricAPIRequestsTotal, Labels{Environment: "production"}, 1)
	if got := r.sumCounter24h(MetricAPIRequestsTotal, nil); got != 1 {
		t.Fatalf("24h requests = %d, want 1", got)
	}
}

func RecordAPIRequestFrom(r *Registry, method, environment string, elapsed time.Duration, out APIRequestOutcome) {
	old := Default
	Default = r
	defer func() { Default = old }()
	RecordAPIRequest(method, environment, elapsed, out)
}

func RecordProductDraftCreateFrom(r *Registry, success bool) {
	old := Default
	Default = r
	defer func() { Default = old }()
	RecordProductDraftCreate(success)
}

func RecordOrderSyncOutcomeFrom(r *Registry, fetched, created, updated int, partialSuccess bool, unmatched, deducted int) {
	old := Default
	Default = r
	defer func() { Default = old }()
	RecordOrderSyncOutcome(fetched, created, updated, partialSuccess, unmatched, deducted)
}
