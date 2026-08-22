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
		"GET /api/v1/inventory/warehouse-transfers":               false,
		"GET /api/v1/inventory/warehouse-transfers/:id":           false,
		"POST /api/v1/inventory/warehouse-transfers":              false,
		"POST /api/v1/inventory/warehouse-transfers/:id/submit":   false,
		"POST /api/v1/inventory/warehouse-transfers/:id/approve":  false,
		"POST /api/v1/inventory/warehouse-transfers/:id/dispatch": false,
		"POST /api/v1/inventory/warehouse-transfers/:id/receive":  false,
		"POST /api/v1/inventory/warehouse-transfers/:id/cancel":   false,
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
