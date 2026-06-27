package taskcenter

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/taskcenter/failureclassifier"
)

func classificationInput(d UnifiedTaskDTO) failureclassifier.Input {
	return failureclassifier.Input{
		TaskType:         d.TaskType,
		Platform:         d.Platform,
		NormalizedStatus: d.NormalizedStatus,
		ErrorMessage:     d.ErrorMessage,
		ErrorCode:        d.ErrorCode,
		Title:            d.Title,
		RawSummary:       d.RawSummary,
	}
}

func applyClassification(d *UnifiedTaskDTO) failureclassifier.Result {
	if d != nil && strings.EqualFold(strings.TrimSpace(d.TaskType), TaskTypeAIText) {
		return applyAITextClassification(d)
	}
	in := classificationInput(*d)
	r := failureclassifier.Classify(in)
	d.FailureCategory = r.Category
	d.Severity = r.Severity
	d.ClassificationReason = r.Reason
	d.MatchedRule = r.MatchedRule
	d.SuggestedAction = r.SuggestedAction
	return r
}

func alertViewStatus(st string) string {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case TaskAlertStatusOpen:
		return AlertStatusGenerated
	case TaskAlertStatusHandled:
		return AlertStatusHandled
	case TaskAlertStatusIgnored:
		return AlertStatusIgnored
	default:
		return AlertStatusNone
	}
}

type alertTriple struct {
	TaskType        string
	SourceID        string
	FailureCategory string
}

func (s *Service) batchAlertStatuses(ctx context.Context, keys []alertTriple) (map[alertTriple]*TaskAlert, error) {
	out := make(map[alertTriple]*TaskAlert)
	if s == nil || s.DB == nil || len(keys) == 0 {
		return out, nil
	}
	// Dedup keys
	uniq := make(map[alertTriple]struct{})
	list := make([]alertTriple, 0, len(keys))
	for _, k := range keys {
		if _, ok := uniq[k]; ok {
			continue
		}
		uniq[k] = struct{}{}
		list = append(list, k)
	}
	var conds []string
	var args []any
	for _, k := range list {
		conds = append(conds, "(task_type = ? AND source_id = ? AND failure_category = ?)")
		args = append(args, k.TaskType, k.SourceID, k.FailureCategory)
	}
	var rows []TaskAlert
	q := s.DB.WithContext(ctx).Model(&TaskAlert{}).Where(strings.Join(conds, " OR "), args...)
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		k := alertTriple{TaskType: rows[i].TaskType, SourceID: rows[i].SourceID, FailureCategory: rows[i].FailureCategory}
		cp := rows[i]
		out[k] = &cp
	}
	return out, nil
}

// attachAlertStatuses loads related task_alerts for already-classified rows.
func (s *Service) attachAlertStatuses(ctx context.Context, rows []UnifiedTaskDTO) error {
	if len(rows) == 0 {
		return nil
	}
	keys := make([]alertTriple, 0, len(rows))
	for i := range rows {
		keys = append(keys, alertTriple{
			TaskType:        rows[i].TaskType,
			SourceID:        rows[i].SourceID,
			FailureCategory: rows[i].FailureCategory,
		})
	}
	m, err := s.batchAlertStatuses(ctx, keys)
	if err != nil {
		return err
	}
	for i := range rows {
		k := alertTriple{TaskType: rows[i].TaskType, SourceID: rows[i].SourceID, FailureCategory: rows[i].FailureCategory}
		if al, ok := m[k]; ok && al != nil {
			rows[i].AlertStatus = alertViewStatus(al.Status)
			rows[i].RelatedAlertID = al.ID.String()
		} else {
			rows[i].AlertStatus = AlertStatusNone
			rows[i].RelatedAlertID = ""
		}
	}
	return nil
}

// ClassifyOne classifies one row and attaches alert linkage (detail view).
func (s *Service) ClassifyOne(ctx context.Context, d *UnifiedTaskDTO) error {
	if d == nil {
		return nil
	}
	applyClassification(d)
	m, err := s.batchAlertStatuses(ctx, []alertTriple{{
		TaskType: d.TaskType, SourceID: d.SourceID, FailureCategory: d.FailureCategory,
	}})
	if err != nil {
		return err
	}
	k := alertTriple{d.TaskType, d.SourceID, d.FailureCategory}
	if al, ok := m[k]; ok && al != nil {
		d.AlertStatus = alertViewStatus(al.Status)
		d.RelatedAlertID = al.ID.String()
	} else {
		d.AlertStatus = AlertStatusNone
		d.RelatedAlertID = ""
	}
	return nil
}

// --- uuid for new alerts

func newTaskAlertID() uuid.UUID {
	return uuid.New()
}
