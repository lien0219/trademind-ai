package ocr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/aliyun"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/ocrerror"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/paddleocr"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/tencent"
)

type paddleOCRProvider struct {
	client *paddleocr.Client
}

type tencentOCRProvider struct {
	client *tencent.Client
}

type aliyunOCRProvider struct {
	client *aliyun.Client
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

func (p *tencentOCRProvider) DetectText(ctx context.Context, req OCRRequest) (*OCRResult, error) {
	res, err := p.client.DetectText(ctx, tencent.DetectRequest{
		ImageURL:          req.ImageURL,
		ImageBase64:       req.ImageBase64,
		LocalPath:         req.LocalPath,
		DetectOrientation: req.DetectOrientation,
	})
	if err != nil {
		return nil, err
	}
	blocks := make([]OCRBlock, 0, len(res.Blocks))
	for _, b := range res.Blocks {
		polygon := make([]OCRPoint, 0, len(b.Polygon))
		for _, p := range b.Polygon {
			polygon = append(polygon, OCRPoint{X: p.X, Y: p.Y})
		}
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
			Polygon:   polygon,
			Angle:     b.Angle,
			Direction: b.Direction,
		})
	}
	return &OCRResult{
		Provider:            res.Provider,
		APIName:             res.APIName,
		DetectedLanguage:    res.DetectedLanguage,
		Blocks:              blocks,
		FilteredBlocksCount: res.FilteredBlocksCount,
		Raw:                 res.Raw,
	}, nil
}

func (p *aliyunOCRProvider) DetectText(ctx context.Context, req OCRRequest) (*OCRResult, error) {
	res, err := p.client.DetectText(ctx, aliyun.DetectRequest{
		ImageURL:    req.ImageURL,
		ImageBase64: req.ImageBase64,
		LocalPath:   req.LocalPath,
	})
	if err != nil {
		return nil, err
	}
	blocks := make([]OCRBlock, 0, len(res.Blocks))
	for _, b := range res.Blocks {
		polygon := make([]OCRPoint, 0, len(b.Polygon))
		for _, p := range b.Polygon {
			polygon = append(polygon, OCRPoint{X: p.X, Y: p.Y})
		}
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
			Polygon:   polygon,
			Angle:     b.Angle,
			Direction: b.Direction,
		})
	}
	return &OCRResult{
		Provider:            res.Provider,
		APIName:             res.APIName,
		DetectedLanguage:    res.DetectedLanguage,
		Blocks:              blocks,
		FilteredBlocksCount: res.FilteredBlocksCount,
		Raw:                 res.Raw,
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
	case "tencent":
		opts, err := tencentOptions(m)
		if err != nil {
			return nil, err
		}
		return &tencentOCRProvider{client: tencent.New(opts)}, nil
	case "aliyun":
		opts, err := aliyunOptions(m)
		if err != nil {
			return nil, err
		}
		cli, err := aliyun.New(opts)
		if err != nil {
			return nil, err
		}
		return &aliyunOCRProvider{client: cli}, nil
	case "baidu":
		return nil, fmt.Errorf("不支持的 OCR 服务：baidu")
	case "ai_vision":
		return nil, fmt.Errorf("AI 视觉 OCR 由 AI Gateway 执行，无需外部 OCR Provider")
	default:
		return nil, fmt.Errorf("unknown OCR provider: %s", providerName)
	}
}

func paddleocrOptions(m map[string]string) (paddleocr.Options, error) {
	baseURL := strings.TrimSpace(m["ocr_base_url"])
	if baseURL == "" {
		baseURL = strings.TrimSpace(m["ocr_service_url"])
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(m["ocr_paddleocr_service_url"])
	}
	if baseURL == "" {
		return paddleocr.Options{}, fmt.Errorf("PaddleOCR 服务地址不能为空，请在「设置 → 图片 AI 设置」填写 OCR 服务地址")
	}

	opts := paddleocr.Options{
		BaseURL: baseURL,
	}

	timeoutStr := strings.TrimSpace(m["ocr_timeout_sec"])
	if timeoutStr == "" {
		timeoutStr = strings.TrimSpace(m["ocr_timeout_seconds"])
	}
	if timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			opts.Timeout = time.Duration(timeout) * time.Second
		}
	}

	return opts, nil
}

func tencentOptions(m map[string]string) (tencent.Options, error) {
	endpoint := firstNonEmpty(m["ocr_tencent_endpoint"], m["ocr_endpoint"], "ocr.tencentcloudapi.com")
	region := firstNonEmpty(m["ocr_tencent_region"], m["ocr_region"], "ap-guangzhou")
	apiName := firstNonEmpty(m["ocr_tencent_api_name"], m["ocr_api_name"], "GeneralBasicOCR")
	secretID := firstNonEmpty(m["ocr_tencent_secret_id"], m["ocr_api_key"])
	secretKey := firstNonEmpty(m["ocr_tencent_secret_key"], m["ocr_secret"])
	if strings.TrimSpace(endpoint) == "" {
		return tencent.Options{}, fmt.Errorf("腾讯云 OCR Endpoint 不能为空")
	}
	if strings.TrimSpace(region) == "" {
		return tencent.Options{}, fmt.Errorf("腾讯云 OCR Region 不能为空")
	}
	if strings.TrimSpace(secretID) == "" {
		return tencent.Options{}, ocrerror.New(ocrerror.CodeSecretMissing, "腾讯云 OCR SecretId 未配置，请填写后再测试")
	}
	if strings.TrimSpace(secretKey) == "" {
		return tencent.Options{}, ocrerror.New(ocrerror.CodeSecretMissing, "腾讯云 OCR SecretKey 未配置，请填写后再测试")
	}

	opts := tencent.Options{
		Endpoint:  endpoint,
		Region:    region,
		SecretID:  secretID,
		SecretKey: secretKey,
		APIName:   apiName,
	}
	timeoutStr := firstNonEmpty(m["ocr_timeout_sec"], m["ocr_timeout_seconds"])
	if timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			opts.Timeout = time.Duration(timeout) * time.Second
		}
	}
	minConfidence := strings.TrimSpace(m["ocr_min_confidence"])
	if minConfidence != "" {
		if v, err := strconv.ParseFloat(minConfidence, 64); err == nil && v > 0 {
			opts.MinConfidence = v
		}
	}
	return opts, nil
}

func aliyunOptions(m map[string]string) (aliyun.Options, error) {
	endpoint := firstNonEmpty(m["ocr_aliyun_endpoint"], "ocr-api.cn-hangzhou.aliyuncs.com")
	region := firstNonEmpty(m["ocr_aliyun_region"], "cn-hangzhou")
	apiName := firstNonEmpty(m["ocr_aliyun_api_name"], "RecognizeGeneral")
	accessKeyID := firstNonEmpty(m["ocr_aliyun_access_key_id"], m["ocr_api_key"])
	accessKeySecret := firstNonEmpty(m["ocr_aliyun_access_key_secret"], m["ocr_secret"])
	if strings.TrimSpace(endpoint) == "" {
		return aliyun.Options{}, fmt.Errorf("阿里云 OCR Endpoint 不能为空")
	}
	if strings.TrimSpace(region) == "" {
		return aliyun.Options{}, fmt.Errorf("阿里云 OCR Region 不能为空")
	}
	if strings.TrimSpace(accessKeyID) == "" {
		return aliyun.Options{}, ocrerror.New(ocrerror.CodeSecretMissing, "阿里云 AccessKeyId 未配置，请填写后再测试")
	}
	if strings.TrimSpace(accessKeySecret) == "" {
		return aliyun.Options{}, ocrerror.New(ocrerror.CodeSecretMissing, "阿里云 AccessKeySecret 未配置，请填写后再测试")
	}
	opts := aliyun.Options{
		Endpoint:        endpoint,
		Region:          region,
		AccessKeyID:     accessKeyID,
		AccessKeySecret: accessKeySecret,
		APIName:         apiName,
	}
	timeoutStr := firstNonEmpty(m["ocr_timeout_sec"], m["ocr_timeout_seconds"])
	if timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			opts.Timeout = time.Duration(timeout) * time.Second
		}
	}
	minConfidence := strings.TrimSpace(m["ocr_min_confidence"])
	if minConfidence != "" {
		if v, err := strconv.ParseFloat(minConfidence, 64); err == nil && v > 0 {
			opts.MinConfidence = v
		}
	}
	return opts, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
