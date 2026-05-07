package kafkaconsumer

import (
	"encoding/json"
	"testing"
)

func TestParseOrderUpdateAcceptsNestedEngineMessage(t *testing.T) {
	data := mustMarshal(t, orderUpdateEnvelope{
		Type: "ORDER_UPDATE",
		Payload: OrderUpdateMessage{
			PlatformID:       "platform-1",
			OrderID:          "ord-456",
			Status:           "PENDING",
			FilledQuantity:   3,
			AverageFillPrice: 15.25,
			ExchangeFee:      0.12,
			MarketTime:       "2026-05-07T10:00:00Z",
		},
	})

	msg, err := parseOrderUpdate(data)
	if err != nil {
		t.Fatalf("parseOrderUpdate returned error: %v", err)
	}

	if msg.PlatformID != "platform-1" || msg.OrderID != "ord-456" || msg.Status != "PENDING" {
		t.Fatalf("parsed update = %#v", msg)
	}
	if msg.FilledQuantity != 3 || msg.AverageFillPrice != 15.25 || msg.ExchangeFee != 0.12 {
		t.Fatalf("parsed fill fields = %#v", msg)
	}
	if msg.MarketTime != "2026-05-07T10:00:00Z" {
		t.Fatalf("market_time = %q", msg.MarketTime)
	}
}

func mustMarshal(t *testing.T, value any) []byte {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal returned %v", err)
	}
	return data
}
