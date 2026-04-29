package main

import (
	"encoding/json"
	"log"
	"net/url"

	"github.com/gorilla/websocket"

	"stock-exchange-ws/internal/ws"
)

func main() {
	conn := connect()
	defer conn.Close()

	readConnected(conn)

	subscribe(conn, ws.SubscribePayload{
		Channel: ws.ChannelPriceFeed,
		Tickers: []string{"AAPL"},
	})
	log.Println("Subscribed to PRICE_FEED for AAPL")

	subscribe(conn, ws.SubscribePayload{
		Channel: ws.ChannelOrderBook,
		Ticker:  "AAPL",
	})
	log.Println("Subscribed to ORDER_BOOK for AAPL")

	subscribe(conn, ws.SubscribePayload{
		Channel: ws.ChannelOrderUpdates,
	})
	log.Println("Subscribed to ORDER_UPDATES")

	subscribe(conn, ws.SubscribePayload{
		Channel: ws.ChannelMarketEvents,
	})
	log.Println("Subscribed to MARKET_EVENTS")

	printIncoming(conn)
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

func readConnected(conn *websocket.Conn) {
	var msg ws.Envelope
	if err := conn.ReadJSON(&msg); err != nil {
		log.Fatal("failed to read CONNECTED message:", err)
	}

	printEnvelope("Received", msg)
}

func subscribe(conn *websocket.Conn, payload ws.SubscribePayload) {
	if err := conn.WriteJSON(ws.NewEnvelope(ws.MessageSubscribe, payload)); err != nil {
		log.Fatal("failed to send subscribe:", err)
	}
}

func printIncoming(conn *websocket.Conn) {
	for {
		var msg ws.Envelope
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("connection closed or read failed:", err)
			return
		}

		printEnvelope("Received", msg)
	}
}

func printEnvelope(prefix string, msg ws.Envelope) {
	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		log.Printf("%s: %#v", prefix, msg)
		return
	}

	log.Printf("%s:\n%s", prefix, string(data))
}
