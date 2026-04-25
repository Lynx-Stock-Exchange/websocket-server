package main

import (
	"encoding/json"
	"log"
	"net/url"

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
	///
	///
	///
	// Test 1: Subscribe to PRICE_FEED
	log.Println("\n\nSubscribing to PRICE_FEED for AAPL and MSFT...")
	subscribeMsg := Envelope{
		Type: MessageSubscribe,
		Payload: SubscribePayload{
			Channel: ChannelPriceFeed,
			Tickers: []string{"AAPL", "MSFT"},
		},
	}
	err = conn.WriteJSON(subscribeMsg)
	if err != nil {
		log.Fatal("X Failed to send subscribe:", err)
	}
	log.Println("✓ Subscription sent")

	// Read price feed updates
	// it should receive nothing else
	for {
		var response Envelope
		err = conn.ReadJSON(&response)
		if err != nil {
			log.Println("X Connection closed or error:", err)
			return
		}
		data, _ := json.MarshalIndent(response, "", "  ")
		log.Printf("Received message:\n%s\n", string(data))

	}

}
