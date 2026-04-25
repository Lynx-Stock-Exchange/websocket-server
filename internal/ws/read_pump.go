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
		// Read incoming JSON
		var envelope IncomingEnvelope // this contains the type and payload
		if err := c.conn.ReadJSON(&envelope); err != nil {
			return
		}

		log.Printf(" Received from %s: %v\n", c.platformID, envelope.Type)

		// Handle the message based on its type
		switch envelope.Type {

		// Subscribe to ticks
		case MessageSubscribe:
			if !c.handleSubscribe(envelope.Payload) {
				return
			}
		// Price update from Price Sim API Microservice
		case MessagePriceUpdate:
			if !c.handlePriceUpdate(envelope.Payload) {
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
