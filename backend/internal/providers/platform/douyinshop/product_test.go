package douyinshop

import (
	"context"
	"testing"
)

func TestCreateProductDraftValidatesRequiredFields(t *testing.T) {
	c := &Client{Config: RuntimeConfig{AppKey: "k", AppSecret: "s"}}
	_, err := c.CreateProductDraft(context.Background(), "shop-1", CreateProductDraftRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinProductPayloadInvalid {
		t.Fatalf("expected payload invalid code, got %v", err)
	}
}

func TestParseCreateProductResultMapsProductID(t *testing.T) {
	res := parseCreateProductResult(map[string]any{
		"product_id": "1234567890",
		"sku": []any{map[string]any{
			"sku_id": "999", "outer_sku_id": "local-1",
		}},
	})
	if res.PlatformProductID != "1234567890" {
		t.Fatalf("unexpected product id: %s", res.PlatformProductID)
	}
	if len(res.SKUMappings) != 1 || res.SKUMappings[0].PlatformSKUID != "999" {
		t.Fatalf("unexpected sku mappings: %+v", res.SKUMappings)
	}
}

func TestSanitizeErrorTextRedactsSecrets(t *testing.T) {
	msg := SanitizeErrorText("access_token invalid secret leaked")
	if msg == "" || msg == "access_token invalid secret leaked" {
		t.Fatalf("expected sanitized message, got %q", msg)
	}
}
