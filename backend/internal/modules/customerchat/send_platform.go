package customerchat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SendPlatformMessageBody POST /customer/conversations/:id/send-platform-message
type SendPlatformMessageBody struct {
	Reply            string `json:"reply"`
	SuggestionID     string `json:"suggestionId"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

// SendPlatformMessage delivers a human-approved reply via the platform Provider.
func (s *Service) SendPlatformMessage(c *gin.Context, conversationID uuid.UUID, body SendPlatformMessageBody, adminID *uuid.UUID) (*CustomerMessage, error) {
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

	var conv CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&conv, "id = ?", conversationID).Error; err != nil {
		return nil, err
	}
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

	prov := platformp.Get(strings.TrimSpace(shopRow.Platform))
	if prov == nil {
		return nil, fmt.Errorf("unknown platform")
	}
	cm, ok := platformp.AsCustomerMessage(prov)
	if !ok {
		return nil, fmt.Errorf("platform does not implement customer messaging")
	}
	st := platformp.CustomerMessageImplementationStatus(prov)
	if st == platformp.StatusPlanned || st == platformp.StatusDisabled {
		return nil, platformp.ErrCustomerMessageNotImplemented
	}
	if err := s.ensurePlatformPartnerConfig(c.Request.Context(), prov); err != nil {
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
		IdempotencyKey:         strings.TrimSpace(body.IdempotencyKey),
	})
	if err != nil {
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
		return nil, err
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
