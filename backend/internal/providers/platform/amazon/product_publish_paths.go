package amazon

import (
	"net/url"
	"strings"
)

const PathListingsItem = "/listings/2021-08-01/items/{sellerId}/{sku}"

func listingsItemPath(sellerID, sku string) string {
	p := strings.Replace(PathListingsItem, "{sellerId}", url.PathEscape(strings.TrimSpace(sellerID)), 1)
	return strings.Replace(p, "{sku}", url.PathEscape(strings.TrimSpace(sku)), 1)
}
