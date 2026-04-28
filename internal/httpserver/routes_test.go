package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"stock-exchange-ws/internal/ws"
)

func TestPushOrderUpdateAcceptsValidPayload(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	handler := NewInternalPushHandler(hub)
	body := []byte(`{
		"platform_id": "platform-xyz",
		"order_id": "ord-1",
		"status": "FILLED",
		"filled_quantity": 50,
		"average_fill_price": 130.10,
		"exchange_fee": 6.51,
		"market_time": "2024-03-15T10:15:00"
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/push/order-update", bytes.NewReader(body))

	handler.PushOrderUpdate(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestPushOrderUpdateRequiresPlatformID(t *testing.T) {
	handler := NewInternalPushHandler(ws.NewHub())
	body := []byte(`{"order_id":"ord-1","status":"FILLED"}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/push/order-update", bytes.NewReader(body))

	handler.PushOrderUpdate(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestPushOrderBookUpdateAcceptsValidPayload(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	handler := NewInternalPushHandler(hub)
	body := []byte(`{
		"ticker": " arka ",
		"bids": [{"price": 129.50, "quantity": 300}],
		"asks": [{"price": 130.10, "quantity": 150}]
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/push/order-book-update", bytes.NewReader(body))

	handler.PushOrderBookUpdate(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestPushOrderBookUpdateRequiresTicker(t *testing.T) {
	handler := NewInternalPushHandler(ws.NewHub())
	body := []byte(`{"bids":[],"asks":[]}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/push/order-book-update", bytes.NewReader(body))

	handler.PushOrderBookUpdate(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestPushMarketEventAcceptsValidPayload(t *testing.T) {
	hub := ws.NewHub()
	go hub.Run()

	handler := NewInternalPushHandler(hub)
	body := []byte(`{
		"event_id": "event-1",
		"event_type": "NEWS",
		"headline": "ARKA announces earnings",
		"scope": "STOCK",
		"target": "ARKA",
		"magnitude": 0.05,
		"duration_ticks": 10,
		"market_time": "2024-03-15T10:15:00"
	}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/push/market-event", bytes.NewReader(body))

	handler.PushMarketEvent(recorder, request)

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusAccepted)
	}
}

func TestPushMarketEventRejectsWrongMethod(t *testing.T) {
	handler := NewInternalPushHandler(ws.NewHub())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/push/market-event", nil)

	handler.PushMarketEvent(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}
