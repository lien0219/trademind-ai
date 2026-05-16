package lazada

import "testing"

func TestSign_sampleFromOpenPlatformDoc(t *testing.T) {
	// From Lazada “HTTP request sample” (GetOrder): secret "helloworld", sign_method sha256.
	params := map[string]string{
		"access_token": "test",
		"app_key":      "123456",
		"order_id":     "1234",
		"sign_method":  "sha256",
		"timestamp":    "1517820392000",
	}
	got := Sign("/order/get", params, "", "helloworld")
	want := "4190D32361CFB9581350222F345CB77F3B19F0E31D162316848A2C1FFD5FAB4A"
	if got != want {
		t.Fatalf("sign mismatch: got %s want %s", got, want)
	}
}
