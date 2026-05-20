package collect

import (
	"context"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
)

// CustomCollectProviderConflict is returned when source=custom but URL belongs to a dedicated provider.
type CustomCollectProviderConflict struct {
	RecommendedProvider string `json:"recommendedProvider"`
	Message             string `json:"message"`
}

func (e *CustomCollectProviderConflict) Error() string {
	if e != nil && strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return "CUSTOM_COLLECT_PROVIDER_CONFLICT"
}

func customCollectConflictMessage(platform collectdomain.PlatformID, providerName string) string {
	name := strings.TrimSpace(providerName)
	switch platform {
	case collectdomain.Platform1688:
		return "该链接属于 1688 平台，请使用「1688 采集器」。1688 采集器已针对商品标题、主图、详情图、属性、SKU 做专门适配，采集结果更稳定。"
	case collectdomain.PlatformAliExpress:
		return "该链接属于 AliExpress 平台，请使用「速卖通采集器」。专用采集器字段识别更稳定。"
	default:
		if name != "" {
			return "该链接属于已配置专用采集器的平台，请使用「" + name + "」。"
		}
		return "该链接属于已配置专用采集器的平台，请使用对应专用采集器。"
	}
}

func (s *Service) checkCustomCollectURLConflict(ctx context.Context, urlStr string) error {
	host := collectdomain.HostnameFromURL(urlStr)
	platform, ok := collectdomain.DetectPlatform(host)
	if !ok {
		return nil
	}
	source := collectdomain.ProviderSourceForPlatform(platform)
	provs := s.ResolveCollectProviders(ctx)
	p := findCollectProvider(provs, source)
	if p == nil {
		return nil
	}
	status := strings.TrimSpace(strings.ToLower(p.Status))
	if status != "available" && status != "beta" {
		return nil
	}
	return &CustomCollectProviderConflict{
		RecommendedProvider: source,
		Message:             customCollectConflictMessage(platform, p.Name),
	}
}
