package image

import "context"

// Provider abstracts image processing (remove.bg, ComfyUI, OpenAI Image, ...).
type Provider interface {
	Ping(ctx context.Context) error
}
