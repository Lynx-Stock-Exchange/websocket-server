// This command acts like fake internal microservices.
// It publishes Kafka messages so broker demo clients can receive them through
// their websocket subscriptions, and it also acts as a fake order service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"

	"stock-exchange-ws/internal/kafkabus"
	"stock-exchange-ws/internal/ws"
)

const (
	platformID = "platform-xyz"
	ticker     = "AAPL"
)

type demoKafka struct {
	config      kafkabus.Config
	price       *kafka.Writer
	orderUpdate *kafka.Writer
	orderBook   *kafka.Writer
	marketEvent *kafka.Writer
	orderResult *kafka.Writer
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	config := kafkabus.ConfigFromEnv()
	log.Printf("Kafka brokers: %v", config.Brokers)

	demo := newDemoKafka(config)
	defer demo.close()

	go demo.runFakeOrderService(ctx)
	demo.publishMarketData(ctx)
}

func newDemoKafka(config kafkabus.Config) *demoKafka {
	return &demoKafka{
		config:      config,
		price:       kafkabus.NewTopicWriter(config.Brokers, config.Topics.PriceUpdates),
		orderUpdate: kafkabus.NewTopicWriter(config.Brokers, config.Topics.OrderUpdates),
		orderBook:   kafkabus.NewTopicWriter(config.Brokers, config.Topics.OrderBookUpdates),
		marketEvent: kafkabus.NewTopicWriter(config.Brokers, config.Topics.MarketEvents),
		orderResult: kafkabus.NewTopicWriter(config.Brokers, config.Topics.OrderCommandResults),
	}
}

func (d *demoKafka) close() {
	for _, writer := range []*kafka.Writer{d.price, d.orderUpdate, d.orderBook, d.marketEvent, d.orderResult} {
		if err := writer.Close(); err != nil {
			log.Printf("failed to close Kafka writer: %v", err)
		}
	}
}

func (d *demoKafka) publishMarketData(ctx context.Context) {
	price := 150.25
	sequence := int64(1)
	tickerTimer := time.NewTicker(3 * time.Second)
	defer tickerTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerTimer.C:
			marketTime := time.Now().UTC().Format(time.RFC3339)
			price += 0.5

			d.write(ctx, d.price, ticker, ws.PriceUpdatePayload{
				Ticker:     ticker,
				Price:      price,
				Change:     0.5,
				ChangePct:  0.33,
				Volume:     1000000 + sequence*100,
				MarketTime: marketTime,
			})

			d.write(ctx, d.orderUpdate, platformID, kafkabus.OrderUpdateMessage{
				PlatformID: platformID,
				Payload: ws.OrderUpdatePayload{
					OrderID:          fmt.Sprintf("demo-order-%03d", sequence),
					Status:           "FILLED",
					FilledQuantity:   10 + sequence,
					AverageFillPrice: price,
					ExchangeFee:      price * 0.001,
					MarketTime:       marketTime,
				},
			})

			d.write(ctx, d.orderBook, ticker, ws.OrderBookUpdatePayload{
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

			d.write(ctx, d.marketEvent, ticker, ws.MarketEventPayload{
				EventID:       fmt.Sprintf("demo-event-%03d", sequence),
				EventType:     "NEWS",
				Headline:      fmt.Sprintf("%s demo market event #%d", ticker, sequence),
				Scope:         "STOCK",
				Target:        ticker,
				Magnitude:     0.05,
				DurationTicks: 10,
				MarketTime:    marketTime,
			})

			log.Printf("published demo Kafka batch #%d for %s", sequence, ticker)
			sequence++
		}
	}
}

func (d *demoKafka) runFakeOrderService(ctx context.Context) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     d.config.Brokers,
		Topic:       d.config.Topics.OrderCommands,
		GroupID:     "fake-order-service",
		MinBytes:    1,
		MaxBytes:    10 * 1024 * 1024,
		StartOffset: kafka.LastOffset,
	})
	defer reader.Close()

	for {
		message, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("fake order service read failed: %v", err)
			time.Sleep(time.Second)
			continue
		}

		var command kafkabus.OrderCommandMessage
		if err := json.Unmarshal(message.Value, &command); err != nil {
			log.Printf("fake order service rejected malformed command: %v", err)
			continue
		}

		result := kafkabus.OrderCommandResultMessage{
			ClientRequestID: command.ClientRequestID,
			PlatformID:      command.PlatformID,
			Accepted:        true,
			OrderID:         fmt.Sprintf("ord-%d", time.Now().UnixNano()),
			Status:          "PENDING",
		}
		d.write(ctx, d.orderResult, command.PlatformID, result)
		log.Printf("fake order service accepted %s", command.ClientRequestID)
	}
}

func (d *demoKafka) write(ctx context.Context, writer *kafka.Writer, key string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("failed to encode Kafka message: %v", err)
	}

	if err := writer.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: body}); err != nil {
		log.Fatalf("failed to publish Kafka message: %v", err)
	}
}
