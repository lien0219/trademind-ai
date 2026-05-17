package tiktok

import "strings"

func inventoryUpdateAPIPath(apiVersion, externalProductID string) string {
	return "/product/" + strings.TrimSpace(apiVersion) + "/products/" + strings.TrimSpace(externalProductID) + "/inventory/update"
}
