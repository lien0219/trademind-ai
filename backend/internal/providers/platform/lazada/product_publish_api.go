package lazada

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/trademind-ai/trademind/backend/internal/pkg/httppublic"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const maxLazadaListingImageBytes = 5 << 20
const maxLazadaListingImages = 8

func maybeRetryableTransportErrLazada(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") ||
		strings.Contains(s, "connection reset") || strings.Contains(s, "eof") {
		return fmt.Errorf("lazada product publish: retryable: %w", err)
	}
	return err
}

func classifyLazadaPublishError(httpStatus int, root map[string]any, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, platformp.ErrPlatformProductPublishPermissionDenied) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("lazada product publish: retryable: %w", err)
	}
	switch httpStatus {
	case http.StatusUnauthorized, http.StatusForbidden:
		return platformp.ErrPlatformProductPublishPermissionDenied
	case http.StatusTooManyRequests:
		return fmt.Errorf("lazada product publish: retryable: %w", err)
	default:
		if httpStatus >= 500 {
			return fmt.Errorf("lazada product publish: retryable: %w", err)
		}
	}
	combined := lazadaHTTPAndBodySummary(httpStatus, root, err)
	if isPermissionLikeLazadaMessage(combined) {
		return platformp.ErrPlatformProductPublishPermissionDenied
	}
	if isPermissionLikeLazadaMessage(err.Error()) {
		return platformp.ErrPlatformProductPublishPermissionDenied
	}
	return err
}

func postProductCreate(ctx context.Context, cfg RuntimeConfig, access, payload string) (map[string]any, int, error) {
	pl := strings.TrimSpace(payload)
	if pl == "" {
		return nil, 0, fmt.Errorf("lazada product create: empty payload")
	}
	return signedPOSTForm(ctx, cfg, cfg.APIRESTBase, PathProductCreate, access, map[string]string{
		"payload": pl,
	})
}

func lazadaMigrateImageURL(ctx context.Context, cfg RuntimeConfig, access, imageURL string) (string, int, error) {
	u := strings.TrimSpace(imageURL)
	if u == "" {
		return "", 0, fmt.Errorf("lazada image migrate: empty url")
	}
	root, st, err := signedPOSTForm(ctx, cfg, cfg.APIRESTBase, PathImageMigrate, access, map[string]string{
		"image_url": u,
	})
	if err != nil {
		return "", st, err
	}
	url := extractLazadaHostedImageURL(root)
	if strings.TrimSpace(url) == "" {
		return "", st, fmt.Errorf("lazada image migrate: platform did not return image url")
	}
	return strings.TrimSpace(url), st, nil
}

func lazadaUploadImageMultipart(ctx context.Context, cfg RuntimeConfig, access, fileName string, blob []byte, contentType string) (string, int, error) {
	_ = contentType
	if len(blob) == 0 {
		return "", 0, fmt.Errorf("lazada image upload: empty file")
	}
	if len(blob) > maxLazadaListingImageBytes {
		return "", 0, fmt.Errorf("lazada image upload: image exceeds max size (%d bytes)", maxLazadaListingImageBytes)
	}
	apiPath := PathImageUpload
	p := map[string]string{
		"app_key":      cfg.AppKey,
		"timestamp":    nowMillisString(),
		"sign_method":  SignMethodSHA256,
		"access_token": access,
	}
	p["sign"] = Sign(apiPath, p, "", cfg.AppSecret)

	fn := strings.TrimSpace(fileName)
	if fn == "" {
		fn = "listing.jpg"
	}
	if filepath.Ext(fn) == "" {
		fn += ".jpg"
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, k := range sortedKeysLazada(p) {
		_ = mw.WriteField(k, p[k])
	}
	part, err := mw.CreateFormFile("image", filepath.Base(fn))
	if err != nil {
		return "", 0, err
	}
	if _, err := part.Write(blob); err != nil {
		return "", 0, err
	}
	if err := mw.Close(); err != nil {
		return "", 0, err
	}

	u := strings.TrimSuffix(strings.TrimSpace(cfg.APIRESTBase), "/") + apiPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &buf)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: cfg.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", resp.StatusCode, err
	}
	st := resp.StatusCode
	if st < 200 || st >= 300 {
		return "", st, fmt.Errorf("lazada http %d: %s", st, trimPreview(string(b), 400))
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		return "", st, fmt.Errorf("lazada: invalid json: %w", err)
	}
	if err := lazadaErr(root); err != nil {
		return "", st, err
	}
	url := extractLazadaHostedImageURL(root)
	if strings.TrimSpace(url) == "" {
		return "", st, fmt.Errorf("lazada image upload: platform did not return image url")
	}
	return strings.TrimSpace(url), st, nil
}

func sortedKeysLazada(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

func extractLazadaHostedImageURL(root map[string]any) string {
	if root == nil {
		return ""
	}
	if d, ok := root["data"].(map[string]any); ok && d != nil {
		if u := firstNonEmptyStr(
			pickStr(d, "image_url", "url", "image"),
			pickStr(d, "Image", "Url"),
		); u != "" {
			return u
		}
	}
	return firstNonEmptyStr(
		pickStr(root, "image_url", "url"),
	)
}

func resolveListingImageToLazadaURL(ctx context.Context, cfg RuntimeConfig, access string, im platformp.PlatformProductImage) (string, error) {
	rawURL := strings.TrimSpace(im.URL)
	if httppublic.IsPublicHTTPURL(rawURL) {
		u, st, err := lazadaMigrateImageURL(ctx, cfg, access, rawURL)
		if err == nil && u != "" {
			return u, nil
		}
		if er := classifyLazadaPublishError(st, nil, err); er != nil {
			return "", er
		}
		return "", err
	}
	if publishImages == nil {
		return "", fmt.Errorf("listing image fetcher not configured or url not public for lazada migrate")
	}
	blob, ct, err := publishImages.FetchProductImageBytes(ctx, im)
	if err != nil {
		return "", fmt.Errorf("lazada listing image: %w", err)
	}
	if len(blob) > maxLazadaListingImageBytes {
		return "", fmt.Errorf("lazada listing image exceeds max size (%d bytes)", maxLazadaListingImageBytes)
	}
	fn := "listing.jpg"
	if im.ObjectKey != "" {
		fn = filepath.Base(im.ObjectKey)
	}
	u, st, err := lazadaUploadImageMultipart(ctx, cfg, access, fn, blob, ct)
	if err == nil && u != "" {
		return u, nil
	}
	if er := classifyLazadaPublishError(st, nil, err); er != nil {
		return "", er
	}
	return "", err
}

func collectLazadaListingImageURLs(ctx context.Context, cfg RuntimeConfig, access string, candidates []platformp.PlatformProductImage) ([]string, error) {
	out := make([]string, 0, maxLazadaListingImages)
	for _, im := range candidates {
		if len(out) >= maxLazadaListingImages {
			break
		}
		u, err := resolveListingImageToLazadaURL(ctx, cfg, access, im)
		if err != nil {
			return nil, err
		}
		if u != "" {
			out = append(out, u)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("lazada product publish: no usable listing images after upload/migrate")
	}
	return out, nil
}

func resolveSKUPreviewToLazadaURL(ctx context.Context, cfg RuntimeConfig, access, raw string, seen map[string]string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if v, ok := seen[raw]; ok {
		return v, nil
	}
	im := platformp.PlatformProductImage{URL: raw, Type: "sku"}
	u, err := resolveListingImageToLazadaURL(ctx, cfg, access, im)
	if err != nil {
		return "", err
	}
	seen[raw] = u
	return u, nil
}

func extractLazadaProductCreateIDs(root map[string]any) (itemID, spuID, listingURL string, skuParts []map[string]any) {
	if root == nil {
		return "", "", "", nil
	}
	d, _ := root["data"].(map[string]any)
	if d == nil {
		return "", "", "", nil
	}
	itemID = lazadaIDString(d, "item_id", "product_id", "global_item_id", "PrimaryProductId")
	spuID = lazadaIDString(d, "global_product_id", "spu_id")
	if spuID == "" {
		spuID = itemID
	}
	listingURL = firstNonEmptyStr(pickStr(d, "product_url", "url", "permalink"))
	skuParts = coalesceLazadaSKUResponse(d)
	return itemID, spuID, listingURL, skuParts
}

func lazadaIDString(m map[string]any, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "0" {
				return s
			}
		}
	}
	return ""
}

func coalesceLazadaSKUResponse(d map[string]any) []map[string]any {
	if d == nil {
		return nil
	}
	candidates := []any{d["skus"], d["sku_list"], d["Sku"], d["sku"]}
	for _, c := range candidates {
		if c == nil {
			continue
		}
		if arr, ok := c.([]any); ok {
			out := make([]map[string]any, 0, len(arr))
			for _, x := range arr {
				if row, ok := x.(map[string]any); ok && row != nil {
					out = append(out, row)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
		if row, ok := c.(map[string]any); ok && row != nil {
			if inner, ok := row["Sku"].([]any); ok {
				var nested []map[string]any
				for _, x := range inner {
					if sub, ok := x.(map[string]any); ok && sub != nil {
						nested = append(nested, sub)
					}
				}
				if len(nested) > 0 {
					return nested
				}
			}
		}
	}
	return nil
}

func buildLazadaSKUMappings(local []platformp.PlatformProductSKU, platformRows []map[string]any) []platformp.PlatformSKUMapping {
	bySeller := map[string]map[string]any{}
	for _, row := range platformRows {
		k := strings.TrimSpace(pickStr(row, "seller_sku", "SellerSku", "sellerSku"))
		if k != "" {
			bySeller[k] = row
		}
	}
	out := make([]platformp.PlatformSKUMapping, 0, len(local))
	for _, sku := range local {
		code := sellerSkuCode(sku)
		row := bySeller[code]
		ext := firstNonEmptyStr(
			lazadaIDString(row, "sku_id", "SkuId", "skuId"),
			strings.TrimSpace(pickStr(row, "seller_sku", "SellerSku")),
			code,
		)
		price := ptrFloatLazada(sku.Price)
		st := sku.Stock
		stPtr := &st
		raw := platformp.TrimRawMap(map[string]any{
			"sellerSku": pickStr(row, "seller_sku", "SellerSku"),
			"skuId":     lazadaIDString(row, "sku_id", "SkuId"),
		}, 8, 160)
		out = append(out, platformp.PlatformSKUMapping{
			LocalSKUID:    sku.LocalSKUID,
			ExternalSKUID: ext,
			SKUCode:       strings.TrimSpace(sku.SKUCode),
			Price:         price,
			Stock:         stPtr,
			RawData:       raw,
		})
	}
	return out
}

func ptrFloatLazada(f float64) *float64 {
	if f == 0 {
		return nil
	}
	v := f
	return &v
}
