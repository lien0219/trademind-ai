package douyinruntime

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/config"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
)

// StartDouyinAlertScanWorker runs periodic ScanDouyinAlerts when settings allow.
func StartDouyinAlertScanWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, reg *worker.Registry, cfg *config.Config) {
	if wg == nil || svc == nil || cfg == nil {
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		ri := reg.Register(ctx, worker.TypeTaskAlertScan, "douyin-alert-scan", map[string]any{"scan": "douyin_alerts"})
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
			m, err := svc.Settings.PlainByGroup(ctx, 0, groupKey)
			if err != nil {
				if log != nil {
					log.Warn("douyin_alert_scan_settings_failed", "error", err)
				}
				time.Sleep(10 * time.Second)
				continue
			}
			if !parseBoolSetting(m["alert_scan_enabled"], true) {
				time.Sleep(5 * time.Second)
				continue
			}
			sec := douyinAlertScanInterval(m["alert_scan_interval_seconds"], 120)
			timer := time.NewTimer(time.Duration(sec) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			runCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			_, err = svc.ScanDouyinAlerts(runCtx)
			cancel()
			if err != nil && log != nil {
				log.Warn("douyin_alert_scan_failed", "error", err)
			}
		}
	}()
}

func douyinAlertScanInterval(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 30 {
		return def
	}
	return n
}
