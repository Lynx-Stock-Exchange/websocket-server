package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"stock-exchange-ws/internal/ws"
)

func main() {
	u := url.URL{
		Scheme:   "ws",
		Host:     "localhost:8080",
		Path:     "/ws",
		RawQuery: "api_key=test-api-key&api_secret=test-api-secret",
	}

	log.Printf("Connecting to %s\n", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("connection failed:", err)
	}
	defer conn.Close()

	var connected ws.Envelope
	if err := conn.ReadJSON(&connected); err != nil {
		log.Fatal("failed to read CONNECTED message:", err)
	}
	log.Printf("Received: %+v\n", connected)

	subscribe := ws.Envelope{
		Type: ws.MessageSubscribe,
		Payload: ws.SubscribePayload{
			Channel: ws.ChannelPriceFeed,
			Tickers: []string{"AAPL", "MSFT"},
		},
	}
	if err := conn.WriteJSON(subscribe); err != nil {
		log.Fatal("failed to send subscription:", err)
	}
	log.Println("Subscription sent")

	go postFakePriceFeed()
	readMessages(conn)
}

func readMessages(conn *websocket.Conn) {
	for {
		var response ws.Envelope
		if err := conn.ReadJSON(&response); err != nil {
			log.Println("connection closed or read error:", err)
			return
		}

		data, _ := json.MarshalIndent(response, "", "  ")
		log.Printf("Received websocket message:\n%s\n", string(data))
	}
}

func postFakePriceFeed() {
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

			body, err := json.Marshal(payload)
			if err != nil {
				log.Fatal("failed to encode price update:", err)
			}

			resp, err := http.Post("http://localhost:8080/internal/push/price-update", "application/json", bytes.NewReader(body))
			if err != nil {
				log.Fatal("failed to post price update:", err)
			}
			_ = resp.Body.Close()

			if resp.StatusCode != http.StatusAccepted {
				log.Fatalf("price update rejected with status: %s", resp.Status)
			}

			log.Printf("Posted price update: %s = %.2f\n", symbol, prices[symbol])
		}
	}
}
