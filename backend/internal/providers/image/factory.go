package image

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/providers/image/removebg"
)

func timeoutSecFromImageMap(m map[string]string) int {
	s := strings.TrimSpace(m["timeout_sec"])
	if s == "" {
		return 60
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 5 {
		return 60
	}
	if n > 600 {
		return 600
	}
	return n
}

// NewForTask builds an image Provider by name using decrypted settings.image when needed.
func NewForTask(ctx context.Context, providerName string, settingsSvc *settings.Service) (Provider, error) {
	name := strings.TrimSpace(strings.ToLower(providerName))
	switch name {
	case "noop":
		return NoopProvider{}, nil
	case "removebg":
		if settingsSvc == nil {
			return nil, fmt.Errorf("remove.bg provider requires settings service")
		}
		m, err := settingsSvc.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		key := strings.TrimSpace(m["removebg_api_key"])
		if key == "" {
			return nil, fmt.Errorf("removebg_api_key is not configured")
		}
		base := strings.TrimSpace(m["removebg_base_url"])
		if base == "" {
			base = "https://api.remove.bg/v1.0"
		}
		sec := timeoutSecFromImageMap(m)
		cli := removebg.NewClient(removebg.Options{
			APIKey:  key,
			BaseURL: base,
			Timeout: time.Duration(sec) * time.Second,
		})
		return removebgProvider{client: cli}, nil
	default:
		return nil, fmt.Errorf("unsupported image provider %q", name)
	}
}

type removebgProvider struct {
	client removebg.Client
}

func (p removebgProvider) Name() string { return "removebg" }

func (p removebgProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	b, err := p.client.RemoveBackgroundPNG(ctx, req.SourceURL)
	if err != nil {
		return nil, err
	}
	return &ImageResult{
		RawPayload:         b,
		PayloadContentType: "image/png",
	}, nil
}

func (p removebgProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: replace_background not implemented")
}

func (p removebgProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: generate_scene not implemented")
}

func (p removebgProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: resize not implemented")
}

func (p removebgProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: enhance not implemented")
}

func (p removebgProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: translate_image not implemented")
}

func (p removebgProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: poster_generate not implemented")
}
