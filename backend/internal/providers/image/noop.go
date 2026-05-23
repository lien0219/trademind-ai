package image

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// NoopProvider is a pass-through / stub for wiring image tasks without real inference.
type NoopProvider struct{}

func trimURL(u string) string {
	return strings.TrimSpace(u)
}

// Name implements Provider.
func (NoopProvider) Name() string { return "noop" }

// RemoveBackground implements Provider.
func (NoopProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_ = ctx
	if trimURL(req.SourceURL) == "" {
		return nil, errors.New("noop: source url required")
	}
	return nil, errors.New("noop: remove_background not implemented (use resize/enhance for passthrough demo)")
}

// ReplaceBackground implements Provider.
func (NoopProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	_ = ctx
	if trimURL(req.SourceURL) == "" {
		return nil, errors.New("noop: source url required")
	}
	return nil, errors.New("noop: replace_background not implemented")
}

// GenerateScene implements Provider.
func (NoopProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	_ = ctx
	if trimURL(req.SourceURL) == "" {
		return nil, errors.New("noop: source url required")
	}
	return nil, errors.New("noop: generate_scene not implemented")
}

// Resize implements Provider — echoes the source URL as a successful “processed” result for UI tests.
func (NoopProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_ = ctx
	u := trimURL(req.SourceURL)
	if u == "" {
		return nil, errors.New("noop: source url required")
	}
	return &ImageResult{
		PublicURL: u,
		Meta: map[string]any{
			"noop":    true,
			"task":    "resize",
			"width":   req.Width,
			"height":  req.Height,
			"message": "passthrough source url",
		},
	}, nil
}

// Enhance implements Provider — cleanup/edit tasks must use a real provider.
func (NoopProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("占位演示不支持图片编辑，请在「设置 → 图片 AI」配置通义万相或其他图片服务")
}

// TranslateImage implements Provider.
func (NoopProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_ = ctx
	if trimURL(req.SourceURL) == "" {
		return nil, errors.New("noop: source url required")
	}
	return nil, fmt.Errorf("noop: translate_image not implemented")
}

// PosterGenerate implements Provider.
func (NoopProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_ = ctx
	if trimURL(req.SourceURL) == "" {
		return nil, errors.New("noop: source url required")
	}
	return nil, errors.New("noop: poster_generate not implemented")
}
