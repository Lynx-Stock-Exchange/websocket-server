package ws

import "github.com/gorilla/websocket"

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
	subscriptions ClientSubscriptions
}

func newClient(hub *Hub, conn *websocket.Conn, platformID string, sendBufferSize int) *Client {
	return &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan Envelope, sendBufferSize),
		platformID: platformID,
		subscriptions: ClientSubscriptions{
			PriceFeedTickers: make(map[string]bool),
			OrderBookTickers: make(map[string]bool),
		},
	}
}

func (c *Client) PlatformID() string {
	return c.platformID
}
