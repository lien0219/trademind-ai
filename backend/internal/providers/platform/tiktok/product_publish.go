package tiktok

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const maxTikTokListingImageBytes = 5 << 20
const maxTikTokMainImages = 9

var errIncompleteTiktokPlatformSettings = fmt.Errorf("platform config incomplete: please configure settings.platform_tiktok first")

func (tikTokProvider) PublishProduct(ctx context.Context, req platformp.PublishProductRequest) (*platformp.PublishProductResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, errIncompleteTiktokPlatformSettings
	}
	if strings.TrimSpace(cfg.ShopCipher) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	access, _, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	merged := mergeTikTokPublish(req.PublishConfig, req.Options)
	if err := validateMergedRequired(merged); err != nil {
		return nil, err
	}

	d := req.Product
	title := strings.TrimSpace(d.Title)
	if title == "" {
		return nil, fmt.Errorf("product title is required")
	}
	desc := strings.TrimSpace(d.Description)
	if !merged.PublishAsDraft && desc == "" {
		return nil, fmt.Errorf("product description is required for TikTok listing unless publishing as draft (enable publish_as_draft)")
	}
	curr := strings.TrimSpace(d.Currency)
	if curr == "" {
		return nil, fmt.Errorf("product currency is required")
	}
	if len(d.SKUs) == 0 {
		return nil, fmt.Errorf("product SKU is required for publish")
	}

	mainCandidates := orderedListingImages(d.Images)
	if len(mainCandidates) == 0 {
		return nil, fmt.Errorf("product main image required for publish")
	}
	hasMain := false
	for _, im := range mainCandidates {
		if strings.TrimSpace(strings.ToLower(im.Type)) == "main" {
			hasMain = true
			break
		}
	}
	if !hasMain {
		return nil, fmt.Errorf("product main image required for publish")
	}

	client := http.Client{Timeout: cfg.HTTPTimeout}

	imageURIs := make([]string, 0, maxTikTokMainImages)
	for _, im := range mainCandidates {
		if len(imageURIs) >= maxTikTokMainImages {
			break
		}
		blob, ct, err := fetchListingImageBytes(ctx, im)
		if err != nil {
			return nil, fmt.Errorf("tiktok listing image: %w", err)
		}
		if len(blob) > maxTikTokListingImageBytes {
			return nil, fmt.Errorf("tiktok listing image exceeds max size (%d bytes)", maxTikTokListingImageBytes)
		}
		uri, err := uploadProductListingImage(ctx, client, cfg, access, blob, ct)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(uri) != "" {
			imageURIs = append(imageURIs, strings.TrimSpace(uri))
		}
	}
	if len(imageURIs) == 0 {
		return nil, fmt.Errorf("tiktok product publish: failed to upload images")
	}

	pAttrs, err := parseProductAttributes(d.Attributes, req.Options)
	if err != nil {
		return nil, err
	}

	body, err := buildCreateProductBody(d, merged, imageURIs, merged.ShippingTemplateID, pAttrs)
	if err != nil {
		return nil, err
	}

	path := productCreatePath(cfg.APIVersion)
	raw, httpStatus, err := signedPOSTJSONStatus(ctx, client, cfg, path, access, body)
	if err != nil {
		return nil, maybeRetryableTransportErr(err)
	}
	root, err := decodeProductAPIResponse(raw, httpStatus)
	if err != nil {
		return nil, err
	}
	data := extractProductAPIData(root)

	productID := coalesce(strField(data, "product_id", "id"), strField(root, "product_id"))
	if strings.TrimSpace(productID) == "" {
		return nil, fmt.Errorf("tiktok product publish: platform did not return product id")
	}

	mappings := skuMappingsFromCreateResponse(data, d)
	extURL := ""
	rawSummary := platformp.TrimRawMap(map[string]any{
		"provider":         "tiktok",
		"bizCode":          bizCodeSummary(root),
		"uploadedImages":   len(imageURIs),
		"deliveryOptionId": merged.ShippingTemplateID,
		"saveMode": func() string {
			if merged.PublishAsDraft {
				return "AS_DRAFT"
			}
			return "LISTING"
		}(),
	}, 14, 220)

	status := "published"
	if merged.PublishAsDraft {
		status = "draft"
	}

	return &platformp.PublishProductResult{
		ExternalProductID: productID,
		ExternalSPUID:     productID,
		ExternalURL:       extURL,
		Status:            status,
		SKUMappings:       mappings,
		RawSummary:        rawSummary,
	}, nil
}

func maybeRetryableTransportErr(err error) error {
	if err == nil {
		return nil
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "timeout") || strings.Contains(s, "timed out") || strings.Contains(s, "connection reset") || strings.Contains(s, "eof") {
		return fmt.Errorf("tiktok product publish: retryable: %w", err)
	}
	return err
}

func orderedListingImages(images []platformp.PlatformProductImage) []platformp.PlatformProductImage {
	type tagged struct {
		im   platformp.PlatformProductImage
		rank int
		ord  int
	}
	var rows []tagged
	for _, im := range images {
		t := strings.ToLower(strings.TrimSpace(im.Type))
		rank := 3
		switch t {
		case "main":
			rank = 0
		case "detail", "description":
			rank = 1
		case "sku":
			rank = 2
		}
		rows = append(rows, tagged{im: im, rank: rank, ord: im.SortOrder})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].rank != rows[j].rank {
			return rows[i].rank < rows[j].rank
		}
		return rows[i].ord < rows[j].ord
	})
	out := make([]platformp.PlatformProductImage, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.im)
	}
	return out
}

func fetchListingImageBytes(ctx context.Context, img platformp.PlatformProductImage) ([]byte, string, error) {
	if publishImages != nil {
		b, ct, err := publishImages.FetchProductImageBytes(ctx, img)
		if err == nil && len(b) > 0 {
			return b, ct, nil
		}
	}
	return nil, "", fmt.Errorf("listing image fetcher not configured or failed for this image (configure storage / public URL)")
}

func uploadProductListingImage(ctx context.Context, client http.Client, cfg RuntimeConfig, access string, imageBytes []byte, contentType string) (string, error) {
	_ = contentType
	body := map[string]interface{}{
		"img_data":  base64.StdEncoding.EncodeToString(imageBytes),
		"img_scene": 1,
	}
	path := productImageUploadPath(cfg.APIVersion)
	raw, httpStatus, err := signedPOSTJSONStatus(ctx, client, cfg, path, access, body)
	if err != nil {
		return "", maybeRetryableTransportErr(err)
	}
	root, err := decodeProductAPIResponse(raw, httpStatus)
	if err != nil {
		return "", err
	}
	data := extractProductAPIData(root)
	uri := extractUploadedImageURI(data)
	if strings.TrimSpace(uri) == "" {
		return "", fmt.Errorf("tiktok image upload: missing uri in response")
	}
	return uri, nil
}

func extractUploadedImageURI(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	if u, ok := data["uri"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u)
	}
	if um, ok := data["image"].(map[string]interface{}); ok && um != nil {
		if u, ok2 := um["uri"].(string); ok2 && strings.TrimSpace(u) != "" {
			return strings.TrimSpace(u)
		}
	}
	for _, k := range []string{"image_uri", "image_url", "url"} {
		if u, ok := data[k].(string); ok && strings.TrimSpace(u) != "" {
			return strings.TrimSpace(u)
		}
	}
	return ""
}

func skuMappingsFromCreateResponse(data map[string]interface{}, draft platformp.PlatformProductDraft) []platformp.PlatformSKUMapping {
	var blocks []interface{}
	if v, ok := data["skus"].([]interface{}); ok {
		blocks = v
	}
	out := make([]platformp.PlatformSKUMapping, 0, len(draft.SKUs))
	used := map[string]bool{}
	for _, ds := range draft.SKUs {
		ext := matchRemoteSKU(blocks, ds, used)
		pr := ds.Price
		st := ds.Stock
		rd := platformp.TrimRawMap(map[string]any{
			"tiktokSkuId": ext,
			"sellerSku":   strings.TrimSpace(ds.SKUCode),
		}, 6, 120)
		out = append(out, platformp.PlatformSKUMapping{
			LocalSKUID:    ds.LocalSKUID,
			ExternalSKUID: ext,
			SKUCode:       strings.TrimSpace(ds.SKUCode),
			Price:         &pr,
			Stock:         &st,
			RawData:       rd,
		})
	}
	return out
}

func matchRemoteSKU(blocks []interface{}, sku platformp.PlatformProductSKU, used map[string]bool) string {
	wantExt := sku.LocalSKUID.String()
	wantSeller := strings.TrimSpace(sku.SKUCode)
	for _, blk := range blocks {
		m, ok := blk.(map[string]interface{})
		if !ok || m == nil {
			continue
		}
		tikID := strings.TrimSpace(fmt.Sprint(m["id"]))
		if tikID == "" || used[tikID] {
			continue
		}
		ext := strings.TrimSpace(fmt.Sprint(m["external_sku_id"]))
		if ext != "" && ext == wantExt {
			used[tikID] = true
			return tikID
		}
	}
	if wantSeller != "" {
		for _, blk := range blocks {
			m, ok := blk.(map[string]interface{})
			if !ok || m == nil {
				continue
			}
			tikID := strings.TrimSpace(fmt.Sprint(m["id"]))
			if tikID == "" || used[tikID] {
				continue
			}
			seller := strings.TrimSpace(fmt.Sprint(m["seller_sku"]))
			if seller == wantSeller {
				used[tikID] = true
				return tikID
			}
		}
	}
	return ""
}
