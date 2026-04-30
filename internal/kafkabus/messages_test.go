package kafkabus

import (
	"testing"

	"stock-exchange-ws/internal/services"
)

func TestOrderCommandFromRequest(t *testing.T) {
	limitPrice := 128.5
	expiresAt := "2026-04-30T14:00:00Z"
	req := services.PlaceOrderRequest{
		ClientRequestID: "req-1",
		PlatformID:      "platform-xyz",
		PlatformUserID:  "user-1",
		InstrumentType:  "STOCK",
		InstrumentID:    "AAPL",
		OrderType:       "LIMIT",
		Side:            "BUY",
		Quantity:        10,
		LimitPrice:      &limitPrice,
		ExpiresAt:       &expiresAt,
	}

	msg := OrderCommandFromRequest(req)

	if msg.ClientRequestID != req.ClientRequestID || msg.PlatformID != req.PlatformID {
		t.Fatalf("message routing fields = %#v, want request ids", msg)
	}
	if msg.InstrumentID != "AAPL" || msg.Quantity != 10 {
		t.Fatalf("message order fields = %#v", msg)
	}
	if msg.LimitPrice == nil || *msg.LimitPrice != limitPrice {
		t.Fatalf("message LimitPrice = %#v, want %v", msg.LimitPrice, limitPrice)
	}
	if msg.ExpiresAt == nil || *msg.ExpiresAt != expiresAt {
		t.Fatalf("message ExpiresAt = %#v, want %q", msg.ExpiresAt, expiresAt)
	}
}

func TestOrderCommandResultToWS(t *testing.T) {
	msg := OrderCommandResultMessage{
		ClientRequestID: "req-1",
		PlatformID:      "platform-xyz",
		Accepted:        true,
		OrderID:         "ord-1",
		Status:          "PENDING",
	}

	result := OrderCommandResultToWS(msg)

	if result.ClientRequestID != "req-1" || result.OrderID != "ord-1" || !result.Accepted {
		t.Fatalf("result = %#v", result)
	}
}
