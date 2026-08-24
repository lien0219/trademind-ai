package salesreturn

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterMountsSalesReturnRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/api/v1"), &Handler{})
	want := map[string]bool{
		"GET /api/v1/orders/:id/sales-returnable-items":            false,
		"GET /api/v1/sales-returns":                                false,
		"GET /api/v1/sales-return-reconciliation":                  false,
		"GET /api/v1/sales-return-reconciliation/:id":              false,
		"GET /api/v1/refund-executions":                            false,
		"GET /api/v1/refund-executions/:id":                        false,
		"POST /api/v1/sales-returns":                               false,
		"GET /api/v1/sales-returns/:id":                            false,
		"POST /api/v1/sales-returns/:id/refund-execution":          false,
		"POST /api/v1/sales-returns/:id/submit":                    false,
		"POST /api/v1/sales-returns/:id/approve":                   false,
		"POST /api/v1/sales-returns/:id/complete":                  false,
		"POST /api/v1/sales-returns/:id/cancel":                    false,
		"POST /api/v1/refund-executions/:id/result":                false,
		"POST /api/v1/refund-executions/:id/confirm-from-platform": false,
		"POST /api/v1/refund-executions/:id/cancel":                false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
