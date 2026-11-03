package profitability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
)

const maxFXSnapshotBytes = 8 << 20

var ErrFXSnapshotCorrupt = errors.New("foreign exchange report snapshot integrity check failed")

// FXReportSnapshot freezes the exact displayed report, including source facts,
// matched rates and missing-data statuses. It never overwrites live orders.
type FXReportSnapshot struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID       int64     `gorm:"not null;index:idx_fx_snap_tenant_time,priority:1" json:"-"`
	CreatedBy      uuid.UUID `gorm:"type:uuid;not null" json:"createdBy"`
	BaseCurrency   string    `gorm:"size:3;not null" json:"baseCurrency"`
	TotalOrders    int       `gorm:"not null" json:"totalOrders"`
	IncludedOrders int       `gorm:"not null" json:"includedOrders"`
	ExcludedOrders int       `gorm:"not null" json:"excludedOrders"`
	FormulaVersion string    `gorm:"size:80;not null" json:"formulaVersion"`
	ContentSHA256  string    `gorm:"size:64;not null" json:"contentSha256"`
	ReportJSON     []byte    `gorm:"not null" json:"-"`
	CreatedAt      time.Time `gorm:"not null;index:idx_fx_snap_tenant_time,priority:2" json:"createdAt"`
}

func (FXReportSnapshot) TableName() string { return "tenant_fx_report_snapshots" }
func (*FXReportSnapshot) BeforeUpdate(*gorm.DB) error { return ErrFXInvalid }
func (*FXReportSnapshot) BeforeDelete(*gorm.DB) error { return ErrFXInvalid }

func freezeFXReport(tenantID int64, actor uuid.UUID, report *FXReport, now time.Time) (*FXReportSnapshot, error) {
	if tenantID <= 0 || actor == uuid.Nil || report == nil || !validFXCurrency(report.BaseCurrency) {
		return nil, ErrFXInvalid
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	if len(payload) > maxFXSnapshotBytes {
		return nil, ErrTooManyRows
	}
	digest := sha256.Sum256(payload)
	return &FXReportSnapshot{
		ID: uuid.New(), TenantID: tenantID, CreatedBy: actor, BaseCurrency: report.BaseCurrency,
		TotalOrders: report.TotalOrders, IncludedOrders: report.IncludedOrders,
		ExcludedOrders: report.ExcludedOrders, FormulaVersion: report.FormulaVersion,
		ContentSHA256: hex.EncodeToString(digest[:]), ReportJSON: payload, CreatedAt: now.UTC(),
	}, nil
}

func parseFrozenFXReport(snapshot *FXReportSnapshot) (*FXReport, error) {
	if snapshot == nil || len(snapshot.ReportJSON) == 0 || len(snapshot.ReportJSON) > maxFXSnapshotBytes {
		return nil, ErrFXSnapshotCorrupt
	}
	digest := sha256.Sum256(snapshot.ReportJSON)
	if hex.EncodeToString(digest[:]) != snapshot.ContentSHA256 {
		return nil, ErrFXSnapshotCorrupt
	}
	var report FXReport
	if err := json.Unmarshal(snapshot.ReportJSON, &report); err != nil ||
		report.BaseCurrency != snapshot.BaseCurrency || report.FormulaVersion != snapshot.FormulaVersion ||
		report.TotalOrders != snapshot.TotalOrders || report.IncludedOrders != snapshot.IncludedOrders ||
		report.ExcludedOrders != snapshot.ExcludedOrders || len(report.Rows) != snapshot.TotalOrders {
		return nil, ErrFXSnapshotCorrupt
	}
	return &report, nil
}

func (s *Service) ListFXSnapshots(ctx context.Context, tenantID int64) ([]FXReportSnapshot, error) {
	if s == nil || s.DB == nil || tenantID <= 0 {
		return nil, ErrFXInvalid
	}
	var rows []FXReportSnapshot
	err := s.DB.WithContext(ctx).Select("id", "tenant_id", "created_by", "base_currency",
		"total_orders", "included_orders", "excluded_orders", "formula_version", "content_sha256",
		"created_at").Where("tenant_id = ?", tenantID).Order("created_at DESC, id DESC").
		Limit(50).Find(&rows).Error
	if rows == nil {
		rows = []FXReportSnapshot{}
	}
	return rows, err
}

func (s *Service) GetFXSnapshot(ctx context.Context, tenantID int64, id uuid.UUID) (*FXReportSnapshot, *FXReport, error) {
	if s == nil || s.DB == nil || tenantID <= 0 || id == uuid.Nil {
		return nil, nil, ErrFXInvalid
	}
	var row FXReportSnapshot
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).Take(&row).Error; err != nil {
		return nil, nil, err
	}
	report, err := parseFrozenFXReport(&row)
	if err != nil {
		return nil, nil, err
	}
	row.ReportJSON = nil
	return &row, report, nil
}

func (h *Handler) fxSnapshotAdmin(c *gin.Context, permission string) (int64, uuid.UUID, bool) {
	tenantID, principal, ok := h.authorize(c, permission)
	if !ok {
		return 0, uuid.Nil, false
	}
	if tenantID <= 0 || principal == nil || !principal.IsAdmin() || principal.IsReadonly() {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "active tenant administrator required")
		return 0, uuid.Nil, false
	}
	return tenantID, principal.UserID, true
}

func (h *Handler) CreateFXSnapshot(c *gin.Context) {
	tenantID, actor, ok := h.fxSnapshotAdmin(c, adminperm.PermOrderProfitExport)
	if !ok {
		return
	}
	if h.Audit == nil {
		response.Fail(c, http.StatusServiceUnavailable, response.CodeInternalError, "FX audit unavailable")
		return
	}
	query, err := bindListQuery(c, true)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	base := strings.ToUpper(strings.TrimSpace(c.Query("baseCurrency")))
	report, err := h.Svc.BuildFXReport(c.Request.Context(), tenantID, Scope{}, query, base)
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
	snapshot, err := freezeFXReport(tenantID, actor, report, h.Svc.now())
	if err != nil {
		response.HandleError(c, err)
		return
	}
	err = h.Svc.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(snapshot).Error; err != nil {
			return err
		}
		return h.Audit.WriteTxContext(c.Request.Context(), tx, operationlog.WriteOpts{
			TenantID: tenantID, AdminUserID: &actor, Action: "finance.fx_snapshot.create",
			Resource: "tenant_fx_report_snapshot", ResourceID: snapshot.ID.String(), Status: "success",
			Message: "sha256=" + snapshot.ContentSHA256 + " currency=" + snapshot.BaseCurrency,
		})
	})
	if err != nil {
		response.HandleError(c, err)
		return
	}
	snapshot.ReportJSON = nil
	response.OK(c, snapshot)
}

func (h *Handler) ListFXSnapshots(c *gin.Context) {
	tenantID, _, ok := h.fxSnapshotAdmin(c, adminperm.PermOrderProfitView)
	if !ok {
		return
	}
	rows, err := h.Svc.ListFXSnapshots(c.Request.Context(), tenantID)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": rows})
}

func (h *Handler) GetFXSnapshot(c *gin.Context) {
	tenantID, _, ok := h.fxSnapshotAdmin(c, adminperm.PermOrderProfitView)
	if !ok {
		return
	}
	id, err := uuid.Parse(strings.TrimSpace(c.Param("snapshotId")))
	if err != nil || id == uuid.Nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, "invalid snapshot id")
		return
	}
	row, report, err := h.Svc.GetFXSnapshot(c.Request.Context(), tenantID, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "not found")
		return
	}
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"snapshot": row, "report": report})
}
