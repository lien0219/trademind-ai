package customerchat

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"gorm.io/gorm"
)

type DashboardSummary struct {
	PendingReplyCount        int64 `json:"pendingReplyCount"`
	TodayNewMessages         int64 `json:"todayNewMessages"`
	AiSuggestionPendingCount int64 `json:"aiSuggestionPendingCount"`
	SendFailureCount         int64 `json:"sendFailureCount"`
	UnauthorizedShopCount    int64 `json:"unauthorizedShopCount"`
	SyncTaskFailureCount     int64 `json:"syncTaskFailureCount"`
	OpenConversationCount    int64 `json:"openConversationCount"`
}

type DashboardFilter struct {
	Platform string
	ShopID   *uuid.UUID
}

func (s *Service) GetDashboard(c *gin.Context, filter DashboardFilter) (*DashboardSummary, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	if !adminperm.CanViewCustomer(c, s.DB) {
		return nil, gorm.ErrRecordNotFound
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	ctx := c.Request.Context()
	out := &DashboardSummary{}
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	platform := strings.TrimSpace(filter.Platform)
	conv := s.DB.WithContext(ctx).Model(&CustomerConversation{}).Where("tenant_id = ?", tid)
	if platform != "" {
		conv = conv.Where("platform = ?", platform)
	}
	if filter.ShopID != nil {
		conv = conv.Where("shop_id = ?", *filter.ShopID)
	}
	conv, err = adminperm.ApplyStoreScope(c, s.DB, conv, "shop_id")
	if err != nil {
		return nil, err
	}
	ids := conv.Session(&gorm.Session{NewDB: false}).Select("id")
	if err := conv.Session(&gorm.Session{NewDB: false}).Where("status = ?", StatusPendingReply).Count(&out.PendingReplyCount).Error; err != nil {
		return nil, err
	}
	if err := conv.Session(&gorm.Session{NewDB: false}).Where("status IN ?", []string{StatusOpen, StatusPendingReply}).Count(&out.OpenConversationCount).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&CustomerMessage{}).Where("conversation_id IN (?) AND created_at >= ?", ids, dayStart).Count(&out.TodayNewMessages).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&CustomerReplySuggestion{}).Where("conversation_id IN (?) AND status IN ?", ids, []string{SuggestionGenerated, SuggestionEdited}).Count(&out.AiSuggestionPendingCount).Error; err != nil {
		return nil, err
	}
	if err := s.DB.WithContext(ctx).Model(&CustomerFailureEvent{}).Where("conversation_id IN (?) AND status = ? AND category IN ?", ids, FailureEventStatusOpen, []string{FailureCategoryReplySendFailed, FailureCategoryReplyPermissionDenied}).Count(&out.SendFailureCount).Error; err != nil {
		return nil, err
	}
	shops := s.DB.WithContext(ctx).Model(&shop.Shop{}).Where("tenant_id = ?", tid)
	if platform != "" {
		shops = shops.Where("platform = ?", platform)
	}
	if filter.ShopID != nil {
		shops = shops.Where("id = ?", *filter.ShopID)
	}
	shops, err = adminperm.ApplyStoreScope(c, s.DB, shops, "id")
	if err != nil {
		return nil, err
	}
	shopIDs := shops.Session(&gorm.Session{NewDB: false}).Select("id")
	if err := shops.Session(&gorm.Session{NewDB: false}).Where("status = ? AND auth_status <> ?", shop.StatusActive, shop.AuthAuthorized).Count(&out.UnauthorizedShopCount).Error; err != nil {
		return nil, err
	}
	syncTasks := s.DB.WithContext(ctx).Table("customer_message_sync_tasks").Where("tenant_id = ? AND shop_id IN (?) AND status IN ? AND updated_at >= ?", tid, shopIDs, []string{"failed", "partial_success"}, dayStart.AddDate(0, 0, -7))
	if platform != "" {
		syncTasks = syncTasks.Where("platform = ?", platform)
	}
	if err := syncTasks.Count(&out.SyncTaskFailureCount).Error; err != nil {
		return nil, err
	}
	return out, nil
}
