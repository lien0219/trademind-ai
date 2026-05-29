package ocr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/paddleocr"
)

type paddleOCRProvider struct {
	client *paddleocr.Client
}

func (p *paddleOCRProvider) DetectText(ctx context.Context, req OCRRequest) (*OCRResult, error) {
	res, err := p.client.DetectText(ctx, paddleocr.DetectRequest{
		ImageBase64: req.ImageBase64,
	})
	if err != nil {
		return nil, err
	}

	blocks := make([]OCRBlock, 0, len(res.Blocks))
	for _, b := range res.Blocks {
		blocks = append(blocks, OCRBlock{
			ID:         b.ID,
			Text:       b.Text,
			Confidence: b.Confidence,
			BBox: OCRBBox{
				X:      b.BBox.X,
				Y:      b.BBox.Y,
				Width:  b.BBox.Width,
				Height: b.BBox.Height,
			},
			Direction: b.Direction,
		})
	}

	return &OCRResult{
		Provider:         "paddleocr",
		DetectedLanguage: "auto",
		Blocks:           blocks,
	}, nil
}

func NewProvider(providerName string, m map[string]string) (Provider, error) {
	switch strings.TrimSpace(strings.ToLower(providerName)) {
	case "paddleocr":
		opts, err := paddleocrOptions(m)
		if err != nil {
			return nil, err
		}
		return &paddleOCRProvider{client: paddleocr.New(opts)}, nil
	default:
		return nil, fmt.Errorf("unknown OCR provider: %s", providerName)
	}
}

func paddleocrOptions(m map[string]string) (paddleocr.Options, error) {
	baseURL := strings.TrimSpace(m["ocr_service_url"])
	if baseURL == "" {
		return paddleocr.Options{}, fmt.Errorf("paddleocr requires ocr_service_url")
	}

	opts := paddleocr.Options{
		BaseURL: baseURL,
	}

	timeoutStr := strings.TrimSpace(m["ocr_timeout_seconds"])
	if timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			opts.Timeout = time.Duration(timeout) * time.Second
		}
	}

	return opts, nil
}
