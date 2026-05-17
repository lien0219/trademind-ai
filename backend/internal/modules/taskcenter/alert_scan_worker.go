package taskcenter

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
)

// StartAlertScanWorker runs periodic ScanAndGenerateTaskAlerts when env and settings allow.
func StartAlertScanWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, reg *worker.Registry, cfg *config.Config) {
	if wg == nil || svc == nil || cfg == nil || !cfg.TaskAlertScanEnabled {
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		ri := reg.Register(ctx, worker.TypeTaskAlertScan, "task-alert-scan", map[string]any{"scan": "task_alerts"})
		defer ri.Stop(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if svc.Settings == nil {
				time.Sleep(5 * time.Second)
				continue
			}
			tc, err := svc.Settings.PlainByGroup(ctx, 0, "taskcenter")
			if err != nil {
				if log != nil {
					log.Warn("alert_scan_settings_read_failed", "error", err)
				}
				time.Sleep(10 * time.Second)
				continue
			}
			if !parseBoolTaskCenter(tc["enable_alert_scan_worker"], false) {
				time.Sleep(5 * time.Second)
				continue
			}
			sec := alertScanIntervalSeconds(tc["alert_scan_interval_seconds"], cfg.TaskAlertScanIntervalSeconds)
			if sec < 10 {
				sec = 10
			}
			timer := time.NewTimer(time.Duration(sec) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			lockSec := cfg.TaskAlertScanLockTTLSeconds
			if lockSec <= 0 {
				lockSec = 120
			}
			runCtx, cancel := context.WithTimeout(ctx, time.Duration(lockSec)*time.Second)
			sum, err := svc.ScanAndGenerateTaskAlerts(runCtx)
			cancel()
			if err != nil {
				if log != nil {
					log.Warn("task_alert_scan_failed", "error", err)
				}
				if svc.OpLog != nil {
					_ = svc.OpLog.WriteBackground(context.Background(), operationlog.WriteOpts{
						Action:     "task_center.alert.scan_worker.failed",
						Resource:   "task_alert",
						ResourceID: "scan",
						Status:     "failed",
						Message:    truncateRunes(err.Error(), 400),
					})
				}
				continue
			}
			if svc.OpLog != nil {
				msg := truncateRunes(fmt.Sprintf("scanned=%d gen=%d up=%d skip=%d", sum.ScannedCount, sum.GeneratedCount, sum.UpdatedCount, sum.IgnoredCount), 480)
				_ = svc.OpLog.WriteBackground(context.Background(), operationlog.WriteOpts{
					Action:     "task_center.alert.scan_worker.run",
					Resource:   "task_alert",
					ResourceID: "scan",
					Status:     "success",
					Message:    msg,
				})
			}
		}
	}()
}

func alertScanIntervalSeconds(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if def > 0 {
			return def
		}
		return 60
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		if def > 0 {
			return def
		}
		return 60
	}
	return n
}
