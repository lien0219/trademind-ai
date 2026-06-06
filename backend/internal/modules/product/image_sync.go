package product

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
)

// SyncImagesBody selects which image groups to mirror into platform storage.
type SyncImagesBody struct {
	Scope string `json:"scope"` // all | main | detail
}

// SyncImagesResult summarizes mirror outcomes.
type SyncImagesResult struct {
	Synced  int      `json:"synced"`
	Skipped int      `json:"skipped"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

func isExternalCollectImageURL(url string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	if u == "" || strings.HasPrefix(u, "data:") {
		return false
	}
	return strings.Contains(u, "alicdn.com") ||
		strings.Contains(u, "tbcdn.cn") ||
		strings.Contains(u, "taobaocdn.com") ||
		strings.Contains(u, "1688.com") ||
		strings.Contains(u, "pinduoduo.com") ||
		strings.Contains(u, "yangkeduo.com") ||
		strings.Contains(u, "pddpic.com")
}

func extFromImageBytes(data []byte, rawURL string) (string, string, error) {
	ct := http.DetectContentType(data)
	switch {
	case strings.Contains(ct, "jpeg"):
		return ".jpg", "image/jpeg", nil
	case strings.Contains(ct, "png"):
		return ".png", "image/png", nil
	case strings.Contains(ct, "webp"):
		return ".webp", "image/webp", nil
	case strings.Contains(ct, "gif"):
		return ".gif", "image/gif", nil
	}
	ext := strings.ToLower(filepath.Ext(strings.Split(rawURL, "?")[0]))
	switch ext {
	case ".jpg", ".jpeg":
		return ".jpg", "image/jpeg", nil
	case ".png":
		return ".png", "image/png", nil
	case ".webp":
		return ".webp", "image/webp", nil
	case ".gif":
		return ".gif", "image/gif", nil
	default:
		return "", "", fmt.Errorf("unsupported image type")
	}
}

func (s *Service) fetchRemoteImage(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TradeMind-ImageSync/1.0")
	req.Header.Set("Accept", "image/*")
	cli := &http.Client{Timeout: 45 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	const max = 15 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("image too large")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	return data, nil
}

// SyncImages mirrors external product images into configured storage.
func (s *Service) SyncImages(c *gin.Context, productID uuid.UUID, body SyncImagesBody, adminID *uuid.UUID, filesSvc *files.Service) (*SyncImagesResult, error) {
	if s == nil || s.DB == nil || filesSvc == nil {
		return nil, fmt.Errorf("product: misconfigured")
	}
	scope := strings.TrimSpace(strings.ToLower(body.Scope))
	if scope == "" {
		scope = "all"
	}

	var prod Product
	if err := s.DB.WithContext(c.Request.Context()).Preload("Images").First(&prod, "id = ?", productID).Error; err != nil {
		return nil, err
	}

	out := &SyncImagesResult{}
	for _, im := range prod.Images {
		imgType := strings.TrimSpace(strings.ToLower(im.ImageType))
		if imgType == ImageTypeDescription {
			imgType = ImageTypeDetail
		}
		if scope == "main" && imgType != ImageTypeMain {
			out.Skipped++
			continue
		}
		if scope == "detail" && imgType != ImageTypeDetail {
			out.Skipped++
			continue
		}
		if strings.TrimSpace(im.ObjectKey) != "" {
			out.Skipped++
			continue
		}
		src := strings.TrimSpace(im.OriginURL)
		if src == "" {
			src = strings.TrimSpace(im.PublicURL)
		}
		if src == "" || !isExternalCollectImageURL(src) {
			out.Skipped++
			continue
		}

		data, err := s.fetchRemoteImage(c.Request.Context(), src)
		if err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}
		ext, ct, err := extFromImageBytes(data, src)
		if err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}
		day := time.Now().UTC().Format("2006/01/02")
		objKey := fmt.Sprintf("%s/sync-%s%s", day, uuid.NewString(), ext)
		rec, err := filesSvc.SaveProcessed(c.Request.Context(), files.SaveProcessedOpts{
			ObjectKey:   objKey,
			ContentType: ct,
			Data:        data,
			CreatedBy:   adminID,
		})
		if err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}

		updates := map[string]interface{}{
			"object_key":  rec.ObjectKey,
			"storage_key": rec.ObjectKey,
			"public_url":  rec.PublicURL,
			"origin_url":  src,
			"source":      "sync",
		}
		if err := s.DB.WithContext(c.Request.Context()).Model(&ProductImage{}).
			Where("id = ? AND product_id = ?", im.ID, productID).
			Updates(updates).Error; err != nil {
			out.Failed++
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", im.ID.String(), err))
			continue
		}
		out.Synced++
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.image.sync",
			Resource:    "product",
			ResourceID:  productID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("scope=%s synced=%d skipped=%d failed=%d", scope, out.Synced, out.Skipped, out.Failed),
		})
	}
	return out, nil
}
