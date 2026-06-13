package douyin

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const window24h = 24 * time.Hour

// Labels are low-cardinality metric dimensions.
type Labels struct {
	Method      string
	Result      string
	ErrorCode   string
	Retryable   string
	Environment string
}

func (l Labels) key() string {
	return strings.Join([]string{
		sanitizeLabel(l.Method),
		sanitizeLabel(l.Result),
		sanitizeLabel(l.ErrorCode),
		sanitizeLabel(l.Retryable),
		sanitizeLabel(l.Environment),
	}, "|")
}

func sanitizeLabel(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return "_"
	}
	if len(v) > 64 {
		return v[:64]
	}
	return v
}

type counterEvent struct {
	at     time.Time
	name   string
	labels string
	delta  int64
}

type durationEvent struct {
	at       time.Time
	name     string
	labels   string
	duration time.Duration
}

// Registry stores in-memory counters and duration aggregates with a 24h rolling window.
type Registry struct {
	mu sync.RWMutex

	counters   map[string]int64
	durations  map[string]*durationAgg
	counterLog []counterEvent
	durLog     []durationEvent
}

type durationAgg struct {
	sumMs int64
	count int64
}

// Default is the process-wide Douyin metrics registry.
var Default = NewRegistry()

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		counters:  map[string]int64{},
		durations: map[string]*durationAgg{},
	}
}

func counterKey(name, labels string) string {
	return name + "{" + labels + "}"
}

func (r *Registry) incCounter(name string, labels Labels, delta int64) {
	if r == nil || delta == 0 {
		return
	}
	lk := labels.key()
	key := counterKey(name, lk)
	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[key] += delta
	r.counterLog = append(r.counterLog, counterEvent{at: now, name: name, labels: lk, delta: delta})
	r.pruneLocked(now)
}

func (r *Registry) observeDuration(name string, labels Labels, d time.Duration) {
	if r == nil || d < 0 {
		return
	}
	lk := labels.key()
	key := counterKey(name, lk)
	ms := d.Milliseconds()
	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()
	agg := r.durations[key]
	if agg == nil {
		agg = &durationAgg{}
		r.durations[key] = agg
	}
	agg.sumMs += ms
	agg.count++
	r.durLog = append(r.durLog, durationEvent{at: now, name: name, labels: lk, duration: d})
	r.pruneLocked(now)
}

func (r *Registry) pruneLocked(now time.Time) {
	cutoff := now.Add(-window24h)
	i := 0
	for _, e := range r.counterLog {
		if e.at.After(cutoff) {
			r.counterLog[i] = e
			i++
		}
	}
	r.counterLog = r.counterLog[:i]

	j := 0
	for _, e := range r.durLog {
		if e.at.After(cutoff) {
			r.durLog[j] = e
			j++
		}
	}
	r.durLog = r.durLog[:j]
}

// CounterValue returns the current counter value (all time, not windowed).
func (r *Registry) CounterValue(name string, labels Labels) int64 {
	if r == nil {
		return 0
	}
	key := counterKey(name, labels.key())
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.counters[key]
}

// DurationStats returns sum ms and count for a duration metric.
func (r *Registry) DurationStats(name string, labels Labels) (sumMs int64, count int64) {
	if r == nil {
		return 0, 0
	}
	key := counterKey(name, labels.key())
	r.mu.RLock()
	defer r.mu.RUnlock()
	if agg := r.durations[key]; agg != nil {
		return agg.sumMs, agg.count
	}
	return 0, 0
}

func (r *Registry) sumCounter24h(name string, match func(Labels) bool) int64 {
	if r == nil {
		return 0
	}
	cutoff := time.Now().UTC().Add(-window24h)
	r.mu.RLock()
	defer r.mu.RUnlock()
	var total int64
	for _, e := range r.counterLog {
		if e.name != name || !e.at.After(cutoff) {
			continue
		}
		if match != nil && !match(parseLabelKey(e.labels)) {
			continue
		}
		total += e.delta
	}
	return total
}

func (r *Registry) durationStats24h(name string, match func(Labels) bool) (sumMs int64, count int64) {
	if r == nil {
		return 0, 0
	}
	cutoff := time.Now().UTC().Add(-window24h)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, e := range r.durLog {
		if e.name != name || !e.at.After(cutoff) {
			continue
		}
		if match != nil && !match(parseLabelKey(e.labels)) {
			continue
		}
		sumMs += e.duration.Milliseconds()
		count++
	}
	return sumMs, count
}

func parseLabelKey(raw string) Labels {
	parts := strings.Split(raw, "|")
	for len(parts) < 5 {
		parts = append(parts, "_")
	}
	return Labels{
		Method:      unblank(parts[0]),
		Result:      unblank(parts[1]),
		ErrorCode:   unblank(parts[2]),
		Retryable:   unblank(parts[3]),
		Environment: unblank(parts[4]),
	}
}

func unblank(v string) string {
	if v == "_" {
		return ""
	}
	return v
}

// SafeRecord runs fn and ignores panics so metrics never block business logic.
func SafeRecord(fn func()) {
	defer func() { _ = recover() }()
	if fn != nil {
		fn()
	}
}

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func envLabel(environment string) string {
	e := strings.TrimSpace(strings.ToLower(environment))
	if e == "" {
		return "production"
	}
	return e
}

func resultLabel(success bool) string {
	if success {
		return "success"
	}
	return "failed"
}

func errorCodeLabel(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "_"
	}
	return sanitizeLabel(code)
}

// MetricNames exported for health/summary consumers.
const (
	MetricAPIRequestsTotal            = "douyin_api_requests_total"
	MetricAPISuccessTotal             = "douyin_api_success_total"
	MetricAPIFailedTotal              = "douyin_api_failed_total"
	MetricAPIDurationMs               = "douyin_api_duration_ms"
	MetricAPITimeoutTotal             = "douyin_api_timeout_total"
	MetricAPIRateLimitedTotal         = "douyin_api_rate_limited_total"
	MetricAPIRetryTotal               = "douyin_api_retry_total"
	MetricTokenRefreshTotal           = "douyin_token_refresh_total"
	MetricTokenRefreshFailedTotal     = "douyin_token_refresh_failed_total"
	MetricRuntimeBlockedTasksTotal    = "douyin_runtime_blocked_tasks_total"
	MetricStaleTasksTotal             = "douyin_stale_tasks_total"
	MetricRecoverySuccessTotal        = "douyin_recovery_success_total"
	MetricRecoveryFailedTotal         = "douyin_recovery_failed_total"
	MetricProductDraftCreateTotal     = "douyin_product_draft_create_total"
	MetricProductDraftCreateFailed    = "douyin_product_draft_create_failed_total"
	MetricImageUploadTotal            = "douyin_image_upload_total"
	MetricImageUploadFailedTotal      = "douyin_image_upload_failed_total"
	MetricSKUAutoBoundTotal           = "douyin_sku_auto_bound_total"
	MetricSKUManualBoundTotal         = "douyin_sku_manual_bound_total"
	MetricSKUUnmatchedTotal           = "douyin_sku_unmatched_total"
	MetricSKUAmbiguousTotal           = "douyin_sku_ambiguous_total"
	MetricOrderFetchedTotal           = "douyin_order_fetched_total"
	MetricOrderCreatedTotal           = "douyin_order_created_total"
	MetricOrderUpdatedTotal           = "douyin_order_updated_total"
	MetricOrderPartialSuccessTotal    = "douyin_order_partial_success_total"
	MetricOrderUnmatchedItemsTotal    = "douyin_order_unmatched_items_total"
	MetricOrderInventoryDeductedTotal = "douyin_order_inventory_deducted_total"
	MetricInventorySyncTotal          = "douyin_inventory_sync_total"
	MetricInventorySyncSuccessTotal   = "douyin_inventory_sync_success_total"
	MetricInventorySyncFailedTotal    = "douyin_inventory_sync_failed_total"
	MetricInventorySyncSkippedTotal   = "douyin_inventory_sync_skipped_total"
	MetricFailureTasksPending         = "douyin_failure_tasks_pending"
	MetricAuthorizationsExpiring      = "douyin_authorizations_expiring"
)

func simpleCounter(name string, delta int64) {
	Default.incCounter(name, Labels{}, delta)
}

func formatAvg(sumMs, count int64) float64 {
	if count <= 0 {
		return 0
	}
	return float64(sumMs) / float64(count)
}

func fmtPct(num, den int64) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) * 100 / float64(den)
}

func counterSnapshot(name string, v int64) map[string]any {
	return map[string]any{"name": name, "value": v}
}

func counterPair(name string, v int64) string {
	return fmt.Sprintf("%s=%d", name, v)
}
