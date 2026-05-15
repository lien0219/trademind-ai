package files

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
	"gorm.io/gorm"
)

// Service handles uploads and file metadata.
type Service struct {
	DB       *gorm.DB
	Settings *settings.Service
	MaxBytes int64
}

// UploadResult is returned to the HTTP layer after a successful upload.
type UploadResult struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ObjectKey   string `json:"objectKey"`
	URL         string `json:"url"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

// Upload reads multipart bytes, stores via Provider, persists metadata.
func (s *Service) Upload(c *gin.Context, originalName string, r io.Reader) (*UploadResult, error) {
	if s == nil || s.DB == nil || s.Settings == nil {
		return nil, fmt.Errorf("files: misconfigured")
	}
	reqCtx := c.Request.Context()
	max := s.MaxBytes
	if max <= 0 {
		max = 10 << 20
	}
	limited := io.LimitReader(r, max+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("files: read upload: %w", err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("file exceeds maximum size (%d bytes)", max)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	ct := http.DetectContentType(data)
	ext, ok := extForContentType(ct)
	if !ok {
		ext2, ok2 := extFromOriginalName(originalName)
		if !ok2 {
			return nil, fmt.Errorf("only jpg, jpeg, png, webp, gif images are allowed")
		}
		if ct != "application/octet-stream" {
			return nil, fmt.Errorf("file content is not a recognized image")
		}
		ext = ext2
		ct = mimeTypeForExt(ext)
	}

	var adminID *uuid.UUID
	if idStr, ok := c.Get(ctxkey.AdminID); ok {
		if sub, ok := idStr.(string); ok {
			if u, err := uuid.Parse(sub); err == nil {
				adminID = &u
			}
		}
	}

	plain, err := s.Settings.PlainByGroup(reqCtx, 0, "storage")
	if err != nil {
		return nil, err
	}
	prov, kind, err := storage.NewFromPlain(plain)
	if err != nil {
		return nil, err
	}

	day := time.Now().UTC().Format("2006/01/02")
	objKey := fmt.Sprintf("%s/%s%s", day, uuid.NewString(), ext)

	if err := prov.Put(reqCtx, objKey, bytes.NewReader(data), int64(len(data)), ct); err != nil {
		return nil, err
	}
	pubURL, err := prov.GetURL(reqCtx, objKey)
	if err != nil {
		_ = prov.Delete(reqCtx, objKey)
		return nil, err
	}

	row := &FileRecord{
		OriginalName: strings.TrimSpace(originalName),
		ObjectKey:    objKey,
		PublicURL:    pubURL,
		ContentType:  ct,
		Size:         int64(len(data)),
		StorageKind:  kind,
		CreatedBy:    adminID,
	}
	if err := s.DB.WithContext(reqCtx).Create(row).Error; err != nil {
		_ = prov.Delete(reqCtx, objKey)
		return nil, err
	}

	return &UploadResult{
		ID:          row.ID.String(),
		Filename:    row.OriginalName,
		ObjectKey:   row.ObjectKey,
		URL:         row.PublicURL,
		ContentType: row.ContentType,
		Size:        row.Size,
	}, nil
}

func extForContentType(ct string) (string, bool) {
	base := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	switch base {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	case "image/gif":
		return ".gif", true
	default:
		return "", false
	}
}

func extFromOriginalName(name string) (string, bool) {
	e := strings.ToLower(filepath.Ext(name))
	switch e {
	case ".jpg", ".jpeg":
		return ".jpg", true
	case ".png":
		return ".png", true
	case ".webp":
		return ".webp", true
	case ".gif":
		return ".gif", true
	default:
		return "", false
	}
}

func mimeTypeForExt(ext string) string {
	switch ext {
	case ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return mime.TypeByExtension(ext)
	}
}

// ListQuery binds list filters.
type ListQuery struct {
	Page        int
	PageSize    int
	ContentType string
}

// ListResult is paginated file rows.
type ListResult struct {
	Items      []FileRecord
	Total      int64
	Page       int
	PageSize   int
	TotalPages int
}

// List returns paginated file metadata.
func (s *Service) List(c *gin.Context, q ListQuery) (*ListResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("files: no db")
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	ps := q.PageSize
	if ps < 1 {
		ps = 20
	}
	if ps > 100 {
		ps = 100
	}
	tx := s.DB.WithContext(c.Request.Context()).Model(&FileRecord{})
	if v := strings.TrimSpace(q.ContentType); v != "" {
		tx = tx.Where("content_type = ?", v)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, err
	}
	offset := (page - 1) * ps
	var items []FileRecord
	if err := tx.Order("created_at DESC").Offset(offset).Limit(ps).Find(&items).Error; err != nil {
		return nil, err
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return &ListResult{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   ps,
		TotalPages: pages,
	}, nil
}

// Delete removes DB metadata and the stored object when using a supported provider.
func (s *Service) Delete(c *gin.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil || s.Settings == nil {
		return fmt.Errorf("files: misconfigured")
	}
	var row FileRecord
	if err := s.DB.WithContext(c.Request.Context()).Where("id = ?", id).First(&row).Error; err != nil {
		return err
	}
	plain, err := s.Settings.PlainByGroup(c.Request.Context(), 0, "storage")
	if err != nil {
		return err
	}
	prov, _, err := storage.NewFromPlain(plain)
	if err != nil {
		return err
	}
	if err := prov.Delete(c.Request.Context(), row.ObjectKey); err != nil {
		return err
	}
	return s.DB.WithContext(c.Request.Context()).Delete(&FileRecord{}, "id = ?", id).Error
}
