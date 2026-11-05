package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/filescanner"
	"github.com/trademind-ai/trademind/backend/internal/pkg/repository"
	"github.com/trademind-ai/trademind/backend/internal/pkg/taskqueue"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	"github.com/trademind-ai/trademind/backend/internal/providers/storage"
	"gorm.io/gorm"
)

const fileScanQueueName = "file:security:scan"

const scanClaimStaleAfter = 2 * time.Minute
const scanClaimRetryDelay = time.Second

var (
	errScanAlreadyClaimed = errors.New("files: scan already claimed")
	errScanClaimLost      = errors.New("files: scan claim lost")
)

// ScanQueueMessage is the Redis payload for file security scan tasks.
type ScanQueueMessage struct {
	TenantID int64  `json:"tenantId"`
	AssetID  string `json:"assetId"`
}

// EnqueueSecurityScan pushes a file scan task after upload.
func (s *Service) EnqueueSecurityScan(ctx context.Context, tenantID int64, assetID uuid.UUID) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return nil
	}
	if err := tasktenant.RequireTaskTenant(tenantID); err != nil {
		return err
	}
	msg := ScanQueueMessage{TenantID: tenantID, AssetID: assetID.String()}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.ObserveFileScan("basic", "enqueue", "queued", "unknown", 0)
	return s.Redis.LPush(ctx, fileScanQueueName, string(b)).Err()
}

// StartScanWorker consumes file security scan queue until ctx cancelled.
func StartScanWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, cfg *config.Config, reg *worker.Registry) {
	if wg == nil || svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	scanner := buildFileScanner(cfg)
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumer, err := taskqueue.RegisterConsumer(ctx, svc.Redis.Client, fileScanQueueName)
		if err != nil {
			if log != nil {
				log.Warn("file_scan_consumer_register_failed", "error", err)
			}
			return
		}
		defer func() {
			if err := consumer.Stop(context.Background()); err != nil && log != nil {
				log.Warn("file_scan_consumer_stop_failed", "error", err)
			}
		}()
		if _, err := taskqueue.RecoverIfIdle(ctx, svc.Redis.Client, fileScanQueueName, consumer.ID()); err != nil && log != nil {
			log.Warn("file_scan_queue_recovery_failed", "error", err)
		}
		var ri *worker.RunningInstance
		if reg != nil {
			ri = reg.Register(ctx, worker.TypeFileSecurityScan, "file-security-scan", map[string]any{"queue": fileScanQueueName})
			if ri != nil {
				defer ri.Stop(ctx)
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			payload, err := taskqueue.Reserve(ctx, svc.Redis.Client, fileScanQueueName, consumer.ID(), 5*time.Second)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			ack := func() {
				if err := taskqueue.Ack(ctx, svc.Redis.Client, fileScanQueueName, consumer.ID(), payload); err != nil && log != nil {
					log.Warn("file_scan_worker_ack_failed", "error", err)
				}
			}
			requeue := func(cause error) {
				if err := taskqueue.Requeue(ctx, svc.Redis.Client, fileScanQueueName, consumer.ID(), payload); err != nil && log != nil {
					log.Error("file_scan_worker_requeue_failed", "cause", cause, "error", err)
				} else if log != nil {
					log.Warn("file_scan_worker_requeued", "cause", cause)
				}
			}

			var msg ScanQueueMessage
			if err := json.Unmarshal([]byte(payload), &msg); err != nil {
				if log != nil {
					log.Warn("file_scan_worker_bad_message", "error", err)
				}
				ack()
				continue
			}
			if err := svc.processScanPayload(ctx, log, scanner, payload); err != nil {
				if log != nil {
					log.Warn("file_scan_worker_error", "error", err)
				}
				if errors.Is(err, errScanAlreadyClaimed) {
					// A fresh durable claim may belong to a live worker or to a
					// worker that crashed before its claim became stale. Keep the
					// queue reservation retryable; acknowledging here would lose
					// the only message and strand the row in scanning forever.
					timer := time.NewTimer(scanClaimRetryDelay)
					select {
					case <-ctx.Done():
						if !timer.Stop() {
							<-timer.C
						}
						return
					case <-timer.C:
					}
					requeue(err)
					continue
				}
				if tasktenant.IsPermanentWorkerScopeError(err) {
					assetID, parseErr := uuid.Parse(strings.TrimSpace(msg.AssetID))
					if parseErr != nil {
						ack()
						continue
					}
					if err := markScanFailed(ctx, svc.DB, assetID, msg.TenantID); err != nil {
						requeue(err)
						continue
					}
					ack()
					continue
				}
				var row FileRecord
				assetID, parseErr := uuid.Parse(strings.TrimSpace(msg.AssetID))
				if parseErr != nil {
					ack()
					continue
				}
				if svc.DB == nil {
					requeue(errors.New("database unavailable"))
					continue
				}
				if qerr := svc.DB.WithContext(ctx).Where("id = ? AND tenant_id = ?", assetID, msg.TenantID).First(&row).Error; qerr != nil {
					if errors.Is(qerr, gorm.ErrRecordNotFound) {
						ack()
					} else {
						requeue(qerr)
					}
					continue
				}
				switch row.SecurityStatus {
				case SecurityPendingScan, SecurityScanning:
					requeue(err)
				default:
					ack()
				}
				continue
			}
			ack()
		}
	}()
}

func buildFileScanner(cfg *config.Config) filescanner.FileScanner {
	scanners := []filescanner.FileScanner{
		&filescanner.BasicFilePolicyScanner{},
		&filescanner.ImageDecodeScanner{},
	}
	_ = cfg
	return &filescanner.CompositeFileScanner{Scanners: scanners}
}

func (s *Service) processScanPayload(ctx context.Context, log *slog.Logger, scanner filescanner.FileScanner, payload string) error {
	start := time.Now()
	var msg ScanQueueMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		return err
	}
	if err := tasktenant.RequireTaskTenant(msg.TenantID); err != nil {
		return err
	}
	assetID, err := uuid.Parse(strings.TrimSpace(msg.AssetID))
	if err != nil {
		return err
	}
	wctx, _, err := tasktenant.BeginWorker(ctx, s.DB, msg.TenantID, uuid.Nil, "file_security_scan")
	if err != nil {
		return err
	}
	var row FileRecord
	if err := repository.FindByID(wctx, s.DB, &row, msg.TenantID, assetID); err != nil {
		return err
	}
	if row.SecurityStatus != SecurityPendingScan && row.SecurityStatus != SecurityScanFailed && row.SecurityStatus != SecurityScanning {
		return nil
	}
	claimID := uuid.NewString()
	claimed, err := claimScan(wctx, s.DB, assetID, msg.TenantID, claimID, time.Now().UTC())
	if err != nil {
		return err
	}
	if !claimed {
		return nil
	}
	row.SecurityStatus = SecurityScanning
	row.ScanStatus = SecurityScanning
	row.ScanClaimID = claimID
	now := time.Now().UTC()
	row.ScanClaimedAt = &now
	mimeGroup := mimeGroup(row.ContentType)
	if !row.CreatedAt.IsZero() {
		s.ObserveFileScan("basic", "queue_age", "claimed", mimeGroup, time.Since(row.CreatedAt))
	}
	s.ObserveFileScan("basic", "claim", "claimed", mimeGroup, 0)
	result, scanErr := s.runScanner(wctx, scanner, &row)
	if scanErr != nil {
		if statusErr := markScanFailedOwned(wctx, s.DB, assetID, msg.TenantID, claimID); statusErr != nil {
			return fmt.Errorf("files: persist scan failure: %w", statusErr)
		}
		s.ObserveFileScan("basic", "result", "failure", mimeGroup, time.Since(start))
		return scanErr
	}
	final := mapResultStatus(result.Status)
	if err := persistScanStatusOwned(wctx, s.DB, assetID, msg.TenantID, claimID, final); err != nil {
		return err
	}
	s.ObserveFileScan("basic", "result", final, mimeGroup, time.Since(start))
	return nil
}

func persistScanStatus(ctx context.Context, db *gorm.DB, assetID uuid.UUID, tenantID int64, status string) error {
	if db == nil {
		return fmt.Errorf("files: database unavailable while persisting scan status")
	}
	result := db.WithContext(ctx).Model(&FileRecord{}).
		Where("id = ? AND tenant_id = ?", assetID, tenantID).
		Updates(map[string]any{"security_status": status, "scan_status": status, "scan_claim_id": "", "scan_claimed_at": nil})
	if result.Error != nil {
		return fmt.Errorf("files: persist scan status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("files: persist scan status: expected one row, affected %d", result.RowsAffected)
	}
	return nil
}

func markScanFailed(ctx context.Context, db *gorm.DB, assetID uuid.UUID, tenantID int64) error {
	return persistScanStatus(ctx, db, assetID, tenantID, SecurityScanFailed)
}

func persistScanStatusOwned(ctx context.Context, db *gorm.DB, assetID uuid.UUID, tenantID int64, claimID, status string) error {
	if db == nil {
		return fmt.Errorf("files: database unavailable while persisting scan status")
	}
	result := db.WithContext(ctx).Model(&FileRecord{}).
		Where("id = ? AND tenant_id = ? AND security_status = ? AND scan_claim_id = ?", assetID, tenantID, SecurityScanning, claimID).
		Updates(map[string]any{"security_status": status, "scan_status": status, "scan_claim_id": "", "scan_claimed_at": nil})
	if result.Error != nil {
		return fmt.Errorf("files: persist scan status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return errScanClaimLost
	}
	return nil
}

func markScanFailedOwned(ctx context.Context, db *gorm.DB, assetID uuid.UUID, tenantID int64, claimID string) error {
	return persistScanStatusOwned(ctx, db, assetID, tenantID, claimID, SecurityScanFailed)
}

// claimScan atomically claims pending work and, after a worker crash, claims
// only a scanning row whose previous reservation is stale. A fresh scanning
// row is left untouched so duplicate queue messages cannot reset an active
// scan or overwrite its result.
func claimScan(ctx context.Context, db *gorm.DB, assetID uuid.UUID, tenantID int64, claimID string, now time.Time) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("files: database unavailable while claiming scan")
	}
	cutoff := now.Add(-scanClaimStaleAfter)
	result := db.WithContext(ctx).Model(&FileRecord{}).
		Where("id = ? AND tenant_id = ? AND ((security_status IN ? AND scan_status IN ?) OR (security_status = ? AND (scan_claimed_at IS NULL OR scan_claimed_at < ?)))",
			assetID,
			tenantID,
			[]string{SecurityPendingScan, SecurityScanFailed},
			[]string{SecurityPendingScan, SecurityScanFailed},
			SecurityScanning,
			cutoff,
		).
		Updates(map[string]any{
			"security_status": SecurityScanning,
			"scan_status":     SecurityScanning,
			"scan_claim_id":   claimID,
			"scan_claimed_at": now,
		})
	if result.Error != nil {
		return false, fmt.Errorf("files: claim scan: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return true, nil
	}

	var current struct {
		SecurityStatus string     `gorm:"column:security_status"`
		ScanClaimedAt  *time.Time `gorm:"column:scan_claimed_at"`
	}
	if err := db.WithContext(ctx).Model(&FileRecord{}).
		Select("security_status, scan_claimed_at").
		Where("id = ? AND tenant_id = ?", assetID, tenantID).
		Take(&current).Error; err != nil {
		return false, err
	}
	if current.SecurityStatus == SecurityScanning && current.ScanClaimedAt != nil && current.ScanClaimedAt.After(cutoff) {
		return false, errScanAlreadyClaimed
	}
	return false, nil
}

func mapResultStatus(st string) string {
	switch strings.TrimSpace(strings.ToLower(st)) {
	case filescanner.ResultClean:
		return SecurityClean
	case filescanner.ResultRejected:
		return SecurityRejected
	case filescanner.ResultQuarantined:
		return SecurityQuarantined
	default:
		return SecurityScanFailed
	}
}

func (s *Service) runScanner(ctx context.Context, scanner filescanner.FileScanner, row *FileRecord) (filescanner.ScanResult, error) {
	if s == nil || s.Settings == nil || row == nil {
		return filescanner.ScanResult{}, fmt.Errorf("files: scan unavailable")
	}
	plain, err := s.Settings.PlainByGroup(ctx, row.TenantID, "storage")
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	prov, _, err := storage.NewFromPlain(plain)
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	data, err := prov.Get(ctx, row.ObjectKey)
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	defer data.Close()
	tmp, err := os.CreateTemp("", "tm-scan-*")
	if err != nil {
		return filescanner.ScanResult{}, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, data); err != nil {
		_ = tmp.Close()
		return filescanner.ScanResult{}, err
	}
	_ = tmp.Close()
	sum := sha256.Sum256([]byte(row.ObjectKey))
	hash := hex.EncodeToString(sum[:])
	in := filescanner.ScanInput{
		TenantID:      row.TenantID,
		AssetID:       row.ID.String(),
		ObjectKey:     row.ObjectKey,
		MimeType:      row.ContentType,
		Size:          row.Size,
		ContentHash:   hash,
		LocalTempPath: filepath.Clean(tmpPath),
	}
	scanCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return scanner.Scan(scanCtx, in)
}
