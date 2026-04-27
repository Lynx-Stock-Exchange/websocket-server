// It is an implementation of a fake price Feed
// It sends data via HTTP POST to the internal push endpoint of the WebSocket server every 3 seconds.
// The server then broadcasts these updates to any subscribed WebSocket clients.
package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"stock-exchange-ws/internal/ws"
)

func main() {
	postFakePriceFeed()
}

func postFakePriceFeed() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	prices := map[string]float64{
		"AAPL": 150.25,
		"MSFT": 320.50,
		"GOOG": 2800.75,
		"BTC":  45000.00,
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
			// Post to internal push endpoint
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
