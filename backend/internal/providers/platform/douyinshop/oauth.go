package douyinshop

import (
	"fmt"
	"net/url"
	"strings"
)

func BuildAuthorizeURL(cfg RuntimeConfig, state string) (string, error) {
	serviceID := strings.TrimSpace(cfg.ServiceID)
	if serviceID == "" {
		return "", fmt.Errorf("douyin_shop service_id is required for OAuth authorize URL")
	}
	base := strings.TrimSpace(cfg.AuthBaseURL)
	if base == "" {
		base = defaultAuthBaseURL
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if err == nil {
			err = fmt.Errorf("missing scheme or host")
		}
		return "", fmt.Errorf("douyin_shop: invalid auth_base_url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/authorize"
	q := u.Query()
	q.Set("service_id", serviceID)
	q.Set("state", strings.TrimSpace(state))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
