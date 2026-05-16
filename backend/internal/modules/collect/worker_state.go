package collect

import "sync/atomic"

var (
	collectWorkersRunning atomic.Bool
	workerConcurrencyCfg  atomic.Int32
	workerQueueEnabled    atomic.Bool
)

// ConfigureWorkerMonitor stores queue/worker flags from process config (called once from main).
func ConfigureWorkerMonitor(queueEnabled bool, concurrency int) {
	workerQueueEnabled.Store(queueEnabled)
	workerConcurrencyCfg.Store(int32(normalizeCollectConcurrency(concurrency)))
}

// SetCollectWorkersRunning marks whether in-process BRPOP workers are active (main sets false after graceful shutdown).
func SetCollectWorkersRunning(v bool) {
	collectWorkersRunning.Store(v)
}

// CollectWorkersRunning reports whether workers were started and not yet marked stopped after WaitGroup drain.
func CollectWorkersRunning() bool {
	return collectWorkersRunning.Load()
}

// CollectWorkerConcurrencyConfigured returns configured worker concurrency (clamped).
func CollectWorkerConcurrencyConfigured() int {
	return int(workerConcurrencyCfg.Load())
}

// CollectWorkerQueueEnabled returns COLLECT_QUEUE_ENABLED from startup configuration.
func CollectWorkerQueueEnabled() bool {
	return workerQueueEnabled.Load()
}
