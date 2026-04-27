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

func NewInternalPushHandler(hub *ws.Hub) *InternalPushHandler {
	return &InternalPushHandler{hub: hub}
}

func (h *InternalPushHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/internal/push/price-update", h.PushPriceUpdate)
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
