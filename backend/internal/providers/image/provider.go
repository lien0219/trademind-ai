package image

import "context"

// Provider abstracts image processing (remove.bg, ComfyUI, OpenAI Image, ...).
type Provider interface {
	Name() string
	RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error)
	ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error)
	GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error)
	Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error)
	Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error)
	TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error)
	PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error)
}
