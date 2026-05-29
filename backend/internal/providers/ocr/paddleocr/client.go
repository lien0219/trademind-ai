package paddleocr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Options struct {
	BaseURL string
	Timeout time.Duration
}

type Client struct {
	opts       Options
	httpClient *http.Client
}

func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 60 * time.Second
	}
	return &Client{
		opts: opts,
		httpClient: &http.Client{
			Timeout: opts.Timeout,
		},
	}
}

type paddleOCRRequest struct {
	Images []string `json:"images"`
}

type paddleOCRResponse struct {
	Code int                     `json:"code"`
	Msg  string                  `json:"msg"`
	Data [][]paddleOCRResultItem `json:"data"`
}

type paddleOCRResultItem struct {
	Text       string      `json:"text"`
	Confidence float64     `json:"confidence"`
	TextRegion [][]float64 `json:"text_region"`
}

type DetectRequest struct {
	ImageBase64 string
}

type DetectBBox struct {
	X      int
	Y      int
	Width  int
	Height int
}

type DetectBlock struct {
	ID         string
	Text       string
	Confidence float64
	BBox       DetectBBox
	Direction  string
}

type DetectResult struct {
	Blocks []DetectBlock
}

func (c *Client) DetectText(ctx context.Context, req DetectRequest) (*DetectResult, error) {
	if c.opts.BaseURL == "" {
		return nil, fmt.Errorf("paddleocr base url is required")
	}

	base64Data := req.ImageBase64
	if base64Data == "" {
		return nil, fmt.Errorf("paddleocr requires image base64 data")
	}

	apiReq := paddleOCRRequest{
		Images: []string{base64Data},
	}

	reqBody, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("paddleocr encode request: %w", err)
	}

	endpoint := c.opts.BaseURL + "/predict/ocr_system"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("paddleocr create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("paddleocr request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paddleocr unexpected status: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("paddleocr read response: %w", err)
	}

	var apiResp paddleOCRResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("paddleocr decode response: %w", err)
	}

	if apiResp.Code != 0 && apiResp.Code != 200 && apiResp.Code != 2000 {
		if apiResp.Msg == "" && apiResp.Code == 0 {
			// Some versions don't return code, but data is present
		} else {
			return nil, fmt.Errorf("paddleocr api error (code %d): %s", apiResp.Code, apiResp.Msg)
		}
	}

	if len(apiResp.Data) == 0 {
		return &DetectResult{
			Blocks: []DetectBlock{},
		}, nil
	}

	blocks := make([]DetectBlock, 0, len(apiResp.Data[0]))
	for i, item := range apiResp.Data[0] {
		// Calculate BBox from TextRegion which is [[x1, y1], [x2, y2], [x3, y3], [x4, y4]]
		if len(item.TextRegion) < 4 {
			continue
		}

		minX, minY := item.TextRegion[0][0], item.TextRegion[0][1]
		maxX, maxY := item.TextRegion[0][0], item.TextRegion[0][1]

		for _, point := range item.TextRegion[1:] {
			if point[0] < minX {
				minX = point[0]
			}
			if point[0] > maxX {
				maxX = point[0]
			}
			if point[1] < minY {
				minY = point[1]
			}
			if point[1] > maxY {
				maxY = point[1]
			}
		}

		blocks = append(blocks, DetectBlock{
			ID:         fmt.Sprintf("paddle_%d", i),
			Text:       item.Text,
			Confidence: item.Confidence,
			BBox: DetectBBox{
				X:      int(minX),
				Y:      int(minY),
				Width:  int(maxX - minX),
				Height: int(maxY - minY),
			},
			Direction: "horizontal",
		})
	}

	return &DetectResult{
		Blocks: blocks,
	}, nil
}
