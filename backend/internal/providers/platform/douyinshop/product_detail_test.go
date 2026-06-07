package douyinshop

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetProductDetailParsesSpecPrices(t *testing.T) {
	client := &Client{
		Config:      testRuntimeConfig(),
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if !strings.Contains(req.URL.Path, "product/detail") {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}
			body := `{"code":10000,"msg":"success","data":{"product_id":"3612228815260098474","name":"测试商品","spec_prices":[{"sku_id":"51430713091","outer_sku_id":"local-1","price":33800,"stock_num":10,"sell_properties":[{"property_name":"颜色","value_name":"红色"}]}]}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	out, err := client.GetProductDetail(context.Background(), "shop-1", "3612228815260098474")
	if err != nil {
		t.Fatalf("GetProductDetail failed: %v", err)
	}
	if out.PlatformProductID != "3612228815260098474" {
		t.Fatalf("unexpected product id: %s", out.PlatformProductID)
	}
	if len(out.SKUs) != 1 || out.SKUs[0].PlatformSKUID != "51430713091" {
		t.Fatalf("unexpected skus: %+v", out.SKUs)
	}
	if out.SKUs[0].PriceYuan != 338 {
		t.Fatalf("expected price 338 yuan, got %v", out.SKUs[0].PriceYuan)
	}
	if out.SKUs[0].Attrs["颜色"] != "红色" {
		t.Fatalf("unexpected attrs: %+v", out.SKUs[0].Attrs)
	}
}

func TestGetProductDetailFailureMapsCode(t *testing.T) {
	client := &Client{
		Config:      testRuntimeConfig(),
		AccessToken: "access",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":40004,"msg":"permission denied","sub_msg":"forbidden"}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	_, err := client.GetProductDetail(context.Background(), "shop-1", "123")
	if err == nil {
		t.Fatal("expected error")
	}
	var de *Error
	if !AsError(err, &de) || de.Code != CodeDouyinProductDetailPermissionDenied {
		t.Fatalf("expected permission denied code, got %v", err)
	}
}

func TestGetProductDetailLogsDoNotLeakSecrets(t *testing.T) {
	var logged SafeRequestLog
	client := &Client{
		Config:      testRuntimeConfig(),
		AccessToken: "secret-access-token",
		Logger: SafeLoggerFunc(func(ctx context.Context, entry SafeRequestLog) {
			logged = entry
		}),
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":10000,"msg":"success","data":{"product_id":"1","spec_prices":[{"sku_id":"2","price":100}]}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		}),
	}
	_, err := client.GetProductDetail(context.Background(), "shop-1", "1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	raw := logged.Method + logged.ErrorCode
	if strings.Contains(strings.ToLower(raw), "secret-access-token") {
		t.Fatal("token leaked into safe log")
	}
}
