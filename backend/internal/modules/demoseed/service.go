package demoseed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/customerchat"
	"github.com/trademind-ai/trademind/backend/internal/modules/inventory"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/ordersync"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/id"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Service seeds dev/demo edge-case samples without calling external platforms.
type Service struct {
	DB     *gorm.DB
	OpLog  *operationlog.Service
	AppEnv string
}

// EdgeCaseResult summarizes one seeded sample.
type EdgeCaseResult struct {
	Tag    string `json:"tag"`
	ID     string `json:"id,omitempty"`
	Note   string `json:"note,omitempty"`
	Status string `json:"status"`
}

// FullProjectEdgeCasesOutput is returned by the dev-only seed endpoint.
type FullProjectEdgeCasesOutput struct {
	Phase   string           `json:"phase"`
	Samples []EdgeCaseResult `json:"samples"`
}

// SeedFullProjectEdgeCases inserts demo failure/partial_success records for F8 acceptance.
func (s *Service) SeedFullProjectEdgeCases(ctx context.Context, adminID *uuid.UUID) (*FullProjectEdgeCasesOutput, error) {
	if s == nil {
		return nil, fmt.Errorf("demoseed: service unavailable")
	}
	if strings.EqualFold(strings.TrimSpace(s.AppEnv), "production") {
		return nil, ErrProductionForbidden
	}
	if s.DB == nil {
		return nil, fmt.Errorf("demoseed: service unavailable")
	}

	out := &FullProjectEdgeCasesOutput{Phase: "F8", Samples: make([]EdgeCaseResult, 0, 8)}
	now := time.Now().UTC()
	finished := now.Add(-2 * time.Minute)

	shopRow, shopNote, err := s.ensureDemoShop(ctx, adminID)
	if err != nil {
		return nil, err
	}
	if shopNote != "" {
		out.Samples = append(out.Samples, EdgeCaseResult{Tag: "demo_shop", ID: shopRow.ID.String(), Note: shopNote, Status: "ready"})
	}

	// Order sync partial_success with page-level errors (no external API).
	orderTask := ordersync.OrderSyncTask{
		ShopID:       shopRow.ID,
		Platform:     shopRow.Platform,
		TaskType:     "order_sync",
		Status:       "partial_success",
		Mode:         "manual",
		StartedAt:    ptrTime(now.Add(-5 * time.Minute)),
		FinishedAt:   &finished,
		TotalCount:   120,
		SuccessCount: 80,
		FailedCount:  40,
		ErrorMessage: "F8 demo: 部分订单页同步失败",
		Input:        mustJSON(map[string]any{"demo": true, "seed": "f8-edge-cases"}),
		Output: mustJSON(map[string]any{
			"totalFetched": 120,
			"totalPages":   3,
			"successPages": 2,
			"failedPages":  1,
			"pageErrors": []map[string]any{
				{"page": 2, "error": "F8 demo: simulated page fetch failure"},
			},
			"demoNote": "dev-only seed; no real platform call",
		}),
		CreatedBy: adminID,
	}
	id.Ensure(&orderTask.ID)
	if err := s.DB.WithContext(ctx).Create(&orderTask).Error; err != nil {
		return nil, fmt.Errorf("demoseed: order sync task: %w", err)
	}
	out.Samples = append(out.Samples, EdgeCaseResult{
		Tag: "order_sync_partial_success", ID: orderTask.ID.String(),
		Note: "订单同步 partial_success + 页级错误", Status: "created",
	})

	prodID, skuID, prodNote, err := s.ensureDemoProduct(ctx, adminID, shopRow.ID)
	if err != nil {
		return nil, err
	}
	if prodNote != "" {
		out.Samples = append(out.Samples, EdgeCaseResult{Tag: "demo_product", ID: prodID.String(), Note: prodNote, Status: "ready"})
	}

	invTask := inventory.InventorySyncTask{
		ProductID:    prodID,
		ProductSKUID: &skuID,
		ShopID:       shopRow.ID,
		Platform:     shopRow.Platform,
		TaskType:     "inventory_sync",
		Status:       "failed",
		Mode:         "manual",
		TargetStock:  10,
		StartedAt:    ptrTime(now.Add(-3 * time.Minute)),
		FinishedAt:   &finished,
		ErrorMessage: "F8 demo: 规格未绑定平台 SKU，库存同步被阻断",
		Input:        mustJSON(map[string]any{"demo": true, "productId": prodID.String()}),
		Output: mustJSON(map[string]any{
			"errorCode": "DOUYIN_SKU_NOT_BOUND",
			"demoNote":  "dev-only seed; no real platform call",
		}),
		CreatedBy: adminID,
	}
	id.Ensure(&invTask.ID)
	if err := s.DB.WithContext(ctx).Create(&invTask).Error; err != nil {
		return nil, fmt.Errorf("demoseed: inventory sync task: %w", err)
	}
	out.Samples = append(out.Samples, EdgeCaseResult{
		Tag: "inventory_sync_failed", ID: invTask.ID.String(),
		Note: "库存同步失败（SKU 未绑定）", Status: "created",
	})

	conv := customerchat.CustomerConversation{
		Platform:         "mock",
		ShopID:           &shopRow.ID,
		CustomerName:     "F8 Demo Send Failed Buyer",
		CustomerLanguage: "zh-CN",
		Status:           "open",
		LastMessageAt:    &now,
		CreatedBy:        adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&conv).Error; err != nil {
		return nil, fmt.Errorf("demoseed: conversation: %w", err)
	}
	msg := customerchat.CustomerMessage{
		ConversationID: conv.ID,
		Role:           "customer",
		Content:        "F8 demo: 这条消息用于演示发送失败样本",
		Language:       "zh-CN",
		MessageType:    "text",
		Source:         "manual",
		CreatedBy:      adminID,
		CreatedAt:      now,
	}
	id.Ensure(&msg.ID)
	if err := s.DB.WithContext(ctx).Create(&msg).Error; err != nil {
		return nil, fmt.Errorf("demoseed: message: %w", err)
	}

	failEv := customerchat.CustomerFailureEvent{
		ConversationID: conv.ID,
		Platform:       "mock",
		ShopID:         &shopRow.ID,
		Category:       customerchat.FailureCategoryReplySendFailed,
		ErrorMessage:   "F8 demo: 平台消息发送失败（dev seed，未调用外部 API）",
		Status:         customerchat.FailureEventStatusOpen,
	}
	id.Ensure(&failEv.ID)
	if err := s.DB.WithContext(ctx).Create(&failEv).Error; err != nil {
		return nil, fmt.Errorf("demoseed: customer failure: %w", err)
	}
	out.Samples = append(out.Samples, EdgeCaseResult{
		Tag: "customer_reply_send_failed", ID: failEv.ID.String(),
		Note: "客服发送失败 + 失败任务中心", Status: "created",
	})

	unauthShop := shop.Shop{
		Platform:   "douyin_shop",
		ShopName:   "F8 Demo 未授权抖店",
		ShopCode:   fmt.Sprintf("f8-unauth-%d", now.Unix()%100000),
		Status:     "inactive",
		AuthStatus: "need_auth",
		Remark:     "F8 dev-only demo: platform not authorized sample",
		CreatedBy:  adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&unauthShop).Error; err != nil {
		return nil, fmt.Errorf("demoseed: unauthorized shop: %w", err)
	}
	unauthEv := customerchat.CustomerFailureEvent{
		ConversationID: conv.ID,
		Platform:       "douyin_shop",
		ShopID:         &unauthShop.ID,
		Category:       customerchat.FailureCategoryPlatformNotAuthorized,
		ErrorMessage:   "F8 demo: 店铺未授权，无法发送平台消息",
		Status:         customerchat.FailureEventStatusOpen,
	}
	id.Ensure(&unauthEv.ID)
	if err := s.DB.WithContext(ctx).Create(&unauthEv).Error; err != nil {
		return nil, fmt.Errorf("demoseed: platform not authorized failure: %w", err)
	}
	out.Samples = append(out.Samples, EdgeCaseResult{
		Tag: "platform_not_authorized", ID: unauthShop.ID.String(),
		Note: "平台未授权店铺 + 客服失败分类", Status: "created",
	})

	return out, nil
}

func (s *Service) ensureDemoShop(ctx context.Context, adminID *uuid.UUID) (*shop.Shop, string, error) {
	var row shop.Shop
	err := s.DB.WithContext(ctx).
		Where("deleted_at IS NULL").
		Order("created_at ASC").
		First(&row).Error
	if err == nil {
		return &row, "", nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, "", err
	}
	row = shop.Shop{
		Platform:   "manual",
		ShopName:   "F8 Demo Manual Shop",
		ShopCode:   fmt.Sprintf("f8-manual-%d", time.Now().Unix()%100000),
		Status:     "active",
		AuthStatus: "authorized",
		Currency:   "CNY",
		CreatedBy:  adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, "", fmt.Errorf("demoseed: create shop: %w", err)
	}
	return &row, "created demo manual shop", nil
}

func (s *Service) ensureDemoProduct(ctx context.Context, adminID *uuid.UUID, shopID uuid.UUID) (uuid.UUID, uuid.UUID, string, error) {
	var cfg product.ProductPlatformPublishConfig
	err := s.DB.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Order("updated_at DESC").
		First(&cfg).Error
	if err == nil && cfg.ProductID != uuid.Nil {
		var sku product.ProductSKU
		if e := s.DB.WithContext(ctx).Where("product_id = ?", cfg.ProductID).Order("created_at ASC").First(&sku).Error; e == nil {
			return cfg.ProductID, sku.ID, "", nil
		}
	}

	stock := 50
	p := product.Product{
		Source:      "manual",
		Title:       "F8 demo edge-case product",
		Description: "Dev-only demo product for inventory sync failure sample.",
		Currency:    "CNY",
		Status:      product.StatusDraft,
		CreatedBy:   adminID,
	}
	if err := s.DB.WithContext(ctx).Create(&p).Error; err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("demoseed: create product: %w", err)
	}
	sku := product.ProductSKU{
		ProductID: p.ID,
		SKUCode:   fmt.Sprintf("F8-DEMO-%d", time.Now().Unix()%100000),
		SKUName:   "Default",
		Price:     ptrFloat(29.9),
		Stock:     &stock,
	}
	if err := s.DB.WithContext(ctx).Create(&sku).Error; err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("demoseed: create sku: %w", err)
	}
	pubCfg := product.ProductPlatformPublishConfig{
		ProductID: p.ID,
		Platform:  "manual",
		ShopID:    &shopID,
	}
	id.Ensure(&pubCfg.ID)
	if err := s.DB.WithContext(ctx).Create(&pubCfg).Error; err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("demoseed: publish config: %w", err)
	}
	return p.ID, sku.ID, "created demo product linked to shop", nil
}

func mustJSON(v any) datatypes.JSON {
	b, _ := json.Marshal(v)
	return datatypes.JSON(b)
}

func ptrTime(t time.Time) *time.Time { return &t }

func ptrFloat(v float64) *float64 { return &v }
