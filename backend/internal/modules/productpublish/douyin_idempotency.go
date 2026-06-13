package productpublish

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	platformdouyin "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

func douyinMappingContentHash(mapping any) string {
	b, err := json.Marshal(mapping)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func tryRecoverDouyinDraftFromPlatform(ctx context.Context, client *platformdouyin.Client, shopID, outerProductID string) (*platformdouyin.PlatformProductResult, bool, error) {
	if client == nil {
		return nil, false, nil
	}
	detail, err := client.GetProductDetailByOuterID(ctx, shopID, outerProductID)
	if err != nil {
		var de *platformdouyin.Error
		if errors.As(err, &de) && de.Code == platformdouyin.CodeDouyinProductNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}
	res := platformdouyin.ProductResultFromDetail(detail)
	if res == nil || res.PlatformProductID == "" {
		return nil, false, nil
	}
	return res, true, nil
}
