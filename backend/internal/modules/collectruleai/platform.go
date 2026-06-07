package collectruleai

import (
	"context"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/modules/collect"
	"github.com/trademind-ai/trademind/backend/internal/pkg/collectdomain"
)

type platformBlockError struct {
	RecommendedProvider string
	Message             string
}

func (e *platformBlockError) Error() string {
	if e != nil && strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return "CUSTOM_COLLECT_PROVIDER_CONFLICT"
}

func plannedPlatformHint(platform collectdomain.PlatformID) string {
	switch platform {
	case collectdomain.PlatformTaobaoTmall:
		return "淘宝/天猫专用采集器已开放 Beta，请优先使用「淘宝/天猫采集器」单链接采集。"
	case collectdomain.PlatformPdd:
		return "拼多多专用采集器已开放测试版，请优先使用「拼多多采集器」单链接采集。"
	case collectdomain.PlatformSheinTemu:
		return "该平台专用采集器尚未开放，商品规格、库存等信息不一定能完整识别。"
	default:
		return "该平台没有专用采集器，商品规格、库存等信息不一定能完整识别。"
	}
}

func checkPlatformForAIGenerate(ctx context.Context, resolver ProviderResolver, urlStr string) (plannedHint string, blockErr error) {
	if resolver == nil {
		return "", nil
	}
	host := collectdomain.HostnameFromURL(urlStr)
	platform, ok := collectdomain.DetectPlatform(host)
	if !ok {
		return "", nil
	}
	source := collectdomain.ProviderSourceForPlatform(platform)
	provs := resolver.ResolveCollectProviders(ctx)
	var p *collect.CollectProviderDTO
	for i := range provs {
		if strings.EqualFold(provs[i].Source, source) {
			p = &provs[i]
			break
		}
	}
	if p == nil {
		return plannedPlatformHint(platform), nil
	}
	status := strings.TrimSpace(strings.ToLower(p.Status))
	if status == "available" || status == "beta" {
		msg := customCollectConflictMessage(platform, p.Name)
		return "", &platformBlockError{RecommendedProvider: source, Message: msg}
	}
	return plannedPlatformHint(platform), nil
}

func customCollectConflictMessage(platform collectdomain.PlatformID, providerName string) string {
	name := strings.TrimSpace(providerName)
	switch platform {
	case collectdomain.Platform1688:
		return "该链接属于 1688 平台，请使用「1688 采集器」。1688 采集器已针对商品标题、主图、详情图、商品参数、商品规格做专门适配，识别更稳定。"
	case collectdomain.PlatformAliExpress:
		return "该链接属于 AliExpress 平台，请使用「速卖通采集器」。专用采集器字段识别更稳定。"
	case collectdomain.PlatformPdd:
		return "该链接属于拼多多平台，请使用「拼多多采集器」。专用采集器字段识别更稳定。"
	case collectdomain.PlatformTaobaoTmall:
		return "该链接属于淘宝/天猫平台，请使用「淘宝/天猫采集器」。专用采集器字段识别更稳定。"
	default:
		if name != "" {
			return "该链接属于已配置专用采集器的平台，请使用「" + name + "」。"
		}
		return "该链接属于已配置专用采集器的平台，请使用对应专用采集器。"
	}
}
