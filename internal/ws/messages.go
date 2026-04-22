package ws

import "encoding/json"

type MessageType string

const (
	MessageConnected       MessageType = "CONNECTED"
	MessageSubscribe       MessageType = "SUBSCRIBE"
	MessagePriceUpdate     MessageType = "PRICE_UPDATE"
	MessageOrderUpdate     MessageType = "ORDER_UPDATE"
	MessageOrderBookUpdate MessageType = "ORDER_BOOK_UPDATE"
	MessageMarketEvent     MessageType = "MARKET_EVENT"
	MessagePlaceOrder      MessageType = "PLACE_ORDER"
	MessageOrderAck        MessageType = "ORDER_ACK"
	MessageOrderRejected   MessageType = "ORDER_REJECTED"
)

type Channel string

const (
	ChannelPriceFeed    Channel = "PRICE_FEED"
	ChannelOrderUpdates Channel = "ORDER_UPDATES"
	ChannelMarketEvents Channel = "MARKET_EVENTS"
	ChannelOrderBook    Channel = "ORDER_BOOK"
)

// writePump() will marshal this struct and send it to the client
type Envelope struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload,omitempty"` // any = interface{} (any type)
}

// readPump() will unmarshal this into the correct struct based on Type
type IncomingEnvelope struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type ConnectedPayload struct {
	PlatformID       string `json:"platform_id"`
	ServerMarketTime string `json:"server_market_time"`
}

type SubscribePayload struct {
	Channel Channel  `json:"channel"`
	Tickers []string `json:"tickers,omitempty"`
	Ticker  string   `json:"ticker,omitempty"`
}

type PriceUpdatePayload struct {
	Ticker     string  `json:"ticker"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangePct  float64 `json:"change_pct"`
	Volume     int64   `json:"volume"`
	MarketTime string  `json:"market_time"`
}

type OrderUpdatePayload struct {
	OrderID          string  `json:"order_id"`
	Status           string  `json:"status"`
	FilledQuantity   int64   `json:"filled_quantity"`
	AverageFillPrice float64 `json:"average_fill_price"`
	ExchangeFee      float64 `json:"exchange_fee"`
	MarketTime       string  `json:"market_time"`
}

type BookLevel struct {
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
}

type OrderBookUpdatePayload struct {
	Ticker string      `json:"ticker"`
	Bids   []BookLevel `json:"bids"`
	Asks   []BookLevel `json:"asks"`
}

type MarketEventPayload struct {
	EventID       string  `json:"event_id"`
	EventType     string  `json:"event_type"`
	Headline      string  `json:"headline"`
	Scope         string  `json:"scope"`
	Target        string  `json:"target"`
	Magnitude     float64 `json:"magnitude"`
	DurationTicks int64   `json:"duration_ticks"`
	MarketTime    string  `json:"market_time"`
}

type PlaceOrderPayload struct {
	PlatformUserID string `json:"platform_user_id"`
	InstrumentType string `json:"instrument_type"`
	InstrumentID   string `json:"instrument_id"`
	OrderType      string `json:"order_type"`
	Side           string `json:"side"`
	Quantity       int64  `json:"quantity"`
}

type OrderAckPayload struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type OrderRejectedPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewEnvelope(messageType MessageType, payload any) Envelope {
	return Envelope{
		Type:    messageType,
		Payload: payload,
	}
}
