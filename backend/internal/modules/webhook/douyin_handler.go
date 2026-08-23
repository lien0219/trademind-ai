package webhook

import (
	"context"
	"log/slog"

	douyinshop "github.com/trademind-ai/trademind/backend/internal/providers/platform/douyinshop"
)

// OrderEventHandler receives normalized order events from Douyin webhooks.
// Implementations may upsert orders, trigger sync tasks, etc.
type OrderEventHandler interface {
	HandleDouyinOrderEvent(ctx context.Context, ev *douyinshop.NormalizedWebhookEvent) error
}

// AfterSaleEventHandler receives normalized refund/after-sale facts. It must
// remain read-only with respect to the provider and payment systems.
type AfterSaleEventHandler interface {
	HandleDouyinAfterSaleEvent(ctx context.Context, ev *douyinshop.NormalizedWebhookEvent) error
}

// douyinEventDispatcher routes normalized Douyin events to typed handlers.
type douyinEventDispatcher struct {
	OrderHandler    OrderEventHandler
	AfterSaleHandler AfterSaleEventHandler
}

// DispatchDouyinEvent routes a normalized event to the appropriate handler.
// Unknown event types are safely ACK'd with a warning log — never dropped silently.
func (d *douyinEventDispatcher) DispatchDouyinEvent(ctx context.Context, ev *douyinshop.NormalizedWebhookEvent) error {
	if ev == nil {
		return nil
	}
	if ev.IsHealthPing {
		slog.InfoContext(ctx, "douyin webhook health ping received")
		return nil
	}
	switch ev.EventType {
	case "order_created", "order_paid", "order_shipped", "order_completed", "order_cancelled":
		if d.OrderHandler != nil {
			return d.OrderHandler.HandleDouyinOrderEvent(ctx, ev)
		}
		slog.WarnContext(ctx, "douyin order event received but no OrderEventHandler configured",
			"eventType", ev.EventType, "msgId", ev.MsgID)
		return nil
	case "after_sale_created", "after_sale_updated", "refund_created", "refund_pending", "refund_success", "refund_failed", "refund_completed":
		if d.AfterSaleHandler != nil {
			return d.AfterSaleHandler.HandleDouyinAfterSaleEvent(ctx, ev)
		}
		slog.WarnContext(ctx, "douyin after-sale event received but no AfterSaleEventHandler configured",
			"eventType", ev.EventType, "msgId", ev.MsgID)
		return nil
	case "inventory_alert":
		slog.InfoContext(ctx, "douyin inventory_alert received; handler is not implemented",
			"msgId", ev.MsgID)
		return nil
	case "product_status_changed":
		slog.InfoContext(ctx, "douyin product_status_changed received; handler is not implemented",
			"msgId", ev.MsgID)
		return nil
	default:
		// Unknown tag — safe ACK with warning
		slog.WarnContext(ctx, "douyin webhook unknown event type — safe ACK",
			"eventType", ev.EventType, "tag", ev.Tag, "msgId", ev.MsgID)
		return nil
	}
}

// HandleDouyinPlatformEvent is the entry point called from processor.handlePlatformEvent
// for platforms "douyin_shop" and "douyin".
// It parses the raw event payload and dispatches to the event dispatcher.
func (s *Service) HandleDouyinPlatformEvent(ctx context.Context, ev *Event) error {
	if ev == nil {
		return nil
	}
	payload := []byte(ev.PayloadBody)
	if len(payload) == 0 {
		payload = []byte(ev.RawSummary)
	}

	// Try standard Douyin envelope first
	if env, err := douyinshop.ParseDouyinWebhookEnvelope(payload); err == nil && env.Event != "" {
		normalized := douyinshop.NormalizeDouyinEnvelope(env, payload)
		applyEventResolution(ev, normalized)
		dispatcher := &douyinEventDispatcher{OrderHandler: s.OrderHandler, AfterSaleHandler: s.AfterSaleHandler}
		return dispatcher.DispatchDouyinEvent(ctx, normalized)
	}

	// Try jinritemai array push
	if items, err := douyinshop.ParseJinriteimaiPushEnvelope(payload); err == nil && len(items) > 0 {
		dispatcher := &douyinEventDispatcher{OrderHandler: s.OrderHandler, AfterSaleHandler: s.AfterSaleHandler}
		for _, item := range items {
			normalized := douyinshop.NormalizeJinriteimaiItem(item, payload)
			applyEventResolution(ev, normalized)
			if err := dispatcher.DispatchDouyinEvent(ctx, normalized); err != nil {
				return err
			}
		}
		return nil
	}

	// Unrecognized shape — safe ACK
	slog.WarnContext(ctx, "douyin webhook payload did not match any known envelope format — safe ACK",
		"platform", ev.Platform, "eventId", ev.EventID)
	return nil
}

func applyEventResolution(src *Event, dst *douyinshop.NormalizedWebhookEvent) {
	if src == nil || dst == nil {
		return
	}
	dst.TenantID = src.TenantID
	dst.PlatformShopID = src.PlatformShopID
	dst.AppID = src.AppID
	if dst.MsgID == "" {
		dst.MsgID = src.EventID
	}
	if src.InternalShopID != nil {
		dst.InternalShopID = src.InternalShopID.String()
	}
	if src.BindingID != nil {
		dst.BindingID = src.BindingID.String()
	}
}
