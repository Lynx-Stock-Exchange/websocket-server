package ws

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"stock-exchange-ws/internal/services"
)

type fakeOrderService struct {
	resp services.PlaceOrderResponse
	err  error
	req  services.PlaceOrderRequest
}

func (f *fakeOrderService) PlaceOrder(_ context.Context, req services.PlaceOrderRequest) (services.PlaceOrderResponse, error) {
	f.req = req
	return f.resp, f.err
}

type codedError struct {
	code string
	err  error
}

func (e codedError) Error() string {
	return e.err.Error()
}

func (e codedError) Unwrap() error {
	return e.err
}

func (e codedError) RejectionCode() string {
	return e.code
}

func TestValidatePlaceOrderMarketWithoutLimitPricePasses(t *testing.T) {
	payload := PlaceOrderPayload{
		PlatformUserID: "user-1",
		InstrumentType: "STOCK",
		InstrumentID:   "ARKA",
		OrderType:      "MARKET",
		Side:           "BUY",
		Quantity:       10,
	}

	if err := validatePlaceOrder(payload); err != nil {
		t.Fatalf("validatePlaceOrder returned %v, want nil", err)
	}
}

func TestValidatePlaceOrderLimitWithoutLimitPriceFails(t *testing.T) {
	payload := PlaceOrderPayload{
		PlatformUserID: "user-1",
		InstrumentType: "STOCK",
		InstrumentID:   "ARKA",
		OrderType:      "LIMIT",
		Side:           "BUY",
		Quantity:       10,
	}

	err := validatePlaceOrder(payload)
	if err == nil || err.Error() != "limit_price is required for LIMIT orders" {
		t.Fatalf("validatePlaceOrder returned %v, want limit_price error", err)
	}
}

func TestValidatePlaceOrderMissingCommonFieldFails(t *testing.T) {
	payload := PlaceOrderPayload{
		InstrumentType: "STOCK",
		InstrumentID:   "ARKA",
		OrderType:      "MARKET",
		Side:           "BUY",
		Quantity:       10,
	}

	err := validatePlaceOrder(payload)
	if err == nil || err.Error() != "platform_user_id is required" {
		t.Fatalf("validatePlaceOrder returned %v, want platform_user_id error", err)
	}
}

func TestHandlePlaceOrderSuccessEnqueuesOrderAck(t *testing.T) {
	service := &fakeOrderService{
		resp: services.PlaceOrderResponse{
			OrderID: "ord-123",
			Status:  "PENDING",
		},
	}
	client := newClient(NewHub(), nil, "platform-1", service, 1)

	raw := mustMarshalRaw(t, PlaceOrderPayload{
		PlatformUserID: "user-1",
		InstrumentType: "stock",
		InstrumentID:   "arka",
		OrderType:      "market",
		Side:           "buy",
		Quantity:       10,
	})

	if !client.handlePlaceOrder(raw) {
		t.Fatalf("handlePlaceOrder returned false, want true")
	}

	msg := <-client.send
	if msg.Type != MessageOrderAck {
		t.Fatalf("message type = %q, want %q", msg.Type, MessageOrderAck)
	}

	ack, ok := msg.Payload.(OrderAckPayload)
	if !ok {
		t.Fatalf("payload type = %T, want OrderAckPayload", msg.Payload)
	}
	if ack.OrderID != "ord-123" || ack.Status != "PENDING" {
		t.Fatalf("ack payload = %#v", ack)
	}
	if service.req.PlatformID != "platform-1" {
		t.Fatalf("service req PlatformID = %q, want %q", service.req.PlatformID, "platform-1")
	}
	if service.req.InstrumentType != "STOCK" || service.req.InstrumentID != "ARKA" || service.req.Side != "BUY" {
		t.Fatalf("service req normalization failed: %#v", service.req)
	}
}

func TestHandlePlaceOrderServiceErrorEnqueuesOrderRejected(t *testing.T) {
	service := &fakeOrderService{
		err: codedError{
			code: "MARKET_CLOSED",
			err:  errors.New("market is currently closed"),
		},
	}
	client := newClient(NewHub(), nil, "platform-1", service, 1)

	raw := mustMarshalRaw(t, PlaceOrderPayload{
		PlatformUserID: "user-1",
		InstrumentType: "STOCK",
		InstrumentID:   "ARKA",
		OrderType:      "MARKET",
		Side:           "BUY",
		Quantity:       10,
	})

	if !client.handlePlaceOrder(raw) {
		t.Fatalf("handlePlaceOrder returned false, want true")
	}

	msg := <-client.send
	if msg.Type != MessageOrderRejected {
		t.Fatalf("message type = %q, want %q", msg.Type, MessageOrderRejected)
	}

	rejected, ok := msg.Payload.(OrderRejectedPayload)
	if !ok {
		t.Fatalf("payload type = %T, want OrderRejectedPayload", msg.Payload)
	}
	if rejected.Code != "MARKET_CLOSED" {
		t.Fatalf("rejected code = %q, want %q", rejected.Code, "MARKET_CLOSED")
	}
	if rejected.Message != "market is currently closed" {
		t.Fatalf("rejected message = %q", rejected.Message)
	}
}

func mustMarshalRaw(t *testing.T, payload PlaceOrderPayload) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal returned %v", err)
	}

	return raw
}
