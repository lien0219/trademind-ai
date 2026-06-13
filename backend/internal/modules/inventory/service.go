package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/settings"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service orchestrates append-only ledger rows + outbound provider inventory_sync tasks.
type Service struct {
	DB           *gorm.DB
	Redis        *rdb.Client
	Shops        *shop.Service
	Settings     *settings.Service
	OpLog        *operationlog.Service
	QueueEnabled bool
	QueueName    string
	TaskTimeout  time.Duration
}

func (s *Service) normalizedQueueName() string {
	q := strings.TrimSpace(s.QueueName)
	if q == "" {
		return "inventory:sync:tasks"
	}
	return q
}

func clampStr(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max]
}

func derefStock(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func pagesOf(total int64, ps int) int {
	if ps < 1 {
		ps = 20
	}
	pages := int(total) / ps
	if int(total)%ps != 0 {
		pages++
	}
	if pages == 0 && total > 0 {
		pages = 1
	}
	return pages
}

func ptrUUID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func taskInputSnap(mode string, target int, publicationSkuID uuid.UUID, productSkuID *uuid.UUID, pubID uuid.UUID,
	shop uuid.UUID, options map[string]any, batchID *uuid.UUID, batchNo string,
) datatypes.JSON {
	m := map[string]any{
		"taskType":     TaskTypeInventorySync,
		"mode":         mode,
		"targetStock":  target,
		"stockVersion": target,
		"shopId":       shop.String(),
	}
	if publicationSkuID != uuid.Nil {
		m["publicationSkuId"] = publicationSkuID.String()
	}
	if pubID != uuid.Nil {
		m["publicationId"] = pubID.String()
	}
	if productSkuID != nil && *productSkuID != uuid.Nil {
		m["productSkuId"] = productSkuID.String()
	}
	if batchID != nil && *batchID != uuid.Nil {
		m["batchId"] = batchID.String()
	}
	if strings.TrimSpace(batchNo) != "" {
		m["batchNo"] = strings.TrimSpace(batchNo)
	}
	if len(options) > 0 {
		m["options"] = platformp.TrimRawMap(options, 12, 200)
	}
	b, _ := json.Marshal(m)
	return b
}

func (s *Service) enqueue(ctx context.Context, taskID uuid.UUID) error {
	if s.Redis == nil || s.Redis.Client == nil {
		return ErrRedisQueueUnavailable
	}
	msg := QueueMessage{TaskID: taskID.String()}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.Redis.LPush(ctx, s.normalizedQueueName(), string(b)).Err()
}

func (s *Service) persistTaskAndMaybeRun(ctx context.Context, t *InventorySyncTask, admin *uuid.UUID) error {
	if t == nil {
		return fmt.Errorf("task nil")
	}
	if err := s.DB.WithContext(ctx).Create(t).Error; err != nil {
		return err
	}
	if t.BatchID == nil && s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: admin,
			Action:      "inventory.sync.create",
			Resource:    "inventory_sync_task",
			ResourceID:  t.ID.String(),
			Status:      "success",
			Message: fmt.Sprintf("taskId=%s shopId=%s platform=%s target=%d mode=%s",
				t.ID.String(), t.ShopID.String(), t.Platform, t.TargetStock, t.Mode),
		})
	}
	return s.enqueueOrRunInventoryTask(ctx, t.ID)
}

func (s *Service) hasDuplicateInventorySync(ctx context.Context, pubSkuID uuid.UUID, target int) (bool, error) {
	if s == nil || s.DB == nil || pubSkuID == uuid.Nil {
		return false, nil
	}
	var n int64
	err := s.DB.WithContext(ctx).Model(&InventorySyncTask{}).
		Where("publication_sku_id = ? AND target_stock = ? AND status IN ?", pubSkuID, target, []string{StatusPending, StatusRunning}).
		Count(&n).Error
	return n > 0, err
}

func (s *Service) enqueueOrRunInventoryTask(ctx context.Context, taskID uuid.UUID) error {
	runInline := func() error {
		return s.ProcessQueuedTask(context.Background(), taskID, worker.GenerateInlineWorkerID(worker.TypeInventorySync))
	}
	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		if err := s.enqueue(ctx, taskID); err != nil {
			slog.Warn("inventory_sync_enqueue_failed_run_inline", "taskId", taskID.String(), "error", err)
			return runInline()
		}
		return nil
	}
	return runInline()
}
