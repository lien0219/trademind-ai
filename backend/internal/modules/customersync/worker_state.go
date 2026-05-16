package customersync

import "sync"

var (
	cmSyncWorkerMu       sync.Mutex
	cmSyncWorkersRunning bool
)

// SetCustomerMessageSyncWorkersRunning toggles global worker heartbeat flag used by /health.
func SetCustomerMessageSyncWorkersRunning(v bool) {
	cmSyncWorkerMu.Lock()
	cmSyncWorkersRunning = v
	cmSyncWorkerMu.Unlock()
}

// CustomerMessageSyncWorkersRunning reports whether workers were started this process.
func CustomerMessageSyncWorkersRunning() bool {
	cmSyncWorkerMu.Lock()
	defer cmSyncWorkerMu.Unlock()
	return cmSyncWorkersRunning
}
