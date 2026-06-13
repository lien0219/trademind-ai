package douyinshop

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	path := filepath.Join("testdata", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	return root
}

func fixtureData(t *testing.T, name string) map[string]any {
	root := loadFixture(t, name)
	data, _ := root["data"].(map[string]any)
	if data == nil {
		t.Fatalf("fixture %s missing data object", name)
	}
	return data
}

func fixtureFileJSON(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(b)
}

func TestContractProductDetailSuccessFixture(t *testing.T) {
	data := fixtureData(t, "product_detail_success.json")
	res := parseProductDetailResult(data)
	if res == nil || res.PlatformProductID != "3612228815260098474" {
		t.Fatalf("unexpected product detail: %+v", res)
	}
	if len(res.SKUs) != 1 || res.SKUs[0].PlatformSKUID != "51430713091" {
		t.Fatalf("unexpected skus: %+v", res.SKUs)
	}
	if res.SKUs[0].PriceYuan != 338 {
		t.Fatalf("expected fen->yuan conversion, got %v", res.SKUs[0].PriceYuan)
	}
}

func TestContractOrderSearchSuccessFixture(t *testing.T) {
	data := fixtureData(t, "order_search_success.json")
	list, _ := data["shop_order_list"].([]any)
	if len(list) != 1 {
		t.Fatalf("expected one order in fixture")
	}
	raw, ok := list[0].(map[string]any)
	if !ok {
		t.Fatal("invalid order row")
	}
	po := mapDouyinOrder(raw)
	if po.ExternalOrderID != "FIXTURE-ORDER-1" {
		t.Fatalf("unexpected external id: %s", po.ExternalOrderID)
	}
	if po.Status != "paid" {
		t.Fatalf("expected paid status, got %s", po.Status)
	}
}

func TestContractProductDetailNotFoundFixture(t *testing.T) {
	root := loadFixture(t, "product_detail_not_found.json")
	err := mapProductDetailError(MapPlatformError(mapString(root, "code"), mapString(root, "msg")+" "+mapString(root, "sub_msg"), ""))
	if err == nil {
		t.Fatal("expected mapped error")
	}
	var de *Error
	if !AsError(err, &de) {
		t.Fatalf("expected douyin error, got %v", err)
	}
	if de.Code != CodeDouyinProductDetailPermissionDenied {
		t.Fatalf("unexpected code: %s", de.Code)
	}
}

func TestContractOrderSearchRateLimitedFixture(t *testing.T) {
	root := loadFixture(t, "order_search_rate_limited.json")
	err := mapOrderListError(MapPlatformError(mapString(root, "code"), mapString(root, "msg"), ""))
	if err == nil {
		t.Fatal("expected mapped error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinOrderRateLimited {
		t.Fatalf("expected rate limited code, got %v", err)
	}
}

func TestGetProductDetailByOuterIDUsesOutProductParam(t *testing.T) {
	client := &Client{
		Config:      testRuntimeConfig(),
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := fixtureFileJSON(t, "product_detail_success.json")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	out, err := client.GetProductDetailByOuterID(context.Background(), "shop-1", "prod-local-1")
	if err != nil {
		t.Fatalf("GetProductDetailByOuterID failed: %v", err)
	}
	if out == nil || out.PlatformProductID == "" {
		t.Fatal("expected product detail")
	}
}

func TestContractTokenCreateFixtureShape(t *testing.T) {
	data := fixtureData(t, "token_create_success.json")
	if mapString(data, "access_token") == "" || mapString(data, "refresh_token") == "" {
		t.Fatalf("token fixture missing tokens: %+v", data)
	}
	if strings.Contains(mapString(data, "access_token"), "sk-") {
		t.Fatal("fixture must not contain real-looking secrets")
	}
}
