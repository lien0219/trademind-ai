package image

import "github.com/google/uuid"

// ImageRequest carries a source image URL and optional structured hints.
type ImageRequest struct {
	SourceURL string
	Input     map[string]any
}

// ReplaceBackgroundRequest is a swap-background style operation.
type ReplaceBackgroundRequest struct {
	ImageRequest
	Background string
}

// GenerateSceneRequest asks for a new scene / lifestyle composition.
type GenerateSceneRequest struct {
	ImageRequest
	Scene string
}

// ResizeRequest defines dimensional resize targets.
type ResizeRequest struct {
	SourceURL string
	Width     int
	Height    int
	Input     map[string]any
}

// TranslateImageRequest targets OCR / overlay translation flows.
type TranslateImageRequest struct {
	ImageRequest
	TargetLang string
}

// PosterGenerateRequest is a marketing poster layout stub.
type PosterGenerateRequest struct {
	ImageRequest
	Title string
}

// ImageResult is a provider outcome (URLs only; no binary).
type ImageResult struct {
	PublicURL string
	FileID    *uuid.UUID
	Meta      map[string]any
}
