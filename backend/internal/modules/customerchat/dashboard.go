package customerchat

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
)

// DashboardSummary is customer center home KPIs.
type DashboardSummary struct {
	PendingReplyCount        int64 `json:"pendingReplyCount"`
	TodayNewMessages         int64 `json:"todayNewMessages"`
	AiSuggestionPendingCount int64 `json:"aiSuggestionPendingCount"`
	SendFailureCount         int64 `json:"sendFailureCount"`
	UnauthorizedShopCount    int64 `json:"unauthorizedShopCount"`
	SyncTaskFailureCount     int64 `json:"syncTaskFailureCount"`
	OpenConversationCount    int64 `json:"openConversationCount"`
}

// GetDashboard returns KPI snapshot for customer center home.
func (s *Service) GetDashboard(c *gin.Context) (*DashboardSummary, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	ctx := c.Request.Context()
	out := &DashboardSummary{}
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)

	_ = s.DB.WithContext(ctx).Model(&CustomerConversation{}).
		Where("status = ?", StatusPendingReply).
		Count(&out.PendingReplyCount).Error

	_ = s.DB.WithContext(ctx).Model(&CustomerConversation{}).
		Where("status IN ?", []string{StatusOpen, StatusPendingReply}).
		Count(&out.OpenConversationCount).Error

	_ = s.DB.WithContext(ctx).Model(&CustomerMessage{}).
		Where("created_at >= ?", dayStart).
		Count(&out.TodayNewMessages).Error

	_ = s.DB.WithContext(ctx).Model(&CustomerReplySuggestion{}).
		Where("status IN ?", []string{SuggestionGenerated, SuggestionEdited}).
		Count(&out.AiSuggestionPendingCount).Error

	_ = s.DB.WithContext(ctx).Model(&CustomerFailureEvent{}).
		Where("status = ? AND category IN ?", FailureEventStatusOpen, []string{
			FailureCategoryReplySendFailed,
			FailureCategoryReplyPermissionDenied,
		}).
		Count(&out.SendFailureCount).Error

	if s.Shops != nil {
		var shops []shop.Shop
		_ = s.DB.WithContext(ctx).Model(&shop.Shop{}).
			Where("status = ? AND auth_status <> ?", shop.StatusActive, shop.AuthAuthorized).
			Find(&shops).Error
		out.UnauthorizedShopCount = int64(len(shops))
	}

	type syncFail struct{ C int64 }
	var sf syncFail
	_ = s.DB.WithContext(ctx).Raw(`
SELECT COUNT(*) AS c FROM customer_message_sync_tasks
WHERE status IN ('failed','partial_success') AND updated_at >= ?
`, dayStart.AddDate(0, 0, -7)).Scan(&sf).Error
	out.SyncTaskFailureCount = sf.C

	return out, nil
}
