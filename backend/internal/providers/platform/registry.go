package platform

import (
	"fmt"
	"sync"
)

var (
	regMu sync.RWMutex
	reg   = map[string]Provider{}
)

// Register adds or replaces a provider by Platform() key.
func Register(p Provider) {
	if p == nil {
		return
	}
	key := p.Platform()
	if key == "" {
		return
	}
	regMu.Lock()
	reg[key] = p
	regMu.Unlock()
}

// Get returns a provider by platform id, or nil.
func Get(platform string) Provider {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[platform]
}

// All returns a stable snapshot of registered providers (unsorted; sort in HTTP layer if needed).
func All() []Provider {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]Provider, 0, len(reg))
	for _, p := range reg {
		out = append(out, p)
	}
	return out
}

// MustGet returns provider or panics — only for tests/bootstrap asserts.
func MustGet(platform string) Provider {
	p := Get(platform)
	if p == nil {
		panic(fmt.Sprintf("platform provider %q not registered", platform))
	}
	return p
}
