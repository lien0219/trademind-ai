package platform

import "context"

// Provider abstracts marketplace OAuth and APIs (TikTok, Shopee, ...).
type Provider interface {
	Ping(ctx context.Context) error
}
