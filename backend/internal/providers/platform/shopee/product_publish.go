package shopee

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

const (
	maxShopeeListingImageBytes = 5 << 20
	maxShopeeListingImages     = 9
)

func (shopeeProvider) PublishProduct(ctx context.Context, req platformp.PublishProductRequest) (*platformp.PublishProductResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}

	if _, err := ResolveRuntime(req.Auth); err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_shopee first")
	}

	if strings.TrimSpace(req.Auth.AccessToken) == "" && strings.TrimSpace(req.Auth.RefreshToken) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	merged := mergeShopeePublish(req.PublishConfig, req.Options)
	if err := validateShopeePublishMerged(merged); err != nil {
		return nil, err
	}

	d := req.Product
	title := strings.TrimSpace(d.Title)
	if title == "" {
		return nil, fmt.Errorf("product title is required")
	}
	desc := strings.TrimSpace(d.Description)
	if desc == "" {
		return nil, fmt.Errorf("product description is required")
	}
	if strings.TrimSpace(d.Currency) == "" {
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

	access, auth2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, maybeRetryableTransportErr(err)
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	cfg, err := ResolveRuntime(auth2)
	if err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_shopee first")
	}

	sid, err := parseShopID(auth2)
	if err != nil {
		return nil, err
	}

	attrRows, err := parseShopeeAttributeList(d.Attributes, req.Options)
	if err != nil {
		return nil, err
	}

	imageIDs, err := uploadListingImages(ctx, cfg, sid, access, mainCandidates)
	if err != nil {
		return nil, err
	}

	skuImgIDs, err := uploadSKUVariationImages(ctx, cfg, sid, access, d.SKUs)
	if err != nil {
		return nil, err
	}

	multi := len(d.SKUs) > 1
	var singlePtr *platformp.PlatformProductSKU
	if !multi {
		singlePtr = &d.SKUs[0]
	}

	payload, err := buildAddItemPayload(d, merged, imageIDs, attrRows, multi, singlePtr)
	if err != nil {
		return nil, err
	}

	addResp, httpSt, err := postShopWithStatus(ctx, cfg, PathProductAddItem, sid, access, payload)
	if err != nil {
		return nil, mapShopeePublishErr(httpSt, maybeRetryableTransportErr(err))
	}
	itemID := extractItemIDUint(addResp)
	if itemID == 0 {
		return nil, fmt.Errorf("shopee product publish: platform did not return item_id")
	}

	var mappings []platformp.PlatformSKUMapping
	if multi {
		dims, err := parseTierDimSpecs(req.Options)
		if err != nil {
			return nil, err
		}
		var tierVar []map[string]any
		var models []map[string]any
		if len(dims) > 0 {
			tierVar, models, err = buildTierMatrices(d.SKUs, dims)
		} else {
			tierVar, models, err = buildSingleTierVariation(d.SKUs, merged.VariationTierKey, skuImgIDs)
		}
		if err != nil {
			return nil, err
		}
		initBody := map[string]any{
			"item_id":        itemID,
			"tier_variation": tierVar,
			"model":          models,
		}
		initResp, initHTTP, err := postShopWithStatus(ctx, cfg, PathProductInitTierVariation, sid, access, initBody)
		if err != nil {
			return nil, mapShopeePublishErr(initHTTP, maybeRetryableTransportErr(err))
		}
		modelRows := extractModelRows(initResp)
		mappings = buildSKUMappings(d.SKUs, itemID, modelRows, false)
	} else {
		mappings = buildSKUMappings(d.SKUs, itemID, nil, true)
	}

	status := "published"
	if merged.PublishAsDraft {
		status = "draft"
	}

	rawSummary := platformp.TrimRawMap(map[string]any{
		"provider":          "shopee",
		"listingImages":     len(imageIDs),
		"variationImages":   countNonEmptySKUImages(skuImgIDs),
		"models":            len(mappings),
		"logisticChannelId": merged.LogisticID,
		"itemStatus": func() string {
			if merged.PublishAsDraft {
				return "UNLIST"
			}
			return "NORMAL"
		}(),
	}, 14, 220)

	return &platformp.PublishProductResult{
		ExternalProductID: strconv.FormatUint(itemID, 10),
		ExternalSPUID:     strconv.FormatUint(itemID, 10),
		ExternalURL:       "",
		Status:            status,
		SKUMappings:       mappings,
		RawSummary:        rawSummary,
	}, nil
}

func fetchListingImageBytes(ctx context.Context, img platformp.PlatformProductImage) ([]byte, string, error) {
	if publishImages != nil {
		b, ct, err := publishImages.FetchProductImageBytes(ctx, img)
		if err == nil && len(b) > 0 {
			return b, ct, nil
		}
		if err != nil {
			return nil, "", err
		}
	}
	return nil, "", fmt.Errorf("listing image fetcher not configured or empty image")
}

func uploadListingImages(ctx context.Context, cfg RuntimeConfig, shopID int64, access string, candidates []platformp.PlatformProductImage) ([]string, error) {
	ids := make([]string, 0, maxShopeeListingImages)
	for _, im := range candidates {
		if len(ids) >= maxShopeeListingImages {
			break
		}
		blob, ct, err := fetchListingImageBytes(ctx, im)
		if err != nil {
			return nil, fmt.Errorf("shopee listing image: %w", err)
		}
		if len(blob) > maxShopeeListingImageBytes {
			return nil, fmt.Errorf("shopee listing image exceeds max size (%d bytes)", maxShopeeListingImageBytes)
		}
		fn := filenameForUpload(ct)
		resp, httpSt, err := postShopMultipartImage(ctx, cfg, PathMediaSpaceUploadImage, shopID, access, fn, blob)
		if err != nil {
			return nil, mapShopeePublishErr(httpSt, maybeRetryableTransportErr(fmt.Errorf("shopee media upload: %w", err)))
		}
		id := extractMediaImageID(resp)
		if id == "" {
			return nil, fmt.Errorf("shopee media upload: missing image_id")
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("shopee publish: failed to upload listing images")
	}
	return ids, nil
}

func uploadSKUVariationImages(ctx context.Context, cfg RuntimeConfig, shopID int64, access string, skus []platformp.PlatformProductSKU) (map[int]string, error) {
	if len(skus) <= 1 {
		return map[int]string{}, nil
	}
	out := map[int]string{}
	for i, sku := range skus {
		u := strings.TrimSpace(sku.ImageURL)
		if u == "" {
			continue
		}
		blob, ct, err := fetchListingImageBytes(ctx, platformp.PlatformProductImage{URL: u})
		if err != nil {
			return nil, fmt.Errorf("shopee SKU image: %w", err)
		}
		if len(blob) > maxShopeeListingImageBytes {
			return nil, fmt.Errorf("shopee SKU image exceeds max size (%d bytes)", maxShopeeListingImageBytes)
		}
		fn := filenameForUpload(ct)
		resp, httpSt, err := postShopMultipartImage(ctx, cfg, PathMediaSpaceUploadImage, shopID, access, fn, blob)
		if err != nil {
			return nil, mapShopeePublishErr(httpSt, maybeRetryableTransportErr(fmt.Errorf("shopee SKU image upload: %w", err)))
		}
		id := extractMediaImageID(resp)
		if id == "" {
			return nil, fmt.Errorf("shopee SKU image upload: missing image_id")
		}
		out[i] = id
	}
	return out, nil
}

func filenameForUpload(ct string) string {
	switch strings.ToLower(strings.TrimSpace(ct)) {
	case "image/png":
		return "listing.png"
	case "image/webp":
		return "listing.webp"
	case "image/gif":
		return "listing.gif"
	default:
		return "listing.jpg"
	}
}

func countNonEmptySKUImages(m map[int]string) int {
	n := 0
	for _, v := range m {
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	return n
}

func buildSKUMappings(skus []platformp.PlatformProductSKU, itemID uint64, modelsResp []map[string]any, single bool) []platformp.PlatformSKUMapping {
	if single {
		if len(skus) != 1 {
			return nil
		}
		s := skus[0]
		pr := s.Price
		st := s.Stock
		ext := strconv.FormatUint(itemID, 10)
		rd := platformp.TrimRawMap(map[string]any{
			"itemId":    itemID,
			"sellerSku": modelSKUCode(s),
		}, 6, 120)
		return []platformp.PlatformSKUMapping{{
			LocalSKUID:    s.LocalSKUID,
			ExternalSKUID: ext,
			SKUCode:       strings.TrimSpace(s.SKUCode),
			Price:         &pr,
			Stock:         &st,
			RawData:       rd,
		}}
	}

	bySKU := map[string]map[string]any{}
	for _, row := range modelsResp {
		ms := strings.TrimSpace(strField(row, "model_sku"))
		if ms != "" {
			bySKU[ms] = row
		}
	}

	out := make([]platformp.PlatformSKUMapping, 0, len(skus))
	for i, s := range skus {
		msk := modelSKUCode(s)
		var row map[string]any
		if row = bySKU[msk]; row == nil && i < len(modelsResp) {
			row = modelsResp[i]
			if row != nil && strings.TrimSpace(strField(row, "model_sku")) != "" &&
				strings.TrimSpace(strField(row, "model_sku")) != msk {
				row = nil
			}
		}
		mid := parseModelID(row)
		ext := strconv.FormatUint(mid, 10)
		if mid == 0 {
			ext = msk
		}
		pr := s.Price
		st := s.Stock
		rd := platformp.TrimRawMap(map[string]any{
			"modelId":  mid,
			"modelSku": msk,
			"itemId":   itemID,
		}, 8, 140)
		out = append(out, platformp.PlatformSKUMapping{
			LocalSKUID:    s.LocalSKUID,
			ExternalSKUID: ext,
			SKUCode:       strings.TrimSpace(s.SKUCode),
			Price:         &pr,
			Stock:         &st,
			RawData:       rd,
		})
	}
	return out
}
