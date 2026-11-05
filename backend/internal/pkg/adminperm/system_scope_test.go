package adminperm

import "testing"

func TestIsSystemControlPlaneRouteIncludesGlobalSettingsDiagnostics(t *testing.T) {
	tests := []struct {
		method string
		route  string
	}{
		{method: "GET", route: "/api/v1/settings/config-status"},
		{method: "POST", route: "/api/v1/storage/test-public-access"},
		{method: "POST", route: "/api/v1/settings/storage/public-check"},
		{method: "GET", route: "/api/v1/settings/storage/public-check/latest"},
	}
	for _, tt := range tests {
		if !IsSystemControlPlaneRoute(tt.method, tt.route) {
			t.Errorf("IsSystemControlPlaneRoute(%q, %q) = false, want true", tt.method, tt.route)
		}
	}
}

func TestIsSystemControlPlaneRouteDoesNotUsePathPrefixes(t *testing.T) {
	if IsSystemControlPlaneRoute("GET", "/api/v1/settings/config-status/other") {
		t.Fatal("route prefix must not grant system control-plane access")
	}
	if IsSystemControlPlaneRoute("GET", "/api/v1/storage/test-public-access") {
		t.Fatal("method mismatch must not grant system control-plane access")
	}
}
