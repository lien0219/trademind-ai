package s3store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Provider is an S3-compatible blob storage backend (implements the parent storage.Provider interface shape).
type Provider struct {
	cfg           Config
	client        *s3.Client
	presignClient *s3.PresignClient
}

func newClients(cfg Config) (*s3.Client, *s3.PresignClient, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("s3 storage: aws config load: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})
	pc := s3.NewPresignClient(client)
	return client, pc, nil
}

// New returns an S3-compatible storage provider.
func New(cfg Config) (*Provider, error) {
	cli, pc, err := newClients(cfg)
	if err != nil {
		return nil, err
	}
	return &Provider{cfg: cfg, client: cli, presignClient: pc}, nil
}

// NewFromSettingsMap parses map and constructs the provider (declaredKind: s3|r2|minio).
func NewFromSettingsMap(declaredKind string, m map[string]string) (*Provider, error) {
	cfg, err := ParseConfigFromMap(declaredKind, m)
	if err != nil {
		return nil, err
	}
	return New(cfg)
}

func normKey(objectKey string) string {
	return strings.TrimLeft(strings.ReplaceAll(objectKey, "\\", "/"), "/")
}

// Put uploads an object via PutObject.
func (p *Provider) Put(ctx context.Context, objectKey string, r io.Reader, size int64, contentType string) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("s3 storage: nil provider")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	key := normKey(objectKey)
	if key == "" {
		return fmt.Errorf("s3 storage: empty object key")
	}

	body, err := readLimited(r, size)
	if err != nil {
		return err
	}

	in := &s3.PutObjectInput{
		Bucket: aws.String(p.cfg.Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}
	if ct := strings.TrimSpace(contentType); ct != "" {
		in.ContentType = aws.String(ct)
	}
	if _, err := p.client.PutObject(ctx, in); err != nil {
		return fmt.Errorf("s3 storage: put object: %w", err)
	}
	return nil
}

func readLimited(r io.Reader, size int64) ([]byte, error) {
	if size < 0 {
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("s3 storage: read body: %w", err)
		}
		return b, nil
	}
	b, err := io.ReadAll(io.LimitReader(r, size))
	if err != nil {
		return nil, fmt.Errorf("s3 storage: read body: %w", err)
	}
	if int64(len(b)) != size {
		return nil, fmt.Errorf("s3 storage: upload size mismatch (expected %d bytes, got %d)", size, len(b))
	}
	return b, nil
}

// GetURL returns a stable public URL using PublicBase or a presigned URL when enabled.
func (p *Provider) GetURL(ctx context.Context, objectKey string) (string, error) {
	key := normKey(objectKey)
	if key == "" {
		return "", fmt.Errorf("s3 storage: empty object key")
	}

	if pb := strings.TrimRight(strings.TrimSpace(p.cfg.PublicBase), "/"); pb != "" {
		return pb + "/" + key, nil
	}

	if p.cfg.PresignEnabled && p.presignClient != nil && p.cfg.PresignExpireSeconds > 0 {
		pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		out, err := p.presignClient.PresignGetObject(pctx, &s3.GetObjectInput{
			Bucket: aws.String(p.cfg.Bucket),
			Key:    aws.String(key),
		}, func(o *s3.PresignOptions) {
			o.Expires = time.Duration(p.cfg.PresignExpireSeconds) * time.Second
		})
		if err != nil {
			return "", fmt.Errorf("s3 storage: presign get url: %w", err)
		}
		return out.URL, nil
	}

	return "", fmt.Errorf("s3 storage: configure s3_public_base or enable s3_presign_enabled for reachable URLs")
}

// Delete removes the remote object (idempotent — NoSuchKey is OK).
func (p *Provider) Delete(ctx context.Context, objectKey string) error {
	if p == nil || p.client == nil {
		return fmt.Errorf("s3 storage: nil provider")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	key := normKey(objectKey)
	if key == "" {
		return fmt.Errorf("s3 storage: empty object key")
	}
	if _, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.cfg.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("s3 storage: delete object: %w", err)
	}
	return nil
}

// TestConnectivity checks bucket access via HeadBucket (no uploads).
func TestConnectivity(ctx context.Context, declaredKind string, m map[string]string) error {
	cfg, err := ParseConfigFromMap(declaredKind, m)
	if err != nil {
		return err
	}
	cli, _, err := newClients(cfg)
	if err != nil {
		return err
	}
	tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	_, err = cli.HeadBucket(tctx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Bucket)})
	if err != nil {
		return fmt.Errorf("cannot access bucket %q (check endpoint, credentials, region, bucket name, and IAM permissions HeadBucket/ListBucket): %w", cfg.Bucket, err)
	}
	return nil
}
