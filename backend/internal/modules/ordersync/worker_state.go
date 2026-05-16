package ordersync

import "sync"

var (
	orderSyncWorkerMu       sync.Mutex
	orderSyncWorkersRunning bool
)

// SetOrderSyncWorkersRunning toggles global worker heartbeat flag used by /health.
func SetOrderSyncWorkersRunning(v bool) {
	orderSyncWorkerMu.Lock()
	orderSyncWorkersRunning = v
	orderSyncWorkerMu.Unlock()
}

// OrderSyncWorkersRunning reports whether workers were started this process.
func OrderSyncWorkersRunning() bool {
	orderSyncWorkerMu.Lock()
	defer orderSyncWorkerMu.Unlock()
	return orderSyncWorkersRunning
}
