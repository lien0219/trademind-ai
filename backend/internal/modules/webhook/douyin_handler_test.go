package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

type testAfterSaleHandler struct {
	called bool
	event  *douyinshop.NormalizedWebhookEvent
	err    error
}

func (h *testAfterSaleHandler) HandleDouyinAfterSaleEvent(_ context.Context, ev *douyinshop.NormalizedWebhookEvent) error {
	h.called = true
	h.event = ev
	return h.err
}

func TestDouyinDispatcherRoutesAfterSaleEventsAndPreservesErrors(t *testing.T) {
	handler := &testAfterSaleHandler{err: errors.New("incomplete after-sale")}
	dispatcher := &douyinEventDispatcher{AfterSaleHandler: handler}
	err := dispatcher.DispatchDouyinEvent(context.Background(), &douyinshop.NormalizedWebhookEvent{EventType: "refund_success"})
	if !handler.called || !errors.Is(err, handler.err) {
		t.Fatalf("after-sale handler was not called or error was swallowed: called=%v err=%v", handler.called, err)
	}
}

func TestDouyinDispatcherSafelyIgnoresUnknownEvents(t *testing.T) {
	handler := &testAfterSaleHandler{}
	dispatcher := &douyinEventDispatcher{AfterSaleHandler: handler}
	if err := dispatcher.DispatchDouyinEvent(context.Background(), &douyinshop.NormalizedWebhookEvent{EventType: "refund_unknown"}); err != nil {
		t.Fatal(err)
	}
	if handler.called {
		t.Fatal("unknown event must not be sent to after-sale handler")
	}
}

func TestHandleDouyinPlatformEventDispatchesStandardAfterSaleEnvelope(t *testing.T) {
	handler := &testAfterSaleHandler{}
	svc := &Service{AfterSaleHandler: handler}
	payload, err := json.Marshal(map[string]any{
		"event": "refund_success",
		"content": map[string]any{
			"refund_id": "refund-standard-1", "order_id": "order-standard-1",
			"status": "success", "refund_amount": "3.20", "currency": "CNY",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	shopID := uuid.New()
	event := &Event{
		TenantID: 7, Platform: "douyin_shop", InternalShopID: &shopID, PlatformShopID: "platform-shop-1",
		EventID: "standard-after-sale-event", PayloadBody: string(payload),
	}
	if err := svc.HandleDouyinPlatformEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if !handler.called || handler.event == nil || handler.event.EventType != "refund_success" || handler.event.MsgID != event.EventID || handler.event.TenantID != event.TenantID || handler.event.InternalShopID != shopID.String() {
		t.Fatalf("standard after-sale envelope was not resolved: called=%v event=%#v", handler.called, handler.event)
	}
}
