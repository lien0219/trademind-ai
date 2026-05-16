package settings

import (
	"context"
	"fmt"
	"strings"
)

// PlainMailSettings returns plaintext SMTP-related settings, preferring group "mail" over legacy "email".
func (s *Service) PlainMailSettings(ctx context.Context) (map[string]string, error) {
	if s == nil {
		return nil, fmt.Errorf("settings: no service")
	}
	legacy, legErr := s.PlainByGroup(ctx, 0, "email")
	primary, primErr := s.PlainByGroup(ctx, 0, "mail")
	if legErr != nil && primErr != nil {
		return nil, legErr
	}
	if legacy == nil {
		legacy = map[string]string{}
	}
	if primary == nil {
		primary = map[string]string{}
	}
	allKeys := map[string]struct{}{}
	for k := range legacy {
		allKeys[k] = struct{}{}
	}
	for k := range primary {
		allKeys[k] = struct{}{}
	}
	out := make(map[string]string, len(allKeys))
	for k := range allKeys {
		pv := strings.TrimSpace(primary[k])
		if pv != "" {
			out[k] = primary[k]
			continue
		}
		out[k] = legacy[k]
	}
	return out, nil
}
