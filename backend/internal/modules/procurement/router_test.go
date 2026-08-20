package procurement

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterExposesPurchaseOrderWorkflowRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine.Group("/api/v1"), &Handler{})
	want := map[string]bool{
		"GET /api/v1/purchase-orders":               false,
		"POST /api/v1/purchase-orders":              false,
		"GET /api/v1/purchase-orders/:id":           false,
		"POST /api/v1/purchase-orders/:id/submit":   false,
		"POST /api/v1/purchase-orders/:id/approve":  false,
		"POST /api/v1/purchase-orders/:id/cancel":   false,
		"POST /api/v1/purchase-orders/:id/close":    false,
		"POST /api/v1/purchase-orders/:id/receipts": false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Fatalf("missing route %s", route)
		}
	}
}
