package services

import "context"

type PlaceOrderRequest struct {
	ClientRequestID string
	PlatformID      string
	PlatformUserID  string
	InstrumentType  string
	InstrumentID    string
	OrderType       string
	Side            string
	Quantity        int64
	LimitPrice      *float64
	ExpiresAt       *string
}

type OrderService interface {
	PlaceOrder(ctx context.Context, req PlaceOrderRequest) error
}
