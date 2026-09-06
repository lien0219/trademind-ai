package logistics

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterExposesLogisticsManagementRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")
	Register(group, &Handler{})
	want := map[string]bool{
		"GET /api/v1/logistics/channels":           false,
		"POST /api/v1/logistics/channels":          false,
		"PUT /api/v1/logistics/channels/:id":       false,
		"GET /api/v1/logistics/rate-templates":     false,
		"POST /api/v1/logistics/rate-templates":    false,
		"PUT /api/v1/logistics/rate-templates/:id": false,
	}
	for _, route := range router.Routes() {
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
