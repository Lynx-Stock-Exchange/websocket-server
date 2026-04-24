package ws

import (
	"log"
	"time"
)

const (
	maxMessageSize = 5120
	pongWait       = 60 * time.Second
)

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var envelope IncomingEnvelope
		if err := c.conn.ReadJSON(&envelope); err != nil {
			return
		}

		log.Printf(" Received from %s: %v\n", c.platformID, envelope.Type)

		switch envelope.Type {
		case MessageSubscribe:
			if !c.handleSubscribe(envelope.Payload) {
				return
			}
		case MessagePlaceOrder:
			if !c.handlePlaceOrder(envelope.Payload) {
				return
			}
		default:
			return
		}
	}
}
