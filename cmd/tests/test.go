// This command acts like fake internal microservices.
// It pushes updates into the websocket server over HTTP so broker demo clients
// can receive them through their websocket subscriptions.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"stock-exchange-ws/internal/ws"
)

const (
	baseURL    = "http://localhost:8080"
	platformID = "platform-xyz"
	ticker     = "AAPL"
)

type orderUpdatePushPayload struct {
	PlatformID       string  `json:"platform_id"`
	OrderID          string  `json:"order_id"`
	Status           string  `json:"status"`
	FilledQuantity   int64   `json:"filled_quantity"`
	AverageFillPrice float64 `json:"average_fill_price"`
	ExchangeFee      float64 `json:"exchange_fee"`
	MarketTime       string  `json:"market_time"`
}

func main() {
	price := 150.25
	sequence := int64(1)

	for {
		marketTime := time.Now().UTC().Format(time.RFC3339)
		price += 0.5

		postJSON("/internal/push/price-update", ws.PriceUpdatePayload{
			Ticker:     ticker,
			Price:      price,
			Change:     0.5,
			ChangePct:  0.33,
			Volume:     1000000 + sequence*100,
			MarketTime: marketTime,
		})

		postJSON("/internal/push/order-update", orderUpdatePushPayload{
			PlatformID:       platformID,
			OrderID:          fmt.Sprintf("demo-order-%03d", sequence),
			Status:           "FILLED",
			FilledQuantity:   10 + sequence,
			AverageFillPrice: price,
			ExchangeFee:      price * 0.001,
			MarketTime:       marketTime,
		})

		postJSON("/internal/push/order-book-update", ws.OrderBookUpdatePayload{
			Ticker: ticker,
			Bids: []ws.BookLevel{
				{Price: price - 0.10, Quantity: 300 + sequence},
				{Price: price - 0.20, Quantity: 450 + sequence},
			},
			Asks: []ws.BookLevel{
				{Price: price + 0.10, Quantity: 250 + sequence},
				{Price: price + 0.20, Quantity: 500 + sequence},
			},
		})

		postJSON("/internal/push/market-event", ws.MarketEventPayload{
			EventID:       fmt.Sprintf("demo-event-%03d", sequence),
			EventType:     "NEWS",
			Headline:      fmt.Sprintf("%s demo market event #%d", ticker, sequence),
			Scope:         "STOCK",
			Target:        ticker,
			Magnitude:     0.05,
			DurationTicks: 10,
			MarketTime:    marketTime,
		})

		log.Printf("posted demo update batch #%d for %s", sequence, ticker)
		sequence++
		time.Sleep(3 * time.Second)
	}
}

func postJSON(path string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("failed to encode payload for %s: %v", path, err)
	}

	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Fatalf("failed to post to %s: %v", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		log.Fatalf("post to %s rejected with status: %s", path, resp.Status)
	}
}
