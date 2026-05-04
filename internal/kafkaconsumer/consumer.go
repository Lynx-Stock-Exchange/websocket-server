package kafkaconsumer

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	kafka "github.com/segmentio/kafka-go"

	"stock-exchange-ws/internal/ws"
)

const (
	TopicPriceUpdates     = "price-updates"
	TopicOrderUpdates     = "order-updates"
	TopicOrderBookUpdates = "order-book-updates"
	TopicMarketEvents     = "market-events"
	consumerGroup         = "websocket-server"
)

type Config struct {
	Brokers []string
}

type Consumer struct {
	hub    *ws.Hub
	config Config
}

func New(hub *ws.Hub, config Config) *Consumer {
	return &Consumer{hub: hub, config: config}
}

func (c *Consumer) Start(ctx context.Context) {
	go c.consume(ctx, TopicPriceUpdates, c.handlePriceUpdate)
	go c.consume(ctx, TopicOrderUpdates, c.handleOrderUpdate)
	go c.consume(ctx, TopicOrderBookUpdates, c.handleOrderBookUpdate)
	go c.consume(ctx, TopicMarketEvents, c.handleMarketEvent)
}

func (c *Consumer) consume(ctx context.Context, topic string, handler func([]byte)) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: c.config.Brokers,
		Topic:   topic,
		GroupID: consumerGroup,
	})
	defer r.Close()

	log.Printf("Kafka consumer started for topic: %s", topic)

	for {
		msg, err := r.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Printf("Kafka consumer stopped for topic: %s", topic)
				return
			}
			log.Printf("Kafka read error on topic %s: %v", topic, err)
			continue
		}
		handler(msg.Value)
	}
}

// OrderUpdateMessage is the canonical wire format for the order-updates topic.
// platform_id is embedded in the message body (not the Kafka key) so a single
// consumer can route updates to the correct platform without key inspection.
type OrderUpdateMessage struct {
	PlatformID       string  `json:"platform_id"`
	OrderID          string  `json:"order_id"`
	Status           string  `json:"status"`
	FilledQuantity   int64   `json:"filled_quantity"`
	AverageFillPrice float64 `json:"average_fill_price"`
	ExchangeFee      float64 `json:"exchange_fee"`
	MarketTime       string  `json:"market_time"`
}

func (c *Consumer) handlePriceUpdate(data []byte) {
	var payload ws.PriceUpdatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("price-updates: invalid message: %v", err)
		return
	}
	payload.Ticker = strings.ToUpper(strings.TrimSpace(payload.Ticker))
	if payload.Ticker == "" {
		log.Println("price-updates: missing ticker, skipping")
		return
	}
	c.hub.PublishPriceUpdate(payload)
}

func (c *Consumer) handleOrderUpdate(data []byte) {
	var msg OrderUpdateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("order-updates: invalid message: %v", err)
		return
	}
	msg.PlatformID = strings.TrimSpace(msg.PlatformID)
	if msg.PlatformID == "" {
		log.Println("order-updates: missing platform_id, skipping")
		return
	}
	c.hub.PublishOrderUpdate(msg.PlatformID, ws.OrderUpdatePayload{
		OrderID:          msg.OrderID,
		Status:           msg.Status,
		FilledQuantity:   msg.FilledQuantity,
		AverageFillPrice: msg.AverageFillPrice,
		ExchangeFee:      msg.ExchangeFee,
		MarketTime:       msg.MarketTime,
	})
}

func (c *Consumer) handleOrderBookUpdate(data []byte) {
	var payload ws.OrderBookUpdatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("order-book-updates: invalid message: %v", err)
		return
	}
	payload.Ticker = strings.ToUpper(strings.TrimSpace(payload.Ticker))
	if payload.Ticker == "" {
		log.Println("order-book-updates: missing ticker, skipping")
		return
	}
	c.hub.PublishOrderBookUpdate(payload)
}

func (c *Consumer) handleMarketEvent(data []byte) {
	var payload ws.MarketEventPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("market-events: invalid message: %v", err)
		return
	}
	c.hub.PublishMarketEvent(payload)
}
