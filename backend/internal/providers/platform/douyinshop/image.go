package douyinshop

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const MethodMaterialBatchUploadImageSync = "supplyCenter.material.batchUploadImageSync"

type UploadImageRequest struct {
	ImageType string
	FileName  string
	MimeType  string
	Reader    io.Reader
	SourceURL string
}

type PlatformImage struct {
	ImageID string         `json:"imageId"`
	URL     string         `json:"url,omitempty"`
	Raw     map[string]any `json:"raw,omitempty"`
}

// UploadImage uploads one image to the Douyin Shop material center.
//
// Official Douyin OpenAPI documentation checked for Phase 6:
// supplyCenter.material.batchUploadImageSync uploads up to 50 image materials
// per request from a public URL or file_uri and returns material_id.
func (c *Client) UploadImage(ctx context.Context, shopID string, req UploadImageRequest) (*PlatformImage, error) {
	sourceURL := strings.TrimSpace(req.SourceURL)
	if sourceURL == "" {
		return nil, fmt.Errorf("douyin image source url required")
	}
	name := strings.TrimSpace(req.FileName)
	if name == "" {
		name = "trademind-image"
	}
	params := map[string]any{
		"materials": []map[string]any{{
			"material_type": "photo",
			"name":          name,
			"url":           sourceURL,
			"need_distinct": false,
		}},
	}
	var data map[string]any
	if err := c.Do(ctx, MethodMaterialBatchUploadImageSync, params, &data); err != nil {
		return nil, err
	}
	item := firstMaterialUploadItem(data)
	id := firstNonEmpty(
		pickString(item, "material_id", "materialId", "image_id", "imageId", "id"),
		pickString(data, "material_id", "materialId", "image_id", "imageId", "id"),
	)
	url := firstNonEmpty(
		pickString(item, "url", "byte_url", "byteUrl", "image_url", "imageUrl"),
		pickString(data, "url", "byte_url", "byteUrl", "image_url", "imageUrl"),
	)
	if id == "" {
		return nil, NewError(CodeDouyinResponseParseFailed, "douyin image upload response missing material id", "", "", "")
	}
	return &PlatformImage{ImageID: id, URL: url, Raw: sanitizeRawMap(data)}, nil
}

func firstMaterialUploadItem(data map[string]any) map[string]any {
	if data == nil {
		return nil
	}
	for _, key := range []string{"materials", "material_list", "materialList", "list", "images", "image_list"} {
		arr, ok := data[key].([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		if item, ok := arr[0].(map[string]any); ok {
			return item
		}
	}
	if item, ok := data["result"].(map[string]any); ok {
		return item
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
