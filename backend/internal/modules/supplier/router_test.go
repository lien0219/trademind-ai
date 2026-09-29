package supplier

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterExposesSupplierManagementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine.Group("/api/v1"), &Handler{})
	want := map[string]bool{
		"GET /api/v1/suppliers":           false,
		"POST /api/v1/suppliers":          false,
		"PUT /api/v1/suppliers/:id":       false,
		"GET /api/v1/suppliers/:id/skus":  false,
		"POST /api/v1/suppliers/:id/skus": false,
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
