package services

import "context"

// TODO: Define the order placement service boundary.
// readPump should call this for PLACE_ORDER messages received over the same websocket connection.
// The response must be asynchronous on that connection as ORDER_ACK or ORDER_REJECTED.

type PlaceOrderRequest struct {
	PlatformID     string
	PlatformUserID string
	InstrumentType string
	InstrumentID   string
	OrderType      string
	Side           string
	Quantity       int64
	LimitPrice     *float64
	ExpiresAt      *string
}

type PlaceOrderResponse struct {
	OrderID string
	Status  string
}

type OrderService interface {
	PlaceOrder(ctx context.Context, req PlaceOrderRequest) (PlaceOrderResponse, error)
}
