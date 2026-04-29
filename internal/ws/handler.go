package ws

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"stock-exchange-ws/internal/services"
)

const defaultSendBufferSize = 256

type MarketTimeProvider interface {
	ServerMarketTime() string
}

type Handler struct {
	hub                *Hub
	authenticator      Authenticator
	orderService       services.OrderService
	marketTimeProvider MarketTimeProvider
	upgrader           websocket.Upgrader
	sendBufferSize     int
}

type HandlerConfig struct {
	Hub                *Hub
	Authenticator      Authenticator
	OrderService       services.OrderService
	MarketTimeProvider MarketTimeProvider
	Upgrader           websocket.Upgrader
	SendBufferSize     int
}

// Websocket main handler - responsible for handling incoming websocket connections, authenticating clients, and registering them with the hub.
func NewHandler(config HandlerConfig) *Handler {
	if config.SendBufferSize == 0 {
		config.SendBufferSize = defaultSendBufferSize
	}

	return &Handler{
		hub:                config.Hub,
		authenticator:      config.Authenticator,
		orderService:       config.OrderService,
		marketTimeProvider: config.MarketTimeProvider,
		upgrader:           config.Upgrader,
		sendBufferSize:     config.SendBufferSize,
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

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := newClient(h.hub, conn, platform.ID, h.orderService, h.sendBufferSize)
	h.hub.Register(client)

	log.Printf("\n\nClient connected - Platform: %s, Remote: %s\n", platform.ID, conn.RemoteAddr())

	go client.writePump()
	h.hub.Send(client, NewEnvelope(MessageConnected, ConnectedPayload{
		PlatformID:       platform.ID,
		ServerMarketTime: h.serverMarketTime(),
	}))
	go client.readPump()
}

func (h *Handler) serverMarketTime() string {
	if h.marketTimeProvider != nil {
		return h.marketTimeProvider.ServerMarketTime()
	}

	return time.Now().UTC().Format(time.RFC3339)
}
