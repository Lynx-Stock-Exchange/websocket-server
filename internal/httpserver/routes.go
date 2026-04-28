package httpserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"stock-exchange-ws/internal/ws"
)

type InternalPushHandler struct {
	hub *ws.Hub
}

type orderUpdatePushPayload struct {
	PlatformID       string  `json:"platform_id"`
	OrderID          string  `json:"order_id"`
	Status           string  `json:"status"`
	FilledQuantity   int64   `json:"filled_quantity"`
	AverageFillPrice float64 `json:"average_fill_price"`
	ExchangeFee      float64 `json:"exchange_fee"`
	MarketTime       string  `json:"market_time"`
}

func NewInternalPushHandler(hub *ws.Hub) *InternalPushHandler {
	return &InternalPushHandler{hub: hub}
}

func (h *InternalPushHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/internal/push/price-update", h.PushPriceUpdate)
	mux.HandleFunc("/internal/push/order-update", h.PushOrderUpdate)
	mux.HandleFunc("/internal/push/order-book-update", h.PushOrderBookUpdate)
	mux.HandleFunc("/internal/push/market-event", h.PushMarketEvent)
}

func (h *InternalPushHandler) PushPriceUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.hub == nil {
		http.Error(w, "websocket hub is not configured", http.StatusInternalServerError)
		return
	}

	var payload ws.PriceUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid price update payload", http.StatusBadRequest)
		return
	}

	payload.Ticker = strings.ToUpper(strings.TrimSpace(payload.Ticker))
	if payload.Ticker == "" {
		http.Error(w, "ticker is required", http.StatusBadRequest)
		return
	}

	h.hub.PublishPriceUpdate(payload)
	w.WriteHeader(http.StatusAccepted)
}

func (h *InternalPushHandler) PushOrderUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.hub == nil {
		http.Error(w, "websocket hub is not configured", http.StatusInternalServerError)
		return
	}

	var body orderUpdatePushPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid order update payload", http.StatusBadRequest)
		return
	}

	body.PlatformID = strings.TrimSpace(body.PlatformID)
	if body.PlatformID == "" {
		http.Error(w, "platform_id is required", http.StatusBadRequest)
		return
	}

	h.hub.PublishOrderUpdate(body.PlatformID, ws.OrderUpdatePayload{
		OrderID:          body.OrderID,
		Status:           body.Status,
		FilledQuantity:   body.FilledQuantity,
		AverageFillPrice: body.AverageFillPrice,
		ExchangeFee:      body.ExchangeFee,
		MarketTime:       body.MarketTime,
	})
	w.WriteHeader(http.StatusAccepted)
}

func (h *InternalPushHandler) PushOrderBookUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.hub == nil {
		http.Error(w, "websocket hub is not configured", http.StatusInternalServerError)
		return
	}

	var payload ws.OrderBookUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid order book update payload", http.StatusBadRequest)
		return
	}

	payload.Ticker = strings.ToUpper(strings.TrimSpace(payload.Ticker))
	if payload.Ticker == "" {
		http.Error(w, "ticker is required", http.StatusBadRequest)
		return
	}

	h.hub.PublishOrderBookUpdate(payload)
	w.WriteHeader(http.StatusAccepted)
}

func (h *InternalPushHandler) PushMarketEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.hub == nil {
		http.Error(w, "websocket hub is not configured", http.StatusInternalServerError)
		return
	}

	var payload ws.MarketEventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid market event payload", http.StatusBadRequest)
		return
	}

	h.hub.PublishMarketEvent(payload)
	w.WriteHeader(http.StatusAccepted)
}
