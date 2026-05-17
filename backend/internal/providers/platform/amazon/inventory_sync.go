package amazon

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
)

func (amazonProvider) SyncInventory(ctx context.Context, req platformp.SyncInventoryRequest) (*platformp.SyncInventoryResult, error) {
	if req.ShopID == uuid.Nil {
		return nil, fmt.Errorf("shop id required")
	}

	if _, err := ResolveRuntime(req.Auth); err != nil {
		return nil, errIncompleteAmazonPlatformSettings
	}
	if strings.TrimSpace(req.Auth.AccessToken) == "" && strings.TrimSpace(req.Auth.RefreshToken) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	sellerID := strings.TrimSpace(firstNonEmptyString(req.Auth.SellerID, req.Auth.MerchantID, req.Auth.Extra["selling_partner_id"], req.Auth.Extra["seller_id"]))
	if sellerID == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	sellerSKU := strings.TrimSpace(req.ExternalSKUID)
	if sellerSKU == "" {
		sellerSKU = strings.TrimSpace(req.SKUCode)
	}
	if sellerSKU == "" {
		return nil, fmt.Errorf("product publication sku mapping incomplete")
	}
	if req.Stock < 0 {
		return nil, fmt.Errorf("invalid stock quantity")
	}

	access, auth2, err := ensureFreshAccess(ctx, req.ShopID, req.Auth)
	if err != nil {
		return nil, maybeRetryableAmazonInventoryTransportErr(err)
	}
	if strings.TrimSpace(access) == "" {
		return nil, fmt.Errorf("shop is not authorized")
	}

	cfg, err := ResolveRuntime(auth2)
	if err != nil {
		return nil, errIncompleteAmazonPlatformSettings
	}

	publishPlain := loadAmazonPublishPlain(ctx)
	merged, err := mergeAmazonPublish(stringMapToAny(publishPlain), req.Options)
	if err != nil {
		return nil, err
	}

	marketplaceID := resolveAmazonInventoryMarketplaceID(req.Options, merged, auth2, cfg)
	fulfillmentChannel := resolveAmazonInventoryFulfillmentChannel(req.Options, merged)
	productType := resolveAmazonInventoryProductType(req.Options, merged)

	if strings.TrimSpace(marketplaceID) == "" {
		return nil, fmt.Errorf("platform inventory config incomplete: missing marketplace_id")
	}
	if strings.TrimSpace(fulfillmentChannel) == "" {
		return nil, fmt.Errorf("platform inventory config incomplete: missing fulfillment_channel")
	}
	if strings.TrimSpace(productType) == "" {
		return nil, fmt.Errorf("platform inventory config incomplete: missing product_type")
	}

	issueLocale := strings.TrimSpace(merged.IssueLocale)

	res, _, err := patchListingsItemInventory(ctx, cfg, access, sellerID, sellerSKU, marketplaceID, issueLocale, productType, fulfillmentChannel, req.Stock)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, fmt.Errorf("amazon inventory sync: empty Listings Items response")
	}
	if hasAmazonListingErrorIssue(res.Issues) {
		return nil, fmt.Errorf("amazon inventory sync: listing issues: %v", summarizeAmazonListingIssues(res.Issues))
	}

	extPID := strings.TrimSpace(req.ExternalProductID)
	sum := platformp.TrimRawMap(map[string]any{
		"provider":       "amazon",
		"path":           "PATCH " + PathListingsItem,
		"marketplaceId":  marketplaceID,
		"listingsStatus": strings.TrimSpace(res.Status),
		"submissionId":   strings.TrimSpace(res.SubmissionID),
		"asin":           strings.TrimSpace(res.ASIN),
		"issues":         summarizeAmazonListingIssues(res.Issues),
		"receivedAt":     time.Now().UTC().Format(time.RFC3339),
	}, 14, 220)

	st := strings.TrimSpace(res.Status)
	if st == "" {
		st = "success"
	}
	return &platformp.SyncInventoryResult{
		ExternalProductID: extPID,
		ExternalSKUID:     sellerSKU,
		Stock:             req.Stock,
		Status:            "success",
		RawSummary:        sum,
	}, nil
}
