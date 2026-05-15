package storage

import "context"

// Provider abstracts blob storage (local first; S3/COS/OSS/R2/MinIO via adapters).
type Provider interface {
	Ping(ctx context.Context) error
}
