package main

import (
	"encoding/json"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"stock-exchange-ws/internal/ws"
)

func main() {
	conn := connect()
	defer conn.Close()

	readAndPrint(conn, "CONNECTED")

	limitPrice := 150.00
	expiresAt := time.Now().UTC().Add(30 * time.Minute).Format(time.RFC3339)
	sendPlaceOrder(conn, "valid LIMIT order", ws.PlaceOrderPayload{
		PlatformUserID: "user-demo-001",
		InstrumentType: "STOCK",
		InstrumentID:   "AAPL",
		OrderType:      "LIMIT",
		Side:           "BUY",
		Quantity:       10,
		LimitPrice:     &limitPrice,
		ExpiresAt:      &expiresAt,
	})
	readAndPrint(conn, "ORDER_ACK")

	sendPlaceOrder(conn, "invalid LIMIT order without limit_price", ws.PlaceOrderPayload{
		PlatformUserID: "user-demo-001",
		InstrumentType: "STOCK",
		InstrumentID:   "AAPL",
		OrderType:      "LIMIT",
		Side:           "BUY",
		Quantity:       10,
	})
	readAndPrint(conn, "ORDER_REJECTED")
}

func connect() *websocket.Conn {
	u := url.URL{
		Scheme:   "ws",
		Host:     "localhost:8080",
		Path:     "/ws",
		RawQuery: "api_key=test-api-key&api_secret=test-api-secret",
	}

	log.Printf("Connecting to %s", u.String())
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("connection failed:", err)
	}

	return conn
}

func sendPlaceOrder(conn *websocket.Conn, label string, payload ws.PlaceOrderPayload) {
	log.Printf("Sending %s", label)
	if err := conn.WriteJSON(ws.NewEnvelope(ws.MessagePlaceOrder, payload)); err != nil {
		log.Fatalf("failed to send %s: %v", label, err)
	}
}

func readAndPrint(conn *websocket.Conn, expected string) {
	var msg ws.Envelope
	if err := conn.ReadJSON(&msg); err != nil {
		log.Fatalf("failed to read %s: %v", expected, err)
	}

	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		log.Printf("Received %s: %#v", expected, msg)
		return
	}

	log.Printf("Received %s:\n%s", expected, string(data))
}
