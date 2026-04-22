package ws

import "github.com/gorilla/websocket"

type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan Envelope
	platformID string
}

func newClient(hub *Hub, conn *websocket.Conn, platformID string, sendBufferSize int) *Client {
	return &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan Envelope, sendBufferSize),
		platformID: platformID,
	}
}

func (c *Client) PlatformID() string {
	return c.platformID
}
