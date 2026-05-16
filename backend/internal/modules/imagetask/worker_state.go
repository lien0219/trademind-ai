package imagetask

import "sync/atomic"

var (
	imageWorkersRunning    atomic.Bool
	imageWorkerConcurrency atomic.Int32
	imageWorkerQueueOn     atomic.Bool
)

// ConfigureImageWorkerMonitor stores queue/worker flags from process config (called once from main).
func ConfigureImageWorkerMonitor(queueEnabled bool, concurrency int) {
	imageWorkerQueueOn.Store(queueEnabled)
	imageWorkerConcurrency.Store(int32(normalizeImageWorkerConcurrency(concurrency)))
}

// SetImageWorkersRunning marks whether BRPOP workers are active (main sets false after graceful shutdown).
func SetImageWorkersRunning(v bool) {
	imageWorkersRunning.Store(v)
}

// ImageWorkersRunning reports whether image workers were started and not yet marked stopped after WaitGroup drain.
func ImageWorkersRunning() bool {
	return imageWorkersRunning.Load()
}

// ImageWorkerConcurrencyConfigured returns configured worker concurrency (clamped).
func ImageWorkerConcurrencyConfigured() int {
	return int(imageWorkerConcurrency.Load())
}

// ImageWorkerQueueEnabled returns IMAGE_QUEUE_ENABLED from startup configuration.
func ImageWorkerQueueEnabled() bool {
	return imageWorkerQueueOn.Load()
}

func normalizeImageWorkerConcurrency(n int) int {
	if n < 1 {
		return 1
	}
	if n > 32 {
		return 32
	}
	return n
}
