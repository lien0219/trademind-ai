package inventory

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterExposesWarehouseLedgerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	Register(router.Group("/api/v1"), &Handler{Svc: &Service{}})
	wanted := map[string]bool{
		"GET /api/v1/products/:id/skus/:skuId/warehouse-balances": false,
		"POST /api/v1/products/:id/skus/:skuId/adjust-stock":      false,
		"GET /api/v1/inventory/warehouse-ledger/reconciliation":   false,
		"POST /api/v1/inventory/warehouse-ledger/migrate-legacy":  false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := wanted[key]; ok {
			wanted[key] = true
		}
	}
	for route, found := range wanted {
		if !found {
			t.Errorf("missing route %s", route)
		}
	}
}
