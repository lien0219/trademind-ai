package profitability

import (
	"encoding/csv"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FXRateInput struct {
	FromCurrency string    `json:"fromCurrency"`
	ToCurrency   string    `json:"toCurrency"`
	Rate         string    `json:"rate"`
	EffectiveAt  time.Time `json:"effectiveAt"`
	Evidence     string    `json:"evidence"`
}

func (h *Handler) ListFXRates(c *gin.Context) {
	tenantID, _, ok := h.authorize(c, adminperm.PermOrderProfitView)
	if !ok {
		return
	}
	base := strings.ToUpper(strings.TrimSpace(c.Query("baseCurrency")))
	rates, err := h.Svc.ListFXRates(c.Request.Context(), tenantID, base)
	if errors.Is(err, ErrFXInvalid) {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid base currency")
		return
	}
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rates})
}

func (h *Handler) CreateFXRate(c *gin.Context) {
	tenantID, principal, ok := h.authorize(c, adminperm.PermSettingsManage)
	if !ok {
		return
	}
	if tenantID <= 0 || principal == nil || !principal.IsAdmin() || h.Audit == nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "active tenant administrator required")
		return
	}
	var input FXRateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid FX rate request")
		return
	}
	now := h.Svc.now()
	row := FXRate{
		ID: uuid.New(), TenantID: tenantID, CreatedBy: principal.UserID,
		FromCurrency: strings.TrimSpace(input.FromCurrency),
		ToCurrency:   strings.TrimSpace(input.ToCurrency),
		Rate:         input.Rate, EffectiveAt: input.EffectiveAt.UTC(),
		Evidence: input.Evidence, CreatedAt: now,
	}
	if err := ValidateFXRate(&row, now); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid FX rate or evidence")
		return
	}
	err := h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var tenant struct {
			ID int64
		}
		if err := tx.Table("tenants").Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").Where("id = ? AND status = ?", tenantID, "active").Take(&tenant).Error; err != nil {
			return err
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return h.Audit.WriteTxContext(c.Request.Context(), tx, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: &principal.UserID,
			Action: "finance.fx_rate.create", Resource: "tenant_fx_rate",
			ResourceID: row.ID.String(), Status: "success",
			Message: row.FromCurrency + "/" + row.ToCurrency + " " + row.Rate +
				" effective=" + row.EffectiveAt.Format(time.RFC3339Nano) +
				" evidence=" + row.Evidence,
		})
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, row)
}

func (h *Handler) FXReport(c *gin.Context) {
	format := strings.ToLower(strings.TrimSpace(c.Query("format")))
	if format != "" && format != "csv" {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid format")
		return
	}
	permission := adminperm.PermOrderProfitView
	if format == "csv" {
		permission = adminperm.PermOrderProfitExport
	}
	tenantID, principal, ok := h.authorize(c, permission)
	if !ok {
		return
	}
	q, err := bindListQuery(c, true)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	base := strings.ToUpper(strings.TrimSpace(c.Query("baseCurrency")))
	report, err := h.Svc.BuildFXReport(c.Request.Context(), tenantID, scopeForPrincipal(principal), q, base)
	if err != nil {
		switch {
		case errors.Is(err, ErrFXInvalid), errors.Is(err, ErrInvalidQuery):
			response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, ErrTooManyRows), errors.Is(err, ErrFXTooMany):
			response.Fail(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, err.Error())
		default:
			response.HandleError(c, err)
		}
		return
	}
	if format == "csv" {
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="order-profit-currency-report.csv"`)
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
		if err := WriteFXCSV(c.Writer, report); err != nil {
			_ = c.Error(err)
		}
		return
	}
	response.OK(c, report)
}

// WriteFXCSV exports the original and converted amounts together with the
// exact rate identifier and timestamp; formula-like text is neutralized.
func WriteFXCSV(writer interface{ Write([]byte) (int, error) }, report *FXReport) error {
	if report == nil {
		return ErrFXInvalid
	}
	csvWriter := csv.NewWriter(writer)
	if err := csvWriter.Write([]string{
		"orderNo", "platform", "shop", "originalCurrency", "baseCurrency",
		"conversionStatus", "originalRevenueMinor", "originalEstimatedProfitMinor",
		"convertedRevenueMinor", "convertedProductCostMinor", "convertedFreightMinor",
		"convertedRefundMinor", "convertedPlatformFeeMinor", "convertedAdvertisingFeeMinor",
		"convertedWarehouseFeeMinor", "convertedKnownContributionMinor",
		"convertedEstimatedProfitMinor", "rate", "rateId", "rateEffectiveAt",
		"orderDate", "calculatedAt", "formulaVersion",
	}); err != nil {
		return err
	}
	for _, row := range report.Rows {
		rateID := ""
		if row.RateID != nil {
			rateID = row.RateID.String()
		}
		effectiveAt, orderedAt := "", ""
		if row.RateEffectiveAt != nil {
			effectiveAt = row.RateEffectiveAt.Format(time.RFC3339Nano)
		}
		if row.OrderedAt != nil {
			orderedAt = row.OrderedAt.Format(time.RFC3339Nano)
		}
		converted := row.Converted
		values := []string{
			csvSafe(row.OrderNo), csvSafe(row.Platform), csvSafe(row.ShopName),
			row.Currency, row.BaseCurrency, row.ConversionStatus,
			optionalInt(row.Components.Revenue.AmountMinor), optionalInt(row.EstimatedProfitMinor),
			optionalInt(converted.RevenueMinor), optionalInt(converted.ProductCostMinor),
			optionalInt(converted.FreightMinor), optionalInt(converted.RefundMinor),
			optionalInt(converted.PlatformFeeMinor), optionalInt(converted.AdvertisingFeeMinor),
			optionalInt(converted.WarehouseFeeMinor), optionalInt(converted.KnownContributionMinor),
			optionalInt(converted.EstimatedProfitMinor), row.Rate,
			rateID, effectiveAt, orderedAt, report.CalculatedAt.Format(time.RFC3339Nano),
			report.FormulaVersion,
		}
		if err := csvWriter.Write(values); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}
