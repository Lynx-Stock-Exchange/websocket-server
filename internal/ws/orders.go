package ws

import (
	"encoding/json"
	"errors"
	"strings"
)

func (c *Client) handlePlaceOrder(raw json.RawMessage) bool {
	var payload PlaceOrderPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}

	normalizePlaceOrder(&payload)
	if err := validatePlaceOrder(payload); err != nil {
		c.send <- NewEnvelope(MessageOrderRejected, OrderRejectedPayload{
			Code:    "INVALID_ORDER",
			Message: err.Error(),
		})
		return true
	}

	// TODO: Hand off to the order service and enqueue ORDER_ACK or ORDER_REJECTED with the result.
	c.send <- NewEnvelope(MessageOrderRejected, OrderRejectedPayload{
		Code:    "ORDER_SERVICE_UNAVAILABLE",
		Message: "order placement service is not wired yet",
	})
	return true
}

func normalizePlaceOrder(payload *PlaceOrderPayload) {
	payload.PlatformUserID = strings.TrimSpace(payload.PlatformUserID)
	payload.InstrumentType = strings.ToUpper(strings.TrimSpace(payload.InstrumentType))
	payload.InstrumentID = strings.ToUpper(strings.TrimSpace(payload.InstrumentID))
	payload.OrderType = strings.ToUpper(strings.TrimSpace(payload.OrderType))
	payload.Side = strings.ToUpper(strings.TrimSpace(payload.Side))
}

func validatePlaceOrder(payload PlaceOrderPayload) error {
	switch {
	case payload.PlatformUserID == "":
		return errors.New("platform_user_id is required")
	case payload.InstrumentType == "":
		return errors.New("instrument_type is required")
	case payload.InstrumentID == "":
		return errors.New("instrument_id is required")
	case payload.OrderType == "":
		return errors.New("order_type is required")
	case payload.Side == "":
		return errors.New("side is required")
	case payload.Quantity <= 0:
		return errors.New("quantity must be greater than 0")
	default:
		return nil
	}
}
