package cosstorage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	tc "github.com/tencentyun/cos-go-sdk-v5"

	"github.com/trademind-ai/trademind/backend/internal/providers/storage/keypath"
)

// Provider implements storage.Provider for Tencent COS.
type Provider struct {
	cfg    Config
	client *tc.Client
}

func newCOSHTTPClient(transport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: transport,
		Timeout:   125 * time.Second,
	}
}

// New constructs a COS provider from parsed config.
func New(cfg Config) (*Provider, error) {
	var bu *url.URL
	var err error
	if override, e := cfg.parseEndpointOverride(); e != nil {
		return nil, fmt.Errorf("cos storage: %w", e)
	} else if override != nil {
		bu = override
	} else {
		bu, err = tc.NewBucketURL(cfg.resolvedBucket, cfg.Region, cfg.UseHTTPS)
		if err != nil {
			return nil, fmt.Errorf("cos storage: bucket url: %w", err)
		}
	}

	base := &tc.BaseURL{BucketURL: bu}
	t := &tc.AuthorizationTransport{
		SecretID:  cfg.SecretID,
		SecretKey: cfg.SecretKey,
		Transport: http.DefaultTransport,
	}
	cli := tc.NewClient(base, newCOSHTTPClient(t))
	return &Provider{cfg: cfg, client: cli}, nil
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
			return nil, fmt.Errorf("cos storage: read body: %w", err)
		}
		return b, nil
	}
	b, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return nil, fmt.Errorf("cos storage: read body: %w", err)
	}
	if int64(len(b)) != size {
		return nil, fmt.Errorf("cos storage: upload size mismatch (expected %d bytes, got %d)", size, len(b))
	}
	return b, nil
}

// Put uploads an object via Put Object.
func (p *Provider) Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("cos storage: nil provider")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return fmt.Errorf("cos storage: %w", err)
	}
	body, err := readPutBody(r, size)
	if err != nil {
		return err
	}

	hdr := &tc.ObjectPutHeaderOptions{
		ContentLength: int64(len(body)),
	}
	if ct := strings.TrimSpace(contentType); ct != "" {
		hdr.ContentType = ct
	}
	opt := &tc.ObjectPutOptions{ObjectPutHeaderOptions: hdr}
	resp, err := p.client.Object.Put(ctx, key, bytes.NewReader(body), opt)
	if err != nil {
		return fmt.Errorf("cos storage: put object: %w", err)
	}
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	return nil
}

func joinPublicURL(prefix, key string) string {
	pp := strings.TrimRight(strings.TrimSpace(prefix), "/")
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return pp + "/" + strings.Join(parts, "/")
}

// GetURL joins cos_public_base with key; otherwise derives from the bucket endpoint.
func (p *Provider) GetURL(ctx context.Context, objectKey string) (string, error) {
	_ = ctx
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return "", fmt.Errorf("cos storage: %w", err)
	}
	if pb := p.cfg.PublicBase; pb != "" {
		return joinPublicURL(pb, key), nil
	}
	if p.client == nil || p.client.BaseURL == nil || p.client.BaseURL.BucketURL == nil {
		return "", fmt.Errorf("cos storage: configure cos_public_base")
	}
	b := strings.TrimRight(p.client.BaseURL.BucketURL.String(), "/")
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return b + "/" + strings.Join(parts, "/"), nil
}

type cosRespBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cosRespBody) Close() error {
	err := b.ReadCloser.Close()
	if b.cancel != nil {
		b.cancel()
	}
	return err
}

// Get downloads the object; caller must Close the reader.
func (p *Provider) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if p == nil || p.client == nil {
		return nil, fmt.Errorf("cos storage: nil provider")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cos storage: %w", err)
	}
	resp, err := p.client.Object.Get(ctx, key, nil)
	if err != nil {
		cancel()
		if isNotFound(err) {
			return nil, fmt.Errorf("cos storage: file not found")
		}
		return nil, fmt.Errorf("cos storage: get object: %w", err)
	}
	if resp == nil || resp.Body == nil {
		cancel()
		return nil, fmt.Errorf("cos storage: empty body")
	}
	return &cosRespBody{ReadCloser: resp.Body, cancel: cancel}, nil
}

// Delete removes the remote object (idempotent — 404 treated as OK).
func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("cos storage: nil provider")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return fmt.Errorf("cos storage: %w", err)
	}
	resp, err := p.client.Object.Delete(ctx, key)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("cos storage: delete object: %w", err)
	}
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	return nil
}

// TestConnectivity calls Head Bucket (no object writes).
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
	resp, err := p.client.Bucket.Head(tctx)
	if err != nil {
		return fmt.Errorf("cannot access COS bucket %q (check cos_region / cos_bucket / credentials / permissions): %w", cfg.resolvedBucket, err)
	}
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "404") || strings.Contains(s, "nosuchkey")
}
