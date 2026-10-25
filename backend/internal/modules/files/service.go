package files

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/metrics"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/security"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage/keypath"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"golang.org/x/image/webp"
	"gorm.io/gorm"
)

// Service handles uploads and file metadata.
type Service struct {
	DB       *gorm.DB
	Redis    *rdb.Client
	Settings *settings.Service
	MaxBytes int64
	Metrics  *metrics.Catalog
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

	if _, _, decErr := image.DecodeConfig(bytes.NewReader(data)); decErr != nil {
		if _, werr := webp.DecodeConfig(bytes.NewReader(data)); werr != nil {
			return nil, fmt.Errorf("file is not a decodable image")
		}
	}
	if strings.Contains(objKeyPath(originalName), "..") {
		return nil, fmt.Errorf("invalid filename")
	}

	var adminID *uuid.UUID
	if idStr, ok := c.Get(ctxkey.AdminID); ok {
		if sub, ok := idStr.(string); ok {
			if u, err := uuid.Parse(sub); err == nil {
				adminID = &u
			}
		}
	}

	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
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
	objKey := fmt.Sprintf("t%d/%s/%s%s", tid, day, uuid.NewString(), ext)
	if strings.Contains(objKey, "..") {
		return nil, fmt.Errorf("invalid object key")
	}

	if err := prov.Put(reqCtx, objKey, bytes.NewReader(data), int64(len(data)), ct); err != nil {
		return nil, err
	}
	pubURL, err := prov.GetURL(reqCtx, objKey)
	if err != nil {
		_ = prov.Delete(reqCtx, objKey)
		return nil, err
	}

	row := &FileRecord{
		TenantID:       tid,
		OriginalName:   strings.TrimSpace(originalName),
		ObjectKey:      objKey,
		PublicURL:      pubURL,
		ContentType:    ct,
		Size:           int64(len(data)),
		StorageKind:    kind,
		SecurityStatus: SecurityPendingScan,
		ScanStatus:     SecurityPendingScan,
		CreatedBy:      adminID,
	}
	if err := s.DB.WithContext(reqCtx).Create(row).Error; err != nil {
		_ = prov.Delete(reqCtx, objKey)
		return nil, err
	}
	_ = s.EnqueueSecurityScan(reqCtx, tid, row.ID)

	return &UploadResult{
		ID:          row.ID.String(),
		Filename:    row.OriginalName,
		ObjectKey:   row.ObjectKey,
		URL:         row.PublicURL,
		ContentType: row.ContentType,
		Size:        row.Size,
	}, nil
}

// SaveProcessedOpts writes arbitrary bytes via the configured Storage Provider (e.g. AI pipeline output).
type SaveProcessedOpts struct {
	OriginalName string
	ObjectKey    string
	Data         []byte
	ContentType  string
	TenantID     int64
	CreatedBy    *uuid.UUID
}

// SaveProcessed stores bytes under ObjectKey and persists a files row (same path as multipart Upload).
func (s *Service) SaveProcessed(ctx context.Context, opts SaveProcessedOpts) (*FileRecord, error) {
	if s == nil || s.DB == nil || s.Settings == nil {
		return nil, fmt.Errorf("files: misconfigured")
	}
	max := s.MaxBytes
	if max <= 0 {
		max = 10 << 20
	}
	data := opts.Data
	if int64(len(data)) > max {
		return nil, fmt.Errorf("file exceeds maximum size (%d bytes)", max)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	objKey := strings.TrimSpace(opts.ObjectKey)
	if objKey == "" {
		return nil, fmt.Errorf("objectKey required")
	}
	ct := strings.TrimSpace(opts.ContentType)
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	tenantID, err := resolveFileTenant(ctx, opts.TenantID)
	if err != nil {
		return nil, err
	}
	objKey, err = tenantObjectKey(tenantID, objKey)
	if err != nil {
		return nil, err
	}

	plain, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return nil, err
	}
	prov, kind, err := storage.NewFromPlain(plain)
	if err != nil {
		return nil, err
	}

	if err := prov.Put(ctx, objKey, bytes.NewReader(data), int64(len(data)), ct); err != nil {
		return nil, err
	}
	pubURL, err := prov.GetURL(ctx, objKey)
	if err != nil {
		_ = prov.Delete(ctx, objKey)
		return nil, err
	}

	row := &FileRecord{
		TenantID:     tenantID,
		OriginalName: strings.TrimSpace(opts.OriginalName),
		ObjectKey:    objKey,
		PublicURL:    pubURL,
		ContentType:  ct,
		Size:         int64(len(data)),
		StorageKind:  kind,
		CreatedBy:    opts.CreatedBy,
	}
	if err := s.DB.WithContext(ctx).Create(row).Error; err != nil {
		_ = prov.Delete(ctx, objKey)
		return nil, err
	}
	return row, nil
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

func objKeyPath(name string) string {
	return strings.ReplaceAll(filepath.Clean("/"+name), "\\", "/")
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
	if scoped, _, err := adminperm.ApplyTenantScope(c, tx); err != nil {
		return nil, err
	} else {
		tx = scoped
	}
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
	return s.DeleteRecordByTenant(c, id)
}

// DeleteRecordByID removes a file row and its storage object.
func (s *Service) DeleteRecordByID(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil || s.Settings == nil {
		return fmt.Errorf("files: misconfigured")
	}
	var row FileRecord
	tc, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	query := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tc.TenantID)
	if err := query.First(&row).Error; err != nil {
		return err
	}
	return s.deleteRecord(ctx, &row)
}

// DeleteRecordByTenant removes a tenant-scoped file row.
func (s *Service) DeleteRecordByTenant(c *gin.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil || s.Settings == nil {
		return fmt.Errorf("files: misconfigured")
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var row FileRecord
	if err := repository.FindByID(c.Request.Context(), s.DB, &row, tid, id); err != nil {
		return err
	}
	return s.deleteRecord(c.Request.Context(), &row)
}

func (s *Service) deleteRecord(ctx context.Context, row *FileRecord) error {
	tc, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return err
	}
	prov, _, err := storage.NewFromPlainForStoredKind(plain, strings.TrimSpace(row.StorageKind))
	if err != nil {
		return err
	}
	if err := prov.Delete(ctx, row.ObjectKey); err != nil {
		return err
	}
	return s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", row.ID, tc.TenantID).Delete(&FileRecord{}).Error
}

// DeleteStorageObject removes an object from configured storage (ignores empty keys).
func (s *Service) DeleteStorageObject(ctx context.Context, objectKey string) error {
	objectKey = strings.TrimSpace(objectKey)
	if s == nil || s.Settings == nil || objectKey == "" {
		return nil
	}
	tc, err := security.RequireTenantContext(ctx)
	if err != nil {
		return err
	}
	if s.DB == nil {
		return fmt.Errorf("files: tenant-scoped storage deletion requires db")
	}
	var row FileRecord
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND object_key = ?", tc.TenantID, objectKey).First(&row).Error; err != nil {
		return err
	}
	plain, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return err
	}
	prov, _, err := storage.NewFromPlain(plain)
	if err != nil {
		return err
	}
	return prov.Delete(ctx, objectKey)
}

func resolveFileTenant(ctx context.Context, requested int64) (int64, error) {
	tc := security.FromContext(ctx)
	if requested > 0 {
		if tc != nil && tc.TenantID > 0 && tc.TenantID != requested {
			return 0, security.ErrTaskTenantMismatch
		}
		return requested, nil
	}
	if tc != nil {
		if tc.TenantID <= 0 {
			return 0, security.ErrTenantContextMissing
		}
		return tc.TenantID, nil
	}
	// Existing SQLite-only unit tests create standalone file rows without an
	// auth context. Production upload and worker paths always carry a tenant.
	return 0, nil
}

func tenantObjectKey(tenantID int64, objectKey string) (string, error) {
	if tenantID < 0 {
		return "", security.ErrTenantContextMissing
	}
	key, err := keypath.NormalizeSafe(objectKey)
	if err != nil {
		return "", err
	}
	if tenantID == 0 {
		return key, nil
	}
	prefix := fmt.Sprintf("t%d/", tenantID)
	if strings.HasPrefix(key, prefix) {
		return key, nil
	}
	if strings.HasPrefix(key, "t") {
		first := strings.SplitN(key, "/", 2)[0]
		if len(first) > 1 && first[1:] != "" && strings.Trim(first[1:], "0123456789") == "" {
			return "", fmt.Errorf("files: object key belongs to another tenant")
		}
	}
	return prefix + key, nil
}
