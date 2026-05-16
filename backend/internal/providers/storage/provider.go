package storage

import (
	"context"
	"io"
)

// Provider abstracts blob storage (local first; S3/COS/OSS/R2/MinIO via adapters).
type Provider interface {
	Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error
	GetURL(ctx context.Context, objectKey string) (string, error)
	Delete(ctx context.Context, objectKey string) error
	// Get returns a stream for the object; the caller must close the ReadCloser.
	Get(ctx context.Context, objectKey string) (io.ReadCloser, error)
}
