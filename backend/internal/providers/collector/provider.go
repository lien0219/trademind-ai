package collector

import "context"

// Provider describes a product-page collector implementation (e.g. 1688).
// The Node Playwright service performs scraping; Go orchestrates tasks and persists results.
type Provider interface {
	Ping(ctx context.Context) error
}
