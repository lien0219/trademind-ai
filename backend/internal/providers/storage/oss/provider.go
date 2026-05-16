package ossstorage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	alioss "github.com/aliyun/aliyun-oss-go-sdk/oss"

	"github.com/trademind-ai/trademind/backend/internal/providers/storage/keypath"
)

// Provider implements storage.Provider for Aliyun OSS.
type Provider struct {
	cfg    Config
	client *alioss.Client
	bucket *alioss.Bucket
}

// New constructs an OSS-backed provider from parsed config.
func New(cfg Config) (*Provider, error) {
	cli, err := alioss.New(cfg.ossClientEndpoint(), cfg.AccessKeyID, cfg.AccessKeySecret,
		alioss.Timeout(30, 120),
	)
	if err != nil {
		return nil, fmt.Errorf("oss storage: client: %w", err)
	}
	bucket, err := cli.Bucket(cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("oss storage: bucket handle: %w", err)
	}
	return &Provider{cfg: cfg, client: cli, bucket: bucket}, nil
}

// NewFromSettingsMap builds the provider from decrypted settings.storage entries.
func NewFromSettingsMap(m map[string]string) (*Provider, error) {
	cfg, err := ParseConfigFromMap(m)
	if err != nil {
		return nil, err
	}
	return New(cfg)
}

func readPutBody(r io.Reader, size int64) ([]byte, error) {
	if size < 0 {
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("oss storage: read body: %w", err)
		}
		return b, nil
	}
	b, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return nil, fmt.Errorf("oss storage: read body: %w", err)
	}
	if int64(len(b)) != size {
		return nil, fmt.Errorf("oss storage: upload size mismatch (expected %d bytes, got %d)", size, len(b))
	}
	return b, nil
}

func joinPublicURL(prefix, key string) string {
	pp := strings.TrimRight(strings.TrimSpace(prefix), "/")
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return pp + "/" + strings.Join(parts, "/")
}

func virtualHostBucketBase(cfg Config) string {
	if cfg.internalEndpointURL == nil {
		return ""
	}
	scheme := cfg.internalEndpointURL.Scheme
	if scheme == "" {
		scheme = "https"
	}
	host := cfg.internalEndpointURL.Hostname()
	return scheme + "://" + cfg.Bucket + "." + host
}

// Put uploads bytes to OSS.
func (p *Provider) Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	if p.bucket == nil {
		return fmt.Errorf("oss storage: nil provider")
	}
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return fmt.Errorf("oss storage: %w", err)
	}
	body, err := readPutBody(r, size)
	if err != nil {
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	opts := []alioss.Option{alioss.WithContext(tctx)}
	if ct := strings.TrimSpace(contentType); ct != "" {
		opts = append(opts, alioss.ContentType(ct))
	}
	if err := p.bucket.PutObject(key, bytes.NewReader(body), opts...); err != nil {
		return fmt.Errorf("oss storage: put object: %w", sanitizeOssErr(err))
	}
	return nil
}

// GetURL uses oss_public_base or virtual-host bucket URL plus object key.
func (p *Provider) GetURL(ctx context.Context, objectKey string) (string, error) {
	_ = ctx
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return "", fmt.Errorf("oss storage: %w", err)
	}
	if pb := p.cfg.PublicBase; pb != "" {
		return joinPublicURL(pb, key), nil
	}
	vh := virtualHostBucketBase(p.cfg)
	if vh == "" {
		return "", fmt.Errorf("oss storage: configure oss_public_base")
	}
	return joinPublicURL(vh, key), nil
}

// Get downloads an object stream; caller closes.
func (p *Provider) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if p.bucket == nil {
		return nil, fmt.Errorf("oss storage: nil provider")
	}
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return nil, fmt.Errorf("oss storage: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	rc, err := p.bucket.GetObject(key, alioss.WithContext(tctx))
	if err != nil {
		cancel()
		if isOssNotFound(err) {
			return nil, fmt.Errorf("oss storage: file not found")
		}
		return nil, fmt.Errorf("oss storage: get object: %w", sanitizeOssErr(err))
	}
	return &cancelOnClose{ReadCloser: rc, cancel: cancel}, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	if c.cancel != nil {
		c.cancel()
	}
	return err
}

// Delete deletes an OSS object (idempotent on missing key).
func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	if p.bucket == nil {
		return fmt.Errorf("oss storage: nil provider")
	}
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return fmt.Errorf("oss storage: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := p.bucket.DeleteObject(key, alioss.WithContext(tctx)); err != nil && !isOssNotFound(err) {
		return fmt.Errorf("oss storage: delete object: %w", sanitizeOssErr(err))
	}
	return nil
}

// TestConnectivity lists at most one object key (metadata only, no uploads).
func TestConnectivity(ctx context.Context, m map[string]string) error {
	cfg, err := ParseConfigFromMap(m)
	if err != nil {
		return err
	}
	p, err := New(cfg)
	if err != nil {
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err = p.bucket.ListObjects(alioss.WithContext(tctx), alioss.MaxKeys(1))
	if err != nil {
		return fmt.Errorf("cannot access OSS bucket %q (check oss_endpoint / oss_bucket / credentials / ACL): %w", cfg.Bucket, sanitizeOssErr(err))
	}
	return nil
}

func sanitizeOssErr(err error) error {
	if err == nil {
		return nil
	}
	var se alioss.ServiceError
	if errors.As(err, &se) {
		code := strings.TrimSpace(se.Code)
		if code != "" {
			if se.StatusCode != 0 {
				return fmt.Errorf("%s (HTTP %d)", code, se.StatusCode)
			}
			return fmt.Errorf("%s", code)
		}
		if se.StatusCode != 0 {
			return fmt.Errorf("HTTP %d", se.StatusCode)
		}
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "secret") || strings.Contains(msg, "accesskey") || strings.Contains(msg, "authorization") {
		return fmt.Errorf("oss request rejected (check credentials)")
	}
	return err
}

func isOssNotFound(err error) bool {
	var se alioss.ServiceError
	if errors.As(err, &se) {
		if se.StatusCode == 404 || strings.EqualFold(se.Code, "NoSuchKey") ||
			strings.EqualFold(se.Code, "NoSuchBucket") {
			return true
		}
	}
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nosuchkey") || strings.Contains(msg, "404")
}
