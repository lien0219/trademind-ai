package customersync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	minimumAutoReplyPollInterval = 15 * time.Second
	autoReplySchedulerBatchSize  = 100
)

// StartAutoReplyPollingScheduler creates incremental sync tasks only for stores that
// explicitly enabled automatic replies. Poll ownership is persisted for multi-instance safety.
func StartAutoReplyPollingScheduler(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service) {
	if svc == nil || svc.DB == nil || svc.Redis == nil || svc.Redis.Client == nil || svc.Shops == nil || svc.CustomerChat == nil {
		return
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		setAutoReplyPollingSchedulerRunning(true)
		defer setAutoReplyPollingSchedulerRunning(false)
		ticker := time.NewTicker(minimumAutoReplyPollInterval)
		defer ticker.Stop()
		for {
			if err := svc.scheduleAutoReplySyncs(ctx); err != nil && log != nil && ctx.Err() == nil {
				log.Warn("customer_auto_reply_poll_failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *Service) scheduleAutoReplySyncs(ctx context.Context) error {
	now := time.Now().UTC()
	var policies []customerchat.CustomerAutoReplyPolicy
	if err := s.DB.WithContext(ctx).
		Where("enabled = ? AND (next_poll_at IS NULL OR next_poll_at <= ?)", true, now).
		Order("next_poll_at ASC NULLS FIRST, id ASC").Limit(autoReplySchedulerBatchSize).
		Find(&policies).Error; err != nil {
		// SQLite does not support the PostgreSQL NULLS FIRST syntax in all supported versions.
		if s.DB.Dialector.Name() != "sqlite" {
			return err
		}
		policies = nil
		if retryErr := s.DB.WithContext(ctx).
			Where("enabled = ? AND (next_poll_at IS NULL OR next_poll_at <= ?)", true, now).
			Order("next_poll_at ASC, id ASC").Limit(autoReplySchedulerBatchSize).
			Find(&policies).Error; retryErr != nil {
			return retryErr
		}
	}
	settingsByTenant := make(map[int64]customerchat.CustomerAutoReplySetting)
	for i := range policies {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		policy := &policies[i]
		if policy.ShopID == uuid.Nil || policy.TenantID < 0 {
			continue
		}
		setting, loaded := settingsByTenant[policy.TenantID]
		var err error
		if !loaded {
			setting, err = s.CustomerChat.LoadAutoReplySetting(ctx, policy.TenantID)
			if err == nil {
				settingsByTenant[policy.TenantID] = setting
			}
		}
		if err != nil || !setting.MessageSyncEnabled || !setting.AutoReplyEnabled {
			cooldown := now.Add(time.Hour)
			if updateErr := s.DB.WithContext(ctx).Model(&customerchat.CustomerAutoReplyPolicy{}).
				Where("id = ? AND (next_poll_at IS NULL OR next_poll_at <= ?)", policy.ID, now).
				Updates(map[string]any{"next_poll_at": &cooldown, "updated_at": now}).Error; updateErr != nil {
				return updateErr
			}
			continue
		}
		interval := time.Duration(setting.PollIntervalSeconds) * time.Second
		if interval < minimumAutoReplyPollInterval {
			interval = minimumAutoReplyPollInterval
		}
		next := now.Add(interval)
		claim := s.DB.WithContext(ctx).Model(&customerchat.CustomerAutoReplyPolicy{}).
			Where("id = ? AND enabled = ? AND (next_poll_at IS NULL OR next_poll_at <= ?)", policy.ID, true, now).
			Updates(map[string]any{"next_poll_at": &next, "updated_at": now})
		if claim.Error != nil {
			return claim.Error
		}
		if claim.RowsAffected != 1 {
			continue
		}
		wctx, _, err := tasktenant.BeginWorkerWithLegacyZero(ctx, s.DB, policy.TenantID, policy.ShopID, "customer_auto_reply_poll", s.AllowLegacyTenantZero)
		if err != nil {
			continue
		}
		if err := s.scheduleAutoReplyShopSync(wctx, policy); err != nil {
			slog.Warn("customer_auto_reply_shop_poll_skipped", "shopId", policy.ShopID.String(), "error", err)
		}
	}
	return nil
}

func (s *Service) scheduleAutoReplyShopSync(ctx context.Context, policy *customerchat.CustomerAutoReplyPolicy) error {
	if policy == nil || policy.ShopID == uuid.Nil || policy.TenantID < 0 || s == nil || s.DB == nil || s.Redis == nil || s.Redis.Client == nil || s.Shops == nil {
		return fmt.Errorf("auto reply polling unavailable")
	}
	var shopRow shop.Shop
	if err := s.DB.WithContext(ctx).First(&shopRow, "id = ? AND tenant_id = ?", policy.ShopID, policy.TenantID).Error; err != nil {
		return err
	}
	prov := platformp.Get(strings.TrimSpace(shopRow.Platform))
	if err := ValidateShopCustomerMessageSync(&shopRow, prov); err != nil {
		return err
	}
	_, auth, err := s.Shops.PlainAuthForProviderCtx(ctx, policy.ShopID)
	if err != nil {
		return err
	}
	if err := ensureShopAuthorizedForSync(&shopRow, auth); err != nil {
		return err
	}
	if err := ensurePlatformPartnerConfigStatic(s.Settings, ctx, prov); err != nil {
		return err
	}

	snapshot := syncInputSnapshot{Mode: ModeIncremental, Limit: 100}
	var latest CustomerMessageSyncTask
	latestErr := s.DB.WithContext(ctx).
		Where("tenant_id = ? AND shop_id = ?", policy.TenantID, policy.ShopID).
		Order("created_at DESC").First(&latest).Error
	if latestErr == nil {
		if latest.Status == StatusFailed {
			return fmt.Errorf("latest customer message sync failed; manual retry required")
		}
		snapshot.Cursor = strings.TrimSpace(latest.Cursor)
		if snapshot.Cursor == "" && latest.FinishedAt != nil {
			snapshot.Start = latest.FinishedAt.UTC().Add(-2 * time.Minute).Format(time.RFC3339)
		}
	} else if latestErr != gorm.ErrRecordNotFound {
		return latestErr
	}
	input, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	task := CustomerMessageSyncTask{
		TenantID: policy.TenantID, ShopID: policy.ShopID, Platform: strings.TrimSpace(shopRow.Platform),
		TaskType: TaskTypeCustomerMessageSync, Status: StatusPending, Mode: ModeIncremental,
		Cursor: snapshot.Cursor, Input: datatypes.JSON(input),
	}
	created, err := s.createSyncTaskIfIdle(ctx, &task)
	if err != nil || !created {
		return err
	}
	if err := s.enqueue(ctx, task.ID); err != nil {
		fin := time.Now().UTC()
		updateErr := s.DB.WithContext(ctx).Model(&CustomerMessageSyncTask{}).Where("id = ? AND status = ?", task.ID, StatusPending).Updates(map[string]any{
			"status": StatusFailed, "error_message": "customer message queue unavailable", "finished_at": &fin, "updated_at": fin,
		}).Error
		if updateErr != nil {
			return fmt.Errorf("enqueue customer sync: %v; persist failure: %w", err, updateErr)
		}
		return err
	}
	return nil
}
