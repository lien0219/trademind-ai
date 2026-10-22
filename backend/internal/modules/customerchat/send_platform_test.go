package customerchat

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/idempotency"
	"github.com/trademind-ai/trademind/backend/internal/modules/shop"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	platformp "github.com/trademind-ai/trademind/backend/internal/providers/platform"
	"gorm.io/gorm"
)

func openCustomerSendTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:customer_send_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CustomerMessage{}))
	return db
}

func TestCustomerSendLeaseOutlivesPlatformTimeout(t *testing.T) {
	require.Greater(t, customerSendLease, 120*time.Second)
	require.Less(t, customerSendHeartbeatInterval, customerSendLease/2)
}

func TestLoadSentMessageReplaysSameContentAndRejectsConflict(t *testing.T) {
	db := openCustomerSendTestDB(t)
	svc := &Service{DB: db}
	conversationID := uuid.New()
	msg := CustomerMessage{
		ConversationID: conversationID, ClientMessageID: "auto:message-1", Role: RoleAgent,
		Content: "Already sent", Language: "en", MessageType: MessageTypeText, Source: SourcePlatform,
	}
	require.NoError(t, db.Create(&msg).Error)

	loaded, err := svc.loadSentMessage(context.Background(), conversationID, msg.ClientMessageID, "")
	require.NoError(t, err)
	require.Equal(t, msg.ID, loaded.ID)
	require.NoError(t, validateCustomerSendReplay(loaded, " Already sent "))
	require.ErrorIs(t, validateCustomerSendReplay(loaded, "Different reply"), idempotency.ErrKeyConflict)
}

func TestSendPlatformMessageDoesNotRetryAfterLocalMessagePersistedWhenIdempotencyCompleteFails(t *testing.T) {
	db := openCustomerSendTestDB(t)
	require.NoError(t, db.AutoMigrate(&CustomerConversation{}, &shop.Shop{}, &shop.ShopAuthToken{}, &idempotency.Record{}))
	platformp.Bootstrap()

	shopRow := shop.Shop{
		Base: model.Base{ID: uuid.New()}, TenantID: 7, Platform: "mock", ShopName: "Mock shop",
		Status: shop.StatusActive, AuthStatus: shop.AuthAuthorized,
	}
	require.NoError(t, db.Create(&shopRow).Error)
	externalConversationID := "mock-conversation"
	conversation := CustomerConversation{
		Base: model.Base{ID: uuid.New()}, TenantID: 7, Platform: "mock", ShopID: &shopRow.ID,
		ExternalConversationID: &externalConversationID, CustomerName: "buyer", CustomerLanguage: "en", Status: StatusPendingReply,
	}
	require.NoError(t, db.Create(&conversation).Error)

	const callbackName = "test:fail-customer-send-complete"
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table != (idempotency.Record{}).TableName() {
			return
		}
		updates, ok := tx.Statement.Dest.(map[string]any)
		if ok && updates["status"] == idempotency.StatusSucceeded {
			tx.AddError(errors.New("injected idempotency complete failure"))
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Update().Remove(callbackName) })

	svc := &Service{
		DB: db, Shops: &shop.Service{DB: db}, Idempotency: &idempotency.Service{DB: db},
	}
	newContext := func() *gin.Context {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/customer/send", nil)
		require.NoError(t, err)
		c := &gin.Context{Request: req}
		c.Set("requestId", uuid.NewString())
		c.Set(ctxkey.TenantID, int64(7))
		return c
	}
	body := SendPlatformMessageBody{Reply: "Your order is on the way.", ClientMessageID: "auto:source-message"}

	first, err := svc.SendPlatformMessage(newContext(), conversation.ID, body, nil)
	require.NoError(t, err)
	require.NotNil(t, first)
	second, err := svc.SendPlatformMessage(newContext(), conversation.ID, body, nil)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	var total int64
	require.NoError(t, db.Model(&CustomerMessage{}).Where("conversation_id = ? AND client_message_id = ?", conversation.ID, body.ClientMessageID).Count(&total).Error)
	require.Equal(t, int64(1), total)
}
