package ws

import (
	"encoding/json"
	"strings"
)

func (c *Client) handleSubscribe(raw json.RawMessage) bool {
	var payload SubscribePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}

	switch payload.Channel {
	case ChannelPriceFeed:
		return normalizeTickers(payload.Tickers) != nil
	case ChannelOrderUpdates, ChannelMarketEvents:
		return true
	case ChannelOrderBook:
		return normalizeTicker(payload.Ticker) != ""
	default:
		return false
	}
}

func normalizeTickers(tickers []string) []string {
	normalized := make([]string, 0, len(tickers))
	seen := make(map[string]struct{}, len(tickers))

	for _, ticker := range tickers {
		ticker = normalizeTicker(ticker)
		if ticker == "" {
			continue
		}
		if _, exists := seen[ticker]; exists {
			continue
		}

		seen[ticker] = struct{}{}
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
