package imagetask

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	imgprov "github.com/trademind-ai/trademind/backend/internal/providers/image"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr"
	"github.com/trademind-ai/trademind/backend/internal/providers/ocr/ocrerror"
)

type testImageBody struct {
	Provider string            `json:"provider"`
	TestMode string            `json:"testMode"`
	Settings map[string]string `json:"settings"`
}

type testOCRBody struct {
	Provider string            `json:"provider"`
	Settings map[string]string `json:"settings"`
}

type testOCRResult struct {
	Provider          string  `json:"provider"`
	OK                bool    `json:"ok"`
	Available         bool    `json:"available"`
	Message           string  `json:"message"`
	ErrorCode         string  `json:"errorCode,omitempty"`
	LatencyMs         int64   `json:"latencyMs"`
	Blocks            int     `json:"blocks"`
	BlocksCount       int     `json:"blocksCount"`
	AverageConfidence float64 `json:"averageConfidence,omitempty"`
	BBoxOK            bool    `json:"bboxOk"`
	TestMode          string  `json:"testMode"`
	ConfigHint        string  `json:"configHint,omitempty"`
}

// TestImage POST /api/v1/settings/test-image
// Optional JSON settings lets the admin test unsaved form values; empty body uses saved settings.image only.
func (h *Handler) TestImage(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Settings == nil {
		response.Fail(c, 500, response.CodeInternalError, "image settings unavailable")
		return
	}
	var body testImageBody
	_ = c.ShouldBindJSON(&body)
	m, err := h.Svc.Settings.PlainByGroup(c.Request.Context(), 0, "image")
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	m = MergeImagePlain(m, body.Settings)
	res := imgprov.TestConnection(c.Request.Context(), m, body.Provider, body.TestMode)
	if res != nil && !res.OK {
		if h.Svc.OpLog != nil {
			_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "test_image",
				Resource: "settings",
				Status:   "failed",
				Message:  res.Message,
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, res.Message)
		return
	}
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "test_image",
			Resource: "settings",
			Status:   "success",
		})
	}
	response.OK(c, res)
}

// TestOCR POST /api/v1/settings/test-ocr
// Tests OCR settings with a generated local sample image and does not persist form values.
func (h *Handler) TestOCR(c *gin.Context) {
	if h == nil || h.Svc == nil || h.Svc.Settings == nil {
		response.Fail(c, 500, response.CodeInternalError, "OCR 设置不可用")
		return
	}
	var body testOCRBody
	_ = c.ShouldBindJSON(&body)
	m, err := h.Svc.Settings.PlainByGroup(c.Request.Context(), 0, "image")
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, err.Error())
		return
	}
	m = MergeImagePlain(m, body.Settings)
	provider := strings.TrimSpace(strings.ToLower(body.Provider))
	if provider == "" {
		provider = strings.TrimSpace(strings.ToLower(m["ocr_provider"]))
	}
	if provider == "" {
		provider = "paddleocr"
	}

	start := time.Now()
	res := &testOCRResult{
		Provider: provider,
		TestMode: "sample_image",
	}
	fail := func(msg string) {
		code, humanMsg := classifyOCRError(errors.New(msg))
		if humanMsg == "" {
			humanMsg = msg
		}
		res.OK = false
		res.ErrorCode = code
		res.Message = humanMsg
		res.LatencyMs = time.Since(start).Milliseconds()
		if h.Svc.OpLog != nil {
			_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
				Action:   "test_ocr",
				Resource: "settings",
				Status:   "failed",
				Message:  humanMsg,
			})
		}
		response.Fail(c, 400, response.CodeBadRequest, humanMsg)
	}

	switch provider {
	case "ai_vision":
		if h.Svc.AIGateway == nil {
			fail("未配置 AI 服务，无法真实测试 AI 视觉 OCR。请先在「设置 → AI 设置」配置支持图片输入的视觉模型。")
			return
		}
		sample, _, _, err := sampleOCRImageBase64()
		if err != nil {
			fail("生成 OCR 测试图片失败，请稍后重试。")
			return
		}
		timeoutSec := comfyIntSetting(firstNonEmptyString(m["ocr_timeout_sec"], m["ocr_timeout_seconds"]), 30, 5, 120)
		oCtx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		content, err := h.Svc.chatVisionJSON(oCtx, ocrPromptBase("auto", "en", 0, 0), "data:image/png;base64,"+sample, 1200)
		if err != nil {
			fail("AI 视觉 OCR 真实调用失败：" + err.Error())
			return
		}
		parsed, err := parseOCRJSON(content)
		if err != nil || parsed == nil || len(parsed.Blocks) == 0 {
			fail("AI 视觉 OCR 已响应，但未返回可用文字 blocks。请确认当前 AI 模型支持图片识别和 JSON 输出。")
			return
		}
		res.OK = true
		res.Available = true
		res.Message = "当前 OCR 服务可用：AI 视觉 OCR 已真实识别测试图片。"
		res.Blocks = len(parsed.Blocks)
		res.BlocksCount = len(parsed.Blocks)
		res.LatencyMs = time.Since(start).Milliseconds()
		res.ConfigHint = "uses_ai_settings"
		response.OK(c, res)
		return
	case "paddleocr":
		if strings.TrimSpace(m["ocr_base_url"]) == "" && strings.TrimSpace(m["ocr_service_url"]) == "" && strings.TrimSpace(m["ocr_paddleocr_service_url"]) == "" {
			fail("请先填写 PaddleOCR 服务地址，例如 http://127.0.0.1:xxxx。")
			return
		}
		if err := testPaddleOCRHealth(c.Request.Context(), m); err != nil {
			fail("本地 PaddleOCR 未配置或不可用，请先启动 OCR 服务并测试通过：" + err.Error())
			return
		}
	case "tencent":
		if strings.TrimSpace(m["ocr_tencent_endpoint"]) == "" && strings.TrimSpace(m["ocr_endpoint"]) == "" {
			fail("请先填写腾讯云 OCR Endpoint。")
			return
		}
		if strings.TrimSpace(m["ocr_tencent_region"]) == "" && strings.TrimSpace(m["ocr_region"]) == "" {
			fail("请先填写腾讯云 OCR Region。")
			return
		}
		if strings.TrimSpace(m["ocr_tencent_secret_id"]) == "" && strings.TrimSpace(m["ocr_api_key"]) == "" {
			fail("请先填写腾讯云 OCR SecretId。")
			return
		}
		if strings.TrimSpace(m["ocr_tencent_secret_key"]) == "" && strings.TrimSpace(m["ocr_secret"]) == "" {
			fail("请先填写腾讯云 OCR SecretKey。")
			return
		}
	case "aliyun":
		if strings.TrimSpace(m["ocr_aliyun_endpoint"]) == "" {
			fail("请先填写阿里云 OCR Endpoint。")
			return
		}
		if strings.TrimSpace(m["ocr_aliyun_region"]) == "" {
			fail("请先填写阿里云 OCR Region。")
			return
		}
		if strings.TrimSpace(m["ocr_aliyun_access_key_id"]) == "" && strings.TrimSpace(m["ocr_api_key"]) == "" {
			fail("请先填写阿里云 AccessKeyId。")
			return
		}
		if strings.TrimSpace(m["ocr_aliyun_access_key_secret"]) == "" && strings.TrimSpace(m["ocr_secret"]) == "" {
			fail("请先填写阿里云 AccessKeySecret。")
			return
		}
	case "baidu":
		fail("不支持的 OCR 服务：baidu。")
		return
	default:
		fail("不支持的 OCR 服务，请选择 AI 视觉 OCR、本地 PaddleOCR、阿里云 OCR或腾讯云 OCR。")
		return
	}

	prov, err := ocr.NewProvider(provider, m)
	if err != nil {
		fail(err.Error())
		return
	}
	sample, width, height, err := sampleOCRImageBase64()
	if err != nil {
		fail("生成 OCR 测试图片失败，请稍后重试。")
		return
	}
	timeoutRaw := strings.TrimSpace(m["ocr_timeout_sec"])
	if timeoutRaw == "" {
		timeoutRaw = strings.TrimSpace(m["ocr_timeout_seconds"])
	}
	timeoutSec := comfyIntSetting(timeoutRaw, 30, 5, 120)
	oCtx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	ocrRes, err := prov.DetectText(oCtx, ocr.OCRRequest{
		ImageBase64: sample,
		ImageWidth:  width,
		ImageHeight: height,
	})
	if err != nil {
		fail(err.Error())
		return
	}
	if ocrRes == nil || len(ocrRes.Blocks) == 0 {
		if provider == "tencent" || provider == "aliyun" {
			fail("OCR 未检测到文字，请更换图片或降低最低置信度。")
			return
		}
		fail("OCR 服务已响应，但没有识别出测试图片文字 blocks；请确认服务使用的是文字检测 + 文字识别接口。")
		return
	}
	bboxOK := false
	confSum := 0.0
	confCount := 0
	for _, block := range ocrRes.Blocks {
		if strings.TrimSpace(block.Text) != "" && block.BBox.Width > 0 && block.BBox.Height > 0 {
			bboxOK = true
		}
		if block.Confidence > 0 {
			confSum += block.Confidence
			confCount++
		}
	}
	if !bboxOK {
		fail("OCR 服务返回了文字 blocks，但 bbox 坐标不完整；请确认返回 text_region/bbox 字段正常。")
		return
	}

	res.OK = true
	res.Available = true
	if provider == "tencent" {
		res.Message = "当前 OCR 服务可用：腾讯云 OCR 测试成功，已可用于图片文字翻译。"
	} else if provider == "aliyun" {
		res.Message = "当前 OCR 服务可用：阿里云 OCR 测试成功，已可用于图片文字翻译。"
	} else {
		res.Message = "当前 OCR 服务可用：服务可连接，能返回文字 blocks，bbox 坐标正常。"
	}
	res.Blocks = len(ocrRes.Blocks)
	res.BlocksCount = len(ocrRes.Blocks)
	if confCount > 0 {
		res.AverageConfidence = confSum / float64(confCount)
	}
	res.BBoxOK = true
	res.LatencyMs = time.Since(start).Milliseconds()
	if h.Svc.OpLog != nil {
		_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
			Action:   "test_ocr",
			Resource: "settings",
			Status:   "success",
		})
	}
	response.OK(c, res)
}

func testPaddleOCRHealth(ctx context.Context, m map[string]string) error {
	baseURL := firstNonEmptyString(m["ocr_base_url"], m["ocr_service_url"], m["ocr_paddleocr_service_url"])
	if strings.TrimSpace(baseURL) == "" {
		return ocrerror.New(ocrerror.CodeSecretMissing, "PaddleOCR 服务地址未配置")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ocrerror.New(ocrerror.CodeUnknown, "PaddleOCR /health 返回异常状态："+resp.Status)
	}
	return nil
}

func sampleOCRImageBase64() (string, int, int, error) {
	const scale = 6
	const glyphW = 5
	const glyphH = 7
	patterns := map[rune][]string{
		'O': {"01110", "10001", "10001", "10001", "10001", "10001", "01110"},
		'C': {"01111", "10000", "10000", "10000", "10000", "10000", "01111"},
		'R': {"11110", "10001", "10001", "11110", "10100", "10010", "10001"},
	}
	text := []rune("OCR")
	width := (glyphW*len(text)+len(text)-1)*scale + 32
	height := glyphH*scale + 32
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, white)
		}
	}
	x0, y0 := 16, 16
	for i, ch := range text {
		rows := patterns[ch]
		ox := x0 + i*(glyphW+1)*scale
		for gy, row := range rows {
			for gx, px := range row {
				if px != '1' {
					continue
				}
				for yy := 0; yy < scale; yy++ {
					for xx := 0; xx < scale; xx++ {
						img.SetRGBA(ox+gx*scale+xx, y0+gy*scale+yy, black)
					}
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", 0, 0, err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), width, height, nil
}
