package customerchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SyncPlatformCustomerMessages upserts rows from a normalized provider pull result.
func (s *Service) SyncPlatformCustomerMessages(ctx context.Context, shopRow *shop.Shop, pull *platformp.PullMessagesResult) (conversationsTouched int, messagesInserted int, err error) {
	if s == nil || s.DB == nil {
		return 0, 0, fmt.Errorf("customerchat: no db")
	}
	if shopRow == nil || pull == nil {
		return 0, 0, fmt.Errorf("invalid sync payload")
	}
	shopID := shopRow.ID
	platformKey := strings.TrimSpace(shopRow.Platform)

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range pull.Conversations {
			pc := &pull.Conversations[i]
			ext := strings.TrimSpace(pc.ExternalConversationID)
			if ext == "" {
				continue
			}

			var conv CustomerConversation
			q := tx.Where("shop_id = ? AND platform = ? AND external_conversation_id = ?", shopID, platformKey, ext)
			findErr := q.First(&conv).Error
			rawConv := platformp.TrimRawMap(pc.RawData, 12, 400)
			rawJSON, _ := json.Marshal(rawConv)

			custName := strings.TrimSpace(pc.CustomerName)
			if custName == "" {
				custName = "Customer"
			}
			lang := strings.TrimSpace(pc.CustomerLanguage)
			if lang == "" {
				lang = "en"
			}
			avatar := strings.TrimSpace(pc.CustomerAvatar)
			convStatus := strings.TrimSpace(pc.Status)
			if convStatus == "" {
				convStatus = StatusOpen
			}
			lastAt := pc.LastMessageAt

			if errors.Is(findErr, gorm.ErrRecordNotFound) {
				extCopy := ext
				conv = CustomerConversation{
					Platform:               platformKey,
					ShopID:                 &shopID,
					ExternalConversationID: &extCopy,
					CustomerName:           custName,
					CustomerAvatar:         avatar,
					CustomerLanguage:       lang,
					Status:                 convStatus,
					LastMessageAt:          lastAt,
					RawData:                datatypes.JSON(rawJSON),
				}
				if err := tx.Create(&conv).Error; err != nil {
					return err
				}
				conversationsTouched++
			} else if findErr != nil {
				return findErr
			} else {
				updates := map[string]any{
					"customer_name":     custName,
					"customer_avatar":   avatar,
					"customer_language": lang,
					"status":            convStatus,
					"raw_data":          datatypes.JSON(rawJSON),
					"updated_at":        time.Now().UTC(),
				}
				if lastAt != nil {
					updates["last_message_at"] = lastAt
				}
				if err := tx.Model(&CustomerConversation{}).Where("id = ?", conv.ID).Updates(updates).Error; err != nil {
					return err
				}
				conversationsTouched++
			}

			var latestRole string
			var latestAt *time.Time
			for j := range pc.Messages {
				pm := &pc.Messages[j]
				emid := strings.TrimSpace(pm.ExternalMessageID)
				if emid == "" {
					continue
				}
				var n int64
				if err := tx.Model(&CustomerMessage{}).
					Where("conversation_id = ? AND external_message_id = ?", conv.ID, emid).
					Count(&n).Error; err != nil {
					return err
				}
				if n > 0 {
					continue
				}
				role := normalizePlatformRole(pm.Role)
				mt := strings.TrimSpace(pm.MessageType)
				if mt == "" {
					mt = MessageTypeText
				}
				mlang := strings.TrimSpace(pm.Language)
				if mlang == "" {
					mlang = lang
				}
				content := strings.TrimSpace(pm.Content)
				emCopy := emid
				rawMsg := platformp.TrimRawMap(pm.RawData, 12, 400)
				rb, _ := json.Marshal(rawMsg)
				msg := &CustomerMessage{
					ConversationID:    conv.ID,
					Role:              role,
					Content:           content,
					Language:          mlang,
					MessageType:       mt,
					Source:            SourcePlatform,
					ExternalMessageID: &emCopy,
					RawData:           datatypes.JSON(rb),
				}
				if err := tx.Create(msg).Error; err != nil {
					return err
				}
				messagesInserted++
				latestRole = role
				if pm.CreatedAt != nil {
					t := *pm.CreatedAt
					latestAt = &t
				} else {
					t := time.Now().UTC()
					latestAt = &t
				}
			}

			if latestAt != nil {
				st := conv.Status
				switch latestRole {
				case RoleCustomer:
					st = StatusPendingReply
				case RoleAgent:
					st = StatusReplied
				}
				_ = tx.Model(&CustomerConversation{}).Where("id = ?", conv.ID).Updates(map[string]any{
					"last_message_at": latestAt,
					"status":          st,
					"updated_at":      time.Now().UTC(),
				}).Error
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return conversationsTouched, messagesInserted, nil
}

func normalizePlatformRole(r string) string {
	v := strings.TrimSpace(strings.ToLower(r))
	switch v {
	case "agent", "shop", "seller", "merchant", "operator":
		return RoleAgent
	case "system":
		return RoleAI
	case "customer", "buyer", "user", "":
		return RoleCustomer
	default:
		return RoleCustomer
	}
}
