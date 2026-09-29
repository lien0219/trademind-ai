package advertisingfee

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterExposesOnlyAppendOnlyAdvertisingWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/api/v1"), &Handler{})
	writes := map[string]bool{}
	for _, route := range router.Routes() {
		if route.Method != http.MethodGet {
			writes[route.Method+" "+route.Path] = true
		}
		if route.Method == http.MethodPut || route.Method == http.MethodPatch || route.Method == http.MethodDelete {
			t.Fatalf("mutable advertising fee route registered: %s %s", route.Method, route.Path)
		}
	}
	for _, expected := range []string{
		"POST /api/v1/advertising-fee-imports/preview",
		"POST /api/v1/advertising-fee-imports",
		"POST /api/v1/advertising-fees/:id/adjustments",
		"POST /api/v1/advertising-fees/:id/adjustments/:adjustmentId/reverse",
	} {
		if !writes[expected] {
			t.Fatalf("missing append-only route %s", expected)
		}
	}
}
