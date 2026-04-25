package ws

import (
	"encoding/json"
	"log"
)

func (c *Client) handlePriceUpdate(raw json.RawMessage) bool {
	var payload PriceUpdatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("Failed to unmarshal price update: %v\n", err)
		return false
	}

	log.Printf("Price feed received update: %s = $%.2f\n", payload.Ticker, payload.Price)

	// Publish to all subscribed clients
	c.hub.PublishPriceUpdate(payload)

	return true
}
