package tiktok

import "strings"

func productCreatePath(apiVersion string) string {
	return "/product/" + strings.TrimSpace(apiVersion) + "/products"
}

func productImageUploadPath(apiVersion string) string {
	return "/product/" + strings.TrimSpace(apiVersion) + "/images/upload"
}
