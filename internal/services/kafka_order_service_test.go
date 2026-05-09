package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

type fakeKafkaWriter struct {
	messages chan kafka.Message
	err      error
}

func newFakeKafkaWriter() *fakeKafkaWriter {
	return &fakeKafkaWriter{messages: make(chan kafka.Message, 1)}
}

func (f *fakeKafkaWriter) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	if f.err != nil {
		return f.err
	}
	for _, msg := range msgs {
		f.messages <- msg
	}
	return nil
}

func (f *fakeKafkaWriter) Close() error {
	return nil
}

func TestKafkaOrderServiceDefaultResponseTopicUsesOrderUpdates(t *testing.T) {
	if DefaultOrderResponsesTopic != "orders.updates" {
		t.Fatalf("DefaultOrderResponsesTopic = %q, want orders.updates", DefaultOrderResponsesTopic)
	}
}

func TestKafkaOrderServicePublishesExactEnginePayloadAndWaitsForAck(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)

	resultCh := make(chan PlaceOrderResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
			PlatformID:     "platform-1",
			PlatformUserID: "user-1",
			InstrumentType: "STOCK",
			InstrumentID:   "ARKA",
			OrderType:      "MARKET",
			Side:           "BUY",
			Quantity:       10,
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- resp
	}()

	msg := <-writer.messages
	if msg.Topic != "orders.requests" {
		t.Fatalf("topic = %q, want orders.requests", msg.Topic)
	}
	if string(msg.Key) == "" {
		t.Fatal("Kafka key is empty, want correlation id")
	}

	var raw map[string]any
	if err := json.Unmarshal(msg.Value, &raw); err != nil {
		t.Fatalf("request payload is not JSON: %v", err)
	}
	if _, ok := raw["order_id"]; ok {
		t.Fatalf("request payload must not include order_id: %#v", raw)
	}
	wantKeys := map[string]bool{
		"platform_id":      true,
		"platform_user_id": true,
		"instrument_type":  true,
		"instrument_id":    true,
		"order_type":       true,
		"side":             true,
		"quantity":         true,
	}
	if len(raw) != len(wantKeys) {
		t.Fatalf("request keys = %#v, want exactly %#v", raw, wantKeys)
	}
	for key := range wantKeys {
		if _, ok := raw[key]; !ok {
			t.Fatalf("request missing key %q: %#v", key, raw)
		}
	}
	if raw["platform_id"] != "platform-1" || raw["platform_user_id"] != "user-1" || raw["instrument_id"] != "ARKA" {
		t.Fatalf("request payload = %#v", raw)
	}

	service.handleOrderResponse(msg.Key, mustMarshalOrderResponse(t, orderResponse{
		OrderID: "ord-456",
		Status:  "PENDING",
	}))

	select {
	case err := <-errCh:
		t.Fatalf("PlaceOrder returned error: %v", err)
	case resp := <-resultCh:
		if resp.OrderID != "ord-456" || resp.Status != "PENDING" {
			t.Fatalf("response = %#v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for PlaceOrder response")
	}
}

func TestKafkaOrderServiceIncludesOptionalOrderFieldsOnlyWhenPresent(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)
	limitPrice := 150.25
	expiresAt := "2026-04-27T16:00:00Z"

	go func() {
		_, _ = service.PlaceOrder(context.Background(), PlaceOrderRequest{
			PlatformID:     "platform-1",
			PlatformUserID: "user-1",
			InstrumentType: "STOCK",
			InstrumentID:   "ARKA",
			OrderType:      "LIMIT",
			Side:           "BUY",
			Quantity:       10,
			LimitPrice:     &limitPrice,
			ExpiresAt:      &expiresAt,
		})
	}()

	msg := <-writer.messages
	var raw map[string]any
	if err := json.Unmarshal(msg.Value, &raw); err != nil {
		t.Fatalf("request payload is not JSON: %v", err)
	}
	if raw["limit_price"] != limitPrice {
		t.Fatalf("limit_price = %#v, want %#v", raw["limit_price"], limitPrice)
	}
	if raw["expires_at"] != expiresAt {
		t.Fatalf("expires_at = %#v, want %#v", raw["expires_at"], expiresAt)
	}
}

func TestKafkaOrderServiceCorrelatesResponseByKafkaKey(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)

	resultCh := make(chan PlaceOrderResponse, 1)
	go func() {
		resp, _ := service.PlaceOrder(context.Background(), PlaceOrderRequest{
			PlatformID:     "platform-1",
			PlatformUserID: "user-1",
			InstrumentType: "STOCK",
			InstrumentID:   "ARKA",
			OrderType:      "MARKET",
			Side:           "BUY",
			Quantity:       10,
		})
		resultCh <- resp
	}()

	msg := <-writer.messages
	service.handleOrderResponse([]byte("wrong-key"), mustMarshalOrderResponse(t, orderResponse{
		OrderID: "ord-wrong",
		Status:  "PENDING",
	}))

	select {
	case resp := <-resultCh:
		t.Fatalf("unexpected response for wrong key: %#v", resp)
	case <-time.After(20 * time.Millisecond):
	}

	service.handleOrderResponse(msg.Key, mustMarshalOrderResponse(t, orderResponse{
		OrderID: "ord-456",
		Status:  "PENDING",
	}))

	select {
	case resp := <-resultCh:
		if resp.OrderID != "ord-456" || resp.Status != "PENDING" {
			t.Fatalf("response = %#v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for correlated response")
	}
}

func TestKafkaOrderServiceCorrelatesEngineOrderUpdateByPlatformUser(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)

	resultCh := make(chan PlaceOrderResponse, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
			PlatformID:     "platform-1",
			PlatformUserID: "user-1",
			InstrumentType: "STOCK",
			InstrumentID:   "ARKA",
			OrderType:      "MARKET",
			Side:           "BUY",
			Quantity:       10,
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- resp
	}()

	<-writer.messages
	service.handleOrderResponse([]byte("ord-456"), mustMarshalOrderResponse(t, orderResponse{
		PlatformID:     "platform-1",
		PlatformUserID: "user-1",
		OrderID:        "ord-456",
		Status:         "PENDING",
	}))

	select {
	case err := <-errCh:
		t.Fatalf("PlaceOrder returned error: %v", err)
	case resp := <-resultCh:
		if resp.OrderID != "ord-456" || resp.Status != "PENDING" {
			t.Fatalf("response = %#v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for PlaceOrder response")
	}
}

func TestKafkaOrderServiceReturnsRejectedOrderUpdate(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)

	errCh := make(chan error, 1)
	go func() {
		_, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
			PlatformID:     "platform-1",
			PlatformUserID: "user-1",
			InstrumentType: "STOCK",
			InstrumentID:   "ARKA",
			OrderType:      "MARKET",
			Side:           "BUY",
			Quantity:       10,
		})
		errCh <- err
	}()

	<-writer.messages
	service.handleOrderResponse([]byte("ord-456"), mustMarshalOrderResponse(t, orderResponse{
		PlatformID:      "platform-1",
		PlatformUserID:  "user-1",
		OrderID:         "ord-456",
		Status:          "REJECTED",
		RejectionReason: "MARKET_CLOSED",
	}))

	err := <-errCh
	if err == nil {
		t.Fatal("PlaceOrder returned nil error, want rejection")
	}
	var placementErr OrderPlacementError
	if !errors.As(err, &placementErr) {
		t.Fatalf("error type = %T, want OrderPlacementError", err)
	}
	if placementErr.RejectionCode() != "MARKET_CLOSED" || placementErr.Error() != "Market is currently closed." {
		t.Fatalf("rejection = %#v", placementErr)
	}
}

func TestKafkaOrderServiceRejectsConcurrentOrderForSamePlatformUser(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)

	firstResultCh := make(chan PlaceOrderResponse, 1)
	firstErrCh := make(chan error, 1)
	go func() {
		resp, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
			PlatformID:     "platform-1",
			PlatformUserID: "user-1",
			InstrumentType: "STOCK",
			InstrumentID:   "ARKA",
			OrderType:      "MARKET",
			Side:           "BUY",
			Quantity:       10,
		})
		if err != nil {
			firstErrCh <- err
			return
		}
		firstResultCh <- resp
	}()

	firstMsg := <-writer.messages
	_, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
		PlatformID:     "platform-1",
		PlatformUserID: "user-1",
		InstrumentType: "STOCK",
		InstrumentID:   "ARKA",
		OrderType:      "MARKET",
		Side:           "BUY",
		Quantity:       10,
	})
	if err == nil {
		t.Fatal("second PlaceOrder returned nil error, want duplicate rejection")
	}
	var placementErr OrderPlacementError
	if !errors.As(err, &placementErr) {
		t.Fatalf("error type = %T, want OrderPlacementError", err)
	}
	if placementErr.RejectionCode() != "ORDER_ALREADY_PENDING" {
		t.Fatalf("rejection code = %q", placementErr.RejectionCode())
	}

	select {
	case msg := <-writer.messages:
		t.Fatalf("unexpected second Kafka publish: %#v", msg)
	default:
	}

	service.handleOrderResponse(firstMsg.Key, mustMarshalOrderResponse(t, orderResponse{
		OrderID: "ord-456",
		Status:  "PENDING",
	}))

	select {
	case err := <-firstErrCh:
		t.Fatalf("first PlaceOrder returned error: %v", err)
	case resp := <-firstResultCh:
		if resp.OrderID != "ord-456" || resp.Status != "PENDING" {
			t.Fatalf("first response = %#v", resp)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first PlaceOrder response")
	}
}

func TestKafkaOrderServiceReturnsEngineRejection(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)

	errCh := make(chan error, 1)
	go func() {
		_, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
			PlatformID:     "platform-1",
			PlatformUserID: "user-1",
			InstrumentType: "STOCK",
			InstrumentID:   "ARKA",
			OrderType:      "MARKET",
			Side:           "BUY",
			Quantity:       10,
		})
		errCh <- err
	}()

	msg := <-writer.messages
	service.handleOrderResponse(msg.Key, mustMarshalOrderResponse(t, orderResponse{
		Code:    "MARKET_CLOSED",
		Message: "Market is currently closed.",
	}))

	err := <-errCh
	if err == nil {
		t.Fatal("PlaceOrder returned nil error, want rejection")
	}
	var placementErr OrderPlacementError
	if !errors.As(err, &placementErr) {
		t.Fatalf("error type = %T, want OrderPlacementError", err)
	}
	if placementErr.RejectionCode() != "MARKET_CLOSED" || placementErr.Error() != "Market is currently closed." {
		t.Fatalf("rejection = %#v", placementErr)
	}
}

func TestKafkaOrderServiceTimeoutReturnsUnavailable(t *testing.T) {
	writer := newFakeKafkaWriter()
	service := newKafkaOrderService(writer, nil, "orders.requests", 10*time.Millisecond)

	_, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
		PlatformID:     "platform-1",
		PlatformUserID: "user-1",
		InstrumentType: "STOCK",
		InstrumentID:   "ARKA",
		OrderType:      "MARKET",
		Side:           "BUY",
		Quantity:       10,
	})
	if err == nil {
		t.Fatal("PlaceOrder returned nil error, want timeout")
	}
	var placementErr OrderPlacementError
	if !errors.As(err, &placementErr) {
		t.Fatalf("error type = %T, want OrderPlacementError", err)
	}
	if placementErr.RejectionCode() != "ORDER_SERVICE_UNAVAILABLE" {
		t.Fatalf("rejection code = %q", placementErr.RejectionCode())
	}
}

func TestKafkaOrderServicePublishFailureReturnsUnavailable(t *testing.T) {
	writer := newFakeKafkaWriter()
	writer.err = errors.New("broker unavailable")
	service := newKafkaOrderService(writer, nil, "orders.requests", time.Second)

	_, err := service.PlaceOrder(context.Background(), PlaceOrderRequest{
		PlatformID:     "platform-1",
		PlatformUserID: "user-1",
		InstrumentType: "STOCK",
		InstrumentID:   "ARKA",
		OrderType:      "MARKET",
		Side:           "BUY",
		Quantity:       10,
	})
	if err == nil {
		t.Fatal("PlaceOrder returned nil error, want publish failure")
	}
	var placementErr OrderPlacementError
	if !errors.As(err, &placementErr) {
		t.Fatalf("error type = %T, want OrderPlacementError", err)
	}
	if placementErr.RejectionCode() != "ORDER_SERVICE_UNAVAILABLE" {
		t.Fatalf("rejection code = %q", placementErr.RejectionCode())
	}
}

func mustMarshalOrderResponse(t *testing.T, response orderResponse) []byte {
	t.Helper()

	messageType := "ORDER_UPDATE"
	if response.Code != "" {
		messageType = "ORDER_REJECTED"
	}

	data, err := json.Marshal(orderResponseEnvelope{
		Type:    messageType,
		Payload: response,
	})
	if err != nil {
		t.Fatalf("json.Marshal returned %v", err)
	}
	return data
}
