package cosstorage

import (
	"fmt"
	"net/url"
	"strings"
)

// Config holds Tencent COS connectivity settings (from decrypted settings.storage).
type Config struct {
	Bucket     string // logical bucket name from user (may omit AppID suffix when cos_app_id is set)
	AppID      string
	Region     string
	SecretID   string
	SecretKey  string
	Endpoint   string // full base URL optional override (scheme + host, optional port)
	PublicBase string
	UseHTTPS   bool

	// resolvedBucket is the subdomain label used by COS REST host: "{name}-{appid}" — required by Tencent client.
	resolvedBucket string
}

func boolFromSetting(v string, defaultTrue bool) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	if s == "" {
		return defaultTrue
	}
	return s != "false" && s != "0" && s != "no"
}

func resolvedBucketLabel(bucket, appID string) (string, error) {
	b := strings.TrimSpace(bucket)
	if b == "" {
		return "", fmt.Errorf("cos_bucket is empty")
	}
	app := strings.TrimSpace(appID)
	if strings.Contains(b, "-") {
		return b, nil
	}
	if app == "" {
		return "", fmt.Errorf("cos_bucket %q must include AppID suffix like name-appid, or set cos_app_id", b)
	}
	return b + "-" + app, nil
}

// ParseConfigFromMap reads decrypted storage settings (snake_case item keys).
func ParseConfigFromMap(m map[string]string) (Config, error) {
	bucket := strings.TrimSpace(m["cos_bucket"])
	if bucket == "" {
		return Config{}, fmt.Errorf("cos storage: cos_bucket is required")
	}
	region := strings.TrimSpace(m["cos_region"])
	if region == "" {
		return Config{}, fmt.Errorf("cos storage: cos_region is required")
	}
	secretID := strings.TrimSpace(m["cos_secret_id"])
	if secretID == "" {
		return Config{}, fmt.Errorf("cos storage: cos_secret_id is required")
	}
	secretKey := strings.TrimSpace(m["cos_secret_key"])
	if secretKey == "" {
		return Config{}, fmt.Errorf("cos storage: cos_secret_key is required")
	}
	rb, err := resolvedBucketLabel(bucket, m["cos_app_id"])
	if err != nil {
		return Config{}, fmt.Errorf("cos storage: %w", err)
	}

	return Config{
		Bucket:         bucket,
		AppID:          strings.TrimSpace(m["cos_app_id"]),
		Region:         region,
		SecretID:       secretID,
		SecretKey:      secretKey,
		Endpoint:       strings.TrimSpace(m["cos_endpoint"]),
		PublicBase:     strings.TrimRight(strings.TrimSpace(m["cos_public_base"]), "/"),
		UseHTTPS:       boolFromSetting(m["cos_use_https"], true),
		resolvedBucket: rb,
	}, nil
}

// parseEndpointOverride parses cos_endpoint when provided.
func (c Config) parseEndpointOverride() (*url.URL, error) {
	ep := strings.TrimSpace(c.Endpoint)
	if ep == "" {
		return nil, nil
	}
	u, err := url.Parse(ep)
	if err != nil {
		return nil, fmt.Errorf("cos_endpoint parse: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("cos_endpoint must include scheme and host")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u, nil
}
