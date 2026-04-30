package kafkabus

import (
	"os"
	"strings"
)

type Topics struct {
	PriceUpdates        string
	OrderUpdates        string
	OrderBookUpdates    string
	MarketEvents        string
	OrderCommands       string
	OrderCommandResults string
}

type Config struct {
	Brokers []string
	GroupID string
	Topics  Topics
}

func ConfigFromEnv() Config {
	return Config{
		Brokers: splitCSV(envOrDefault("KAFKA_BROKERS", "localhost:19092")),
		GroupID: envOrDefault("KAFKA_GROUP_ID", "websocket-server"),
		Topics: Topics{
			PriceUpdates:        envOrDefault("KAFKA_TOPIC_PRICE_UPDATES", "lynx.price-updates"),
			OrderUpdates:        envOrDefault("KAFKA_TOPIC_ORDER_UPDATES", "lynx.order-updates"),
			OrderBookUpdates:    envOrDefault("KAFKA_TOPIC_ORDER_BOOK_UPDATES", "lynx.order-book-updates"),
			MarketEvents:        envOrDefault("KAFKA_TOPIC_MARKET_EVENTS", "lynx.market-events"),
			OrderCommands:       envOrDefault("KAFKA_TOPIC_ORDER_COMMANDS", "lynx.order-commands"),
			OrderCommandResults: envOrDefault("KAFKA_TOPIC_ORDER_COMMAND_RESULTS", "lynx.order-command-results"),
		},
	}
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}
