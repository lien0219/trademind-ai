package storagepublic

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
)

// EndToEndResult is returned after upload → probe → cleanup.
type EndToEndResult struct {
	ProbeResult
	ObjectKey   string `json:"objectKey,omitempty"`
	StorageKind string `json:"storageKind,omitempty"`
	TestDeleted bool   `json:"testDeleted"`
}

// TestEndToEnd uploads a tiny PNG, probes the public URL anonymously, then deletes the test object.
func TestEndToEnd(ctx context.Context, plain map[string]string) (EndToEndResult, error) {
	var out EndToEndResult
	if plain == nil {
		return out, verifyErr(CodePublicBaseMissing, "storage settings unavailable", nil)
	}
	pubBase := ResolvePublicBase(plain)
	if pubBase == "" {
		return EndToEndResult{ProbeResult: fail(CodePublicBaseMissing, "未配置图片公开访问地址", map[string]any{"field": "public_base"})}, nil
	}
	if !strings.Contains(pubBase, "://") {
		return EndToEndResult{ProbeResult: fail(CodePublicURLInvalid, "图片地址不是完整的公网 URL，请配置 HTTPS 域名", map[string]any{
			"publicBase": pubBase,
			"hint":       "相对路径（如 /static）仅适用于开发代理，抖店等外部平台无法访问",
		})}, nil
	}

	prov, kind, err := storage.NewFromPlain(plain)
	if err != nil {
		return out, fmt.Errorf("storage provider: %w", err)
	}
	out.StorageKind = kind

	data, err := testPNGBytes()
	if err != nil {
		return out, err
	}
	day := time.Now().UTC().Format("2006/01/02")
	objKey := fmt.Sprintf("%s/.trademind-public-probe-%s.png", day, uuid.NewString())
	if err := prov.Put(ctx, objKey, bytes.NewReader(data), int64(len(data)), "image/png"); err != nil {
		return out, fmt.Errorf("storage upload probe: %w", err)
	}
	out.ObjectKey = objKey

	pubURL, err := prov.GetURL(ctx, objKey)
	if err != nil {
		_ = prov.Delete(ctx, objKey)
		return out, fmt.Errorf("storage public url: %w", err)
	}

	out.ProbeResult = VerifyPublicURL(ctx, pubURL)
	if out.OK {
		out.Message = "图片存储可以被外部平台正常访问"
	} else if out.Message == "" {
		out.Message = "图片地址无法被外部平台访问，请检查公开访问域名、HTTPS 证书和 Bucket 权限"
	}

	_ = prov.Delete(ctx, objKey)
	out.TestDeleted = true
	return out, nil
}

func testPNGBytes() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 0x22, G: 0x88, B: 0xff, A: 0xff})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode probe png: %w", err)
	}
	return buf.Bytes(), nil
}
