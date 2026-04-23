package ws

import "log"

type SubscriptionRequest struct {
	Client  *Client
	Channel Channel
	Tickers []string
	Ticker  string
}

type Hub struct {
	register                chan *Client
	unregister              chan *Client
	subscribe               chan SubscriptionRequest
	priceUpdates            chan PriceUpdatePayload
	clients                 map[*Client]bool
	priceFeedByTicker       map[string]map[*Client]bool
	orderBookByTicker       map[string]map[*Client]bool
	orderUpdatesByPlatform  map[string]map[*Client]bool
	marketEventsSubscribers map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{
		register:                make(chan *Client),
		unregister:              make(chan *Client),
		subscribe:               make(chan SubscriptionRequest),
		priceUpdates:            make(chan PriceUpdatePayload),
		clients:                 make(map[*Client]bool),
		priceFeedByTicker:       make(map[string]map[*Client]bool),
		orderBookByTicker:       make(map[string]map[*Client]bool),
		orderUpdatesByPlatform:  make(map[string]map[*Client]bool),
		marketEventsSubscribers: make(map[*Client]bool),
	}
}

// Run starts the hub's main loop to handle client registration and unregistration.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("Client registered. Client: ", client.conn.RemoteAddr(), " PlatformID: ", client.platformID)
		case client := <-h.unregister:
			if !h.clients[client] {
				continue
			}

			delete(h.clients, client)
			h.removeClientSubscriptions(client)
			close(client.send)
			log.Println("Client unregistered. Client: ", client.conn.RemoteAddr(), " PlatformID: ", client.platformID)
		case req := <-h.subscribe:
			h.applySubscription(req)
		case payload := <-h.priceUpdates:
			for client := range h.priceFeedByTicker[payload.Ticker] {
				client.send <- NewEnvelope(MessagePriceUpdate, payload)
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Subscribe(req SubscriptionRequest) {
	h.subscribe <- req
}

func (h *Hub) PublishPriceUpdate(payload PriceUpdatePayload) {
	h.priceUpdates <- payload
}

func (h *Hub) applySubscription(req SubscriptionRequest) {
	switch req.Channel {

	case ChannelPriceFeed:
		for _, ticker := range req.Tickers {
			if h.priceFeedByTicker[ticker] == nil {
				h.priceFeedByTicker[ticker] = make(map[*Client]bool)
			}
			h.priceFeedByTicker[ticker][req.Client] = true
			req.Client.subscriptions.PriceFeedTickers[ticker] = true
		}

	case ChannelOrderBook:
		if h.orderBookByTicker[req.Ticker] == nil {
			h.orderBookByTicker[req.Ticker] = make(map[*Client]bool)
		}
		h.orderBookByTicker[req.Ticker][req.Client] = true
		req.Client.subscriptions.OrderBookTickers[req.Ticker] = true

	case ChannelOrderUpdates:
		if h.orderUpdatesByPlatform[req.Client.platformID] == nil {
			h.orderUpdatesByPlatform[req.Client.platformID] = make(map[*Client]bool)
		}
		h.orderUpdatesByPlatform[req.Client.platformID][req.Client] = true
		req.Client.subscriptions.OrderUpdates = true

	case ChannelMarketEvents:
		h.marketEventsSubscribers[req.Client] = true
		req.Client.subscriptions.MarketEvents = true
	}
}

func (h *Hub) removeClientSubscriptions(client *Client) {
	for ticker := range client.subscriptions.PriceFeedTickers {
		delete(h.priceFeedByTicker[ticker], client)
		if len(h.priceFeedByTicker[ticker]) == 0 {
			delete(h.priceFeedByTicker, ticker)
		}
	}

	for ticker := range client.subscriptions.OrderBookTickers {
		delete(h.orderBookByTicker[ticker], client)
		if len(h.orderBookByTicker[ticker]) == 0 {
			delete(h.orderBookByTicker, ticker)
		}
	}

	if client.subscriptions.OrderUpdates {
		delete(h.orderUpdatesByPlatform[client.platformID], client)
		if len(h.orderUpdatesByPlatform[client.platformID]) == 0 {
			delete(h.orderUpdatesByPlatform, client.platformID)
		}
	}

	if client.subscriptions.MarketEvents {
		delete(h.marketEventsSubscribers, client)
	}
}
