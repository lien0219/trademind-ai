package douyin

import (
	"strings"
	"time"
)

const codeDouyinRequestTimeout = "DOUYIN_REQUEST_TIMEOUT"

// APIRequestOutcome carries low-cardinality fields for API metrics (no import of douyinshop).
type APIRequestOutcome struct {
	Success     bool
	ErrorCode   string
	Retryable   bool
	RateLimited bool
	Timeout     bool
}

// RecordAPIRequest records one Douyin OpenAPI call outcome.
func RecordAPIRequest(method, environment string, elapsed time.Duration, out APIRequestOutcome) {
	SafeRecord(func() {
		env := envLabel(environment)
		method = strings.TrimSpace(method)
		labels := Labels{Method: method, Environment: env}
		Default.incCounter(MetricAPIRequestsTotal, labels, 1)

		if out.Success {
			ok := Labels{Method: method, Result: "success", ErrorCode: "_", Retryable: "false", Environment: env}
			Default.incCounter(MetricAPISuccessTotal, ok, 1)
			Default.observeDuration(MetricAPIDurationMs, ok, elapsed)
			return
		}

		code := strings.TrimSpace(out.ErrorCode)
		if code == "" {
			code = "UNKNOWN_DOUYIN_ERROR"
		}
		if out.RateLimited {
			Default.incCounter(MetricAPIRateLimitedTotal, Labels{Method: method, Environment: env}, 1)
		}
		if out.Timeout || strings.EqualFold(code, codeDouyinRequestTimeout) {
			Default.incCounter(MetricAPITimeoutTotal, Labels{Method: method, Environment: env}, 1)
		}
		fail := Labels{
			Method:      method,
			Result:      "failed",
			ErrorCode:   errorCodeLabel(code),
			Retryable:   boolLabel(out.Retryable),
			Environment: env,
		}
		Default.incCounter(MetricAPIFailedTotal, fail, 1)
		Default.observeDuration(MetricAPIDurationMs, fail, elapsed)
	})
}

// RecordAPIRetry increments retry counter for a method.
func RecordAPIRetry(method, environment string) {
	SafeRecord(func() {
		Default.incCounter(MetricAPIRetryTotal, Labels{Method: strings.TrimSpace(method), Environment: envLabel(environment)}, 1)
	})
}

// RecordTokenRefresh records token refresh outcome.
func RecordTokenRefresh(environment string, err error) {
	SafeRecord(func() {
		env := envLabel(environment)
		Default.incCounter(MetricTokenRefreshTotal, Labels{Environment: env}, 1)
		if err != nil {
			Default.incCounter(MetricTokenRefreshFailedTotal, Labels{Environment: env}, 1)
		}
	})
}

// RecordRuntimeBlockedTask records a worker blocked by runtime/gray guard.
func RecordRuntimeBlockedTask() {
	SafeRecord(func() { simpleCounter(MetricRuntimeBlockedTasksTotal, 1) })
}

// RecordStaleTask records a stale/result_unknown task mark.
func RecordStaleTask() {
	SafeRecord(func() { simpleCounter(MetricStaleTasksTotal, 1) })
}

// RecordRecoverySuccess records successful stale/recovery operation.
func RecordRecoverySuccess() {
	SafeRecord(func() { simpleCounter(MetricRecoverySuccessTotal, 1) })
}

// RecordRecoveryFailed records failed recovery attempt.
func RecordRecoveryFailed() {
	SafeRecord(func() { simpleCounter(MetricRecoveryFailedTotal, 1) })
}

// RecordProductDraftCreate records product draft create outcome (once per business outcome).
func RecordProductDraftCreate(success bool) {
	SafeRecord(func() {
		simpleCounter(MetricProductDraftCreateTotal, 1)
		if !success {
			simpleCounter(MetricProductDraftCreateFailed, 1)
		}
	})
}

// RecordImageUpload records image upload outcome.
func RecordImageUpload(success bool) {
	SafeRecord(func() {
		simpleCounter(MetricImageUploadTotal, 1)
		if !success {
			simpleCounter(MetricImageUploadFailedTotal, 1)
		}
	})
}

// RecordSKUAutoBound records auto-bound SKU count.
func RecordSKUAutoBound(n int) {
	SafeRecord(func() {
		if n > 0 {
			simpleCounter(MetricSKUAutoBoundTotal, int64(n))
		}
	})
}

// RecordSKUManualBound records manual SKU bind.
func RecordSKUManualBound() {
	SafeRecord(func() { simpleCounter(MetricSKUManualBoundTotal, 1) })
}

// RecordSKUUnmatched records unmatched SKU count from binding sync.
func RecordSKUUnmatched(n int) {
	SafeRecord(func() {
		if n > 0 {
			simpleCounter(MetricSKUUnmatchedTotal, int64(n))
		}
	})
}

// RecordSKUAmbiguous records ambiguous SKU count from binding sync.
func RecordSKUAmbiguous(n int) {
	SafeRecord(func() {
		if n > 0 {
			simpleCounter(MetricSKUAmbiguousTotal, int64(n))
		}
	})
}

// RecordOrderSyncOutcome records order sync business counters once per task completion.
func RecordOrderSyncOutcome(fetched, created, updated int, partialSuccess bool, unmatched, deducted int) {
	SafeRecord(func() {
		if fetched > 0 {
			simpleCounter(MetricOrderFetchedTotal, int64(fetched))
		}
		if created > 0 {
			simpleCounter(MetricOrderCreatedTotal, int64(created))
		}
		if updated > 0 {
			simpleCounter(MetricOrderUpdatedTotal, int64(updated))
		}
		if partialSuccess {
			simpleCounter(MetricOrderPartialSuccessTotal, 1)
		}
		if unmatched > 0 {
			simpleCounter(MetricOrderUnmatchedItemsTotal, int64(unmatched))
		}
		if deducted > 0 {
			simpleCounter(MetricOrderInventoryDeductedTotal, int64(deducted))
		}
	})
}

// RecordInventorySync records inventory sync task outcome.
func RecordInventorySync(outcome string) {
	SafeRecord(func() {
		simpleCounter(MetricInventorySyncTotal, 1)
		switch strings.TrimSpace(strings.ToLower(outcome)) {
		case "success":
			simpleCounter(MetricInventorySyncSuccessTotal, 1)
		case "failed":
			simpleCounter(MetricInventorySyncFailedTotal, 1)
		case "skipped":
			simpleCounter(MetricInventorySyncSkippedTotal, 1)
		}
	})
}

// SetFailureTasksPending sets gauge-like pending failure counter.
func SetFailureTasksPending(n int64) {
	SafeRecord(func() {
		Default.mu.Lock()
		Default.counters[MetricFailureTasksPending] = n
		Default.mu.Unlock()
	})
}

// SetAuthorizationsExpiring sets gauge-like expiring auth counter.
func SetAuthorizationsExpiring(n int64) {
	SafeRecord(func() {
		Default.mu.Lock()
		Default.counters[MetricAuthorizationsExpiring] = n
		Default.mu.Unlock()
	})
}
