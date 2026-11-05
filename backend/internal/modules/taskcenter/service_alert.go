package taskcenter

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

// ListAlertsParams binds GET /task-center/alerts.
type ListAlertsParams struct {
	TenantID          int64
	AllowedShopIDs    []uuid.UUID
	RestrictShopScope bool
	Status            string
	Severity          string
	FailureCategory   string
	TaskType          string
	Platform          string
	Start             *time.Time
	End               *time.Time
	Page              int
	PageSize          int
}

func taskCenterPlain(ctx context.Context, s *settings.Service) (map[string]string, error) {
	if s == nil {
		return map[string]string{}, nil
	}
	m, err := s.PlainByGroup(ctx, 0, "taskcenter")
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func parseBoolTaskCenter(v string, def bool) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return def
	}
}

func severityOrder(sev string) int {
	switch strings.TrimSpace(strings.ToLower(sev)) {
	case failureclassifier.SeverityCritical:
		return 4
	case failureclassifier.SeverityHigh:
		return 3
	case failureclassifier.SeverityMedium:
		return 2
	case failureclassifier.SeverityLow:
		return 1
	default:
		return 0
	}
}

func shouldEmitAutoAlert(m map[string]string, dto UnifiedTaskDTO, class failureclassifier.Result, now time.Time) bool {
	if !parseBoolTaskCenter(m["enable_task_alerts"], false) {
		return false
	}
	if parseBoolTaskCenter(m["alert_on_repeated_failures"], false) {
		thStr := strings.TrimSpace(m["repeated_failure_threshold"])
		n, err := strconv.Atoi(thStr)
		winStr := strings.TrimSpace(m["repeated_failure_window_minutes"])
		win, winErr := strconv.Atoi(winStr)
		if err == nil && n > 0 && winErr == nil && win > 0 {
			winDur := time.Duration(win) * time.Minute
			if dto.RetryCount >= n && dto.UpdatedAt.After(now.Add(-winDur)) {
				return true
			}
		}
	}
	sevRank := severityOrder(class.Severity)
	if sevRank <= severityOrder(failureclassifier.SeverityLow) {
		return false
	}
	if minStr := strings.TrimSpace(m["alert_min_severity"]); minStr != "" {
		minRank := severityOrder(minStr)
		if minRank > 0 && sevRank >= minRank {
			return true
		}
	}
	switch class.Category {
	case failureclassifier.CategoryPlatformPermission:
		return parseBoolTaskCenter(m["alert_on_platform_permission"], false)
	case failureclassifier.CategoryPlatformConfigIncomplete:
		return parseBoolTaskCenter(m["alert_on_platform_config"], false)
	case failureclassifier.CategoryInventoryMappingMissing:
		return parseBoolTaskCenter(m["alert_on_inventory_mapping_missing"], false)
	case failureclassifier.CategoryWorkerLeaseExpired:
		return parseBoolTaskCenter(m["alert_on_worker_lease_expired"], false)
	}
	return false
}

func alertTitle(tt, platform string) string {
	p := strings.TrimSpace(platform)
	label := truncateRunes(strings.ReplaceAll(strings.ReplaceAll(tt, "_", " "), "  ", " "), 80)
	if p != "" {
		return truncateRunes("任务告警 · "+p+" · "+label, 255)
	}
	return truncateRunes("任务告警 · "+label, 255)
}

func toAlertDTO(r TaskAlert, notificationStatus string) TaskAlertDTO {
	dto := TaskAlertDTO{
		ID:                 r.ID.String(),
		TaskType:           r.TaskType,
		SourceID:           r.SourceID,
		SourceTable:        r.SourceTable,
		Platform:           r.Platform,
		FailureCategory:    r.FailureCategory,
		Severity:           r.Severity,
		Title:              r.Title,
		Message:            truncateRunes(r.Message, 900),
		SuggestedAction:    truncateRunes(r.SuggestedAction, 600),
		Status:             r.Status,
		AlertCount:         r.AlertCount,
		FirstSeenAt:        r.FirstSeenAt,
		LastSeenAt:         r.LastSeenAt,
		HandledAt:          r.HandledAt,
		NotificationStatus: notificationStatus,
	}
	return dto
}

// ListAlerts page.
func (s *Service) ListAlerts(ctx context.Context, p ListAlertsParams) (ListAlertsResult, error) {
	var out ListAlertsResult
	if s == nil || s.DB == nil {
		return out, fmt.Errorf("taskcenter: no db")
	}
	page, pageSize := clampPage(p.Page, p.PageSize)
	q := s.DB.WithContext(ctx).Model(&TaskAlert{})
	q = q.Where("tenant_id = ?", p.TenantID)
	q = applyTaskAlertStoreScope(q, p.TenantID, p.RestrictShopScope, p.AllowedShopIDs)
	if st := strings.TrimSpace(p.Status); st != "" {
		q = q.Where("status = ?", st)
	}
	if se := strings.TrimSpace(p.Severity); se != "" {
		q = q.Where("severity = ?", se)
	}
	if fc := strings.TrimSpace(p.FailureCategory); fc != "" {
		q = q.Where("failure_category = ?", fc)
	}
	if tt := strings.TrimSpace(p.TaskType); tt != "" {
		q = q.Where("task_type = ?", tt)
	}
	if pl := strings.TrimSpace(p.Platform); pl != "" {
		q = q.Where("LOWER(platform) = LOWER(?)", pl)
	}
	if p.Start != nil {
		q = q.Where("last_seen_at >= ?", *p.Start)
	}
	if p.End != nil {
		q = q.Where("last_seen_at <= ?", *p.End)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return out, err
	}
	var rows []TaskAlert
	off := (page - 1) * pageSize
	if err := q.Order("last_seen_at DESC").Offset(off).Limit(pageSize).Find(&rows).Error; err != nil {
		return out, err
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}
	badges := s.notificationBadgeMap(ctx, ids)
	list := make([]TaskAlertDTO, 0, len(rows))
	for i := range rows {
		st := badges[rows[i].ID]
		if st == "" {
			st = "none"
		}
		list = append(list, toAlertDTO(rows[i], st))
	}
	out.List = list
	out.Total = total
	return out, nil
}

// UpsertAlertForFailure creates or bumps an OPEN alert row. force=true skips policy gate and can reopen handled/ignored.
func (s *Service) UpsertAlertForFailure(ctx context.Context, dto UnifiedTaskDTO, class failureclassifier.Result, now time.Time, force bool, admin *uuid.UUID) (generated bool, updatedOpen bool, err error) {
	if s == nil || s.DB == nil {
		return false, false, fmt.Errorf("taskcenter: no db")
	}
	m, err := taskCenterPlain(ctx, s.Settings)
	if err != nil {
		return false, false, errors.Join(errTaskDependencyUnavailable, err)
	}
	if !force && !shouldEmitAutoAlert(m, dto, class, now) {
		return false, false, nil
	}
	platform := strings.TrimSpace(dto.Platform)
	msg := truncateRunes(strings.TrimSpace(class.Reason)+" · "+strings.TrimSpace(dto.ErrorMessage), 1200)

	var cur TaskAlert
	errFind := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND task_type = ? AND source_id = ? AND failure_category = ?", dto.TenantID, dto.TaskType, dto.SourceID, class.Category).
		First(&cur).Error

	if errors.Is(errFind, gorm.ErrRecordNotFound) {
		a := TaskAlert{
			ID:              newTaskAlertID(),
			TenantID:        dto.TenantID,
			TaskType:        dto.TaskType,
			SourceID:        dto.SourceID,
			SourceTable:     dto.SourceTable,
			FailureCategory: class.Category,
			Severity:        class.Severity,
			Platform:        platform,
			Title:           alertTitle(dto.TaskType, platform),
			Message:         msg,
			SuggestedAction: class.SuggestedAction,
			Status:          TaskAlertStatusOpen,
			AlertCount:      1,
			FirstSeenAt:     now,
			LastSeenAt:      now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := s.DB.WithContext(ctx).Create(&a).Error; err != nil {
			return false, false, err
		}
		return true, false, nil
	}
	if errFind != nil {
		return false, false, errFind
	}

	// Existing row.
	if force {
		up := map[string]any{
			"last_seen_at":     now,
			"updated_at":       now,
			"severity":         class.Severity,
			"message":          msg,
			"suggested_action": class.SuggestedAction,
			"title":            alertTitle(dto.TaskType, platform),
			"platform":         platform,
			"source_table":     dto.SourceTable,
			"status":           TaskAlertStatusOpen,
			"handled_at":       nil,
			"handled_by":       nil,
			"alert_count":      gorm.Expr("alert_count + 1"),
		}
		if err := s.DB.WithContext(ctx).Model(&TaskAlert{}).Where("id = ? AND tenant_id = ?", cur.ID, dto.TenantID).Updates(up).Error; err != nil {
			return false, false, err
		}
		return false, true, nil
	}

	if strings.EqualFold(cur.Status, TaskAlertStatusIgnored) || strings.EqualFold(cur.Status, TaskAlertStatusHandled) {
		return false, false, nil
	}
	if err := s.DB.WithContext(ctx).Model(&TaskAlert{}).Where("id = ? AND tenant_id = ?", cur.ID, dto.TenantID).
		Updates(map[string]any{
			"last_seen_at":     now,
			"updated_at":       now,
			"alert_count":      gorm.Expr("alert_count + 1"),
			"severity":         class.Severity,
			"message":          msg,
			"suggested_action": class.SuggestedAction,
			"title":            alertTitle(dto.TaskType, platform),
			"platform":         platform,
			"source_table":     dto.SourceTable,
		}).Error; err != nil {
		return false, false, err
	}
	return false, true, nil
}

// ScanAndGenerateTaskAlerts evaluates recent problematic tasks and upserts OPEN alerts according to settings.
func (s *Service) ScanAndGenerateTaskAlerts(ctx context.Context) (ScanAlertsSummary, error) {
	return s.scanAndGenerateTaskAlerts(ctx, ListFailureParams{}, nil)
}

func (s *Service) ScanAndGenerateTaskAlertsForScope(ctx context.Context, scope ListFailureParams) (ScanAlertsSummary, error) {
	return s.ScanAndGenerateTaskAlertsForScopeRequest(ctx, scope, nil)
}

func (s *Service) ScanAndGenerateTaskAlertsForScopeRequest(ctx context.Context, scope ListFailureParams, ginC *gin.Context) (ScanAlertsSummary, error) {
	if !scope.TenantScopeSet {
		return ScanAlertsSummary{}, fmt.Errorf("taskcenter: tenant scope is required")
	}
	return s.scanAndGenerateTaskAlerts(ctx, scope, ginC)
}

func (s *Service) scanAndGenerateTaskAlerts(ctx context.Context, scope ListFailureParams, ginC *gin.Context) (sum ScanAlertsSummary, err error) {
	if s == nil || s.DB == nil {
		return sum, fmt.Errorf("taskcenter: no db")
	}
	requestAudit := operationlog.WriteOpts{
		TenantID:   scope.TenantID,
		Action:     "task_center.alert.scan.requested",
		Resource:   "task_alert",
		ResourceID: "scan",
		Status:     "requested",
	}
	if ginC != nil {
		if err := s.writeRequiredAudit(ginC, requestAudit); err != nil {
			return sum, err
		}
	} else if err := s.writeRequiredAuditBackground(ctx, requestAudit); err != nil {
		return sum, err
	}
	defer func() {
		if err == nil {
			return
		}
		failedAudit := requestAudit
		failedAudit.Action = "task_center.alert.scan.completed"
		failedAudit.Status = "failed"
		failedAudit.Message = truncateRunes(fmt.Sprintf("scanned=%d gen=%d up=%d skip=%d error=%s", sum.ScannedCount, sum.GeneratedCount, sum.UpdatedCount, sum.IgnoredCount, err.Error()), 1000)
		var auditErr error
		if ginC != nil {
			auditErr = s.writeRequiredAudit(ginC, failedAudit)
		} else {
			auditErr = s.writeRequiredAuditBackground(ctx, failedAudit)
		}
		if auditErr != nil {
			err = errors.Join(err, auditErr)
		}
	}()
	now := time.Now().UTC()
	p := scope
	p.Page = 1
	p.PageSize = 100
	p.IncludeMarked = true
	p.IncludeResolved = false
	types := []string{
		TaskTypeCollect, TaskTypeImage, TaskTypeOrderSync, TaskTypeCustomerMessageSync,
		TaskTypeProductPublish, TaskTypeInventorySync, TaskTypeAIText,
	}
	perTypeLimit := 400
	gen := 0
	up := 0
	ig := 0
	scanned := 0
	var candidates []alertNotifyCandidate

	for _, tt := range types {
		p.TaskType = tt
		part, err := s.listOneType(ctx, tt, p, now, perTypeLimit, TaskSourceCursor{SourceID: tt})
		if err != nil {
			return sum, err
		}
		for i := range part {
			class := applyClassification(&part[i])
			d := part[i]
			if !passesUnifiedFilters(d, p) {
				continue
			}
			scanned++
			generated, bumped, err := s.UpsertAlertForFailure(ctx, d, class, now, false, nil)
			if err != nil {
				return sum, err
			}
			switch {
			case generated:
				gen++
			case bumped:
				up++
			default:
				ig++
			}
			if generated || bumped {
				var a TaskAlert
				if err := s.DB.WithContext(ctx).
					Where("tenant_id = ? AND task_type = ? AND source_id = ? AND failure_category = ?", d.TenantID, d.TaskType, d.SourceID, class.Category).
					First(&a).Error; err == nil {
					candidates = append(candidates, alertNotifyCandidate{Alert: a, IsNew: generated})
				}
			}
		}
	}
	sum.ScannedCount = scanned
	sum.GeneratedCount = gen
	sum.UpdatedCount = up
	sum.IgnoredCount = ig

	if err := s.NotifyGeneratedAlerts(ctx, candidates, false, ginC, nil); err != nil {
		return sum, err
	}
	resultAudit := requestAudit
	resultAudit.Action = "task_center.alert.scan.completed"
	resultAudit.Status = "success"
	resultAudit.Message = truncateRunes(fmt.Sprintf("scanned=%d gen=%d up=%d skip=%d", sum.ScannedCount, sum.GeneratedCount, sum.UpdatedCount, sum.IgnoredCount), 480)
	if ginC != nil {
		if err := s.writeRequiredAudit(ginC, resultAudit); err != nil {
			return sum, err
		}
	} else if err := s.writeRequiredAuditBackground(ctx, resultAudit); err != nil {
		return sum, err
	}
	return sum, nil
}

// HandleTaskAlert marks one alert handled (does not change source failing task rows).
func (s *Service) HandleTaskAlert(c *gin.Context, alertID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	ctx := c.Request.Context()
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var a TaskAlert
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", alertID, tenantID).First(&a).Error; err != nil {
		return err
	}
	if err := s.ensureTaskAlertStoreScope(c, a, true); err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.finishAlertStatusChange(ctx, c, a, TaskAlertStatusHandled, now, adminFromGin(c), "task_center.alert.handle")
}

// IgnoreTaskAlert marks ignored.
func (s *Service) IgnoreTaskAlert(c *gin.Context, alertID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	ctx := c.Request.Context()
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var a TaskAlert
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", alertID, tenantID).First(&a).Error; err != nil {
		return err
	}
	if err := s.ensureTaskAlertStoreScope(c, a, true); err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.finishAlertStatusChange(ctx, c, a, TaskAlertStatusIgnored, now, adminFromGin(c), "task_center.alert.ignore")
}

// UnmarkTaskAlert restores open state.
func (s *Service) UnmarkTaskAlert(c *gin.Context, alertID uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("taskcenter: no db")
	}
	ctx := c.Request.Context()
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	var a TaskAlert
	if err := s.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", alertID, tenantID).First(&a).Error; err != nil {
		return err
	}
	if err := s.ensureTaskAlertStoreScope(c, a, true); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&TaskAlert{}).Where("id = ? AND tenant_id = ?", alertID, tenantID).
			Updates(map[string]any{
				"status":     TaskAlertStatusOpen,
				"handled_at": nil,
				"handled_by": nil,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		return s.writeRequiredAuditTx(c, tx, operationlog.WriteOpts{
			TenantID:   tenantID,
			Action:     "task_center.alert.unmark",
			Resource:   "task_alert",
			ResourceID: alertID.String(),
			Status:     "success",
			Message:    summarizeAlertAudit(a.TaskType, a.SourceID, a.FailureCategory, a.Severity),
		})
	}); err != nil {
		return err
	}
	return nil
}

func summarizeAlertAudit(taskType, sourceID, category, severity string) string {
	return truncateRunes(fmt.Sprintf("taskType=%s sourceId=%s category=%s severity=%s",
		strings.TrimSpace(taskType), strings.TrimSpace(sourceID), strings.TrimSpace(category), strings.TrimSpace(severity)), 480)
}

func (s *Service) finishAlertStatusChange(ctx context.Context, ginC *gin.Context, a TaskAlert, st string, now time.Time, admin *uuid.UUID, action string) error {
	up := map[string]any{"status": st, "handled_at": now, "handled_by": admin, "updated_at": now}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&TaskAlert{}).Where("id = ? AND tenant_id = ?", a.ID, a.TenantID).Updates(up).Error; err != nil {
			return err
		}
		return s.writeRequiredAuditTx(ginC, tx, operationlog.WriteOpts{
			TenantID:   a.TenantID,
			Action:     action,
			Resource:   "task_alert",
			ResourceID: a.ID.String(),
			Status:     "success",
			Message:    summarizeAlertAudit(a.TaskType, a.SourceID, a.FailureCategory, a.Severity),
		})
	})
}

// GenerateAlertForFailure force-upserts alert for task row.
func (s *Service) GenerateAlertForFailure(c *gin.Context, taskTypeRaw string, id uuid.UUID) (al *TaskAlert, err error) {
	if s == nil {
		return nil, fmt.Errorf("taskcenter unavailable")
	}
	taskType, err := parseTaskType(taskTypeRaw)
	if err != nil {
		return nil, err
	}
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	base, err := s.unifiedOne(c.Request.Context(), tenantID, taskType, id, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if err := s.ensureTaskStoreScope(c, base, true); err != nil {
		return nil, err
	}
	requestAudit := operationlog.WriteOpts{
		TenantID: tenantID, Action: "task_center.alert.generate.requested", Resource: "task_alert",
		ResourceID: taskType + "/" + id.String(), Status: "requested",
		Message: truncateRunes(fmt.Sprintf("taskType=%s sourceId=%s", taskType, id.String()), 240),
	}
	if err := s.writeRequiredAudit(c, requestAudit); err != nil {
		return nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		failedAudit := requestAudit
		failedAudit.Action = "task_center.alert.generated"
		failedAudit.Status = "failed"
		failedAudit.Message = truncateRunes(fmt.Sprintf("taskType=%s sourceId=%s error=%s", taskType, id.String(), err.Error()), 1000)
		if auditErr := s.writeRequiredAudit(c, failedAudit); auditErr != nil {
			err = errors.Join(err, auditErr)
		}
	}()
	class := applyClassification(&base)
	now := time.Now().UTC()
	gen, bumped, err := s.UpsertAlertForFailure(c.Request.Context(), base, class, now, true, adminFromGin(c))
	if err != nil {
		return nil, err
	}
	if !gen && !bumped {
		return nil, fmt.Errorf("could not create or bump alert")
	}
	var alertRow TaskAlert
	err = s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND task_type = ? AND source_id = ? AND failure_category = ?", base.TenantID, base.TaskType, base.SourceID, class.Category).
		First(&alertRow).Error
	if err != nil {
		return nil, err
	}
	if err := s.writeRequiredAudit(c, operationlog.WriteOpts{
		TenantID: tenantID, Action: "task_center.alert.generated", Resource: "task_alert",
		ResourceID: alertRow.ID.String(), Status: "success",
		Message: truncateRunes(fmt.Sprintf("taskType=%s sourceId=%s category=%s severity=%s", alertRow.TaskType, alertRow.SourceID, alertRow.FailureCategory, alertRow.Severity), 480),
	}); err != nil {
		return nil, err
	}
	if err := s.NotifyGeneratedAlerts(c.Request.Context(), []alertNotifyCandidate{{Alert: alertRow, IsNew: gen}}, false, c, nil); err != nil {
		return nil, err
	}
	return &alertRow, nil
}
