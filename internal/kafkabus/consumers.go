package kafkabus

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"

	"stock-exchange-ws/internal/ws"
)

type Consumers struct {
	config  Config
	hub     *ws.Hub
	readers []*kafka.Reader
}

func NewConsumers(config Config, hub *ws.Hub) *Consumers {
	return &Consumers{
		config: config,
		hub:    hub,
	}
}

func (c *Consumers) Start(ctx context.Context) {
	c.consume(ctx, c.config.Topics.PriceUpdates, "price updates", c.handlePriceUpdate)
	c.consume(ctx, c.config.Topics.OrderUpdates, "order updates", c.handleOrderUpdate)
	c.consume(ctx, c.config.Topics.OrderBookUpdates, "order book updates", c.handleOrderBookUpdate)
	c.consume(ctx, c.config.Topics.MarketEvents, "market events", c.handleMarketEvent)
	c.consume(ctx, c.config.Topics.OrderCommandResults, "order command results", c.handleOrderCommandResult)
}

func (c *Consumers) Close() error {
	var result error
	for _, reader := range c.readers {
		if err := reader.Close(); err != nil {
			result = errors.Join(result, err)
		}
	}

	return result
}

func (c *Consumers) consume(ctx context.Context, topic string, label string, handler func([]byte) error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     c.config.Brokers,
		Topic:       topic,
		GroupID:     c.config.GroupID + "-" + strings.ReplaceAll(topic, ".", "-"),
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.LastOffset,
	})
	c.readers = append(c.readers, reader)

	go func() {
		for {
			message, err := reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("Kafka %s consumer read failed: %v", label, err)
				time.Sleep(time.Second)
				continue
			}

			if err := handler(message.Value); err != nil {
				log.Printf("Kafka %s message rejected: %v", label, err)
			}
		}
	}()
}

func (c *Consumers) handlePriceUpdate(data []byte) error {
	var payload ws.PriceUpdatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	payload.Ticker = strings.ToUpper(strings.TrimSpace(payload.Ticker))
	if payload.Ticker == "" {
		return errors.New("price update ticker is required")
	}

	c.hub.PublishPriceUpdate(payload)
	return nil
}

func (c *Consumers) handleOrderUpdate(data []byte) error {
	var message OrderUpdateMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}

	message.PlatformID = strings.TrimSpace(message.PlatformID)
	if message.PlatformID == "" {
		return errors.New("order update platform_id is required")
	}

	c.hub.PublishOrderUpdate(message.PlatformID, message.Payload)
	return nil
}

func (c *Consumers) handleOrderBookUpdate(data []byte) error {
	var payload ws.OrderBookUpdatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	payload.Ticker = strings.ToUpper(strings.TrimSpace(payload.Ticker))
	if payload.Ticker == "" {
		return errors.New("order book update ticker is required")
	}

	c.hub.PublishOrderBookUpdate(payload)
	return nil
}

func (c *Consumers) handleMarketEvent(data []byte) error {
	var payload ws.MarketEventPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	c.hub.PublishMarketEvent(payload)
	return nil
}

func (c *Consumers) handleOrderCommandResult(data []byte) error {
	var message OrderCommandResultMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}

	if strings.TrimSpace(message.ClientRequestID) == "" {
		return errors.New("order command result client_request_id is required")
	}

	c.hub.CompleteOrderRequest(OrderCommandResultToWS(message))
	return nil
}
