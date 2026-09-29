package advertisingfee

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	adminmodel "github.com/trademind-ai/trademind/backend/internal/modules/admin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/httpapi"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

const maxJSONBody = int64(32 << 10)

type Handler struct {
	Svc   *Service
	OpLog *operationlog.Service
}

func (h *Handler) authorize(c *gin.Context, permission string, write bool) (int64, *adminperm.Principal, bool) {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, http.StatusInternalServerError, response.CodeInternalError, "广告费用归属账不可用")
		return 0, nil, false
	}
	principal, err := adminperm.LoadPrincipal(c, h.Svc.DB)
	if err != nil || principal == nil || !principal.Can(permission) || (write && principal.IsReadonly()) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "无广告费用归属账操作权限")
		return 0, nil, false
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "缺少租户上下文")
		return 0, nil, false
	}
	return tenantID, principal, true
}

func actorID(principal *adminperm.Principal) *uuid.UUID {
	if principal == nil || principal.UserID == uuid.Nil {
		return nil
	}
	id := principal.UserID
	return &id
}

func scopeForPrincipal(principal *adminperm.Principal, write bool) Scope {
	if principal == nil || principal.IsAdmin() {
		return Scope{}
	}
	if !write {
		return Scope{RestrictStoreScope: true, AllowedShopIDs: principal.AllowedStoreIDs()}
	}
	allowed := make([]uuid.UUID, 0, len(principal.StoreGrants))
	for _, grant := range principal.StoreGrants {
		scope := strings.ToLower(strings.TrimSpace(grant.PermissionScope))
		if grant.StoreID != uuid.Nil && (scope == adminmodel.StorePermScopeOperate || scope == adminmodel.StorePermScopeManage) {
			allowed = append(allowed, grant.StoreID)
		}
	}
	return Scope{RestrictStoreScope: true, AllowedShopIDs: allowed}
}

func (h *Handler) Preview(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermAdvertisingFeeImport, true)
	if !ok {
		return
	}
	shopID, fileName, data, ok := bindUpload(c)
	if !ok {
		return
	}
	if !principal.CanOperateStore(shopID) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "需要店铺操作权限")
		return
	}
	result, err := h.Svc.Preview(c.Request.Context(), tenantID, scopeForPrincipal(principal, true), shopID, fileName, data)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) Confirm(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermAdvertisingFeeImport, true)
	if !ok {
		return
	}
	shopID, fileName, data, ok := bindUpload(c)
	if !ok {
		return
	}
	if !principal.CanOperateStore(shopID) {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "需要店铺操作权限")
		return
	}
	result, err := h.Svc.Confirm(
		c.Request.Context(), tenantID, scopeForPrincipal(principal, true), shopID, fileName, data,
		c.PostForm("expectedFileHash"), c.PostForm("expectedCalculationHash"), c.PostForm("idempotencyKey"), actorID(principal),
	)
	if err != nil {
		handleError(c, err)
		return
	}
	if !result.Replayed && h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			TenantID: tenantID, Action: "advertising_fee.import.confirm", Resource: "advertising_fee_import",
			ResourceID: result.Import.ID.String(), ShopID: &shopID, Platform: result.Import.Platform,
			Permission: adminperm.PermAdvertisingFeeImport, Status: "success",
			Message: fmt.Sprintf("spends=%d allocations=%d duplicates=%d", result.Import.ImportedSpends, result.Import.AllocationCount, result.Import.DuplicateSpends),
		})
	}
	response.OK(c, result)
}

func (h *Handler) ListAllocations(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermAdvertisingFeeView, false)
	if !ok {
		return
	}
	query := ListQuery{
		Page: positiveInt(c.Query("page"), 1), PageSize: positiveInt(c.Query("pageSize"), 20),
		OrderNo: c.Query("orderNo"), Currency: c.Query("currency"), SettlementCoverage: c.Query("settlementCoverage"),
	}
	if value := strings.TrimSpace(c.Query("shopId")); value != "" {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil {
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "店铺编号无效")
			return
		}
		query.ShopID = &id
	}
	result, err := h.Svc.ListAllocations(c.Request.Context(), tenantID, scopeForPrincipal(principal, false), query)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) GetAllocation(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermAdvertisingFeeView, false)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	result, err := h.Svc.GetAllocation(c.Request.Context(), tenantID, scopeForPrincipal(principal, false), id)
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) ListImports(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermAdvertisingFeeView, false)
	if !ok {
		return
	}
	result, err := h.Svc.ListImports(c.Request.Context(), tenantID, scopeForPrincipal(principal, false), positiveInt(c.Query("page"), 1), positiveInt(c.Query("pageSize"), 20))
	if err != nil {
		handleError(c, err)
		return
	}
	response.OK(c, result)
}

func (h *Handler) CreateAdjustment(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermAdvertisingFeeManage, true)
	if !ok {
		return
	}
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input CreateAdjustmentInput
	if !bindJSON(c, &input) {
		return
	}
	result, err := h.Svc.CreateAdjustment(c.Request.Context(), tenantID, scopeForPrincipal(principal, true), actorID(principal), id, input)
	if err != nil {
		handleError(c, err)
		return
	}
	h.log(c, tenantID, adminperm.PermAdvertisingFeeManage, "advertising_fee.adjustment.create", "advertising_fee_adjustment", result.Adjustment.ID)
	response.OK(c, result)
}

func (h *Handler) ReverseAdjustment(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermAdvertisingFeeManage, true)
	if !ok {
		return
	}
	allocationID, ok := parseID(c, "id")
	if !ok {
		return
	}
	adjustmentID, ok := parseID(c, "adjustmentId")
	if !ok {
		return
	}
	var input ReverseAdjustmentInput
	if !bindJSON(c, &input) {
		return
	}
	result, err := h.Svc.ReverseAdjustment(c.Request.Context(), tenantID, scopeForPrincipal(principal, true), actorID(principal), allocationID, adjustmentID, input)
	if err != nil {
		handleError(c, err)
		return
	}
	h.log(c, tenantID, adminperm.PermAdvertisingFeeManage, "advertising_fee.adjustment.reverse", "advertising_fee_adjustment", result.Adjustment.ID)
	response.OK(c, result)
}

func bindUpload(c *gin.Context) (uuid.UUID, string, []byte, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxFileBytes+1024*1024)
	shopID, err := uuid.Parse(strings.TrimSpace(c.PostForm("shopId")))
	if err != nil || shopID == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "店铺编号无效")
		return uuid.Nil, "", nil, false
	}
	file, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请选择 CSV 文件")
		return uuid.Nil, "", nil, false
	}
	fileName := filepath.Base(strings.ReplaceAll(file.Filename, "\\", "/"))
	if fileName == "" || len(fileName) > 255 || !strings.EqualFold(filepath.Ext(fileName), ".csv") {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "文件必须使用 .csv 扩展名")
		return uuid.Nil, "", nil, false
	}
	if file.Size > MaxFileBytes {
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, "CSV 文件不能超过 2 MiB")
		return uuid.Nil, "", nil, false
	}
	source, err := file.Open()
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "CSV 文件无法打开")
		return uuid.Nil, "", nil, false
	}
	defer source.Close()
	data, err := io.ReadAll(io.LimitReader(source, MaxFileBytes+1))
	if err != nil || len(data) > MaxFileBytes {
		response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, "CSV 文件不能超过 2 MiB")
		return uuid.Nil, "", nil, false
	}
	return shopID, fileName, data, true
}

func bindJSON(c *gin.Context, target any) bool {
	if err := httpapi.BindStrictJSON(c, target, maxJSONBody); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "请求内容无效")
		return false
	}
	return true
}

func parseID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param(name)))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "广告费用记录编号无效")
		return uuid.Nil, false
	}
	return id, true
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
	case errors.Is(err, ErrNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "广告费用记录不存在")
	case errors.Is(err, ErrConflict):
		response.Fail(c, http.StatusConflict, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func (h *Handler) log(c *gin.Context, tenantID int64, permission, action, resource string, resourceID uuid.UUID) {
	if h.OpLog == nil {
		return
	}
	_ = h.OpLog.Write(c, operationlog.WriteOpts{TenantID: tenantID, Action: action, Resource: resource, ResourceID: resourceID.String(), Permission: permission, Status: "success"})
}
