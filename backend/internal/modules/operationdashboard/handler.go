package operationdashboard

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler serves GET /dashboard/product-operations.
type Handler struct {
	Svc *Service
}

func parseRFC3339Dashboard(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ProductOperations GET /api/v1/dashboard/product-operations
func (h *Handler) ProductOperations(c *gin.Context) {
	if h == nil || h.Svc == nil {
		response.Fail(c, 500, response.CodeInternalError, "dashboard unavailable")
		return
	}
	startPtr, err := parseRFC3339Dashboard(c.Query("start"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid start time (RFC3339)")
		return
	}
	endPtr, err := parseRFC3339Dashboard(c.Query("end"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid end time (RFC3339)")
		return
	}
	q := Query{
		Start:    startPtr,
		End:      endPtr,
		Platform: c.Query("platform"),
		ShopID:   c.Query("shopId"),
		Source:   c.Query("source"),
	}
	out, err := h.Svc.GetProductOperationDashboard(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}
