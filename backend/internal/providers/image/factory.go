package image

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/providers/image/comfyui"
	"github.com/trademind-ai/trademind/backend/internal/providers/image/dashscopeimage"
	"github.com/trademind-ai/trademind/backend/internal/providers/image/openaiimage"
	"github.com/trademind-ai/trademind/backend/internal/providers/image/removebg"
)

func timeoutSecFromImageMap(m map[string]string) int {
	s := strings.TrimSpace(m["timeout_sec"])
	if s == "" {
		return 60
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 5 {
		return 60
	}
	if n > 600 {
		return 600
	}
	return n
}

func intFromImageSettings(m map[string]string, key string, def, minV, maxV int) int {
	s := strings.TrimSpace(m[key])
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < minV {
		return def
	}
	if n > maxV {
		return maxV
	}
	return n
}

// NewForTask builds an image Provider by name using decrypted settings.image when needed.
func NewForTask(ctx context.Context, providerName string, settingsSvc *settings.Service) (Provider, error) {
	name := strings.TrimSpace(strings.ToLower(providerName))
	switch name {
	case "noop":
		return NoopProvider{}, nil
	case "removebg":
		if settingsSvc == nil {
			return nil, fmt.Errorf("remove.bg provider requires settings service")
		}
		m, err := settingsSvc.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		key := strings.TrimSpace(m["removebg_api_key"])
		if key == "" {
			return nil, fmt.Errorf("removebg_api_key is not configured")
		}
		base := strings.TrimSpace(m["removebg_base_url"])
		if base == "" {
			base = "https://api.remove.bg/v1.0"
		}
		sec := timeoutSecFromImageMap(m)
		cli := removebg.NewClient(removebg.Options{
			APIKey:  key,
			BaseURL: base,
			Timeout: time.Duration(sec) * time.Second,
		})
		return removebgProvider{client: cli}, nil
	case "openai_image":
		if settingsSvc == nil {
			return nil, fmt.Errorf("openai Image provider requires settings service")
		}
		im, err := settingsSvc.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		// Dedicated key only — we intentionally do NOT fall back to settings.ai.api_key here
		// (narrower blast radius / billing clarity; explicit bridging can be added later if needed).
		key := strings.TrimSpace(im["openai_image_api_key"])
		if key == "" {
			return nil, fmt.Errorf("openai_image_api_key is not configured")
		}
		base := strings.TrimSpace(im["openai_image_base_url"])
		model := strings.TrimSpace(im["openai_image_model"])
		if model == "" {
			model = "gpt-image-1"
		}
		size := strings.TrimSpace(im["openai_image_size"])
		if size == "" {
			size = "1024x1024"
		}
		quality := strings.TrimSpace(im["openai_image_quality"])
		if quality == "" {
			quality = "standard"
		}
		background := strings.TrimSpace(im["openai_image_background"])
		sec := timeoutSecFromImageMap(im)
		cli, err := openaiimage.NewClient(openaiimage.Options{
			BaseURL:    base,
			APIKey:     key,
			Model:      model,
			Size:       size,
			Quality:    quality,
			Background: background,
			Timeout:    time.Duration(sec) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		return openaiImageProvider{client: cli}, nil
	case "comfyui":
		if settingsSvc == nil {
			return nil, fmt.Errorf("comfyui provider requires settings service")
		}
		im, err := settingsSvc.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		base := strings.TrimSpace(im["comfyui_base_url"])
		if base == "" {
			return nil, fmt.Errorf("comfyui_base_url is not configured")
		}
		wf := strings.TrimSpace(im["comfyui_workflow_json"])
		if wf == "" || wf == "{}" {
			return nil, fmt.Errorf("comfyui_workflow_json is not configured")
		}
		apiKey := strings.TrimSpace(im["comfyui_api_key"])
		httpSec := intFromImageSettings(im, "comfyui_timeout_sec", 180, 5, 3600)
		pollEvery := intFromImageSettings(im, "comfyui_poll_interval_seconds", 2, 1, 60)
		maxPoll := intFromImageSettings(im, "comfyui_max_poll_seconds", 180, 5, 7200)
		cli, err := comfyui.NewClient(comfyui.Options{
			BaseURL:      base,
			APIKey:       apiKey,
			WorkflowJSON: wf,
			PromptNodeID: strings.TrimSpace(im["comfyui_prompt_node_id"]),
			ImageNodeID:  strings.TrimSpace(im["comfyui_image_node_id"]),
			OutputNodeID: strings.TrimSpace(im["comfyui_output_node_id"]),
			HTTPTimeout:  time.Duration(httpSec) * time.Second,
			PollInterval: time.Duration(pollEvery) * time.Second,
			MaxPoll:      time.Duration(maxPoll) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		return comfyuiProvider{client: cli}, nil
	case "dashscope_image":
		if settingsSvc == nil {
			return nil, fmt.Errorf("dashscope_image provider requires settings service")
		}
		im, err := settingsSvc.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		opts, err := dashscopeOpts(im)
		if err != nil {
			return nil, err
		}
		cli, err := dashscopeimage.NewClient(opts)
		if err != nil {
			return nil, err
		}
		return dashscopeImageProvider{client: cli}, nil
	case "volcengine_image":
		if settingsSvc == nil {
			return nil, fmt.Errorf("volcengine_image provider requires settings service")
		}
		im, err := settingsSvc.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		opts, err := compatImageKeys("volcengine_image", im)
		if err != nil {
			return nil, err
		}
		if opts.Model == "" {
			opts.Model = "doubao-seedream-3-0-t2i"
		}
		if opts.BaseURL == "" {
			opts.BaseURL = "https://ark.cn-beijing.volces.com/api/v3"
		}
		cli, err := openaiimage.NewClient(opts)
		if err != nil {
			return nil, err
		}
		return compatGenerationsProvider{name: "volcengine_image", client: cli}, nil
	case "siliconflow_image":
		if settingsSvc == nil {
			return nil, fmt.Errorf("siliconflow_image provider requires settings service")
		}
		im, err := settingsSvc.PlainByGroup(ctx, 0, "image")
		if err != nil {
			return nil, err
		}
		opts, err := compatImageKeys("siliconflow_image", im)
		if err != nil {
			return nil, err
		}
		if opts.Model == "" {
			opts.Model = "black-forest-labs/FLUX.1-schnell"
		}
		if opts.BaseURL == "" {
			opts.BaseURL = "https://api.siliconflow.cn/v1"
		}
		cli, err := openaiimage.NewClient(opts)
		if err != nil {
			return nil, err
		}
		return compatGenerationsProvider{name: "siliconflow_image", client: cli}, nil
	case "hunyuan_image":
		return plannedProvider{name: "hunyuan_image"}, nil
	default:
		return nil, fmt.Errorf("unsupported image provider %q", name)
	}
}

type removebgProvider struct {
	client removebg.Client
}

func (p removebgProvider) Name() string { return "removebg" }

func (p removebgProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	var rdr io.Reader
	if req.SourceFile != nil {
		rdr = req.SourceFile
	}
	b, err := p.client.RemoveBackground(ctx, removebg.RemoveBackgroundInput{
		ImageURL:         strings.TrimSpace(req.SourceURL),
		Image:            rdr,
		ImageFilename:    strings.TrimSpace(req.SourceFilename),
		ImageContentType: strings.TrimSpace(req.SourceContentType),
	})
	if err != nil {
		return nil, err
	}
	return &ImageResult{
		RawPayload:         b,
		PayloadContentType: "image/png",
	}, nil
}

func (p removebgProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: replace_background not implemented")
}

func (p removebgProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: generate_scene not implemented")
}

func (p removebgProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: resize not implemented")
}

func (p removebgProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: enhance not implemented")
}

func (p removebgProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: translate_image not implemented")
}

func (p removebgProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove.bg: poster_generate not implemented")
}

type openaiImageProvider struct {
	client openaiimage.Client
}

func assembledScenePrompt(input map[string]any) string {
	if input == nil {
		return ""
	}
	switch v := input["assembled_prompt"].(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		if v != nil {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

func inputStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func (p openaiImageProvider) Name() string { return "openai_image" }

func (p openaiImageProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove_background is not implemented for openai_image (use provider removebg)")
}

func (p openaiImageProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	prompt := assembledScenePrompt(req.Input)
	if prompt == "" {
		return nil, fmt.Errorf("assembled prompt required for openai_image replace_background")
	}
	in := openaiimage.ReplaceBackgroundInput{
		Prompt:           prompt,
		ImageFilename:    strings.TrimSpace(req.SourceFilename),
		ImageContentType: strings.TrimSpace(req.SourceContentType),
		Size:             inputStr(req.Input, "size"),
		Quality:          inputStr(req.Input, "quality"),
		Model:            inputStr(req.Input, "model"),
	}
	if req.SourceFile != nil {
		in.Image = req.SourceFile
	} else {
		in.ImageURL = strings.TrimSpace(req.SourceURL)
	}
	img, ct, err := p.client.ReplaceBackground(ctx, in)
	if err != nil {
		return nil, err
	}
	return &ImageResult{
		RawPayload:         img,
		PayloadContentType: ct,
		Meta: map[string]any{
			"model":       p.client.ResolvedModel(),
			"contentType": ct,
		},
	}, nil
}

func (p openaiImageProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	prompt := assembledScenePrompt(req.Input)
	if prompt == "" {
		return nil, fmt.Errorf("assembled prompt required for generate_scene (service must set input.assembled_prompt)")
	}
	img, ct, err := p.client.GenerateScene(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return &ImageResult{
		RawPayload:         img,
		PayloadContentType: ct,
		Meta: map[string]any{
			"model":       p.client.ResolvedModel(),
			"contentType": ct,
		},
	}, nil
}

func (p openaiImageProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("openai_image: resize not implemented")
}

func (p openaiImageProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("openai_image: enhance not implemented")
}

func (p openaiImageProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("openai_image: translate_image not implemented")
}

func (p openaiImageProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("openai_image: poster_generate not implemented")
}

type comfyuiProvider struct {
	client *comfyui.Client
}

func (p comfyuiProvider) Name() string { return "comfyui" }

func (p comfyuiProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("remove_background is not implemented for comfyui (use provider removebg)")
}

func (p comfyuiProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	r, err := p.client.RunReplaceBackground(ctx, strings.TrimSpace(req.SourceURL), req.Input)
	if err != nil {
		return nil, err
	}
	return &ImageResult{
		RawPayload:         r.PNGBytes,
		PayloadContentType: "image/png",
		Meta:               r.Meta,
	}, nil
}

func (p comfyuiProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	r, err := p.client.RunGenerateScene(ctx, strings.TrimSpace(req.SourceURL), req.Input)
	if err != nil {
		return nil, err
	}
	return &ImageResult{
		RawPayload:         r.PNGBytes,
		PayloadContentType: "image/png",
		Meta:               r.Meta,
	}, nil
}

func (p comfyuiProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("comfyui: resize not implemented")
}

func (p comfyuiProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("comfyui: enhance not implemented")
}

func (p comfyuiProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("comfyui: translate_image not implemented")
}

func (p comfyuiProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, fmt.Errorf("comfyui: poster_generate not implemented")
}

type dashscopeImageProvider struct {
	client *dashscopeimage.Client
}

func (p dashscopeImageProvider) Name() string { return "dashscope_image" }

func (p dashscopeImageProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p dashscopeImageProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	prompt := assembledScenePrompt(req.Input)
	if prompt == "" {
		return nil, fmt.Errorf("assembled prompt required for dashscope_image edit")
	}
	imageRef, err := dashscopeImageRef(req.ImageRequest)
	if err != nil {
		return nil, err
	}
	img, ct, err := p.client.EditImage(ctx, imageRef, prompt)
	if err != nil {
		return nil, WrapProviderError(err)
	}
	return &ImageResult{
		RawPayload:         img,
		PayloadContentType: ct,
		Meta: map[string]any{
			"model":       p.client.ResolvedModel(),
			"contentType": ct,
		},
	}, nil
}

func (p dashscopeImageProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	prompt := assembledScenePrompt(req.Input)
	if prompt == "" {
		return nil, fmt.Errorf("assembled prompt required for generate_scene")
	}
	img, ct, err := p.client.GenerateScene(ctx, prompt)
	if err != nil {
		return nil, WrapProviderError(err)
	}
	return &ImageResult{
		RawPayload:         img,
		PayloadContentType: ct,
		Meta: map[string]any{
			"model":       p.client.ResolvedModel(),
			"contentType": ct,
		},
	}, nil
}

func (p dashscopeImageProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p dashscopeImageProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	prompt := assembledScenePrompt(req.Input)
	if prompt == "" {
		return nil, fmt.Errorf("assembled prompt required for dashscope_image edit")
	}
	imageRef, err := dashscopeImageRef(req)
	if err != nil {
		return nil, err
	}
	img, ct, err := p.client.EditImage(ctx, imageRef, prompt)
	if err != nil {
		return nil, WrapProviderError(err)
	}
	return &ImageResult{
		RawPayload:         img,
		PayloadContentType: ct,
		Meta: map[string]any{
			"model":       p.client.ResolvedModel(),
			"contentType": ct,
		},
	}, nil
}

func (p dashscopeImageProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p dashscopeImageProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

type compatGenerationsProvider struct {
	name   string
	client openaiimage.Client
}

func (p compatGenerationsProvider) Name() string { return p.name }

func (p compatGenerationsProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p compatGenerationsProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p compatGenerationsProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	prompt := assembledScenePrompt(req.Input)
	if prompt == "" {
		return nil, fmt.Errorf("assembled prompt required for generate_scene")
	}
	img, ct, err := p.client.GenerateScene(ctx, prompt)
	if err != nil {
		return nil, WrapProviderError(err)
	}
	return &ImageResult{
		RawPayload:         img,
		PayloadContentType: ct,
		Meta: map[string]any{
			"model":       p.client.ResolvedModel(),
			"contentType": ct,
		},
	}, nil
}

func (p compatGenerationsProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p compatGenerationsProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p compatGenerationsProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

func (p compatGenerationsProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, ErrTaskNotSupported
}

type plannedProvider struct {
	name string
}

func (p plannedProvider) Name() string { return p.name }

func (p plannedProvider) notReady() error {
	return fmt.Errorf("「%s」尚未开放，请更换其他图片 AI 服务", p.name)
}

func (p plannedProvider) RemoveBackground(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, p.notReady()
}

func (p plannedProvider) ReplaceBackground(ctx context.Context, req ReplaceBackgroundRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, p.notReady()
}

func (p plannedProvider) GenerateScene(ctx context.Context, req GenerateSceneRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, p.notReady()
}

func (p plannedProvider) Resize(ctx context.Context, req ResizeRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, p.notReady()
}

func (p plannedProvider) Enhance(ctx context.Context, req ImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, p.notReady()
}

func (p plannedProvider) TranslateImage(ctx context.Context, req TranslateImageRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, p.notReady()
}

func (p plannedProvider) PosterGenerate(ctx context.Context, req PosterGenerateRequest) (*ImageResult, error) {
	_, _ = ctx, req
	return nil, p.notReady()
}

// ProviderSupportsGenerateSceneNoSource reports providers that can run text-only scene generation.
func ProviderSupportsGenerateSceneNoSource(name string) bool {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "openai_image", "comfyui", "dashscope_image", "volcengine_image", "siliconflow_image":
		return true
	default:
		return false
	}
}

const maxDashScopeSourceBytes = 25 << 20

func dashscopeImageRef(req ImageRequest) (string, error) {
	if req.SourceFile != nil {
		data, err := io.ReadAll(io.LimitReader(req.SourceFile, maxDashScopeSourceBytes))
		if err != nil {
			return "", fmt.Errorf("dashscope_image: read source image: %w", err)
		}
		ct := strings.TrimSpace(req.SourceContentType)
		if ct == "" {
			ct = http.DetectContentType(data)
		}
		return dashscopeimage.DataURL(data, ct), nil
	}
	u := strings.TrimSpace(req.SourceURL)
	if u == "" {
		return "", fmt.Errorf("dashscope_image: source image required")
	}
	return u, nil
}
