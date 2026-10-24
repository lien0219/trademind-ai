package securitymod

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// StartRotation creates or resumes a rotation job (idempotent by active key).
func (s *Service) StartRotation(ctx context.Context, startedBy uuid.UUID, dryRun bool) (*KeyRotationJob, error) {
	if s == nil || s.DB == nil || s.Cfg == nil {
		return nil, fmt.Errorf("security: unavailable")
	}
	kr, err := s.keyRing()
	if err != nil {
		return nil, err
	}
	var existing KeyRotationJob
	err = s.DB.WithContext(ctx).
		Where("active_key_id = ? AND status IN ?", kr.ActiveID, []string{RotationRunning, RotationPaused, RotationPrepared, RotationDryRunCompleted}).
		Order("created_at DESC").
		First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	total, err := s.countPendingReencrypt(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	status := RotationPrepared
	if dryRun {
		status = RotationDryRunCompleted
	} else {
		status = RotationRunning
	}
	job := &KeyRotationJob{
		ActiveKeyID:  kr.ActiveID,
		SourceKeyIDs: strings.Join(kr.PreviousKeyIDs(), ","),
		Scope:        "global",
		DryRun:       dryRun,
		Status:       status,
		TotalRecords: total,
		StartedBy:    startedBy,
		StartedAt:    &now,
	}
	if err := s.DB.WithContext(ctx).Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

// GetRotation returns rotation job by id.
func (s *Service) GetRotation(ctx context.Context, id uuid.UUID) (*KeyRotationJob, error) {
	var job KeyRotationJob
	if err := s.DB.WithContext(ctx).First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// RotationProgress returns progress snapshot.
func (s *Service) RotationProgress(ctx context.Context, id uuid.UUID) (*KeyRotationJob, error) {
	return s.GetRotation(ctx, id)
}

// PauseRotation marks job paused.
func (s *Service) PauseRotation(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("security: unavailable")
	}
	result := s.DB.WithContext(ctx).Model(&KeyRotationJob{}).
		Where("id = ? AND status = ?", id, RotationRunning).
		Update("status", RotationPaused)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ResumeRotation resumes paused job.
func (s *Service) ResumeRotation(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("security: unavailable")
	}
	result := s.DB.WithContext(ctx).Model(&KeyRotationJob{}).
		Where("id = ? AND status = ?", id, RotationPaused).
		Update("status", RotationRunning)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountSecretReferencesByKeyID aggregates old-key and legacy references across all secret targets.
func (s *Service) CountSecretReferencesByKeyID(ctx context.Context, keyIDs []string) ([]SecretReferenceCount, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("security: unavailable")
	}
	return s.aggregateSecretReferences(ctx, keyIDs)
}

func containsKey(ids []string, kid string) bool {
	for _, id := range ids {
		if strings.TrimSpace(id) == kid {
			return true
		}
	}
	return false
}

// VerifyRotation checks no old key references remain.
func (s *Service) VerifyRotation(ctx context.Context, id uuid.UUID) (bool, []SecretReferenceCount, error) {
	job, err := s.GetRotation(ctx, id)
	if err != nil {
		return false, nil, err
	}
	kr, err := s.keyRing()
	if err != nil {
		return false, nil, err
	}
	counts, err := s.CountSecretReferencesByKeyID(ctx, kr.PreviousKeyIDs())
	if err != nil {
		return false, nil, err
	}
	remaining := int64(0)
	unknown := int64(0)
	for _, c := range counts {
		remaining += c.ReferenceCount
		unknown += c.UnknownFormat
	}
	ok := remaining == 0 && unknown == 0 && job.FailedRecords == 0
	status := RotationVerificationPending
	if ok {
		status = RotationVerified
	}
	if err := s.DB.WithContext(ctx).Model(job).Updates(map[string]any{
		"verification_status": status,
		"status":              ternaryStatus(ok, RotationVerified, RotationCompletedWithWarn),
	}).Error; err != nil {
		return false, nil, fmt.Errorf("persist key rotation verification: %w", err)
	}
	return ok, counts, nil
}

func ternaryStatus(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

// ProcessReencryptBatch re-encrypts one batch across all registered secret targets.
func (s *Service) ProcessReencryptBatch(ctx context.Context, rotationID uuid.UUID, batchSize int) (int, error) {
	if s == nil || s.DB == nil || s.Cfg == nil {
		return 0, fmt.Errorf("security: unavailable")
	}
	job, err := s.GetRotation(ctx, rotationID)
	if err != nil {
		return 0, err
	}
	if job.DryRun || job.Status != RotationRunning {
		return 0, nil
	}
	targets, kr, err := s.reencryptTargets()
	if err != nil {
		return 0, err
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	idx := targetIndexByName(targets, job.TableScope)
	if idx >= len(targets) {
		idx = 0
		job.TableScope = targets[0].Name()
	}
	totalProcessed := 0
	for idx < len(targets) {
		job.TableScope = targets[idx].Name()
		n, done, err := s.processTargetReencryptBatch(ctx, job, targets[idx], kr, batchSize-totalProcessed)
		if err != nil {
			return totalProcessed, err
		}
		totalProcessed += n
		if !done {
			break
		}
		job.LastCursor = ""
		idx++
		if totalProcessed >= batchSize {
			break
		}
	}
	if idx >= len(targets) && job.LastCursor == "" {
		now := time.Now().UTC()
		job.Status = RotationCompleted
		job.FinishedAt = &now
	}
	job.ProcessedRecords += int64(totalProcessed)
	_ = s.DB.WithContext(ctx).Save(job).Error
	return totalProcessed, nil
}

func (s *Service) recordFailure(ctx context.Context, rotationID uuid.UUID, table, recordID string, tenantID int64, keyID, code, summary string) error {
	f := &KeyRotationItemFailure{
		RotationID:  rotationID,
		TargetTable: table,
		RecordID:    recordID,
		TenantID:    tenantID,
		KeyID:       keyID,
		ReasonCode:  code,
		SafeSummary: summary,
	}
	return s.DB.WithContext(ctx).Create(f).Error
}
