package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *Service) execTimeout() time.Duration {
	if s.TaskTimeout <= 0 {
		return 180 * time.Second
	}
	return s.TaskTimeout
}

func (s *Service) ProcessQueuedTask(ctx context.Context, taskID uuid.UUID, workerID string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("productpublish: no db")
	}
	defer func() {
		if r := recover(); r != nil {
			s.handlePublishPanic(ctx, taskID, workerID, r)
		}
	}()

	lease := s.publishLeaseTTL()
	taskRow, claimed, err := s.tryClaimProductPublishTask(ctx, taskID, workerID, lease)
	if err != nil {
		return err
	}
	if !claimed || taskRow == nil {
		return nil
	}

	cancelRen := s.startPublishLeaseRenewal(ctx, taskID, workerID, lease)
	defer cancelRen()

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "product.publish.running",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s shopId=%s platform=%s", taskID.String(), taskRow.ShopID.String(), taskRow.Platform),
		})
	}

	fail := func(msg string) error {
		fin := time.Now().UTC()
		_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
			Updates(map[string]any{
				"status":        TaskFailed,
				"error_message": msg,
				"finished_at":   &fin,
				"locked_by":     nil,
				"locked_until":  nil,
				"updated_at":    fin,
			}).Error
		if rid, ok := snapshotPublicationFromTask(taskRow); ok {
			_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", rid).
				Updates(map[string]any{
					"status":         StatusPubFailed,
					"publish_status": StatusPubFailed,
					"updated_at":     fin,
				}).Error
		}
		if s.OpLog != nil {
			_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
				AdminUserID: taskRow.CreatedBy,
				Action:      "product.publish.failed",
				Resource:    "product_publish_task",
				ResourceID:  taskID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s err=%s", taskID.String(), truncateMsg(msg)),
			})
		}
		return fmt.Errorf("%s", msg)
	}

	snap, err := parsePublishSnapshot(taskRow.Input)
	if err != nil {
		return fail(err.Error())
	}

	var prod product.Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&prod, "id = ?", taskRow.ProductID).Error; err != nil {
		return fail(fmt.Sprintf("load product: %v", err))
	}
	draft, err := BuildPlatformDraftFromProduct(prod)
	if err != nil {
		return fail(err.Error())
	}

	_, plainAuth, err := s.Shops.PlainAuthForProviderCtx(ctx, taskRow.ShopID)
	if err != nil {
		return fail("shop not available")
	}

	prov := platformp.Get(strings.TrimSpace(strings.ToLower(taskRow.Platform)))
	if prov == nil || !platformp.IsProductPublishRunnable(prov) {
		return fail(platformp.ErrProductPublishNotImplemented.Error())
	}
	pp, ok := platformp.AsProductPublish(prov)
	if !ok || pp == nil {
		return fail(platformp.ErrProductPublishNotImplemented.Error())
	}

	pubCfg := stringifyPublishMap(snap.MergedPublish)
	req := platformp.PublishProductRequest{
		ShopID:        taskRow.ShopID,
		Platform:      taskRow.Platform,
		Auth:          plainAuth,
		Product:       draft,
		PublishConfig: pubCfg,
		Options:       snap.Options,
	}

	xctx, cancel := context.WithTimeout(ctx, s.execTimeout())
	defer cancel()
	res, pubErr := pp.PublishProduct(xctx, req)
	if pubErr != nil {
		msg := pubErr.Error()
		_ = fail(msg)
		if errors.Is(pubErr, platformp.ErrPlatformProductPublishPermissionDenied) {
			return platformp.ErrPlatformProductPublishPermissionDenied
		}
		return fmt.Errorf("%s", msg)
	}
	if res == nil {
		return fail("empty publish result")
	}
	if strings.TrimSpace(res.ExternalProductID) == "" {
		return fail("platform did not return external product id")
	}

	fin := time.Now().UTC()
	outSnap := platformp.TrimRawMap(map[string]any{
		"externalProductId": res.ExternalProductID,
		"externalSpuId":     res.ExternalSPUID,
		"externalUrl":       res.ExternalURL,
		"status":            res.Status,
		"providerSummary":   res.RawSummary,
	}, 20, 300)
	rawOut, _ := json.Marshal(outSnap)

	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        TaskSuccess,
			"error_message": "",
			"finished_at":   &fin,
			"output":        datatypes.JSON(rawOut),
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error

	pubSnap := platformp.TrimRawMap(map[string]any{
		"externalProductId": res.ExternalProductID,
		"skuMapped":         len(res.SKUMappings),
	}, 12, 200)
	rd, _ := json.Marshal(pubSnap)
	pubStatus := normalizePublicationStatus(res.Status)
	var publishedAt *time.Time
	if pubStatus == StatusPublishedRecord {
		publishedAt = &fin
	}

	_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", snap.PublicationID).
		Updates(map[string]any{
			"publish_status":      pubStatus,
			"status":              pubStatus,
			"external_product_id": strings.TrimSpace(res.ExternalProductID),
			"external_spu_id":     strings.TrimSpace(res.ExternalSPUID),
			"external_url":        strings.TrimSpace(res.ExternalURL),
			"published_at":        publishedAt,
			"last_synced_at":      &fin,
			"raw_data":            datatypes.JSON(rd),
			"updated_at":          fin,
		}).Error

	_ = s.DB.WithContext(ctx).Where("publication_id = ?", snap.PublicationID).Delete(&ProductPublicationSKU{}).Error
	for _, m := range res.SKUMappings {
		skuRow := ProductPublicationSKU{
			PublicationID: snap.PublicationID,
			ProductSKUID:  nilUUIDPtr(m.LocalSKUID),
			ExternalSKUID: strings.TrimSpace(m.ExternalSKUID),
			SKUCode:       strings.TrimSpace(m.SKUCode),
			Price:         m.Price,
			Stock:         m.Stock,
		}
		if skuRow.ExternalSKUID == "" {
			continue
		}
		rdSrc := m.RawData
		if len(rdSrc) == 0 {
			rdSrc = platformp.TrimRawMap(map[string]any{"mapped": true}, 6, 80)
		}
		rdm, _ := json.Marshal(platformp.TrimRawMap(rdSrc, 12, 200))
		skuRow.RawData = datatypes.JSON(rdm)
		_ = s.DB.WithContext(ctx).Create(&skuRow).Error
	}

	if s.OpLog != nil {
		_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
			AdminUserID: taskRow.CreatedBy,
			Action:      "product.publish.success",
			Resource:    "product_publish_task",
			ResourceID:  taskID.String(),
			Status:      "success",
			Message: fmt.Sprintf("taskId=%s publicationId=%s externalProductId=%s skuMappings=%d",
				taskID.String(), snap.PublicationID.String(), res.ExternalProductID, len(res.SKUMappings)),
		})
	}
	return nil
}

func truncateMsg(msg string) string {
	runes := []rune(msg)
	if len(runes) > 480 {
		return string(runes[:480]) + "…"
	}
	return msg
}

func nilUUIDPtr(u uuid.UUID) *uuid.UUID {
	if u == uuid.Nil {
		return nil
	}
	return &u
}

func normalizePublicationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case StatusDraft:
		return StatusDraft
	case StatusPublishing, "submitted", "processing":
		return StatusPublishing
	case StatusPublishedRecord, "success":
		return StatusPublishedRecord
	case StatusRejected:
		return StatusRejected
	case StatusOffline:
		return StatusOffline
	case StatusPubFailed:
		return StatusPubFailed
	default:
		return StatusPublishedRecord
	}
}

func (s *Service) handlePublishPanic(parent context.Context, taskID uuid.UUID, workerID string, panicVal any) {
	if s == nil || s.DB == nil {
		return
	}
	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	var cur ProductPublishTask
	if err := s.DB.WithContext(ctx).First(&cur, "id = ?", taskID).Error; err != nil {
		return
	}
	if cur.Status != TaskRunning || cur.LockedBy == nil || *cur.LockedBy != workerID {
		return
	}
	msg := fmt.Sprintf("publish worker panic: %v", panicVal)
	fin := time.Now().UTC()
	_ = s.DB.WithContext(ctx).Model(&ProductPublishTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":        TaskFailed,
			"error_message": msg,
			"finished_at":   &fin,
			"locked_by":     nil,
			"locked_until":  nil,
			"updated_at":    fin,
		}).Error
	if rid, ok := snapshotPublicationFromTask(&cur); ok {
		_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", rid).
			Updates(map[string]any{
				"status":         StatusPubFailed,
				"publish_status": StatusPubFailed,
				"updated_at":     fin,
			}).Error
	}
	slog.Warn("product_publish_worker_panic_recovered", "taskId", taskID.String(), "worker", workerID)
}
