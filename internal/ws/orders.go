package ws

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"stock-exchange-ws/internal/services"
)

func (c *Client) handlePlaceOrder(raw json.RawMessage) bool {
	var payload PlaceOrderPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}

	normalizePlaceOrder(&payload)
	if err := validatePlaceOrder(payload); err != nil {
		c.hub.Send(c, NewEnvelope(MessageOrderRejected, OrderRejectedPayload{
			Code:    "INVALID_ORDER",
			Message: err.Error(),
		}))
		return true
	}

	if c.orderService == nil {
		c.hub.Send(c, NewEnvelope(MessageOrderRejected, OrderRejectedPayload{
			Code:    "ORDER_SERVICE_UNAVAILABLE",
			Message: "order placement service is not wired yet",
		}))
		return true
	}

	resp, err := c.orderService.PlaceOrder(context.Background(), services.PlaceOrderRequest{
		PlatformID:     c.platformID,
		PlatformUserID: payload.PlatformUserID,
		InstrumentType: payload.InstrumentType,
		InstrumentID:   payload.InstrumentID,
		OrderType:      payload.OrderType,
		Side:           payload.Side,
		Quantity:       payload.Quantity,
		LimitPrice:     payload.LimitPrice,
		ExpiresAt:      payload.ExpiresAt,
	})
	if err != nil {
		c.hub.Send(c, NewEnvelope(MessageOrderRejected, OrderRejectedPayload{
			Code:    rejectionCode(err),
			Message: err.Error(),
		}))
		return true
	}

	c.hub.Send(c, NewEnvelope(MessageOrderAck, OrderAckPayload{
		OrderID: resp.OrderID,
		Status:  resp.Status,
	}))
	return true
}

func normalizePlaceOrder(payload *PlaceOrderPayload) {
	payload.PlatformUserID = strings.TrimSpace(payload.PlatformUserID)
	payload.InstrumentType = strings.ToUpper(strings.TrimSpace(payload.InstrumentType))
	payload.InstrumentID = strings.ToUpper(strings.TrimSpace(payload.InstrumentID))
	payload.OrderType = strings.ToUpper(strings.TrimSpace(payload.OrderType))
	payload.Side = strings.ToUpper(strings.TrimSpace(payload.Side))
	if payload.ExpiresAt != nil {
		trimmed := strings.TrimSpace(*payload.ExpiresAt)
		if trimmed == "" {
			payload.ExpiresAt = nil
		} else {
			payload.ExpiresAt = &trimmed
		}
	}
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
	case payload.OrderType == "LIMIT" && payload.LimitPrice == nil:
		return errors.New("limit_price is required for LIMIT orders")
	case payload.OrderType == "LIMIT" && payload.LimitPrice != nil && *payload.LimitPrice <= 0:
		return errors.New("limit_price must be greater than 0")
	default:
		return nil
	}
}

type rejectionCoder interface {
	RejectionCode() string
}

func rejectionCode(err error) string {
	var coder rejectionCoder
	if errors.As(err, &coder) {
		return coder.RejectionCode()
	}

	return "ORDER_REJECTED"
}
