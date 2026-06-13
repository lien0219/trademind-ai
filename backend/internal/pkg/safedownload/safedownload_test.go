package safedownload

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestValidateURLBlocksLocalhost(t *testing.T) {
	for _, u := range []string{
		"http://localhost/x.png",
		"http://127.0.0.1/x.png",
		"http://[::1]/x.png",
		"http://10.0.0.1/x.png",
		"http://172.16.0.1/x.png",
		"http://192.168.1.1/x.png",
		"http://169.254.169.254/latest/meta-data",
	} {
		if err := ValidateURL(context.Background(), u); err == nil {
			t.Fatalf("expected block for %s", u)
		}
	}
}

func TestValidateURLBlocksCredentials(t *testing.T) {
	err := ValidateURL(context.Background(), "http://user:pass@example.com/x.png")
	if err == nil || !strings.Contains(err.Error(), ErrCredentialsInURL) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateURLBlocksNonHTTP(t *testing.T) {
	err := ValidateURL(context.Background(), "file:///etc/passwd")
	if err == nil {
		t.Fatal("expected scheme block")
	}
}

func TestIsPrivateIP(t *testing.T) {
	if !IsPrivateIP(parseIP("10.1.2.3")) {
		t.Fatal("10.x should be private")
	}
}

func parseIP(s string) net.IP {
	return net.ParseIP(s)
}
