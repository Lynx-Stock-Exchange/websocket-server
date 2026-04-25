package ws

import (
	"encoding/json"
	"log"
	"strings"
)

// handleSubscribe processes subscription requests from the client
func (c *Client) handleSubscribe(raw json.RawMessage) bool {
	var payload SubscribePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}

	switch payload.Channel {

	// PRICE_FEED
	case ChannelPriceFeed:
		// If no tickers specified
		tickers := normalizeTickers(payload.Tickers)
		if tickers == nil {
			return false
		}
		c.hub.Subscribe(SubscriptionRequest{
			Client:  c,
			Channel: ChannelPriceFeed,
			Tickers: tickers,
		})
		log.Println("Subscribed to price feed for tickers: ", tickers)
		return true

	// ORDER_UPDATES
	case ChannelOrderUpdates:
		c.hub.Subscribe(SubscriptionRequest{
			Client:  c,
			Channel: ChannelOrderUpdates,
		})
		return true
	case ChannelMarketEvents:
		c.hub.Subscribe(SubscriptionRequest{
			Client:  c,
			Channel: ChannelMarketEvents,
		})
		return true
	case ChannelOrderBook:
		ticker := normalizeTicker(payload.Ticker)
		if ticker == "" {
			return false
		}
		c.hub.Subscribe(SubscriptionRequest{
			Client:  c,
			Channel: ChannelOrderBook,
			Ticker:  ticker,
		})
		return true
	default:
		return false
	}
}

func normalizeTickers(tickers []string) []string {
	normalized := make([]string, 0, len(tickers))
	seen := make(map[string]bool, len(tickers))

	for _, ticker := range tickers {
		ticker = normalizeTicker(ticker)
		if ticker == "" {
			continue
		}
		if _, exists := seen[ticker]; exists {
			continue
		}

		seen[ticker] = true
		normalized = append(normalized, ticker)
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func normalizeTicker(ticker string) string {
	return strings.ToUpper(strings.TrimSpace(ticker))
}
