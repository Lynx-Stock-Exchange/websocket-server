package kafkabus

import (
	"context"
	"encoding/json"

	"github.com/segmentio/kafka-go"

	"stock-exchange-ws/internal/services"
)

type OrderProducer struct {
	writer *kafka.Writer
}

func NewOrderProducer(config Config) *OrderProducer {
	return &OrderProducer{
		writer: NewTopicWriter(config.Brokers, config.Topics.OrderCommands),
	}
}

func NewTopicWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
	}
}

func (p *OrderProducer) PlaceOrder(ctx context.Context, req services.PlaceOrderRequest) error {
	body, err := json.Marshal(OrderCommandFromRequest(req))
	if err != nil {
		return err
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(req.PlatformID),
		Value: body,
	})
}

func (p *OrderProducer) Close() error {
	return p.writer.Close()
}
