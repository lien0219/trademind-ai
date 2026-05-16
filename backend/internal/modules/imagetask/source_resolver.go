package imagetask

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/files"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
)

func sanitizeStorageObjectKey(raw string) (string, error) {
	s := strings.Trim(strings.ReplaceAll(raw, "\\", "/"), "/")
	if s == "" {
		return "", fmt.Errorf("invalid object key")
	}
	if strings.Contains(s, "..") {
		return "", fmt.Errorf("invalid object key")
	}
	if strings.HasPrefix(s, "/") {
		return "", fmt.Errorf("invalid object key")
	}
	return s, nil
}

func normalizedSettingsKind(sm map[string]string) string {
	k := strings.TrimSpace(strings.ToLower(sm["kind"]))
	if k == "" {
		return "local"
	}
	return k
}

// staticURLToObjectKey maps /static/... references to a storage object key when storage is local
// and the URL is path-only or loopback with /static/ path.
func staticURLToObjectKey(raw string, storageKind string) (string, bool) {
	if strings.ToLower(strings.TrimSpace(storageKind)) != "local" {
		return "", false
	}
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", false
	}
	if !strings.Contains(u, "://") {
		p := strings.TrimPrefix(u, "/")
		if !strings.HasPrefix(strings.ToLower(p), "static/") {
			return "", false
		}
		rest := strings.TrimPrefix(p, "static/")
		rest = strings.TrimPrefix(rest, "/")
		key, err := sanitizeStorageObjectKey(rest)
		if err != nil {
			return "", false
		}
		return key, true
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed == nil {
		return "", false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host != "" && host != "localhost" && host != "127.0.0.1" && host != "::1" {
		return "", false
	}
	path := strings.Trim(strings.TrimSpace(parsed.Path), "/")
	if !strings.HasPrefix(strings.ToLower(path), "static/") {
		return "", false
	}
	rest := strings.TrimPrefix(path, "static/")
	rest = strings.TrimPrefix(rest, "/")
	key, err := sanitizeStorageObjectKey(rest)
	if err != nil {
		return "", false
	}
	return key, true
}

func filenameForObjectKey(objectKey string, fallbackOriginal string) string {
	k := strings.Trim(strings.ReplaceAll(objectKey, "\\", "/"), "/")
	base := filepath.Base(k)
	if base != "" && base != "." && base != "/" {
		return base
	}
	o := strings.TrimSpace(fallbackOriginal)
	if o != "" {
		if b := filepath.Base(o); b != "" && b != "." {
			return b
		}
	}
	return "source.png"
}

func contentTypeForFilename(filename string, dbCT string) string {
	if ct := strings.TrimSpace(dbCT); ct != "" {
		return ct
	}
	if ext := filepath.Ext(filename); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}
	return ""
}

func isNotFoundLikeErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "file not found") || strings.Contains(s, "nosuchkey")
}

func (s *Service) openFileRecordObject(ctx context.Context, sm map[string]string, fr *files.FileRecord) (io.ReadCloser, string, string, error) {
	if fr == nil {
		return nil, "", "", fmt.Errorf("invalid object key")
	}
	k := strings.TrimSpace(fr.ObjectKey)
	if k == "" {
		return nil, "", "", fmt.Errorf("invalid object key")
	}
	cleanKey, err := sanitizeStorageObjectKey(strings.TrimLeft(strings.ReplaceAll(k, "\\", "/"), "/"))
	if err != nil {
		return nil, "", "", err
	}
	storedKind := strings.TrimSpace(fr.StorageKind)
	prov, _, err := storage.NewFromPlainForStoredKind(sm, storedKind)
	if err != nil {
		return nil, "", "", fmt.Errorf("unsupported storage provider: %w", err)
	}
	rc, err := prov.Get(ctx, cleanKey)
	if err != nil {
		return nil, "", "", err
	}
	fn := filenameForObjectKey(cleanKey, fr.OriginalName)
	ct := contentTypeForFilename(fn, fr.ContentType)
	return rc, fn, ct, nil
}

func (s *Service) openProductImageObject(ctx context.Context, sm map[string]string, pi *product.ProductImage) (io.ReadCloser, string, string, error) {
	if pi == nil {
		return nil, "", "", fmt.Errorf("invalid object key")
	}
	k := strings.TrimSpace(pi.ObjectKey)
	if k == "" {
		return nil, "", "", fmt.Errorf("invalid object key")
	}
	cleanKey, err := sanitizeStorageObjectKey(strings.TrimLeft(strings.ReplaceAll(k, "\\", "/"), "/"))
	if err != nil {
		return nil, "", "", err
	}
	kind := normalizedSettingsKind(sm)
	prov, _, err := storage.NewFromPlainForStoredKind(sm, kind)
	if err != nil {
		return nil, "", "", fmt.Errorf("unsupported storage provider: %w", err)
	}
	rc, err := prov.Get(ctx, cleanKey)
	if err != nil {
		return nil, "", "", err
	}
	fn := filenameForObjectKey(cleanKey, "")
	ct := contentTypeForFilename(fn, "")
	return rc, fn, ct, nil
}

func (s *Service) tryOpenStaticLocal(ctx context.Context, sm map[string]string, urlish string) (io.ReadCloser, string, string, error) {
	sk := normalizedSettingsKind(sm)
	key, ok := staticURLToObjectKey(urlish, sk)
	if !ok {
		return nil, "", "", errors.New("not static local")
	}
	return s.openLocalObjectKey(ctx, sm, key)
}

// tryOpenStaticAsLocalFile opens a /static/... URL using the local storage provider when the URL shape
// is valid for local serving (path-only or loopback), regardless of current default storage kind.
func (s *Service) tryOpenStaticAsLocalFile(ctx context.Context, sm map[string]string, urlish string) (io.ReadCloser, string, string, error) {
	key, ok := staticURLToObjectKey(urlish, "local")
	if !ok {
		return nil, "", "", errors.New("not static local")
	}
	return s.openLocalObjectKey(ctx, sm, key)
}

func (s *Service) openLocalObjectKey(ctx context.Context, sm map[string]string, key string) (io.ReadCloser, string, string, error) {
	prov, _, err := storage.NewFromPlainForStoredKind(sm, "local")
	if err != nil {
		return nil, "", "", err
	}
	rc, err := prov.Get(ctx, key)
	if err != nil {
		return nil, "", "", err
	}
	fn := filenameForObjectKey(key, "")
	ct := contentTypeForFilename(fn, "")
	return rc, fn, ct, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ResolveSource resolves source_image_id / source_image_url for persisting image_tasks (display URL + id).
func (s *Service) ResolveSource(ctx context.Context, sourceImageID *uuid.UUID, sourceURL string) (imageID *uuid.UUID, resolvedURL string, err error) {
	urlStr := strings.TrimSpace(sourceURL)
	if sourceImageID != nil && *sourceImageID != uuid.Nil {
		sid := *sourceImageID
		var fr files.FileRecord
		if err := s.DB.WithContext(ctx).First(&fr, "id = ?", sid).Error; err == nil {
			out := firstNonEmpty(fr.PublicURL)
			if out == "" && strings.TrimSpace(fr.ObjectKey) != "" && s.Settings != nil {
				sm, e2 := s.Settings.PlainByGroup(ctx, 0, "storage")
				if e2 == nil {
					if prov, _, e3 := storage.NewFromPlainForStoredKind(sm, fr.StorageKind); e3 == nil {
						if u, e4 := prov.GetURL(ctx, fr.ObjectKey); e4 == nil {
							out = strings.TrimSpace(u)
						}
					}
				}
			}
			return &sid, out, nil
		}
		var pi product.ProductImage
		if err := s.DB.WithContext(ctx).First(&pi, "id = ?", sid).Error; err == nil {
			out := firstNonEmpty(pi.PublicURL, pi.OriginURL)
			if out == "" && strings.TrimSpace(pi.ObjectKey) != "" && s.Settings != nil {
				sm, e2 := s.Settings.PlainByGroup(ctx, 0, "storage")
				if e2 == nil {
					kind := normalizedSettingsKind(sm)
					if prov, _, e3 := storage.NewFromPlainForStoredKind(sm, kind); e3 == nil {
						if u, e4 := prov.GetURL(ctx, pi.ObjectKey); e4 == nil {
							out = strings.TrimSpace(u)
						}
					}
				}
			}
			return &sid, out, nil
		}
		if urlStr != "" {
			return nil, urlStr, nil
		}
		return nil, "", fmt.Errorf("sourceImageId not found")
	}
	if urlStr != "" {
		return nil, urlStr, nil
	}
	return nil, "", fmt.Errorf("sourceImageId or sourceImageUrl required")
}

// removeBGSource is the outcome of resolving a remove_background source image.
type removeBGSource struct {
	PublicURL   string
	File        io.ReadCloser
	Filename    string
	ContentType string
}

// resolveRemoveBGSource resolves storage-backed or public URL sources for remove.bg.
func (s *Service) resolveRemoveBGSource(ctx context.Context, task *ImageTask) (removeBGSource, error) {
	var zero removeBGSource
	if s == nil || s.DB == nil {
		return zero, errors.New("source image is not readable and not publicly accessible")
	}
	if s.Settings == nil {
		return zero, errors.New("source image is not readable and not publicly accessible")
	}
	sm, err := s.Settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return zero, err
	}

	if task.SourceImageID != nil && *task.SourceImageID != uuid.Nil {
		sid := *task.SourceImageID
		var fr files.FileRecord
		if err := s.DB.WithContext(ctx).First(&fr, "id = ?", sid).Error; err == nil {
			if strings.TrimSpace(fr.ObjectKey) != "" {
				rc, fn, ct, err := s.openFileRecordObject(ctx, sm, &fr)
				if err == nil {
					return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
				}
				if !isNotFoundLikeErr(err) {
					return zero, err
				}
			}
			u := firstNonEmpty(fr.PublicURL)
			if httppublic.IsPublicHTTPURL(u) {
				return removeBGSource{PublicURL: u}, nil
			}
			if u != "" {
				if rc, fn, ct, err := s.tryOpenStaticLocal(ctx, sm, u); err == nil {
					return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
				}
				if strings.EqualFold(strings.TrimSpace(fr.StorageKind), "local") {
					if rc, fn, ct, err := s.tryOpenStaticAsLocalFile(ctx, sm, u); err == nil {
						return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
					}
				}
			}
			return zero, errors.New("source image is not readable and not publicly accessible")
		}

		var pi product.ProductImage
		if err := s.DB.WithContext(ctx).First(&pi, "id = ?", sid).Error; err == nil {
			if strings.TrimSpace(pi.ObjectKey) != "" {
				rc, fn, ct, err := s.openProductImageObject(ctx, sm, &pi)
				if err == nil {
					return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
				}
				if !isNotFoundLikeErr(err) {
					return zero, err
				}
			}
			for _, u := range []string{strings.TrimSpace(pi.PublicURL), strings.TrimSpace(pi.OriginURL)} {
				if httppublic.IsPublicHTTPURL(u) {
					return removeBGSource{PublicURL: u}, nil
				}
				if u != "" {
					if rc, fn, ct, err := s.tryOpenStaticLocal(ctx, sm, u); err == nil {
						return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
					}
				}
			}
			return zero, errors.New("source image is not readable and not publicly accessible")
		}

		if u := strings.TrimSpace(task.SourceImageURL); u != "" {
			return s.resolveRemoveBGFromURL(ctx, sm, u)
		}
		return zero, fmt.Errorf("sourceImageId not found")
	}

	u := strings.TrimSpace(task.SourceImageURL)
	if u == "" {
		return zero, errors.New("source image is not readable and not publicly accessible")
	}
	return s.resolveRemoveBGFromURL(ctx, sm, u)
}

func (s *Service) resolveRemoveBGFromURL(ctx context.Context, sm map[string]string, u string) (removeBGSource, error) {
	var zero removeBGSource
	sk := normalizedSettingsKind(sm)
	if httppublic.IsPublicHTTPURL(u) {
		return removeBGSource{PublicURL: u}, nil
	}
	if sk == "local" {
		if rc, fn, ct, err := s.tryOpenStaticLocal(ctx, sm, u); err == nil {
			return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
		}
	}
	var fr files.FileRecord
	if err := s.DB.WithContext(ctx).Where("public_url = ?", u).First(&fr).Error; err == nil {
		if strings.TrimSpace(fr.ObjectKey) != "" {
			rc, fn, ct, err := s.openFileRecordObject(ctx, sm, &fr)
			if err == nil {
				return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
			}
			if !isNotFoundLikeErr(err) {
				return zero, err
			}
		}
		if strings.EqualFold(strings.TrimSpace(fr.StorageKind), "local") {
			if rc, fn, ct, err := s.tryOpenStaticAsLocalFile(ctx, sm, u); err == nil {
				return removeBGSource{File: rc, Filename: fn, ContentType: ct}, nil
			}
		}
	}
	return zero, errors.New("source image is not readable and not publicly accessible")
}

// resolveOpenAIReplaceBackgroundSource resolves a readable source for OpenAI Image edits (multipart / image[]).
func (s *Service) resolveOpenAIReplaceBackgroundSource(ctx context.Context, task *ImageTask) (removeBGSource, error) {
	var zero removeBGSource
	rb, err := s.resolveRemoveBGSource(ctx, task)
	if err != nil {
		return zero, fmt.Errorf("source image is required and must be readable for openai replace_background: %w", err)
	}
	return rb, nil
}
