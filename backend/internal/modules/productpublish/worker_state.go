package productpublish

import "sync"

var (
	ppWorkerMu    sync.Mutex
	ppWorkersOn   bool
)

func SetProductPublishWorkersRunning(v bool) {
	ppWorkerMu.Lock()
	ppWorkersOn = v
	ppWorkerMu.Unlock()
}

func ProductPublishWorkersRunning() bool {
	ppWorkerMu.Lock()
	defer ppWorkerMu.Unlock()
	return ppWorkersOn
}
