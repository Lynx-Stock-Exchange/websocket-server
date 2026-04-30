package kafkabus

import (
	"stock-exchange-ws/internal/services"
	"stock-exchange-ws/internal/ws"
)

type OrderUpdateMessage struct {
	PlatformID string                `json:"platform_id"`
	Payload    ws.OrderUpdatePayload `json:"payload"`
}

type OrderCommandMessage struct {
	ClientRequestID string   `json:"client_request_id"`
	PlatformID      string   `json:"platform_id"`
	PlatformUserID  string   `json:"platform_user_id"`
	InstrumentType  string   `json:"instrument_type"`
	InstrumentID    string   `json:"instrument_id"`
	OrderType       string   `json:"order_type"`
	Side            string   `json:"side"`
	Quantity        int64    `json:"quantity"`
	LimitPrice      *float64 `json:"limit_price,omitempty"`
	ExpiresAt       *string  `json:"expires_at,omitempty"`
}

type OrderCommandResultMessage struct {
	ClientRequestID string `json:"client_request_id"`
	PlatformID      string `json:"platform_id"`
	Accepted        bool   `json:"accepted"`
	OrderID         string `json:"order_id,omitempty"`
	Status          string `json:"status,omitempty"`
	Code            string `json:"code,omitempty"`
	Message         string `json:"message,omitempty"`
}

func OrderCommandFromRequest(req services.PlaceOrderRequest) OrderCommandMessage {
	return OrderCommandMessage{
		ClientRequestID: req.ClientRequestID,
		PlatformID:      req.PlatformID,
		PlatformUserID:  req.PlatformUserID,
		InstrumentType:  req.InstrumentType,
		InstrumentID:    req.InstrumentID,
		OrderType:       req.OrderType,
		Side:            req.Side,
		Quantity:        req.Quantity,
		LimitPrice:      req.LimitPrice,
		ExpiresAt:       req.ExpiresAt,
	}
}

func OrderCommandResultToWS(msg OrderCommandResultMessage) ws.OrderCommandResult {
	return ws.OrderCommandResult{
		ClientRequestID: msg.ClientRequestID,
		PlatformID:      msg.PlatformID,
		Accepted:        msg.Accepted,
		OrderID:         msg.OrderID,
		Status:          msg.Status,
		Code:            msg.Code,
		Message:         msg.Message,
	}
}
