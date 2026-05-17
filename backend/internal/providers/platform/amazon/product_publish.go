package amazon

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

var errIncompleteAmazonPlatformSettings = fmt.Errorf("platform config incomplete: please configure settings.platform_amazon first")

func PublishProduct(ctx context.Context, req platformp.PublishProductRequest) (*platformp.PublishProductResult, error) {
	return amazonProvider{}.PublishProduct(ctx, req)
}

func (amazonProvider) publishProduct(ctx context.Context, req platformp.PublishProductRequest) (*platformp.PublishProductResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}
	cfg, err := ResolveRuntime(req.Auth)
	if err != nil {
		return nil, errIncompleteAmazonPlatformSettings
	}
	if strings.TrimSpace(req.Auth.AccessToken) == "" && strings.TrimSpace(req.Auth.RefreshToken) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}
	sellerID := strings.TrimSpace(firstNonEmptyString(req.Auth.SellerID, req.Auth.MerchantID, req.Auth.Extra["selling_partner_id"], req.Auth.Extra["seller_id"]))
	if sellerID == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	merged, err := mergeAmazonPublish(req.PublishConfig, req.Options)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(merged.MarketplaceID) == "" {
		merged.MarketplaceID = EffectiveMarketplaceID(req.Auth, cfg)
	}
	if err := validateAmazonPublishConfig(merged); err != nil {
		return nil, err
	}
	if err := validateAmazonDraft(req.Product); err != nil {
		return nil, err
	}

	access, auth2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, maybeRetryableAmazonPublishTransportErr(err)
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}
	cfg, err = ResolveRuntime(auth2)
	if err != nil {
		return nil, errIncompleteAmazonPlatformSettings
	}

	imageURLs, err := collectAmazonPublicImageURLs(orderedAmazonListingImages(req.Product.Images))
	if err != nil {
		return nil, err
	}

	mappings := make([]platformp.PlatformSKUMapping, 0, len(req.Product.SKUs))
	var firstASIN string
	var firstSubmission string
	var firstSellerSKU string
	var firstStatus string
	totalIssues := 0
	for idx, sku := range req.Product.SKUs {
		sellerSKU := amazonSellerSKU(sku)
		if sellerSKU == "" {
			return nil, fmt.Errorf("amazon product publish: seller sku required")
		}
		body, err := buildAmazonListingBody(req.Product, sku, merged, imageURLs, idx)
		if err != nil {
			return nil, err
		}
		res, _, err := putListingsItem(ctx, cfg, access, sellerID, sellerSKU, merged.MarketplaceID, merged.IssueLocale, body)
		if err != nil {
			return nil, err
		}
		if res == nil {
			return nil, fmt.Errorf("amazon product publish: empty Listings Items response")
		}
		if hasAmazonListingErrorIssue(res.Issues) {
			return nil, fmt.Errorf("amazon product publish: listing submitted with issues: %v", summarizeAmazonListingIssues(res.Issues))
		}
		totalIssues += len(res.Issues)
		if firstSellerSKU == "" {
			firstSellerSKU = sellerSKU
		}
		if firstASIN == "" {
			firstASIN = strings.TrimSpace(res.ASIN)
		}
		if firstSubmission == "" {
			firstSubmission = strings.TrimSpace(res.SubmissionID)
		}
		if firstStatus == "" {
			firstStatus = strings.TrimSpace(res.Status)
		}
		pr := sku.Price
		st := sku.Stock
		mappings = append(mappings, platformp.PlatformSKUMapping{
			LocalSKUID:    sku.LocalSKUID,
			ExternalSKUID: sellerSKU,
			SKUCode:       sellerSKU,
			Price:         &pr,
			Stock:         &st,
			RawData: platformp.TrimRawMap(map[string]any{
				"sellerSku":    sellerSKU,
				"asin":         strings.TrimSpace(res.ASIN),
				"submissionId": strings.TrimSpace(res.SubmissionID),
				"status":       strings.TrimSpace(res.Status),
				"issues":       summarizeAmazonListingIssues(res.Issues),
			}, 10, 180),
		})
	}

	externalID := strings.TrimSpace(firstASIN)
	if externalID == "" {
		externalID = strings.TrimSpace(firstSubmission)
	}
	if externalID == "" {
		externalID = strings.TrimSpace(firstSellerSKU)
	}
	status := amazonPublicationStatus(firstStatus, merged.PublishAsDraft, firstASIN)
	rawSummary := platformp.TrimRawMap(map[string]any{
		"provider":       "amazon",
		"marketplaceId":  merged.MarketplaceID,
		"productType":    merged.ProductType,
		"sellerSkuCount": len(mappings),
		"submissionId":   firstSubmission,
		"asin":           firstASIN,
		"listingsStatus": firstStatus,
		"issuesCount":    totalIssues,
		"publishAsDraft": merged.PublishAsDraft,
	}, 14, 220)
	return &platformp.PublishProductResult{
		ExternalProductID: externalID,
		ExternalSPUID:     firstNonEmptyString(firstASIN, strings.TrimSpace(merged.ParentSellerSKU), firstSellerSKU),
		ExternalURL:       amazonListingURL(merged.MarketplaceID, firstASIN),
		Status:            status,
		SKUMappings:       mappings,
		RawSummary:        rawSummary,
	}, nil
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func amazonPublicationStatus(listingStatus string, draft bool, asin string) string {
	if draft {
		return "draft"
	}
	st := strings.ToLower(strings.TrimSpace(listingStatus))
	switch st {
	case "accepted", "submitted", "in_progress", "processing":
		return "publishing"
	case "buyable", "discoverable", "published":
		return "published"
	}
	if strings.TrimSpace(asin) != "" {
		return "published"
	}
	return "publishing"
}

func amazonListingURL(marketplaceID, asin string) string {
	_ = marketplaceID
	_ = asin
	return ""
}
