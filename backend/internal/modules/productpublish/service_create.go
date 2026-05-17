package productpublish

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CreatePublishTask validates settings + draft, persists task + publishing row snapshot, enqueue or runs inline.
func (s *Service) CreatePublishTask(c *gin.Context, productID uuid.UUID, body PublishRequestBody, adminID *uuid.UUID) (*TaskDTO, error) {
	if s == nil || s.DB == nil || s.Settings == nil || s.Shops == nil || c == nil {
		return nil, fmt.Errorf("product publish unavailable")
	}
	ctx := c.Request.Context()
	sid, err := uuid.Parse(strings.TrimSpace(body.ShopID))
	if err != nil {
		return nil, fmt.Errorf("invalid shopId")
	}

	var prod product.Product
	if err := s.DB.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort_order ASC, created_at ASC") }).
		Preload("SKUs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at ASC") }).
		First(&prod, "id = ?", productID).Error; err != nil {
		return nil, err
	}
	if prod.DeletedAt.Valid {
		return nil, fmt.Errorf("deleted product cannot be published")
	}
	if _, err := BuildPlatformDraftFromProduct(prod); err != nil {
		return nil, err
	}

	row, plainAuth, err := s.Shops.PlainAuthForProviderCtx(ctx, sid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("shop not found")
		}
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("shop not found")
	}
	if prod.TenantID != 0 && row.TenantID != 0 && prod.TenantID != row.TenantID {
		return nil, fmt.Errorf("product tenant does not match shop tenant")
	}

	platKey := strings.TrimSpace(strings.ToLower(row.Platform))

	prov := platformp.Get(platKey)
	if prov == nil {
		return nil, fmt.Errorf("unknown platform")
	}
	if !platformp.IsProductPublishRunnable(prov) {
		return nil, platformp.ErrProductPublishNotImplemented
	}
	if _, ok := platformp.AsProductPublish(prov); !ok {
		return nil, platformp.ErrProductPublishNotImplemented
	}

	if err := ensurePartnerOpenConfig(ctx, s.Settings, prov); err != nil {
		return nil, err
	}

	if err := ensureShopAuthorizedForPublish(row, plainAuth); err != nil {
		return nil, err
	}

	pubSch := prov.PublishConfigSchema()
	pubGK := strings.TrimSpace(pubSch.GroupKey)
	if pubGK == "" {
		return nil, fmt.Errorf("platform %q does not expose publish configuration schema", platKey)
	}
	curPub, err := s.Settings.PlainByGroup(ctx, 0, pubGK)
	if err != nil {
		return nil, err
	}
	base := mergePublishBaseline(pubSch, curPub)
	merged := ApplyPublishOptions(base, body.Options)
	if err := validateMergedPublishAgainstSchema(pubSch, merged); err != nil {
		return nil, err
	}

	optsSnap := map[string]any{}
	if body.Options != nil {
		for k, v := range body.Options {
			optsSnap[k] = v
		}
	}

	var pubRow ProductPublication
	q := s.DB.WithContext(ctx).Where("product_id = ? AND shop_id = ? AND platform = ? AND publish_status = ?",
		prod.ID, sid, platKey, StatusPublishing).
		Order("updated_at DESC")
	if err := q.First(&pubRow).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		title := strings.TrimSpace(prod.Title)
		if title == "" {
			title = strings.TrimSpace(prod.AITitle)
		}
		curr := strings.TrimSpace(prod.Currency)
		if curr == "" {
			curr = "USD"
		}
		pubRow = ProductPublication{
			ProductID:     prod.ID,
			ShopID:        sid,
			Platform:      platKey,
			Status:        StatusPublishing,
			PublishStatus: StatusPublishing,
			Title:         title,
			Currency:      curr,
			CreatedBy:     adminID,
		}
		if err := s.DB.WithContext(ctx).Create(&pubRow).Error; err != nil {
			return nil, err
		}
	} else {
		title := strings.TrimSpace(prod.Title)
		if title == "" {
			title = strings.TrimSpace(prod.AITitle)
		}
		curr := strings.TrimSpace(prod.Currency)
		if curr == "" {
			curr = "USD"
		}
		_ = s.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", pubRow.ID).
			Updates(map[string]any{
				"title":               title,
				"currency":            curr,
				"publish_task_id":     nil,
				"external_product_id": "",
				"external_spu_id":     "",
				"external_url":        "",
				"raw_data":            datatypes.JSON(nil),
				"published_at":        nil,
				"status":              StatusPublishing,
				"publish_status":      StatusPublishing,
			}).Error
	}
	inp := publishSnapshot{
		PublicationID: pubRow.ID,
		MergedPublish: merged,
		Options:       optsSnap,
	}
	rawIn, err := json.Marshal(inp)
	if err != nil {
		return nil, err
	}

	task := ProductPublishTask{
		ProductID: prod.ID,
		ShopID:    sid,
		Platform:  platKey,
		TaskType:  TaskTypeProductPublish,
		Status:    TaskPending,
		Mode:      ModeManual,
		Input:     rawIn,
		CreatedBy: adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&task).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&ProductPublication{}).
		Where("id = ?", pubRow.ID).
		Updates(map[string]any{"publish_task_id": task.ID, "updated_at": task.CreatedAt}).Error; err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "product.publish.create",
			Resource:    "product_publish_task",
			ResourceID:  task.ID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s productId=%s shopId=%s platform=%s", task.ID.String(), prod.ID.String(), sid.String(), platKey),
		})
	}

	runInline := func() error {
		return s.ProcessQueuedTask(context.Background(), task.ID, worker.GenerateInlineWorkerID(worker.TypeProductPublish))
	}

	if s.QueueEnabled && s.Redis != nil && s.Redis.Client != nil {
		if err := s.enqueue(ctx, task.ID); err != nil {
			slog.Warn("product_publish_enqueue_failed_run_inline", "taskId", task.ID.String(), "error", err)
			if err := runInline(); err != nil {
				return nil, err
			}
		}
	} else {
		if err := runInline(); err != nil {
			return nil, err
		}
	}

	out, err := s.GetDTO(ctx, task.ID)
	return &out, err
}
