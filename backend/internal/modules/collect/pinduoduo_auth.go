package collect

import (
	"context"
	"strings"
)

// LatestFailedPinduoduoSourceURL returns the most recent failed collect task URL for pinduoduo.
func (s *Service) LatestFailedPinduoduoSourceURL(ctx context.Context) string {
	if s == nil || s.DB == nil {
		return ""
	}
	var task CollectTask
	err := s.DB.WithContext(ctx).
		Where("LOWER(source) IN ?", []string{"pinduoduo", "pdd"}).
		Where("status = ?", StatusFailed).
		Where("source_url <> ''").
		Order("updated_at DESC").
		Limit(1).
		Find(&task).Error
	if err != nil || strings.TrimSpace(task.SourceURL) == "" {
		return ""
	}
	return strings.TrimSpace(task.SourceURL)
}

// ResolvePinduoduoAuthCheckInputs picks context URL (body → latest failure) and settings test URL.
func (s *Service) ResolvePinduoduoAuthCheckInputs(ctx context.Context, bodyURL string) (contextURL string, settingsTestURL string) {
	contextURL = strings.TrimSpace(bodyURL)
	if contextURL == "" {
		contextURL = s.LatestFailedPinduoduoSourceURL(ctx)
	}
	if s != nil && s.Settings != nil {
		m, err := s.Settings.PlainByGroup(ctx, 0, "collector")
		if err == nil {
			settingsTestURL = strings.TrimSpace(m["collect_pinduoduo_auth_check_url"])
		}
	}
	return contextURL, settingsTestURL
}
