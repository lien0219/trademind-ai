package customerchat

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/gorm"
)

func newCustomerScopeContext(tenantID int64) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/customer", nil)
	c.Set(ctxkey.TenantID, tenantID)
	return c
}

func TestCustomerScopeRejectsOtherTenantConversationAndMessages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:customer_scope_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CustomerConversation{}, &CustomerMessage{}, &CustomerReplySuggestion{}))
	svc := &Service{DB: db}
	conv := CustomerConversation{Base: model.Base{ID: uuid.New()}, TenantID: 7, Platform: "manual", CustomerName: "buyer", CustomerLanguage: "en", Status: StatusOpen}
	require.NoError(t, db.Create(&conv).Error)
	msg := CustomerMessage{ConversationID: conv.ID, Role: RoleCustomer, Content: "private", Language: "en", MessageType: MessageTypeText, Source: SourceManual}
	require.NoError(t, db.Create(&msg).Error)
	suggestion := CustomerReplySuggestion{ConversationID: conv.ID, SuggestedReply: "private", Status: SuggestionGenerated}
	require.NoError(t, db.Create(&suggestion).Error)

	outsider := newCustomerScopeContext(8)
	_, err = svc.authorizedConversation(outsider, conv.ID, false)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = svc.ListMessages(outsider, conv.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = svc.ListSuggestions(outsider, conv.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.ErrorIs(t, svc.UpdateSuggestion(outsider, suggestion.ID, UpdateSuggestionBody{EditedReply: "wrong"}, nil), gorm.ErrRecordNotFound)
	require.ErrorIs(t, svc.DeleteConversation(outsider, conv.ID, nil), gorm.ErrRecordNotFound)
	_, err = svc.CreateMessage(outsider, conv.ID, CreateMessageBody{Role: RoleCustomer, Content: "forged"}, nil)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	var persisted CustomerReplySuggestion
	require.NoError(t, db.First(&persisted, "id = ?", suggestion.ID).Error)
	require.Equal(t, SuggestionGenerated, persisted.Status)
	owner := newCustomerScopeContext(7)
	loaded, err := svc.authorizedConversation(owner, conv.ID, false)
	require.NoError(t, err)
	require.Equal(t, conv.ID, loaded.ID)
	messages, err := svc.ListMessages(owner, conv.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
}

func TestCustomerScopeRejectsUnscopedRequest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:customer_scope_missing_"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&CustomerConversation{}))
	svc := &Service{DB: db}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	_, err = svc.authorizedConversation(c, uuid.New(), false)
	require.Error(t, err)
}
