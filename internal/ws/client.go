package ws

import (
	"net"

	"github.com/gorilla/websocket"

	"stock-exchange-ws/internal/services"
)

type ClientSubscriptions struct {
	PriceFeedTickers map[string]bool
	OrderBookTickers map[string]bool
	OrderUpdates     bool
	MarketEvents     bool
}

type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan Envelope
	platformID    string
	orderService  services.OrderService
	subscriptions ClientSubscriptions
}

func newClient(hub *Hub, conn *websocket.Conn, platformID string, orderService services.OrderService, sendBufferSize int) *Client {
	return &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan Envelope, sendBufferSize),
		platformID:   platformID,
		orderService: orderService,
		subscriptions: ClientSubscriptions{
			PriceFeedTickers: make(map[string]bool),
			OrderBookTickers: make(map[string]bool),
		},
	}
}

func (c *Client) PlatformID() string {
	return c.platformID
}

func (c *Client) remoteAddr() net.Addr {
	if c.conn == nil {
		return nil
	}

	return c.conn.RemoteAddr()
}
