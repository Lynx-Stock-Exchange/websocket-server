package kafkaconsumer

import (
	"encoding/json"
	"testing"

	"stock-exchange-ws/internal/ws"
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

func TestParseMarketEventAcceptsNestedMessageAndPreservesPayload(t *testing.T) {
	data := mustMarshal(t, marketEventEnvelope{
		Type: "MARKET_EVENT",
		Payload: ws.MarketEventPayload{
			EventID:       "evt-001",
			EventType:     "SECTOR_SLUMP",
			Headline:      "Regulatory concerns shake the Tech sector",
			Scope:         "SECTOR",
			Target:        "Tech",
			Magnitude:     1.8,
			DurationTicks: 20,
			MarketTime:    "2024-03-15T11:00:00",
		},
	})

	payload, err := parseMarketEvent(data)
	if err != nil {
		t.Fatalf("parseMarketEvent returned error: %v", err)
	}

	if payload.EventID != "evt-001" || payload.EventType != "SECTOR_SLUMP" || payload.Target != "Tech" {
		t.Fatalf("parsed event = %#v", payload)
	}
	if payload.Magnitude != 1.8 || payload.DurationTicks != 20 {
		t.Fatalf("parsed event numeric fields = %#v", payload)
	}

	outbound := mustMarshal(t, ws.NewEnvelope(ws.MessageMarketEvent, payload))
	var envelope map[string]any
	if err := json.Unmarshal(outbound, &envelope); err != nil {
		t.Fatalf("outbound event is not JSON: %v", err)
	}
	if envelope["type"] != string(ws.MessageMarketEvent) {
		t.Fatalf("outbound type = %#v, want MARKET_EVENT", envelope["type"])
	}

	payloadMap, ok := envelope["payload"].(map[string]any)
	if !ok {
		t.Fatalf("outbound payload type = %T, want object", envelope["payload"])
	}
	if _, ok := payloadMap["code"]; ok {
		t.Fatalf("market event payload must not include code: %#v", payloadMap)
	}
	if _, ok := payloadMap["message"]; ok {
		t.Fatalf("market event payload must not include message: %#v", payloadMap)
	}
	if payloadMap["event_id"] != "evt-001" || payloadMap["event_type"] != "SECTOR_SLUMP" {
		t.Fatalf("outbound payload = %#v", payloadMap)
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
