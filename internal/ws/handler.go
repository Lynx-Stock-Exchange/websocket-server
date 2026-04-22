package ws

import (
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const defaultSendBufferSize = 256

type MarketTimeProvider interface {
	ServerMarketTime() string
}

type Handler struct {
	hub                *Hub
	authenticator      Authenticator
	marketTimeProvider MarketTimeProvider
	upgrader           websocket.Upgrader
	sendBufferSize     int
}

func NewHandler(config Handler) *Handler {
	if config.sendBufferSize == 0 {
		config.sendBufferSize = defaultSendBufferSize
	}

	return &Handler{
		hub:                config.hub,
		authenticator:      config.authenticator,
		marketTimeProvider: config.marketTimeProvider,
		upgrader:           config.upgrader,
		sendBufferSize:     config.sendBufferSize,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.hub == nil {
		http.Error(w, "websocket hub is not configured", http.StatusInternalServerError)
		return
	}
	if h.authenticator == nil {
		http.Error(w, "websocket authenticator is not configured", http.StatusInternalServerError)
		return
	}

	credentials, err := credentialsFromRequest(r)
	if err != nil {
		http.Error(w, "missing api_key or api_secret", http.StatusUnauthorized)
		return
	}

	platform, err := h.authenticator.AuthenticatePlatform(r.Context(), credentials)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrUnauthorized) {
			status = http.StatusUnauthorized
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	if platform.ID == "" {
		http.Error(w, "authenticated platform has no platform_id", http.StatusInternalServerError)
		return
	}

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := newClient(h.hub, conn, platform.ID, h.sendBufferSize)
	h.hub.Register(client)

	go client.writePump()
	client.send <- NewEnvelope(MessageConnected, ConnectedPayload{
		PlatformID:       platform.ID,
		ServerMarketTime: h.serverMarketTime(),
	})
	go client.readPump()
}

func (h *Handler) serverMarketTime() string {
	if h.marketTimeProvider != nil {
		return h.marketTimeProvider.ServerMarketTime()
	}

	return time.Now().UTC().Format(time.RFC3339)
}
