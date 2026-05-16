package s3store

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind values using this adapter.
const (
	KindAWS   = "s3"
	KindR2    = "r2"
	KindMinIO = "minio"
)

// Config is resolved from decrypted settings.storage (snake_case) with legacy key fallbacks.
type Config struct {
	DeclaredKind         string // s3|r2|minio
	Endpoint             string // full URL incl. scheme, or empty for default AWS partitions
	Region               string
	Bucket               string
	AccessKeyID          string
	SecretAccessKey      string
	UsePathStyle         bool
	PublicBase           string // joined with object key → public URL
	PresignEnabled       bool
	PresignExpireSeconds int64
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func parseBool(raw string, def bool) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		return def
	}
	switch s {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// NormalizeEndpoint inserts http/https scheme when omitted.
func normalizeEndpoint(endpoint string, useSSL bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if strings.Contains(endpoint, "://") {
		return strings.TrimRight(endpoint, "/")
	}
	scheme := "https"
	if !useSSL {
		scheme = "http"
	}
	return scheme + "://" + strings.TrimLeft(endpoint, "/")
}

// ParseConfigFromMap builds S3-compatible config from settings.storage map after decryption.
func ParseConfigFromMap(declaredKind string, m map[string]string) (Config, error) {
	kind := strings.ToLower(strings.TrimSpace(declaredKind))
	endpointRaw := firstNonEmpty(m["s3_endpoint"], m["endpoint"])

	sslTok := strings.TrimSpace(m["s3_use_ssl"])
	useSSL := strings.HasPrefix(strings.ToLower(endpointRaw), "https://") || endpointRaw == ""
	switch {
	case sslTok != "":
		useSSL = parseBool(sslTok, useSSL)
	case endpointRaw == "":
		useSSL = true
	}
	endpoint := normalizeEndpoint(endpointRaw, useSSL)

	region := firstNonEmpty(m["s3_region"], m["region"])
	switch {
	case region != "":
		break
	case kind == KindR2:
		region = "auto"
	case kind == KindMinIO:
		region = "us-east-1"
	default:
		region = "us-east-1"
	}

	bucket := firstNonEmpty(m["s3_bucket"], m["bucket"])
	accessKey := firstNonEmpty(m["s3_access_key_id"], m["access_key"])
	secretKey := firstNonEmpty(m["s3_secret_access_key"], m["secret_key"])

	pub := strings.TrimRight(strings.TrimSpace(firstNonEmpty(m["s3_public_base"], m["public_base"])), "/")

	pathStyleTok := strings.TrimSpace(firstNonEmpty(m["s3_force_path_style"]))
	usePathStyle := parseBool(pathStyleTok, kind == KindMinIO)

	expire, err := strconv.ParseInt(strings.TrimSpace(m["s3_presign_expire_seconds"]), 10, 64)
	if err != nil || expire <= 0 {
		expire = 3600
	}

	cfg := Config{
		DeclaredKind:         kind,
		Endpoint:             endpoint,
		Region:               region,
		Bucket:               bucket,
		AccessKeyID:          accessKey,
		SecretAccessKey:      secretKey,
		UsePathStyle:         usePathStyle,
		PublicBase:           pub,
		PresignEnabled:       parseBool(strings.TrimSpace(m["s3_presign_enabled"]), false),
		PresignExpireSeconds: expire,
	}

	switch kind {
	case KindR2, KindMinIO:
		if cfg.Endpoint == "" {
			return Config{}, fmt.Errorf("s3-compatible storage (%s): s3_endpoint is required", kind)
		}
	}
	if cfg.Bucket == "" {
		return Config{}, fmt.Errorf("s3-compatible storage: s3_bucket is required")
	}
	if cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return Config{}, fmt.Errorf("s3-compatible storage: s3_access_key_id and s3_secret_access_key are required")
	}
	return cfg, nil
}
