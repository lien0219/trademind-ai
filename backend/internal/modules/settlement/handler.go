package settlement

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "settlement reconciliation unavailable")
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

func (h *Handler) Preview(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSettlementImport)
	if !ok {
		return
	}
	shopID, fileName, data, ok := bindUpload(c)
	if !ok {
		return
	}
	if principal == nil || !principal.CanOperateStore(shopID) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "shop operation permission required")
		return
	}
	result, err := h.Svc.Preview(c.Request.Context(), tenantID, scopeForPrincipal(principal), shopID, fileName, data)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Confirm(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSettlementImport)
	if !ok {
		return
	}
	shopID, fileName, data, ok := bindUpload(c)
	if !ok {
		return
	}
	if principal == nil || !principal.CanOperateStore(shopID) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "shop operation permission required")
		return
	}
	result, err := h.Svc.Confirm(
		c.Request.Context(), tenantID, scopeForPrincipal(principal), shopID, fileName, data,
		c.PostForm("expectedFileHash"), c.PostForm("idempotencyKey"), &principal.UserID,
	)
	if err != nil {
		handleError(c, err)
		return
	}
	if !result.Replayed && h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			TenantID: tenantID, Action: "settlement.import.confirm", Resource: "settlement_import",
			ResourceID: result.Import.ID.String(), ShopID: &shopID, Platform: result.Import.Platform,
			Permission: adminperm.PermSettlementImport, Status: "success",
			Message: fmt.Sprintf("rows=%d duplicates=%d", result.Import.ImportedRows, result.Import.DuplicateRows),
		})
	}
	response.OK(c, result)
}

func (h *Handler) List(c *gin.Context) {
	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	permission := adminperm.PermSettlementView
	if format == "csv" {
		permission = adminperm.PermSettlementExport
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
		handleError(c, err)
		return
	}
	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="settlement-reconciliation.csv"`)
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
		_ = WriteCSV(c.Writer, result.List)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Get(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSettlementView)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid reconciliation id")
		return
	}
	result, err := h.Svc.Get(c.Request.Context(), tenantID, scopeForPrincipal(principal), id)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func bindUpload(c *gin.Context) (uuid.UUID, string, []byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxFileBytes+1024*1024)
	shopID, err := uuid.Parse(strings.TrimSpace(c.PostForm("shopId")))
	if err != nil || shopID == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid shopId")
		return uuid.Nil, "", nil, false
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "CSV file is required")
		return uuid.Nil, "", nil, false
	}
	fileName := filepath.Base(strings.ReplaceAll(file.Filename, "\\", "/"))
	if fileName == "" || len(fileName) > 255 || !strings.EqualFold(filepath.Ext(fileName), ".csv") {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "file must use .csv extension")
		return uuid.Nil, "", nil, false
	}
	if file.Size > MaxFileBytes {
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, "CSV file exceeds 2 MiB")
		return uuid.Nil, "", nil, false
	}
	source, err := file.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "CSV file cannot be opened")
		return uuid.Nil, "", nil, false
	}
	defer source.Close()
	data, err := io.ReadAll(io.LimitReader(source, MaxFileBytes+1))
	if err != nil || len(data) > MaxFileBytes {
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, "CSV file exceeds 2 MiB")
		return uuid.Nil, "", nil, false
	}
	return shopID, fileName, data, true
}

func bindListQuery(c *gin.Context, export bool) (ListQuery, error) {
	q := ListQuery{
		Page: positiveInt(c.Query("page"), 1), PageSize: positiveInt(c.Query("pageSize"), 20),
		OrderNo: c.Query("orderNo"), Platform: c.Query("platform"), Currency: c.Query("currency"),
		Status: c.Query("status"), Export: export,
	}
	if value := strings.TrimSpace(c.Query("shopId")); value != "" {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			return q, fmt.Errorf("invalid shopId")
		}
		q.ShopID = &id
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

func handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrConflict):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrTooManyRows):
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
	default:
		response.HandleError(c, err)
	}
}

func WriteCSV(writer interface{ Write([]byte) (int, error) }, rows []ReconciliationRow) error {
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{"对账ID", "订单号", "平台", "店铺", "币种", "本地订单金额(最小单位)", "账单交易总额(最小单位)", "平台费用(最小单位)", "结算金额(最小单位)", "交易数", "对账状态", "问题代码", "最后结算时间"}); err != nil {
		return err
	}
	for _, row := range rows {
		codes := make([]string, 0, len(row.Issues))
		for _, issue := range row.Issues {
			codes = append(codes, issue.Code)
		}
		values := []string{
			row.ID.String(), csvSafe(row.OrderNo), csvSafe(row.Platform), csvSafe(row.ShopName), row.Currency,
			optionalInt(row.OrderAmountMinor), optionalInt(row.OrderGrossMinor), optionalInt(row.PlatformFeeMinor),
			optionalInt(row.SettlementAmountMinor), strconv.Itoa(row.TransactionCount), row.Status,
			strings.Join(codes, "|"), row.LastSettledAt.Format(time.RFC3339),
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
