package inventory

import (
	"sync"
)

var (
	inventoryMu      sync.Mutex
	inventoryRunning bool
)

// SetInventorySyncWorkersRunning toggles worker heartbeat hints for /health.
func SetInventorySyncWorkersRunning(v bool) {
	inventoryMu.Lock()
	inventoryRunning = v
	inventoryMu.Unlock()
}

// InventorySyncWorkersRunning reports consumers started this process.
func InventorySyncWorkersRunning() bool {
	inventoryMu.Lock()
	defer inventoryMu.Unlock()
	return inventoryRunning
}
