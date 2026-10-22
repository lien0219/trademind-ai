package customerchat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/gorm"
)

type AutoReplyProcessSummary struct {
	GlobalEnabled bool `json:"globalEnabled"`
	PolicyEnabled bool `json:"policyEnabled"`
	Candidates    int  `json:"candidates"`
	Sent          int  `json:"sent"`
	HumanRequired int  `json:"humanRequired"`
	Skipped       int  `json:"skipped"`
	Failed        int  `json:"failed"`
}

type autoReplyQueueMessage struct {
	ShopID        string `json:"shopId"`
	MessageID     string `json:"messageId"`
	DispatchToken string `json:"dispatchToken,omitempty"`
}

func (s *Service) normalizedAutoReplyQueueName() string {
	if q := strings.TrimSpace(s.AutoReplyQueueName); q != "" {
		return q
	}
	return "customer:auto:reply:tasks"
}

// EnqueueAutoReplies persists one idempotent run before queueing, so a process crash cannot silently duplicate sends.
func (s *Service) EnqueueAutoReplies(ctx context.Context, shopRow *shop.Shop, messageIDs []uuid.UUID) AutoReplyProcessSummary {
	summary := AutoReplyProcessSummary{}
	if s == nil || s.DB == nil || shopRow == nil || len(messageIDs) == 0 {
		return summary
	}
	setting, err := s.LoadAutoReplySetting(ctx, shopRow.TenantID)
	if err != nil {
		summary.Failed = len(messageIDs)
		return summary
	}
	summary.GlobalEnabled = setting.AutoReplyEnabled
	policy, err := s.loadAutoReplyPolicy(ctx, shopRow)
	if err != nil {
		summary.Failed = len(messageIDs)
		return summary
	}
	summary.PolicyEnabled = policy.Enabled
	if !setting.MessageSyncEnabled || !setting.AutoReplyEnabled || !policy.Enabled {
		summary.Skipped = len(messageIDs)
		return summary
	}
	for _, messageID := range messageIDs {
		var msg CustomerMessage
		if err := s.DB.WithContext(ctx).First(&msg, "id = ?", messageID).Error; err != nil {
			summary.Failed++
			continue
		}
		_, acquired, err := s.beginAutoReplyRun(ctx, shopRow, &msg)
		if err != nil {
			summary.Failed++
			continue
		}
		if !acquired {
			summary.Skipped++
			continue
		}
		if s.Redis == nil || s.Redis.Client == nil {
			summary.Failed++
			continue
		}
		if err := s.enqueueAutoReplyWakeup(ctx, shopRow.ID, messageID); err != nil {
			summary.Failed++
			continue
		}
		summary.Candidates++
	}
	return summary
}

func backgroundGinContext(ctx context.Context, requestID string) *gin.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://internal/customer/auto-reply", nil)
	c := &gin.Context{Request: req}
	c.Set("requestId", requestID)
	return c
}

func (s *Service) beginAutoReplyRun(ctx context.Context, shopRow *shop.Shop, msg *CustomerMessage) (*CustomerAutoReplyRun, bool, error) {
	if shopRow == nil || msg == nil {
		return nil, false, fmt.Errorf("invalid auto reply candidate")
	}
	var existing CustomerAutoReplyRun
	err := s.DB.WithContext(ctx).Where("message_id = ?", msg.ID).First(&existing).Error
	if err == nil {
		return &existing, false, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, false, err
	}
	now := time.Now().UTC()
	run := &CustomerAutoReplyRun{
		TenantID:       shopRow.TenantID,
		ShopID:         shopRow.ID,
		ConversationID: msg.ConversationID,
		MessageID:      msg.ID,
		Status:         AutoReplyRunPending,
		StartedAt:      &now,
	}
	if err := s.DB.WithContext(ctx).Create(run).Error; err != nil {
		if findErr := s.DB.WithContext(ctx).Where("message_id = ?", msg.ID).First(&existing).Error; findErr == nil {
			return &existing, false, nil
		}
		return nil, false, err
	}
	return run, true, nil
}

func (s *Service) autoReplySentLastHour(ctx context.Context, shopID uuid.UUID) (int64, error) {
	var total int64
	err := s.DB.WithContext(ctx).Model(&CustomerAutoReplyRun{}).
		Where("shop_id = ? AND status = ? AND finished_at >= ?", shopID, AutoReplyRunSent, time.Now().UTC().Add(-time.Hour)).
		Count(&total).Error
	return total, err
}

func (s *Service) reserveAutoReplySlot(ctx context.Context, shopID uuid.UUID, limit int) (bool, error) {
	if limit < 1 || s.Redis == nil || s.Redis.Client == nil {
		return false, fmt.Errorf("auto reply rate limiter unavailable")
	}
	key := fmt.Sprintf("customer:auto:reply:rate:%s:%s", shopID.String(), time.Now().UTC().Format("2006010215"))
	floor, err := s.autoReplySentLastHour(ctx, shopID)
	if err != nil {
		return false, fmt.Errorf("load auto reply rate floor: %w", err)
	}
	const script = `
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
local floor = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
if current < floor then current = floor end
if current >= limit then return 0 end
current = current + 1
redis.call('SET', KEYS[1], current, 'EX', 7200)
return 1`
	reserved, err := s.Redis.Eval(ctx, script, []string{key}, floor, limit).Int64()
	return reserved == 1, err
}

func autoReplyOutputRequiresHuman(reply string) bool {
	text := strings.ToLower(strings.TrimSpace(reply))
	if text == "" {
		return true
	}
	blocked := []string{
		"退款", "退钱", "赔偿", "赔付", "补偿", "返现", "保证到账", "承诺退款",
		"refund", "reimburse", "compensation", "cashback", "chargeback",
	}
	for _, term := range blocked {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func (s *Service) logAutoReplyDecision(ctx context.Context, shopRow *shop.Shop, run *CustomerAutoReplyRun, status, reason string) {
	if s.OpLog == nil || shopRow == nil || run == nil {
		return
	}
	_ = s.OpLog.WriteBackground(ctx, operationlog.WriteOpts{
		TenantID:   shopRow.TenantID,
		ShopID:     &shopRow.ID,
		Platform:   shopRow.Platform,
		Action:     "customer.auto_reply." + status,
		Resource:   "customer_auto_reply_run",
		ResourceID: run.ID.String(),
		Status:     status,
		Message:    fmt.Sprintf("conversationId=%s messageId=%s reason=%s", run.ConversationID, run.MessageID, reason),
	})
}

func (s *Service) autoReplyConversationStillEligible(ctx context.Context, convID, messageID uuid.UUID) (string, error) {
	var conv CustomerConversation
	if err := s.DB.WithContext(ctx).Select("id, status").First(&conv, "id = ?", convID).Error; err != nil {
		return "reply_state_unavailable", err
	}
	if conv.Status != StatusOpen && conv.Status != StatusPendingReply {
		return "conversation_not_replyable", nil
	}
	var msg CustomerMessage
	if err := s.DB.WithContext(ctx).First(&msg, "id = ? AND conversation_id = ?", messageID, convID).Error; err != nil {
		return "reply_state_unavailable", err
	}
	var latestCustomerMessage CustomerMessage
	if err := s.DB.WithContext(ctx).
		Where("conversation_id = ? AND role = ?", convID, RoleCustomer).
		Order("created_at DESC, id DESC").First(&latestCustomerMessage).Error; err != nil {
		return "reply_state_unavailable", err
	}
	if latestCustomerMessage.ID != messageID {
		return "superseded_by_newer_message", nil
	}
	var laterAgentMessages int64
	if err := s.DB.WithContext(ctx).Model(&CustomerMessage{}).
		Where("conversation_id = ? AND role = ? AND created_at >= ?", convID, RoleAgent, msg.CreatedAt).
		Count(&laterAgentMessages).Error; err != nil {
		return "reply_state_unavailable", err
	}
	if laterAgentMessages > 0 {
		return "already_replied", nil
	}
	return "", nil
}

func (s *Service) processOneAutoReply(ctx context.Context, shopRow *shop.Shop, policy CustomerAutoReplyPolicy, messageID uuid.UUID, workerID string) (string, error) {
	var msg CustomerMessage
	if err := s.DB.WithContext(ctx).First(&msg, "id = ?", messageID).Error; err != nil {
		return AutoReplyRunFailed, err
	}
	var run CustomerAutoReplyRun
	if err := s.DB.WithContext(ctx).Where("message_id = ? AND shop_id = ?", messageID, shopRow.ID).First(&run).Error; err != nil {
		return AutoReplyRunFailed, err
	}
	claimedRun, lease, claimed, err := s.claimAutoReplyRun(ctx, run.ID, workerID)
	if err != nil {
		return AutoReplyRunFailed, err
	}
	if !claimed {
		return AutoReplyRunSkipped, nil
	}
	run = *claimedRun
	stopRenewal := s.startAutoReplyLeaseRenewal(ctx, run.ID, lease)
	defer stopRenewal()

	var conv CustomerConversation
	if err := s.DB.WithContext(ctx).First(&conv, "id = ? AND shop_id = ?", msg.ConversationID, shopRow.ID).Error; err != nil {
		if finishErr := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunFailed, "", "conversation_unavailable", err.Error(), nil, nil); finishErr != nil {
			return AutoReplyRunFailed, errors.Join(err, finishErr)
		}
		return AutoReplyRunFailed, err
	}
	if reason, stateErr := s.autoReplyConversationStillEligible(ctx, conv.ID, msg.ID); reason != "" {
		status := AutoReplyRunSkipped
		if stateErr != nil {
			status = AutoReplyRunFailed
		}
		if finishErr := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, status, "", reason, errorString(stateErr), nil, nil); finishErr != nil {
			return AutoReplyRunFailed, finishErr
		}
		return status, stateErr
	}
	if policy.RequireOrderContext && conv.OrderID == nil {
		if err := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunHumanRequired, "", "order_context_required", "", nil, nil); err != nil {
			return AutoReplyRunFailed, err
		}
		s.logAutoReplyDecision(ctx, shopRow, &run, AutoReplyRunHumanRequired, "order_context_required")
		return AutoReplyRunHumanRequired, nil
	}
	c := backgroundGinContext(ctx, "customer-auto-reply:"+messageID.String())
	c.Set(ctxkey.TenantID, shopRow.TenantID)
	generated, err := s.GenerateReply(c, conv.ID, GenerateReplyBody{
		MessageID:  msg.ID.String(),
		Language:   msg.Language,
		Tone:       policy.Tone,
		Platform:   conv.Platform,
		ShopPolicy: policy.ShopPolicy,
	}, nil)
	if err != nil {
		if finishErr := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunFailed, "", "ai_generation_failed", err.Error(), nil, nil); finishErr != nil {
			return AutoReplyRunFailed, errors.Join(err, finishErr)
		}
		s.logAutoReplyDecision(ctx, shopRow, &run, AutoReplyRunFailed, "ai_generation_failed")
		return AutoReplyRunFailed, err
	}
	suggestionID := generated.SuggestionID
	risk := strings.ToLower(strings.TrimSpace(generated.RiskLevel))
	if policy.LowRiskOnly && risk != "low" {
		if err := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunHumanRequired, risk, "risk_requires_human", "", &suggestionID, nil); err != nil {
			return AutoReplyRunFailed, err
		}
		s.logAutoReplyDecision(ctx, shopRow, &run, AutoReplyRunHumanRequired, "risk_requires_human")
		return AutoReplyRunHumanRequired, nil
	}
	reply := strings.TrimSpace(generated.Reply)
	if reply == "" || utf8.RuneCountInString(reply) > policy.MaxReplyRunes {
		if err := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunHumanRequired, risk, "reply_length_guard", "", &suggestionID, nil); err != nil {
			return AutoReplyRunFailed, err
		}
		s.logAutoReplyDecision(ctx, shopRow, &run, AutoReplyRunHumanRequired, "reply_length_guard")
		return AutoReplyRunHumanRequired, nil
	}
	if autoReplyOutputRequiresHuman(reply) {
		if err := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunHumanRequired, risk, "reply_content_guard", "", &suggestionID, nil); err != nil {
			return AutoReplyRunFailed, err
		}
		s.logAutoReplyDecision(ctx, shopRow, &run, AutoReplyRunHumanRequired, "reply_content_guard")
		return AutoReplyRunHumanRequired, nil
	}
	currentPolicy, policyErr := s.loadAutoReplyPolicy(ctx, shopRow)
	currentSetting, settingErr := s.LoadAutoReplySetting(ctx, shopRow.TenantID)
	if policyErr != nil || settingErr != nil || !currentSetting.MessageSyncEnabled || !currentSetting.AutoReplyEnabled || !currentPolicy.Enabled {
		if err := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunSkipped, risk, "policy_disabled_before_send", "", &suggestionID, nil); err != nil {
			return AutoReplyRunFailed, err
		}
		if policyErr != nil {
			return AutoReplyRunSkipped, policyErr
		}
		return AutoReplyRunSkipped, settingErr
	}
	var sent *CustomerMessage
	err = s.withConversationMutationLock(ctx, conv.ID, func() error {
		reason, stateErr := s.autoReplyConversationStillEligible(ctx, conv.ID, msg.ID)
		if stateErr != nil {
			return stateErr
		}
		if reason != "" {
			return &autoReplyNoLongerEligibleError{Reason: reason}
		}
		reserved, reserveErr := s.reserveAutoReplySlot(ctx, shopRow.ID, currentPolicy.MaxRepliesPerHour)
		if reserveErr != nil {
			return &autoReplyRateLimitError{Err: reserveErr}
		}
		if !reserved {
			return &autoReplyRateLimitError{LimitReached: true}
		}
		if err := s.transitionAutoReplyToSending(ctx, run.ID, lease); err != nil {
			return err
		}
		var sendErr error
		sent, sendErr = s.sendPlatformMessageUnlocked(c, conv.ID, SendPlatformMessageBody{
			Reply: reply, SuggestionID: suggestionID.String(), ClientMessageID: "auto:" + messageID.String(),
		}, nil)
		return sendErr
	})
	if err != nil {
		var staleErr *autoReplyNoLongerEligibleError
		if errors.As(err, &staleErr) {
			if finishErr := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunSkipped, risk, staleErr.Reason, "", &suggestionID, nil); finishErr != nil {
				return AutoReplyRunFailed, finishErr
			}
			return AutoReplyRunSkipped, nil
		}
		var rateErr *autoReplyRateLimitError
		if errors.As(err, &rateErr) {
			status := AutoReplyRunFailed
			reason := "rate_limit_unavailable"
			if rateErr.LimitReached {
				status = AutoReplyRunHumanRequired
				reason = "hourly_limit_reached"
			}
			if finishErr := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, status, risk, reason, errorString(rateErr.Err), &suggestionID, nil); finishErr != nil {
				return AutoReplyRunFailed, errors.Join(rateErr.Err, finishErr)
			}
			s.logAutoReplyDecision(ctx, shopRow, &run, status, reason)
			return status, rateErr.Err
		}
		status := AutoReplyRunFailed
		reason := "platform_send_failed"
		var platformErr *PlatformSendError
		if errors.As(err, &platformErr) && platformErr.ManualReviewRequired {
			status = AutoReplyRunHumanRequired
			reason = "platform_send_result_unknown"
		}
		if finishErr := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, status, risk, reason, err.Error(), &suggestionID, nil); finishErr != nil {
			return AutoReplyRunFailed, errors.Join(err, finishErr)
		}
		s.logAutoReplyDecision(ctx, shopRow, &run, status, reason)
		return status, err
	}
	sentID := sent.ID
	if err := s.finishClaimedAutoReplyRun(ctx, run.ID, lease, AutoReplyRunSent, risk, "low_risk_auto_sent", "", &suggestionID, &sentID); err != nil {
		return AutoReplyRunFailed, err
	}
	s.logAutoReplyDecision(ctx, shopRow, &run, AutoReplyRunSent, "low_risk_auto_sent")
	return AutoReplyRunSent, nil
}

func (s *Service) ProcessQueuedAutoReply(ctx context.Context, shopID, messageID uuid.UUID, workerID string) (string, error) {
	shopRow, err := s.loadShopForAutoReply(ctx, shopID)
	if err != nil {
		return AutoReplyRunFailed, err
	}
	policy, err := s.loadAutoReplyPolicy(ctx, shopRow)
	if err != nil {
		return AutoReplyRunFailed, err
	}
	setting, settingErr := s.LoadAutoReplySetting(ctx, shopRow.TenantID)
	if settingErr != nil {
		return AutoReplyRunFailed, settingErr
	}
	if !setting.MessageSyncEnabled || !setting.AutoReplyEnabled || !policy.Enabled {
		var run CustomerAutoReplyRun
		if err := s.DB.WithContext(ctx).Where("message_id = ?", messageID).First(&run).Error; err == nil {
			if finishErr := s.finishPendingAutoReplyRun(ctx, run.ID, AutoReplyRunSkipped, "policy_disabled", ""); finishErr != nil {
				return AutoReplyRunFailed, finishErr
			}
		}
		return AutoReplyRunSkipped, nil
	}
	return s.processOneAutoReply(ctx, shopRow, policy, messageID, workerID)
}

type autoReplyNoLongerEligibleError struct{ Reason string }

func (e *autoReplyNoLongerEligibleError) Error() string { return e.Reason }

type autoReplyRateLimitError struct {
	Err          error
	LimitReached bool
}

func (e *autoReplyRateLimitError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "auto reply hourly limit reached"
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
