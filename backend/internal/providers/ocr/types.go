package ocr

import (
	"context"
)

// OCRRequest defines the input for OCR tasks.
type OCRRequest struct {
	ImageURL          string `json:"imageUrl"`
	ImageBase64       string `json:"imageBase64,omitempty"`
	SourceLanguage    string `json:"sourceLanguage,omitempty"`
	TargetLanguage    string `json:"targetLanguage,omitempty"`
	DetectOrientation bool   `json:"detectOrientation,omitempty"`
	ImageWidth        int    `json:"imageWidth,omitempty"`
	ImageHeight       int    `json:"imageHeight,omitempty"`
}

type OCRBBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type OCRBlock struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	BBox       OCRBBox `json:"bbox"`
	// Direction could be "horizontal" or "vertical"
	Direction string `json:"direction,omitempty"`
}

// OCRResult defines the output of OCR tasks.
type OCRResult struct {
	Provider         string     `json:"provider"`
	DetectedLanguage string     `json:"detectedLanguage"`
	Blocks           []OCRBlock `json:"blocks"`
}

// Provider defines the interface for OCR services.
type Provider interface {
	// DetectText detects text in an image and returns structural blocks.
	DetectText(ctx context.Context, req OCRRequest) (*OCRResult, error)
}
