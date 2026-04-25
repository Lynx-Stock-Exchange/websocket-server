package main

import (
	"log"
	"net/url"
	"stock-exchange-ws/internal/ws"
	"time"

	"github.com/gorilla/websocket"
)

type MessageType string

const (
	MessageSubscribe   MessageType = "SUBSCRIBE"
	MessagePlaceOrder  MessageType = "PLACE_ORDER"
	MessagePriceUpdate MessageType = "PRICE_UPDATE"
	MessageConnected   MessageType = "CONNECTED"
	MessageOrderAck    MessageType = "ORDER_ACK"
)

type Channel string

const (
	ChannelPriceFeed    Channel = "PRICE_FEED"
	ChannelOrderUpdates Channel = "ORDER_UPDATES"
	ChannelOrderBook    Channel = "ORDER_BOOK"
)

type Envelope struct {
	Type    MessageType `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

type SubscribePayload struct {
	Channel Channel  `json:"channel"`
	Tickers []string `json:"tickers,omitempty"`
	Ticker  string   `json:"ticker,omitempty"`
}

func main() {
	// Build WebSocket URL with fake credentials
	u := url.URL{
		Scheme:   "ws",
		Host:     "localhost:8080",
		Path:     "/ws",
		RawQuery: "api_key=test-api-key&api_secret=test-api-secret",
	}

	log.Printf("Connecting to %s\n", u.String())

	// Connect to server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("X Connection failed:", err)
	}
	defer conn.Close()

	log.Println("✓ Connected to server")

	// Read CONNECTED message
	var msg Envelope
	err = conn.ReadJSON(&msg)
	if err != nil {
		log.Fatal("X Failed to read CONNECTED message:", err)
	}
	log.Printf("✓ Received: %+v\n", msg)

	go simulatePriceFeed(conn)

	time.Sleep(30 * time.Second)

}

func simulatePriceFeed(conn *websocket.Conn) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	prices := map[string]float64{
		"AAPL": 150.25,
		"MSFT": 320.50,
	}

	for range ticker.C {
		for symbol := range prices {
			prices[symbol] += 0.5
			payload := ws.PriceUpdatePayload{
				Ticker:     symbol,
				Price:      prices[symbol],
				Change:     0.5,
				ChangePct:  0.33,
				Volume:     1000000,
				MarketTime: time.Now().Format(time.RFC3339),
			}
			log.Printf("Publishing price update: %s = $%.2f\n", symbol, prices[symbol])

			// Send as an Envelope with PRICE_UPDATE type
			envelope := Envelope{
				Type:    MessagePriceUpdate,
				Payload: payload,
			}
			err := conn.WriteJSON(envelope)
			if err != nil {
				log.Fatal("X Failed to send price update:", err)
			}
			log.Println("✓ Price update sent")
		}
	}
}
