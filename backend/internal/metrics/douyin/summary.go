package douyin

import "time"

// Summary24h is a rolling 24-hour metrics snapshot for health endpoints.
type Summary24h struct {
	GeneratedAt time.Time `json:"generatedAt"`

	APIRequestsTotal        int64   `json:"apiRequestsTotal"`
	APISuccessTotal         int64   `json:"apiSuccessTotal"`
	APIFailedTotal          int64   `json:"apiFailedTotal"`
	APISuccessRate          float64 `json:"apiSuccessRate"`
	APIDurationAvgMs        float64 `json:"apiDurationAvgMs"`
	APITimeoutTotal         int64   `json:"apiTimeoutTotal"`
	APIRateLimitedTotal     int64   `json:"apiRateLimitedTotal"`
	APIRetryTotal           int64   `json:"apiRetryTotal"`
	TokenRefreshTotal       int64   `json:"tokenRefreshTotal"`
	TokenRefreshFailedTotal int64   `json:"tokenRefreshFailedTotal"`

	RuntimeBlockedTasksTotal int64 `json:"runtimeBlockedTasksTotal"`
	StaleTasksTotal          int64 `json:"staleTasksTotal"`
	RecoverySuccessTotal     int64 `json:"recoverySuccessTotal"`
	RecoveryFailedTotal      int64 `json:"recoveryFailedTotal"`

	ProductDraftCreateTotal     int64 `json:"productDraftCreateTotal"`
	ProductDraftCreateFailed    int64 `json:"productDraftCreateFailedTotal"`
	ImageUploadTotal            int64 `json:"imageUploadTotal"`
	ImageUploadFailedTotal      int64 `json:"imageUploadFailedTotal"`
	SKUAutoBoundTotal           int64 `json:"skuAutoBoundTotal"`
	SKUManualBoundTotal         int64 `json:"skuManualBoundTotal"`
	SKUUnmatchedTotal           int64 `json:"skuUnmatchedTotal"`
	SKUAmbiguousTotal           int64 `json:"skuAmbiguousTotal"`
	OrderFetchedTotal           int64 `json:"orderFetchedTotal"`
	OrderCreatedTotal           int64 `json:"orderCreatedTotal"`
	OrderUpdatedTotal           int64 `json:"orderUpdatedTotal"`
	OrderPartialSuccessTotal    int64 `json:"orderPartialSuccessTotal"`
	OrderUnmatchedItemsTotal    int64 `json:"orderUnmatchedItemsTotal"`
	OrderInventoryDeductedTotal int64 `json:"orderInventoryDeductedTotal"`
	InventorySyncTotal          int64 `json:"inventorySyncTotal"`
	InventorySyncSuccessTotal   int64 `json:"inventorySyncSuccessTotal"`
	InventorySyncFailedTotal    int64 `json:"inventorySyncFailedTotal"`
	InventorySyncSkippedTotal   int64 `json:"inventorySyncSkippedTotal"`

	FailureTasksPending    int64 `json:"failureTasksPending"`
	AuthorizationsExpiring int64 `json:"authorizationsExpiring"`
}

// Summary24hFrom returns rolling 24h summary from a registry.
func Summary24hFrom(r *Registry) Summary24h {
	if r == nil {
		r = Default
	}
	now := time.Now().UTC()
	sumMs, durCount := r.durationStats24h(MetricAPIDurationMs, func(l Labels) bool {
		return l.Result == "success"
	})
	success := r.sumCounter24h(MetricAPISuccessTotal, nil)
	requests := r.sumCounter24h(MetricAPIRequestsTotal, nil)

	r.mu.RLock()
	pending := r.counters[MetricFailureTasksPending]
	expiring := r.counters[MetricAuthorizationsExpiring]
	r.mu.RUnlock()

	return Summary24h{
		GeneratedAt: now,

		APIRequestsTotal:        requests,
		APISuccessTotal:         success,
		APIFailedTotal:          r.sumCounter24h(MetricAPIFailedTotal, nil),
		APISuccessRate:          fmtPct(success, requests),
		APIDurationAvgMs:        formatAvg(sumMs, durCount),
		APITimeoutTotal:         r.sumCounter24h(MetricAPITimeoutTotal, nil),
		APIRateLimitedTotal:     r.sumCounter24h(MetricAPIRateLimitedTotal, nil),
		APIRetryTotal:           r.sumCounter24h(MetricAPIRetryTotal, nil),
		TokenRefreshTotal:       r.sumCounter24h(MetricTokenRefreshTotal, nil),
		TokenRefreshFailedTotal: r.sumCounter24h(MetricTokenRefreshFailedTotal, nil),

		RuntimeBlockedTasksTotal: r.sumCounter24h(MetricRuntimeBlockedTasksTotal, nil),
		StaleTasksTotal:          r.sumCounter24h(MetricStaleTasksTotal, nil),
		RecoverySuccessTotal:     r.sumCounter24h(MetricRecoverySuccessTotal, nil),
		RecoveryFailedTotal:      r.sumCounter24h(MetricRecoveryFailedTotal, nil),

		ProductDraftCreateTotal:     r.sumCounter24h(MetricProductDraftCreateTotal, nil),
		ProductDraftCreateFailed:    r.sumCounter24h(MetricProductDraftCreateFailed, nil),
		ImageUploadTotal:            r.sumCounter24h(MetricImageUploadTotal, nil),
		ImageUploadFailedTotal:      r.sumCounter24h(MetricImageUploadFailedTotal, nil),
		SKUAutoBoundTotal:           r.sumCounter24h(MetricSKUAutoBoundTotal, nil),
		SKUManualBoundTotal:         r.sumCounter24h(MetricSKUManualBoundTotal, nil),
		SKUUnmatchedTotal:           r.sumCounter24h(MetricSKUUnmatchedTotal, nil),
		SKUAmbiguousTotal:           r.sumCounter24h(MetricSKUAmbiguousTotal, nil),
		OrderFetchedTotal:           r.sumCounter24h(MetricOrderFetchedTotal, nil),
		OrderCreatedTotal:           r.sumCounter24h(MetricOrderCreatedTotal, nil),
		OrderUpdatedTotal:           r.sumCounter24h(MetricOrderUpdatedTotal, nil),
		OrderPartialSuccessTotal:    r.sumCounter24h(MetricOrderPartialSuccessTotal, nil),
		OrderUnmatchedItemsTotal:    r.sumCounter24h(MetricOrderUnmatchedItemsTotal, nil),
		OrderInventoryDeductedTotal: r.sumCounter24h(MetricOrderInventoryDeductedTotal, nil),
		InventorySyncTotal:          r.sumCounter24h(MetricInventorySyncTotal, nil),
		InventorySyncSuccessTotal:   r.sumCounter24h(MetricInventorySyncSuccessTotal, nil),
		InventorySyncFailedTotal:    r.sumCounter24h(MetricInventorySyncFailedTotal, nil),
		InventorySyncSkippedTotal:   r.sumCounter24h(MetricInventorySyncSkippedTotal, nil),

		FailureTasksPending:    pending,
		AuthorizationsExpiring: expiring,
	}
}

// GetSummary24h returns rolling 24h summary from Default registry.
func GetSummary24h() Summary24h {
	return Summary24hFrom(Default)
}
