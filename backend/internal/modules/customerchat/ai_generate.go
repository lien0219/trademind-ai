package customerchat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/modules/aiprompt"
	"github.com/trademind-ai/trademind/backend/internal/modules/aitask"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/modules/order"
	aigate "github.com/trademind-ai/trademind/backend/internal/providers/ai"
)

// GenerateReplyBody POST .../ai/generate-reply
type GenerateReplyBody struct {
	MessageID  string `json:"messageId"`
	Language   string `json:"language"`
	Tone       string `json:"tone"`
	Platform   string `json:"platform"`
	ShopPolicy string `json:"shopPolicy"`
}

// GenerateReplyResult response
type GenerateReplyResult struct {
	SuggestionID uuid.UUID `json:"suggestionId"`
	Reply        string    `json:"reply"`
	Intent       string    `json:"intent"`
	Sentiment    string    `json:"sentiment"`
	RiskLevel    string    `json:"riskLevel"`
	Notes        string    `json:"notes"`
	TaskID       uuid.UUID `json:"taskId"`
}

type customerReplyAIOut struct {
	Reply     string `json:"reply"`
	Intent    string `json:"intent"`
	Sentiment string `json:"sentiment"`
	RiskLevel string `json:"riskLevel"`
	Notes     string `json:"notes"`
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimLeft(s, "\n")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimLeft(s, "\n")
	if idx := strings.LastIndex(s, "```"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func parseCustomerReplyJSON(content string) (customerReplyAIOut, error) {
	content = stripCodeFences(content)
	var out customerReplyAIOut
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		return customerReplyAIOut{}, err
	}
	out.Reply = strings.TrimSpace(out.Reply)
	out.Intent = strings.TrimSpace(out.Intent)
	out.Sentiment = strings.TrimSpace(out.Sentiment)
	out.RiskLevel = strings.TrimSpace(out.RiskLevel)
	if out.RiskLevel == "" {
		out.RiskLevel = "low"
	}
	out.Notes = strings.TrimSpace(out.Notes)
	return out, nil
}

func buildHistoryLines(msgs []CustomerMessage, maxLines int, maxRunePerLine int) string {
	if maxLines < 1 {
		maxLines = 20
	}
	start := 0
	if len(msgs) > maxLines {
		start = len(msgs) - maxLines
	}
	var b strings.Builder
	for _, m := range msgs[start:] {
		ts := m.CreatedAt.UTC().Format("2006-01-02 15:04")
		line := truncateRunes(strings.TrimSpace(m.Content), maxRunePerLine)
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(ts)
		b.WriteString(" [")
		b.WriteString(m.Role)
		b.WriteString("] ")
		b.WriteString(line)
	}
	return b.String()
}

func marshalJSONPretty(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func adjustCustomerReplyRisk(msg string, o *customerReplyAIOut, cautiousOrder bool) {
	if o == nil {
		return
	}
	ml := strings.ToLower(msg)
	cnTriggers := strings.Contains(ml, "退款") || strings.Contains(ml, "赔偿") ||
		strings.Contains(ml, "投诉") || strings.Contains(ml, "差评") ||
		strings.Contains(ml, "破损") || strings.Contains(ml, "索赔") ||
		strings.Contains(msg, "发错货") || strings.Contains(msg, "少发")

	sensitive := cnTriggers ||
		strings.Contains(ml, "refund") ||
		strings.Contains(ml, "chargeback") ||
		strings.Contains(ml, "compensation") ||
		strings.Contains(ml, "dispute") ||
		strings.Contains(ml, "complaint") ||
		strings.Contains(ml, "damaged") ||
		strings.Contains(ml, "wrong item") ||
		strings.Contains(ml, "missing item") ||
		strings.Contains(ml, "lost package") ||
		strings.Contains(ml, "lost order")

	level := strings.ToLower(strings.TrimSpace(o.RiskLevel))
	if level != "medium" && level != "high" {
		level = "low"
	}
	if cautiousOrder && level == "low" {
		level = "medium"
		if o.Notes == "" {
			o.Notes = "Order flagged for policy review."
		}
	}
	if sensitive && (level == "low" || level == "") {
		level = "medium"
		n := strings.TrimSpace(o.Notes)
		if !strings.Contains(strings.ToLower(n), "human") && !strings.Contains(strings.ToLower(n), "人工") && !strings.Contains(strings.ToLower(n), "manual") && !strings.Contains(strings.ToLower(n), "staff") {
			o.Notes = strings.TrimSpace("Escalate: sensitive topic detected. Human confirmation required.\n" + n)
		}
	}
	if sensitive && strings.Contains(ml, "lawsuit") && level != "high" {
		level = "high"
	}
	o.RiskLevel = level
}

func orderRiskContext(o *order.AIContext) bool {
	if o == nil || o.OrderInfo == nil {
		return false
	}
	status, _ := o.OrderInfo["status"].(string)
	payment, _ := o.OrderInfo["paymentStatus"].(string)
	fulfill, _ := o.OrderInfo["fulfillmentStatus"].(string)
	status = strings.TrimSpace(strings.ToLower(status))
	payment = strings.TrimSpace(strings.ToLower(payment))
	fulfill = strings.TrimSpace(strings.ToLower(fulfill))
	switch status {
	case order.StatusCancelled, order.StatusRefunded:
		return true
	}
	switch payment {
	case order.PaymentRefunded, order.PaymentPartiallyRefunded:
		return true
	}
	switch fulfill {
	case order.FulfillmentReturned:
		return true
	}
	return false
}

// GenerateReply runs customer_reply_generate via AI gateway; records ai_tasks + customer_reply_suggestions.
func (s *Service) GenerateReply(c *gin.Context, conversationID uuid.UUID, body GenerateReplyBody, adminID *uuid.UUID) (*GenerateReplyResult, error) {
	if s == nil || s.DB == nil {
		return nil, fmt.Errorf("customerchat: no db")
	}
	if s.Prompts == nil || s.AITasks == nil || s.AIGateway == nil {
		return nil, fmt.Errorf("customerchat: ai not configured")
	}

	var conv CustomerConversation
	if err := s.DB.WithContext(c.Request.Context()).First(&conv, "id = ?", conversationID).Error; err != nil {
		return nil, err
	}

	var msgs []CustomerMessage
	if err := s.DB.WithContext(c.Request.Context()).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&msgs).Error; err != nil {
		return nil, err
	}

	lang := strings.TrimSpace(body.Language)
	if lang == "" {
		lang = strings.TrimSpace(conv.CustomerLanguage)
	}
	if lang == "" {
		lang = "en"
	}
	tone := strings.TrimSpace(body.Tone)
	if tone == "" {
		tone = "professional"
	}
	platform := strings.TrimSpace(body.Platform)
	if platform == "" {
		platform = strings.TrimSpace(conv.Platform)
	}
	if platform == "" {
		platform = "manual"
	}
	shopPolicy := strings.TrimSpace(body.ShopPolicy)

	customerMsg := ""
	if mid := strings.TrimSpace(body.MessageID); mid != "" {
		muid, err := uuid.Parse(mid)
		if err != nil {
			return nil, fmt.Errorf("invalid messageId")
		}
		var cm CustomerMessage
		if err := s.DB.WithContext(c.Request.Context()).First(&cm, "id = ? AND conversation_id = ?", muid, conversationID).Error; err != nil {
			return nil, err
		}
		customerMsg = strings.TrimSpace(cm.Content)
	} else {
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == RoleCustomer {
				customerMsg = strings.TrimSpace(msgs[i].Content)
				break
			}
		}
	}

	history := buildHistoryLines(msgs, 20, 800)
	productInfo := ""

	var octx *order.AIContext
	cautiousOrder := false
	orderInfoPlain := `{}`
	itemsPlain := `[]`
	shipmentPlain := `[]`

	customerProfile := map[string]any{
		"conversationLanguage": lang,
		"conversationPlatform": platform,
		"linkedOrder":          conv.OrderID != nil,
	}

	if conv.OrderID != nil && s.Orders != nil {
		got, err := s.Orders.BuildAIContext(c, *conv.OrderID)
		if err == nil && got != nil {
			octx = got
			if got.OrderInfo != nil {
				orderInfoPlain = marshalJSONPretty(got.OrderInfo)
			}
			if len(got.OrderItems) > 0 {
				itemsPlain = marshalJSONPretty(got.OrderItems)
			}
			if len(got.ShipmentInfo) > 0 {
				shipmentPlain = marshalJSONPretty(got.ShipmentInfo)
			}
			cautiousOrder = orderRiskContext(got)
			if v := got.OrderInfo["orderNo"]; v != nil {
				customerProfile["orderNo"] = v
			}
		}
	}

	promptRow, err := s.Prompts.GetEnabledByCode(c.Request.Context(), aiprompt.CodeCustomerReplyGenerate)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("prompt %s not found or disabled", aiprompt.CodeCustomerReplyGenerate)
		}
		return nil, err
	}

	customerProfileJSON := marshalJSONPretty(customerProfile)

	vars := map[string]string{
		"customerMessage":     customerMsg,
		"conversationHistory": history,
		"productInfo":         productInfo,
		"orderInfo":           orderInfoPlain,
		"orderItems":          itemsPlain,
		"shipmentInfo":        shipmentPlain,
		"customerProfile":     customerProfileJSON,
		"language":            lang,
		"tone":                tone,
		"platform":            platform,
		"shopPolicy":          shopPolicy,
	}
	sys := aiprompt.ReplaceVariables(promptRow.SystemPrompt, vars)
	user := aiprompt.ReplaceVariables(promptRow.UserPrompt, vars)

	model := strings.TrimSpace(promptRow.Model)
	req := aigate.ChatRequest{
		Model: model,
		Messages: []aigate.Message{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		Temperature: promptRow.Temperature,
		MaxTokens:   promptRow.MaxTokens,
		ResponseFormat: &aigate.ResponseFormat{
			Type: "json_object",
		},
	}

	inputPayload := map[string]any{
		"promptCode":     aiprompt.CodeCustomerReplyGenerate,
		"conversationId": conversationID.String(),
		"language":       lang,
		"tone":           tone,
		"platform":       platform,
		"shopPolicyLen":  len([]rune(shopPolicy)),
		"customerMsgLen": len([]rune(customerMsg)),
		"orderLinked":    conv.OrderID != nil,
		"linkedOrderCue": map[string]any{
			"orderInfoHint":  len(orderInfoPlain) > 4,
			"itemsLines":     len(itemsPlain) > 2,
			"shipmentsLines": len(shipmentPlain) > 2,
		},
	}
	if conv.OrderID != nil {
		inputPayload["orderId"] = conv.OrderID.String()
	}
	if octx != nil {
		inputPayload["orderInfo"] = octx.OrderInfo
		inputPayload["orderItemsSummary"] = octx.OrderItems
		inputPayload["shipmentSummary"] = octx.ShipmentInfo
	}
	if mid := strings.TrimSpace(body.MessageID); mid != "" {
		inputPayload["messageId"] = mid
	}
	inputJSON, _ := json.Marshal(inputPayload)

	var refMsgID *uuid.UUID
	if mid := strings.TrimSpace(body.MessageID); mid != "" {
		if u, err := uuid.Parse(mid); err == nil {
			refMsgID = &u
		}
	}

	task := &aitask.AITask{
		TaskType:       TaskTypeCustomerReplyGenerate,
		Provider:       s.providerName(c),
		Model:          model,
		PromptCode:     aiprompt.CodeCustomerReplyGenerate,
		Input:          datatypes.JSON(inputJSON),
		CreatedBy:      adminID,
		ConversationID: &conversationID,
	}
	if err := s.AITasks.Create(c.Request.Context(), task); err != nil {
		return nil, err
	}
	taskID := task.ID

	resp, err := s.AIGateway.Chat(c.Request.Context(), req)
	if err != nil {
		_ = s.AITasks.MarkFailed(c.Request.Context(), taskID, err.Error())
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "customer.reply_generate.failed",
				Resource:    "customer_conversation",
				ResourceID:  conversationID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s conversationId=%s err=%s", taskID.String(), conversationID.String(), truncateRunes(err.Error(), 400)),
			})
		}
		return nil, err
	}

	parsed, perr := parseCustomerReplyJSON(resp.Content)
	if perr != nil {
		msg := fmt.Sprintf("parse ai json: %v", perr)
		_ = s.AITasks.MarkFailed(c.Request.Context(), taskID, msg)
		if s.OpLog != nil {
			_ = s.OpLog.Write(c, operationlog.WriteOpts{
				AdminUserID: adminID,
				Action:      "customer.reply_generate.failed",
				Resource:    "customer_conversation",
				ResourceID:  conversationID.String(),
				Status:      "failed",
				Message:     fmt.Sprintf("taskId=%s conversationId=%s err=invalid_model_output", taskID.String(), conversationID.String()),
			})
		}
		return nil, fmt.Errorf("invalid model output")
	}

	adjustCustomerReplyRisk(customerMsg, &parsed, cautiousOrder)

	outStruct := map[string]any{
		"reply":     parsed.Reply,
		"intent":    parsed.Intent,
		"sentiment": parsed.Sentiment,
		"riskLevel": parsed.RiskLevel,
		"notes":     parsed.Notes,
	}
	outJSON, _ := json.Marshal(outStruct)
	raw := resp.Raw
	if raw == nil {
		raw = []byte("{}")
	}
	usedModel := strings.TrimSpace(resp.Model)
	if usedModel == "" {
		usedModel = model
	}
	_ = s.AITasks.MarkSuccess(c.Request.Context(), taskID, outJSON, raw, resp.InputTokens, resp.OutputTokens, usedModel)

	tid := taskID
	sugg := &CustomerReplySuggestion{
		ConversationID: conversationID,
		MessageID:      refMsgID,
		AITaskID:       &tid,
		Provider:       s.providerName(c),
		Model:          usedModel,
		PromptCode:     aiprompt.CodeCustomerReplyGenerate,
		SuggestedReply: parsed.Reply,
		Status:         SuggestionGenerated,
		Language:       lang,
		Tone:           tone,
		Input:          datatypes.JSON(inputJSON),
		Output:         datatypes.JSON(outJSON),
		CreatedBy:      adminID,
	}
	if err := s.DB.WithContext(c.Request.Context()).Create(sugg).Error; err != nil {
		return nil, err
	}

	if s.OpLog != nil {
		_ = s.OpLog.Write(c, operationlog.WriteOpts{
			AdminUserID: adminID,
			Action:      "customer.reply_generate.success",
			Resource:    "customer_conversation",
			ResourceID:  conversationID.String(),
			Status:      "success",
			Message:     fmt.Sprintf("taskId=%s suggestionId=%s risk=%s replyLen=%d", taskID.String(), sugg.ID.String(), parsed.RiskLevel, len([]rune(parsed.Reply))),
		})
	}

	return &GenerateReplyResult{
		SuggestionID: sugg.ID,
		Reply:        parsed.Reply,
		Intent:       parsed.Intent,
		Sentiment:    parsed.Sentiment,
		RiskLevel:    parsed.RiskLevel,
		Notes:        parsed.Notes,
		TaskID:       taskID,
	}, nil
}
