// This command acts like fake internal microservices.
// It publishes updates to Kafka topics so the websocket server consumes them
// and broadcasts them to connected clients through their subscriptions.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	kafka "github.com/segmentio/kafka-go"

	"stock-exchange-ws/internal/kafkaconsumer"
	"stock-exchange-ws/internal/ws"
)

const (
	platformID = "platform-xyz"
	ticker     = "AAPL"
)

func main() {
	brokers := kafkaBrokers()
	log.Printf("Connecting to Kafka brokers: %s", strings.Join(brokers, ","))

	writers := map[string]*kafka.Writer{
		kafkaconsumer.TopicPriceUpdates:     newWriter(brokers, kafkaconsumer.TopicPriceUpdates),
		kafkaconsumer.TopicOrderUpdates:     newWriter(brokers, kafkaconsumer.TopicOrderUpdates),
		kafkaconsumer.TopicOrderBookUpdates: newWriter(brokers, kafkaconsumer.TopicOrderBookUpdates),
		kafkaconsumer.TopicMarketEvents:     newWriter(brokers, kafkaconsumer.TopicMarketEvents),
	}
	defer func() {
		for _, w := range writers {
			w.Close()
		}
	}()

	price := 150.25
	sequence := int64(1)

	for {
		marketTime := time.Now().UTC().Format(time.RFC3339)
		price += 0.5

		produce(writers[kafkaconsumer.TopicPriceUpdates], ws.PriceUpdatePayload{
			Ticker:     ticker,
			Price:      price,
			Change:     0.5,
			ChangePct:  0.33,
			Volume:     1000000 + sequence*100,
			MarketTime: marketTime,
		})

		produce(writers[kafkaconsumer.TopicOrderUpdates], kafkaconsumer.OrderUpdateMessage{
			PlatformID:       platformID,
			OrderID:          fmt.Sprintf("demo-order-%03d", sequence),
			Status:           "FILLED",
			FilledQuantity:   10 + sequence,
			AverageFillPrice: price,
			ExchangeFee:      price * 0.001,
			MarketTime:       marketTime,
		})

		produce(writers[kafkaconsumer.TopicOrderBookUpdates], ws.OrderBookUpdatePayload{
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

		produce(writers[kafkaconsumer.TopicMarketEvents], ws.MarketEventPayload{
			EventID:       fmt.Sprintf("demo-event-%03d", sequence),
			EventType:     "NEWS",
			Headline:      fmt.Sprintf("%s demo market event #%d", ticker, sequence),
			Scope:         "STOCK",
			Target:        ticker,
			Magnitude:     0.05,
			DurationTicks: 10,
			MarketTime:    marketTime,
		})

		log.Printf("published demo update batch #%d for %s", sequence, ticker)
		sequence++
		time.Sleep(3 * time.Second)
	}
}

func kafkaBrokers() []string {
	env := os.Getenv("KAFKA_BROKERS")
	if env == "" {
		env = "localhost:19092"
	}
	return strings.Split(env, ",")
}

func newWriter(brokers []string, topic string) *kafka.Writer {
	return kafka.NewWriter(kafka.WriterConfig{
		Brokers:  brokers,
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})
}

func produce(w *kafka.Writer, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("failed to encode payload for topic %s: %v", w.Stats().Topic, err)
	}

	if err := w.WriteMessages(context.Background(), kafka.Message{Value: data}); err != nil {
		log.Fatalf("failed to write to topic %s: %v", w.Stats().Topic, err)
	}
}
