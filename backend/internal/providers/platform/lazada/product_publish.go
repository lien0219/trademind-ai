package lazada

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func (lazadaProvider) PublishProduct(ctx context.Context, req platformp.PublishProductRequest) (*platformp.PublishProductResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}

	if _, err := ResolveRuntime(req.Auth); err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_lazada first")
	}

	if strings.TrimSpace(req.Auth.AccessToken) == "" && strings.TrimSpace(req.Auth.RefreshToken) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	mergedCfg := mergeLazadaPublish(req.PublishConfig, req.Options)
	if err := validateLazadaPublishMerged(mergedCfg); err != nil {
		return nil, err
	}

	d := req.Product
	if strings.TrimSpace(d.Title) == "" {
		return nil, fmt.Errorf("product title is required")
	}
	if strings.TrimSpace(d.Description) == "" {
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
		return nil, classifyLazadaPublishError(0, nil, maybeRetryableTransportErrLazada(err))
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	cfg, err := ResolveRuntime(auth2)
	if err != nil {
		return nil, fmt.Errorf("platform config incomplete: please configure settings.platform_lazada first")
	}

	prodAttrs, err := mergeLazadaAttributeMaps(mergedCfg, d, req.Options)
	if err != nil {
		return nil, err
	}

	imageURLs, err := collectLazadaListingImageURLs(ctx, cfg, access, mainCandidates)
	if err != nil {
		return nil, classifyLazadaPublishError(0, nil, err)
	}

	seenSKUImg := map[string]string{}
	skuPrepend := make([]string, len(d.SKUs))
	for i, sku := range d.SKUs {
		if u := strings.TrimSpace(sku.ImageURL); u != "" {
			mu, ierr := resolveSKUPreviewToLazadaURL(ctx, cfg, access, u, seenSKUImg)
			if ierr != nil {
				return nil, ierr
			}
			skuPrepend[i] = mu
		}
	}

	skuEntries, err := buildLazadaSKUEntries(d, mergedCfg, imageURLs, skuPrepend)
	if err != nil {
		return nil, err
	}

	payloadStr, err := buildCreateProductPayloadStr(mergedCfg, prodAttrs, skuEntries)
	if err != nil {
		return nil, err
	}

	root, httpSt, err := postProductCreate(ctx, cfg, access, payloadStr)
	if err != nil {
		return nil, classifyLazadaPublishError(httpSt, root, maybeRetryableTransportErrLazada(err))
	}

	itemID, spuGuess, listingURL, skuRows := extractLazadaProductCreateIDs(root)
	if strings.TrimSpace(itemID) == "" {
		return nil, fmt.Errorf("lazada product publish: platform did not return item_id")
	}

	mappings := buildLazadaSKUMappings(d.SKUs, skuRows)

	status := "published"
	if mergedCfg.PublishAsDraft {
		status = "draft"
	}

	rawSummary := platformp.TrimRawMap(map[string]any{
		"provider":        "lazada",
		"listingImages":   len(imageURLs),
		"skuCount":        len(d.SKUs),
		"mappedSkus":      len(mappings),
		"publishAsDraft":  mergedCfg.PublishAsDraft,
		"primaryCategory": mergedCfg.PrimaryCategory,
	}, 14, 220)

	return &platformp.PublishProductResult{
		ExternalProductID: strings.TrimSpace(itemID),
		ExternalSPUID:     strings.TrimSpace(firstNonEmptyStr(spuGuess, itemID)),
		ExternalURL:       strings.TrimSpace(listingURL),
		Status:            status,
		SKUMappings:       mappings,
		RawSummary:        rawSummary,
	}, nil
}
