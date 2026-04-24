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

	// Wait a bit to receive updates
	// time.Sleep(2 * time.Second)

	// Read subscription response
	var response Envelope
	err = conn.ReadJSON(&response)
	if err != nil {
		log.Println("X Connection closed or error:", err)
		return
	}
	data, _ := json.MarshalIndent(response, "", "  ")
	log.Printf("Received message:\n%s\n", string(data))

	////
	////
	////
	////

	// Test 2: Subscribe to ORDER_UPDATES
	log.Println("\n\n Subscribing to ORDER_UPDATES...")
	orderUpdateMsg := Envelope{
		Type: MessageSubscribe,
		Payload: SubscribePayload{
			Channel: ChannelOrderUpdates,
		},
	}
	err = conn.WriteJSON(orderUpdateMsg)
	if err != nil {
		log.Fatal("X Failed to subscribe to order updates:", err)
	}
	log.Println("✓ Order updates subscription sent")

	// Read Order updates response
	var response2 Envelope
	err = conn.ReadJSON(&response2)
	if err != nil {
		log.Println("X Connection closed or error:", err)
		return
	}
	data2, _ := json.MarshalIndent(response2, "", "  ")
	log.Printf("Received message:\n%s\n", string(data2))

	////
	////
	////
	////

	// Test 3: Place an order
	log.Println("\n\nPlacing test order...")
	orderPayload := map[string]interface{}{
		"platform_user_id": "user-456",
		"instrument_type":  "EQUITY",
		"instrument_id":    "AAPL",
		"order_type":       "LIMIT",
		"side":             "BUY",
		"quantity":         100,
		"limit_price":      150.50,
	}
	placeOrderMsg := Envelope{
		Type:    MessagePlaceOrder,
		Payload: orderPayload,
	}
	err = conn.WriteJSON(placeOrderMsg)
	if err != nil {
		log.Fatal("X Failed to place order:", err)
	}
	log.Println("✓ Order placed")

	// Read place order response
	var response3 Envelope
	err = conn.ReadJSON(&response3)
	if err != nil {
		log.Println("X Connection closed or error:", err)
		return
	}
	data3, _ := json.MarshalIndent(response3, "", "  ")
	log.Printf("\nReceived message:\n%s\n", string(data3))

	////
	////
	////
	////

	log.Println("\n✓ Test completed")

}
