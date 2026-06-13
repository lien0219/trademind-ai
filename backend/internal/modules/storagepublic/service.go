package storagepublic

import (
	"context"
	"fmt"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	storagepub "github.com/trademind-ai/trademind/backend/internal/pkg/storagepublic"
)

// Service orchestrates storage public access tests.
type Service struct {
	Settings *settings.Service
	OpLog    *operationlog.Service
}

// TestPublicAccess uploads a probe image and verifies anonymous HTTP access.
func (s *Service) TestPublicAccess(ctx context.Context) (storagepub.EndToEndResult, error) {
	if s == nil || s.Settings == nil {
		return storagepub.EndToEndResult{}, fmt.Errorf("storage public test unavailable")
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return storagepub.EndToEndResult{}, err
	}
	return storagepub.TestEndToEnd(ctx, plain)
}

// ProbeConfiguredBase checks the configured public_base syntactically and via HTTP when absolute HTTPS.
func (s *Service) ProbeConfiguredBase(ctx context.Context) storagepub.ProbeResult {
	if s == nil || s.Settings == nil {
		return storagepub.ProbeResult{OK: false, ErrorCode: storagepub.CodePublicAccessFailed, Message: "存储配置不可用"}
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return storagepub.ProbeResult{OK: false, ErrorCode: storagepub.CodePublicAccessFailed, Message: "读取存储配置失败", TechnicalDetails: map[string]any{"error": err.Error()}}
	}
	pubBase := storagepub.ResolvePublicBase(plain)
	if pubBase == "" {
		return storagepub.ProbeResult{OK: false, ErrorCode: storagepub.CodePublicBaseMissing, Message: "未配置图片公开访问地址", TechnicalDetails: map[string]any{"field": "public_base"}}
	}
	if !containsScheme(pubBase) {
		return storagepub.ProbeResult{OK: false, ErrorCode: storagepub.CodePublicURLInvalid, Message: "图片公开地址不是 HTTPS 公网 URL", TechnicalDetails: map[string]any{
			"publicBase": pubBase,
			"hint":       "生产环境须配置完整 HTTPS 域名，抖店无法访问 /static 等相对路径",
		}}
	}
	probeURL := pubBase + "/.trademind-probe-not-found.png"
	res := storagepub.VerifyPublicURL(ctx, probeURL)
	if res.ErrorCode == storagepub.CodePublicImageDecodeFailed {
		return storagepub.ProbeResult{
			OK:        false,
			Message:   "公开域名可访问，但未能验证图片格式（建议运行完整公网访问测试）",
			ErrorCode: storagepub.CodePublicAccessFailed,
			TechnicalDetails: map[string]any{
				"publicBase": pubBase,
				"note":       "full e2e upload test recommended",
			},
		}
	}
	return res
}

func containsScheme(s string) bool {
	return len(s) > 8 && (s[:7] == "http://" || s[:8] == "https://")
}
