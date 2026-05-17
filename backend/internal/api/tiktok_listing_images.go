package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
)

// tikTokListingImageFetcher resolves product listing images via Storage Provider or public HTTP (never logs secrets).
type tikTokListingImageFetcher struct {
	settings *settings.Service
}

func newTikTokListingImageFetcher(s *settings.Service) *tikTokListingImageFetcher {
	return &tikTokListingImageFetcher{settings: s}
}

func (f *tikTokListingImageFetcher) FetchProductImageBytes(ctx context.Context, img platformp.PlatformProductImage) ([]byte, string, error) {
	if f == nil || f.settings == nil {
		return nil, "", fmt.Errorf("settings unavailable for image fetch")
	}
	sm, err := f.settings.PlainByGroup(ctx, 0, "storage")
	if err != nil {
		return nil, "", fmt.Errorf("load storage settings: %w", err)
	}

	key := strings.TrimSpace(img.ObjectKey)
	if key != "" {
		clean, err := sanitizeObjectKey(key)
		if err != nil {
			return nil, "", err
		}
		kind := normalizedStorageKind(sm)
		prov, _, err := storage.NewFromPlainForStoredKind(sm, kind)
		if err != nil {
			return nil, "", fmt.Errorf("storage provider: %w", err)
		}
		rc, err := prov.Get(ctx, clean)
		if err != nil {
			return nil, "", fmt.Errorf("storage get image: %w", err)
		}
		defer rc.Close()
		b, err := io.ReadAll(io.LimitReader(rc, 6<<20))
		if err != nil {
			return nil, "", err
		}
		fn := filepath.Base(clean)
		return b, contentTypeGuess(fn, ""), nil
	}

	rawURL := strings.TrimSpace(img.URL)
	if rawURL == "" {
		return nil, "", fmt.Errorf("image has no url or object_key")
	}

	if key2, ok := staticURLToObjectKey(rawURL); ok {
		prov, _, err := storage.NewFromPlainForStoredKind(sm, "local")
		if err != nil {
			return nil, "", err
		}
		rc, err := prov.Get(ctx, key2)
		if err != nil {
			return nil, "", fmt.Errorf("storage get static image: %w", err)
		}
		defer rc.Close()
		b, err := io.ReadAll(io.LimitReader(rc, 6<<20))
		if err != nil {
			return nil, "", err
		}
		return b, contentTypeGuess(filepath.Base(key2), ""), nil
	}

	if httppublic.IsPublicHTTPURL(rawURL) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, "", err
		}
		client := http.Client{Timeout: 45 * time.Second}
		res, err := client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("download listing image: %w", err)
		}
		defer res.Body.Close()
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return nil, "", fmt.Errorf("download listing image: http %d", res.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(res.Body, 6<<20))
		if err != nil {
			return nil, "", err
		}
		ct := strings.TrimSpace(res.Header.Get("Content-Type"))
		return b, ct, nil
	}

	return nil, "", fmt.Errorf("image url is not publicly reachable; upload via TradeMind files or configure public_base/static URL")
}

func normalizedStorageKind(sm map[string]string) string {
	k := strings.TrimSpace(strings.ToLower(sm["kind"]))
	if k == "" {
		return "local"
	}
	return k
}

func sanitizeObjectKey(raw string) (string, error) {
	s := strings.Trim(strings.ReplaceAll(raw, "\\", "/"), "/")
	if s == "" || strings.Contains(s, "..") || strings.HasPrefix(s, "/") {
		return "", fmt.Errorf("invalid object key")
	}
	return s, nil
}

func staticURLToObjectKey(raw string) (string, bool) {
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
		key, err := sanitizeObjectKey(rest)
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
	key, err := sanitizeObjectKey(rest)
	if err != nil {
		return "", false
	}
	return key, true
}

func contentTypeGuess(filename string, hint string) string {
	if strings.TrimSpace(hint) != "" {
		return hint
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
