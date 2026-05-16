package image

import (
	"io"

	"github.com/google/uuid"
)

// ImageRequest carries a source image URL and optional structured hints.
type ImageRequest struct {
	SourceURL string
	Input     map[string]any
	// SourceFile when non-nil: remove.bg uses multipart image_file (server reads bytes).
	SourceFile        io.ReadCloser
	SourceFilename    string
	SourceContentType string
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
	// RawPayload when non-empty: imagetask persists bytes via Storage + files row before setting PublicURL/FileID.
	RawPayload []byte
	// PayloadContentType is MIME type for RawPayload (e.g. image/png).
	PayloadContentType string
}
