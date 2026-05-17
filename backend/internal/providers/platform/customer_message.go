package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// PullMessagesRequest is passed to platform adapters (never log Auth secrets).
type PullMessagesRequest struct {
	ShopID    uuid.UUID
	Platform  string
	Auth      TestConnectionRequest
	StartTime *time.Time
	EndTime   *time.Time
	Cursor    string
	Limit     int
}

// PullMessagesResult is one page of normalized conversations.
type PullMessagesResult struct {
	Conversations []PlatformConversation
	NextCursor    string
	HasMore       bool
	RawSummary    map[string]any
}

// PlatformConversation is a provider-neutral chat thread snapshot.
type PlatformConversation struct {
	ExternalConversationID string
	CustomerName           string
	CustomerAvatar         string
	CustomerLanguage       string
	Status                 string
	LastMessageAt          *time.Time
	Messages               []PlatformCustomerMessage
	RawData                map[string]any
}

// PlatformCustomerMessage is one line in a platform conversation.
type PlatformCustomerMessage struct {
	ExternalMessageID string
	Role              string // customer | agent | system
	Content           string
	MessageType       string // text | image | order | system
	Language          string
	CreatedAt         *time.Time
	RawData           map[string]any
}

// SendMessageRequest sends an agent reply to the platform thread.
type SendMessageRequest struct {
	ShopID                 uuid.UUID
	Platform               string
	Auth                   TestConnectionRequest
	ExternalConversationID string
	ConversationID         uuid.UUID
	Reply                  string
	MessageType            string
	IdempotencyKey         string
}

// SendMessageResult is the outcome of a send attempt.
type SendMessageResult struct {
	ExternalMessageID string
	SentAt            *time.Time
	RawSummary        map[string]any
}

// CustomerMessageProvider extends Provider with buyer-seller messaging (implemented per channel).
type CustomerMessageProvider interface {
	Provider
	PullMessages(ctx context.Context, req PullMessagesRequest) (*PullMessagesResult, error)
	SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResult, error)
}

// AsCustomerMessage type-asserts to CustomerMessageProvider.
func AsCustomerMessage(p Provider) (CustomerMessageProvider, bool) {
	if p == nil {
		return nil, false
	}
	cm, ok := p.(CustomerMessageProvider)
	return cm, ok
}

// CustomerMessageImplementationStatus describes rollout for buyer-seller messaging (not order_sync).
func CustomerMessageImplementationStatus(p Provider) string {
	if p == nil {
		return StatusDisabled
	}
	switch strings.TrimSpace(strings.ToLower(p.Platform())) {
	case "mock":
		return StatusAvailable
	case "tiktok", "shopee", "lazada":
		return StatusBeta
	case "amazon":
		return StatusBeta
	default:
		st := p.Status()
		if st == StatusPlanned || st == StatusDisabled {
			return st
		}
		if !HasCapability(p, CapCustomerMessage) {
			return StatusDisabled
		}
		// Declared in meta but not wired in this build.
		return StatusPlanned
	}
}

// ImplementationStatusForCapability maps capability tokens to coarse rollout (for /platform/providers).
func ImplementationStatusForCapability(p Provider, c Capability) string {
	if p == nil {
		return StatusDisabled
	}
	if c == CapCustomerMessage {
		return CustomerMessageImplementationStatus(p)
	}
	if c == CapProductPublish {
		return ProductPublishImplementationStatus(p)
	}
	if c == CapInventorySync {
		return InventorySyncImplementationStatus(p)
	}
	switch p.Status() {
	case StatusPlanned, StatusDisabled:
		return p.Status()
	case StatusBeta:
		return StatusBeta
	default:
		return StatusAvailable
	}
}

// TrimRawMap returns a small plain map safe to persist (no tokens, limited size).
func TrimRawMap(m map[string]any, maxKeys int, maxStr int) map[string]any {
	if len(m) == 0 {
		return nil
	}
	if maxKeys <= 0 {
		maxKeys = 12
	}
	if maxStr <= 0 {
		maxStr = 512
	}
	out := make(map[string]any, min(len(m), maxKeys))
	n := 0
	for k, v := range m {
		if n >= maxKeys {
			out["_truncated"] = true
			break
		}
		ks := strings.TrimSpace(k)
		if ks == "" {
			continue
		}
		switch t := v.(type) {
		case string:
			s := strings.TrimSpace(t)
			if len(s) > maxStr {
				s = s[:maxStr] + "…"
			}
			out[ks] = s
		case float64, bool, int, int64:
			out[ks] = t
		default:
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if len(s) > maxStr {
				s = s[:maxStr] + "…"
			}
			out[ks] = s
		}
		n++
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
