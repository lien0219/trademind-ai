package profitability

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type Handler struct {
	Svc *Service
}

func (h *Handler) authorize(c *gin.Context, permission string) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "order profitability unavailable")
		return 0, nil, false
	}
	if !adminperm.RequirePermission(c, h.Svc.DB, permission) {
		return 0, nil, false
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context missing")
		return 0, nil, false
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil {
		response.HandleError(c, err)
		return 0, nil, false
	}
	return tenantID, principal, true
}

func scopeForPrincipal(principal *adminperm.Principal) Scope {
	if principal == nil || principal.IsAdmin() {
		return Scope{}
	}
	return Scope{RestrictStoreScope: true, AllowedShopIDs: principal.AllowedStoreIDs()}
}

func (h *Handler) List(c *gin.Context) {
	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	permission := adminperm.PermOrderProfitView
	if format == "csv" {
		permission = adminperm.PermOrderProfitExport
	} else if format != "" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid format")
		return
	}
	tenantID, principal, ok := h.authorize(c, permission)
	if !ok {
		return
	}
	query, err := bindListQuery(c, format == "csv")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	result, err := h.Svc.List(c.Request.Context(), tenantID, scopeForPrincipal(principal), query)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidQuery):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, ErrTooManyRows):
			response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, err.Error())
		default:
			response.HandleError(c, err)
		}
		return
	}
	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="order-profit-estimates.csv"`)
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
		_ = WriteCSV(c.Writer, result.List)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Get(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermOrderProfitView)
	if !ok {
		return
	}
	orderID, err := uuid.Parse(strings.TrimSpace(c.Param("orderId")))
	if err != nil || orderID == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid order id")
		return
	}
	result, err := h.Svc.Get(c.Request.Context(), tenantID, scopeForPrincipal(principal), orderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		response.HandleError(c, err)
		return
	}
	response.OK(c, result)
}

func bindListQuery(c *gin.Context, export bool) (ListQuery, error) {
	q := ListQuery{
		Page: positiveInt(c.Query("page"), 1), PageSize: positiveInt(c.Query("pageSize"), 20),
		OrderNo: c.Query("orderNo"), Platform: c.Query("platform"), Currency: c.Query("currency"),
		Status: c.Query("status"), Export: export,
	}
	for key, target := range map[string]**uuid.UUID{"shopId": &q.ShopID, "warehouseId": &q.WarehouseID} {
		value := strings.TrimSpace(c.Query(key))
		if value == "" {
			continue
		}
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return q, fmt.Errorf("invalid %s", key)
		}
		*target = &id
	}
	for key, target := range map[string]**time.Time{"start": &q.Start, "end": &q.End} {
		value := strings.TrimSpace(c.Query(key))
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return q, fmt.Errorf("invalid %s", key)
		}
		parsed = parsed.UTC()
		*target = &parsed
	}
	return q, nil
}

func positiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func WriteCSV(writer interface{ Write([]byte) (int, error) }, rows []OrderProfit) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"订单号", "平台", "店铺", "仓库", "币种", "收入(最小单位)", "已知商品成本(最小单位)", "已知运费(最小单位)", "已成功退款(最小单位)", "已知贡献额(最小单位)", "预估利润(最小单位)", "完整性状态", "缺口代码", "计算版本", "计算时间"}); err != nil {
		return err
	}
	for _, row := range rows {
		issues := make([]string, 0, len(row.Issues))
		for _, issue := range row.Issues {
			issues = append(issues, issue.Code)
		}
		values := []string{
			csvSafe(row.OrderNo), csvSafe(row.Platform), csvSafe(row.ShopName), csvSafe(strings.TrimSpace(row.WarehouseCode + " " + row.WarehouseName)), row.Currency,
			optionalInt(row.Components.Revenue.AmountMinor), strconv.FormatInt(row.Components.ProductCost.KnownAmountMinor, 10),
			strconv.FormatInt(row.Components.Freight.KnownAmountMinor, 10), strconv.FormatInt(row.Components.Refund.KnownAmountMinor, 10),
			optionalInt(row.KnownContributionMinor), optionalInt(row.EstimatedProfitMinor), row.Status, strings.Join(issues, "|"), row.FormulaVersion, row.CalculatedAt.Format(time.RFC3339),
		}
		if err := csvWriter.Write(values); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func csvSafe(value string) string {
	trimmed := strings.TrimLeft(value, "\t\r ")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func optionalInt(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}
