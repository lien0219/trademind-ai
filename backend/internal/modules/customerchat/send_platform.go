package customerchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/taskretry"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	customerSendLease             = 3 * time.Minute
	customerSendHeartbeatInterval = 30 * time.Second
)

// SendPlatformMessageBody POST /customer/conversations/:id/send-platform-message
type SendPlatformMessageBody struct {
	Reply           string `json:"reply"`
	SuggestionID    string `json:"suggestionId"`
	IdempotencyKey  string `json:"idempotencyKey"`
	ClientMessageID string `json:"clientMessageId"`
}

type customerSendHashPayload struct {
	ConversationID  string `json:"conversationId"`
	ClientMessageID string `json:"clientMessageId"`
	Reply           string `json:"reply"`
}

func customerSendRequestHash(conversationID uuid.UUID, clientMessageID, reply string) string {
	payload := customerSendHashPayload{
		ConversationID:  conversationID.String(),
		ClientMessageID: clientMessageID,
		Reply:           reply,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return idempotency.HashRequest([]byte(fmt.Sprintf("%s|%s|%s", conversationID.String(), clientMessageID, reply)))
	}
	return idempotency.HashRequest(b)
}

func isUnknownPlatformSendResult(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	cl := taskretry.Classify(err, 0)
	switch cl.Code {
	case taskretry.CodeTimeout, taskretry.CodeNetworkError:
		return true
	default:
		return false
	}
}

func isPermanentSendFailure(err error) bool {
	if err == nil {
		return false
	}
	cl := taskretry.Classify(err, 0)
	switch cl.Code {
	case taskretry.CodePermissionDenied,
		taskretry.CodeInvalidRequest,
		taskretry.CodeValidationFailed,
		taskretry.CodeResourceNotFound,
		taskretry.CodeUnsupportedOperation,
		taskretry.CodeBusinessRuleRejected,
		taskretry.CodeIdempotencyConflict:
		return true
	default:
		return false
	}
}

func (s *Service) loadSentMessage(ctx context.Context, conversationID uuid.UUID, clientMessageID string, resourceID string) (*CustomerMessage, error) {
	if rid := strings.TrimSpace(resourceID); rid != "" {
		if mid, err := uuid.Parse(rid); err == nil {
			var msg CustomerMessage
			if err := s.DB.WithContext(ctx).First(&msg, "id = ? AND conversation_id = ?", mid, conversationID).Error; err == nil {
				return &msg, nil
			}
		}
	}
	var existing CustomerMessage
	if err := s.DB.WithContext(ctx).
		Where("conversation_id = ? AND client_message_id = ?", conversationID, clientMessageID).
		First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func validateCustomerSendReplay(msg *CustomerMessage, reply string) error {
	if msg == nil {
		return gorm.ErrRecordNotFound
	}
	if strings.TrimSpace(msg.Content) != strings.TrimSpace(reply) {
		return idempotency.ErrKeyConflict
	}
	return nil
}

func (s *Service) startCustomerSendIdempotencyHeartbeat(ctx context.Context, recordID uuid.UUID, owner string) func() {
	if s == nil || s.Idempotency == nil || recordID == uuid.Nil {
		return func() {}
	}
	runCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(customerSendHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				hbCtx, hbCancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := s.Idempotency.Heartbeat(hbCtx, recordID, owner, customerSendLease); err != nil {
					slog.Warn("customer_message_send_idempotency_heartbeat_failed", "recordId", recordID.String(), "error", err)
				}
				hbCancel()
			}
		}
	}()
	return cancel
}

// SendPlatformMessage delivers a human-approved reply via the platform Provider.
func (s *Service) SendPlatformMessage(c *gin.Context, conversationID uuid.UUID, body SendPlatformMessageBody, adminID *uuid.UUID) (*CustomerMessage, error) {
	var out *CustomerMessage
	err := s.withConversationMutationLock(c.Request.Context(), conversationID, func() error {
		var err error
		out, err = s.sendPlatformMessageUnlocked(c, conversationID, body, adminID)
		return err
	})
	return out, err
}

func (s *Service) sendPlatformMessageUnlocked(c *gin.Context, conversationID uuid.UUID, body SendPlatformMessageBody, adminID *uuid.UUID) (*CustomerMessage, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	if s.Shops == nil {
		return nil, fmt.Errorf("shop service unavailable")
	}
	reply := strings.TrimSpace(body.Reply)
	if reply == "" {
		return nil, fmt.Errorf("reply is required")
	}

	clientMsgID := strings.TrimSpace(body.ClientMessageID)
	if clientMsgID == "" {
		return nil, fmt.Errorf("clientMessageId is required")
	}
	if s.Idempotency == nil {
		return nil, fmt.Errorf("customer message idempotency unavailable")
	}
	loaded, err := s.authorizedConversation(c, conversationID, true)
	if err != nil {
		return nil, err
	}
	conv := *loaded
	if conv.ShopID == nil {
		return nil, fmt.Errorf("conversation has no shop")
	}
	if conv.ExternalConversationID == nil || strings.TrimSpace(*conv.ExternalConversationID) == "" {
		return nil, fmt.Errorf("conversation has no platform external id")
	}

	shopRow, auth, err := s.Shops.PlainAuthForProvider(c, *conv.ShopID)
	if err != nil {
		return nil, err
	}
	if err := ensureShopCustomerMessageAuth(shopRow, auth); err != nil {
		return nil, err
	}
	if existing, err := s.loadSentMessage(c.Request.Context(), conversationID, clientMsgID, ""); err == nil {
		if err := validateCustomerSendReplay(existing, reply); err != nil {
			return nil, err
		}
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	owner := idempotency.OwnerFromRequest(c.GetString("requestId"), "customer-send") + ":" + uuid.NewString()
	var acquiredRecordID uuid.UUID
	var acquired bool

	if s.Idempotency != nil {
		key := idempotency.CustomerSend(conversationID.String(), clientMsgID)
		reqHash := customerSendRequestHash(conversationID, clientMsgID, reply)
		res, acqErr := s.Idempotency.Acquire(c.Request.Context(), idempotency.ScopeCustomerSend, key, reqHash, owner, customerSendLease)
		decision, rec, classifyErr := idempotency.Classify(res, acqErr)
		switch decision {
		case idempotency.DecisionAlreadySucceeded:
			rid := ""
			if res != nil {
				rid = res.ResourceID
			}
			if rid == "" && rec != nil {
				rid = rec.ResourceID
			}
			msg, err := s.loadSentMessage(c.Request.Context(), conversationID, clientMsgID, rid)
			if err != nil {
				return nil, &PlatformSendError{
					Code:                 ErrCodeCustomerMessageInProgress,
					Message:              "message replay unavailable",
					ManualReviewRequired: false,
					SafeRetry:            true,
				}
			}
			return msg, nil
		case idempotency.DecisionInProgress:
			return nil, &PlatformSendError{
				Code:                 ErrCodeCustomerMessageInProgress,
				Message:              "send already in progress",
				ManualReviewRequired: false,
				SafeRetry:            true,
			}
		case idempotency.DecisionKeyConflict, idempotency.DecisionPermanentFailure:
			if classifyErr != nil {
				return nil, classifyErr
			}
			return nil, idempotency.ErrKeyConflict
		case idempotency.DecisionAcquired, idempotency.DecisionRetryAllowed:
			if res == nil || res.Record == nil {
				return nil, fmt.Errorf("idempotency: missing record")
			}
			acquiredRecordID = res.Record.ID
			acquired = true
		default:
			if classifyErr != nil {
				return nil, classifyErr
			}
			return nil, acqErr
		}
	}
	stopIdempotencyHeartbeat := s.startCustomerSendIdempotencyHeartbeat(c.Request.Context(), acquiredRecordID, owner)
	defer stopIdempotencyHeartbeat()

	failIdempotency := func(code string, retryable bool) {
		if !acquired || s.Idempotency == nil {
			return
		}
		_ = s.Idempotency.Fail(c.Request.Context(), acquiredRecordID, owner, code, retryable)
	}

	prov := platformp.Get(strings.TrimSpace(shopRow.Platform))
	if prov == nil {
		failIdempotency("unknown platform", false)
		return nil, fmt.Errorf("unknown platform")
	}
	cm, ok := platformp.AsCustomerMessage(prov)
	if !ok {
		failIdempotency("platform does not implement customer messaging", false)
		return nil, fmt.Errorf("platform does not implement customer messaging")
	}
	st := platformp.CustomerMessageImplementationStatus(prov)
	if st == platformp.StatusPlanned || st == platformp.StatusDisabled {
		failIdempotency(platformp.ErrCustomerMessageNotImplemented.Error(), false)
		return nil, platformp.ErrCustomerMessageNotImplemented
	}
	if err := s.ensurePlatformPartnerConfig(c.Request.Context(), prov); err != nil {
		failIdempotency(err.Error(), false)
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	extConv := strings.TrimSpace(*conv.ExternalConversationID)
	res, err := cm.SendMessage(runCtx, platformp.SendMessageRequest{
		ShopID:                 *conv.ShopID,
		Platform:               strings.TrimSpace(shopRow.Platform),
		Auth:                   auth,
		ExternalConversationID: extConv,
		ConversationID:         conv.ID,
		Reply:                  reply,
		MessageType:            MessageTypeText,
		IdempotencyKey:         clientMsgID,
	})
	if err != nil {
		if isUnknownPlatformSendResult(err) {
			failIdempotency(ErrCodeCustomerMessageUnknownResult, false)
			cat := FailureCategoryReplySendFailed
			_ = s.recordFailure(c.Request.Context(), CustomerFailureEvent{
				ConversationID: conv.ID,
				Platform:       strings.TrimSpace(conv.Platform),
				ShopID:         conv.ShopID,
				Category:       cat,
				ErrorMessage:   ErrCodeCustomerMessageUnknownResult,
				Status:         FailureEventStatusOpen,
			})
			if s.OpLog != nil {
				_ = s.OpLog.Write(c, operationlog.WriteOpts{
					AdminUserID: adminID,
					Action:      "customer.platform_message.send.unknown",
					Resource:    "customer_conversation",
					ResourceID:  conv.ID.String(),
					Status:      "failed",
					Message:     fmt.Sprintf("conversationId=%s shopId=%s code=%s", conv.ID.String(), conv.ShopID.String(), ErrCodeCustomerMessageUnknownResult),
				})
			}
			return nil, &PlatformSendError{
				Code:                 ErrCodeCustomerMessageUnknownResult,
				Message:              "platform send result unknown; manual review required",
				ManualReviewRequired: true,
				SafeRetry:            false,
			}
		}

		retryable := !isPermanentSendFailure(err)
		failIdempotency(err.Error(), retryable)
		cat := classifySendFailure(err)
		var sugID *uuid.UUID
		if sid := strings.TrimSpace(body.SuggestionID); sid != "" {
			if u, perr := uuid.Parse(sid); perr == nil {
				sugID = &u
				_ = s.DB.WithContext(c.Request.Context()).Model(&CustomerReplySuggestion{}).
					Where("id = ? AND conversation_id = ?", u, conv.ID).
					Updates(map[string]any{"status": SuggestionSendFailed, "updated_at": time.Now().UTC()}).Error
			}
		}
		_ = s.recordFailure(c.Request.Context(), CustomerFailureEvent{
			ConversationID: conv.ID,
			SuggestionID:   sugID,
			Platform:       strings.TrimSpace(conv.Platform),
			ShopID:         conv.ShopID,
			Category:       cat,
			ErrorMessage:   err.Error(),
			Status:         FailureEventStatusOpen,
		})
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "customer.platform_message.send.failed",
				Resource:    "customer_conversation",
				ResourceID:  conv.ID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("conversationId=%s shopId=%s err=%s", conv.ID.String(), conv.ShopID.String(), err.Error()),
			})
		}
		return nil, err
	}

	s.resolveFailures(c.Request.Context(), conv.ID, FailureCategoryReplySendFailed, nil)
	s.resolveFailures(c.Request.Context(), conv.ID, FailureCategoryReplyPermissionDenied, nil)

	extMid := strings.TrimSpace(res.ExternalMessageID)
	var extPtr *string
	if extMid != "" {
		extPtr = &extMid
	}
	rawOut := platformp.TrimRawMap(res.RawSummary, 12, 400)
	rb, _ := json.Marshal(rawOut)

	now := time.Now().UTC()
	if res.SentAt != nil {
		now = *res.SentAt
	}

	var outMsg *CustomerMessage
	if err := s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		msg := &CustomerMessage{
			ConversationID:    conv.ID,
			ClientMessageID:   clientMsgID,
			Role:              RoleAgent,
			Content:           reply,
			Language:          conv.CustomerLanguage,
			MessageType:       MessageTypeText,
			Source:            SourcePlatform,
			ExternalMessageID: extPtr,
			RawData:           datatypes.JSON(rb),
			CreatedBy:         adminID,
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		outMsg = msg
		if err := tx.Model(&CustomerConversation{}).Where("id = ?", conv.ID).Updates(map[string]any{
			"status":          StatusReplied,
			"last_message_at": &now,
			"updated_at":      time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		if sid := strings.TrimSpace(body.SuggestionID); sid != "" {
			sugID, perr := uuid.Parse(sid)
			if perr == nil {
				_ = tx.Model(&CustomerReplySuggestion{}).Where("id = ? AND conversation_id = ?", sugID, conv.ID).
					Updates(map[string]any{
						"edited_reply": reply,
						"status":       SuggestionAccepted,
						"updated_at":   time.Now().UTC(),
					}).Error
			}
		}
		return nil
	}); err != nil {
		failIdempotency(ErrCodeCustomerMessageUnknownResult, false)
		_ = s.recordFailure(c.Request.Context(), CustomerFailureEvent{
			ConversationID: conv.ID,
			Platform:       strings.TrimSpace(conv.Platform),
			ShopID:         conv.ShopID,
			Category:       FailureCategoryReplySendFailed,
			ErrorMessage:   ErrCodeCustomerMessageUnknownResult,
			Status:         FailureEventStatusOpen,
		})
		return nil, &PlatformSendError{
			Code:                 ErrCodeCustomerMessageUnknownResult,
			Message:              "platform may have accepted message but local persist failed; manual review required",
			ManualReviewRequired: true,
			SafeRetry:            false,
		}
	}

	if acquired && s.Idempotency != nil {
		summary, _ := json.Marshal(map[string]string{
			"messageId":       outMsg.ID.String(),
			"clientMessageId": clientMsgID,
		})
		if err := s.Idempotency.Complete(c.Request.Context(), acquiredRecordID, owner, idempotency.CompleteResult{
			ResponseCode:    "CUSTOMER_MESSAGE_SENT",
			ResponseSummary: string(summary),
			ResourceType:    "customer_message",
			ResourceID:      outMsg.ID.String(),
		}); err != nil {
			// The platform call and local message transaction already succeeded. Returning
			// the persisted message prevents callers from retrying an external side effect;
			// subsequent requests replay it by clientMessageId before acquiring a lease.
			slog.Error("customer_message_send_idempotency_complete_failed", "conversationId", conv.ID.String(), "messageId", outMsg.ID.String(), "error", err)
		}
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.platform_message.send.success",
			Resource:    "customer_conversation",
			ResourceID:  conv.ID.String(),
			Status:      "success",
			Message: fmt.Sprintf("conversationId=%s shopId=%s messageId=%s replyLen=%d",
				conv.ID.String(), conv.ShopID.String(), outMsg.ID.String(), utf8.RuneCountInString(reply)),
		})
	}
	return outMsg, nil
}

func ensureShopCustomerMessageAuth(shopRow *shop.Shop, auth platformp.TestConnectionRequest) error {
	if shopRow == nil {
		return fmt.Errorf("shop not found")
	}
	if strings.TrimSpace(shopRow.Status) != shop.StatusActive {
		return fmt.Errorf("shop is not active")
	}
	if strings.TrimSpace(shopRow.AuthStatus) != shop.AuthAuthorized {
		return fmt.Errorf("shop is not authorized")
	}
	p := strings.TrimSpace(strings.ToLower(shopRow.Platform))
	if p == "mock" {
		return nil
	}
	if strings.TrimSpace(auth.AccessToken) == "" && strings.TrimSpace(auth.RefreshToken) == "" {
		return fmt.Errorf("shop is not authorized")
	}
	return nil
}
