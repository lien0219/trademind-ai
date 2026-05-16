package ossstorage

import (
	"fmt"
	"net/url"
	"strings"
)

// Config holds Aliyun OSS connectivity from decrypted settings.storage.
type Config struct {
	Endpoint            string
	Bucket              string
	AccessKeyID         string
	AccessKeySecret     string
	PublicBase          string
	UseHTTPS            bool
	internalEndpointURL *url.URL // normalized
}

func boolFromSetting(v string, defaultTrue bool) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	if s == "" {
		return defaultTrue
	}
	return s != "false" && s != "0" && s != "no"
}

// ParseConfigFromMap reads snake_case storage settings.
func ParseConfigFromMap(m map[string]string) (Config, error) {
	ep := strings.TrimSpace(m["oss_endpoint"])
	if ep == "" {
		return Config{}, fmt.Errorf("oss storage: oss_endpoint is required")
	}
	bucket := strings.TrimSpace(m["oss_bucket"])
	if bucket == "" {
		return Config{}, fmt.Errorf("oss storage: oss_bucket is required")
	}
	ak := strings.TrimSpace(m["oss_access_key_id"])
	if ak == "" {
		return Config{}, fmt.Errorf("oss storage: oss_access_key_id is required")
	}
	sk := strings.TrimSpace(m["oss_access_key_secret"])
	if sk == "" {
		return Config{}, fmt.Errorf("oss storage: oss_access_key_secret is required")
	}
	u, err := normalizeOSSAPIEndpoint(ep, boolFromSetting(m["oss_use_https"], true))
	if err != nil {
		return Config{}, err
	}
	return Config{
		Endpoint:            ep,
		Bucket:              bucket,
		AccessKeyID:         ak,
		AccessKeySecret:     sk,
		PublicBase:          strings.TrimRight(strings.TrimSpace(m["oss_public_base"]), "/"),
		UseHTTPS:            boolFromSetting(m["oss_use_https"], true),
		internalEndpointURL: u,
	}, nil
}

func normalizeOSSAPIEndpoint(raw string, preferHTTPS bool) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("oss storage: oss_endpoint is empty")
	}
	if !strings.Contains(s, "://") {
		if preferHTTPS {
			s = "https://" + s
		} else {
			s = "http://" + s
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("oss storage: oss_endpoint parse: %w", err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("oss storage: oss_endpoint must include host")
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u, nil
}

func (c Config) ossClientEndpoint() string {
	if c.internalEndpointURL == nil {
		return strings.TrimSuffix(strings.TrimSpace(c.Endpoint), "/")
	}
	return strings.TrimSuffix(c.internalEndpointURL.String(), "/")
}
