package ws

import (
	"log"
)

type orderUpdatePublication struct {
	platformID string
	payload    OrderUpdatePayload
}

type ClientMessage struct {
	Client   *Client
	Envelope Envelope
}

type PendingOrderRequest struct {
	Client          *Client
	ClientRequestID string
}

type OrderCommandResult struct {
	ClientRequestID string
	PlatformID      string
	Accepted        bool
	OrderID         string
	Status          string
	Code            string
	Message         string
}

type SubscriptionRequest struct {
	Client  *Client
	Channel Channel
	Tickers []string
	Ticker  string
}

type Hub struct {
	register       chan *Client
	unregister     chan *Client
	subscribe      chan SubscriptionRequest
	clientMessages chan ClientMessage
	pendingOrders  chan PendingOrderRequest
	orderResults   chan OrderCommandResult
	priceUpdates   chan PriceUpdatePayload
	orderUpdates   chan orderUpdatePublication
	orderBooks     chan OrderBookUpdatePayload
	marketEvents   chan MarketEventPayload
	clients        map[*Client]bool

	// Stores all active clients and their subscriptions
	priceFeedByTicker       map[string]map[*Client]bool
	orderBookByTicker       map[string]map[*Client]bool
	orderUpdatesByPlatform  map[string]map[*Client]bool
	marketEventsSubscribers map[*Client]bool
	pendingOrderClients     map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		register:                make(chan *Client),
		unregister:              make(chan *Client),
		subscribe:               make(chan SubscriptionRequest),
		clientMessages:          make(chan ClientMessage),
		pendingOrders:           make(chan PendingOrderRequest),
		orderResults:            make(chan OrderCommandResult),
		priceUpdates:            make(chan PriceUpdatePayload),
		orderUpdates:            make(chan orderUpdatePublication),
		orderBooks:              make(chan OrderBookUpdatePayload),
		marketEvents:            make(chan MarketEventPayload),
		clients:                 make(map[*Client]bool),
		priceFeedByTicker:       make(map[string]map[*Client]bool),
		orderBookByTicker:       make(map[string]map[*Client]bool),
		orderUpdatesByPlatform:  make(map[string]map[*Client]bool),
		marketEventsSubscribers: make(map[*Client]bool),
		pendingOrderClients:     make(map[string]*Client),
	}
}

// Run starts the hub's main loop to handle client registration and unregistration, subscription requests.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			log.Println("Client registered. Client: ", client.remoteAddr(), " PlatformID: ", client.platformID)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case req := <-h.subscribe:
			h.applySubscription(req)
		case message := <-h.clientMessages:
			if h.clients[message.Client] {
				h.enqueue(message.Client, message.Envelope)
			}
		case req := <-h.pendingOrders:
			if h.clients[req.Client] {
				h.pendingOrderClients[req.ClientRequestID] = req.Client
			}
		case result := <-h.orderResults:
			h.completeOrderRequest(result)

		// Broadcast price updates to subscribed clients
		case payload := <-h.priceUpdates:
			for client := range h.priceFeedByTicker[payload.Ticker] {
				h.enqueue(client, NewEnvelope(MessagePriceUpdate, payload))
			}

		case update := <-h.orderUpdates:
			for client := range h.orderUpdatesByPlatform[update.platformID] {
				h.enqueue(client, NewEnvelope(MessageOrderUpdate, update.payload))
			}

		case payload := <-h.orderBooks:
			for client := range h.orderBookByTicker[payload.Ticker] {
				h.enqueue(client, NewEnvelope(MessageOrderBookUpdate, payload))
			}

		case payload := <-h.marketEvents:
			for client := range h.marketEventsSubscribers {
				h.enqueue(client, NewEnvelope(MessageMarketEvent, payload))
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

func (h *Hub) Send(client *Client, envelope Envelope) {
	h.clientMessages <- ClientMessage{
		Client:   client,
		Envelope: envelope,
	}
}

func (h *Hub) TrackOrderRequest(client *Client, clientRequestID string) {
	h.pendingOrders <- PendingOrderRequest{
		Client:          client,
		ClientRequestID: clientRequestID,
	}
}

func (h *Hub) CompleteOrderRequest(result OrderCommandResult) {
	h.orderResults <- result
}

func (h *Hub) PublishPriceUpdate(payload PriceUpdatePayload) {
	h.priceUpdates <- payload
}

func (h *Hub) PublishOrderUpdate(platformID string, payload OrderUpdatePayload) {
	h.orderUpdates <- orderUpdatePublication{
		platformID: platformID,
		payload:    payload,
	}
}

func (h *Hub) PublishOrderBookUpdate(payload OrderBookUpdatePayload) {
	h.orderBooks <- payload
}

func (h *Hub) PublishMarketEvent(payload MarketEventPayload) {
	h.marketEvents <- payload
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
			log.Printf("Client %s subscribed to price feed for ticker: %s\n", req.Client.PlatformID(), ticker)
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

func (h *Hub) enqueue(client *Client, envelope Envelope) {
	select {
	case client.send <- envelope:
	default:
		h.unregisterClient(client)
	}
}

func (h *Hub) unregisterClient(client *Client) {
	if !h.clients[client] {
		return
	}

	delete(h.clients, client)
	h.removeClientSubscriptions(client)
	h.removePendingOrderRequests(client)
	close(client.send)
	log.Println("Client unregistered. Client: ", client.remoteAddr(), " PlatformID: ", client.platformID)
}

func (h *Hub) completeOrderRequest(result OrderCommandResult) {
	client, exists := h.pendingOrderClients[result.ClientRequestID]
	if !exists {
		return
	}

	delete(h.pendingOrderClients, result.ClientRequestID)
	if !h.clients[client] {
		return
	}

	if result.Accepted {
		h.enqueue(client, NewEnvelope(MessageOrderAck, OrderAckPayload{
			OrderID: result.OrderID,
			Status:  result.Status,
		}))
		return
	}

	code := result.Code
	if code == "" {
		code = "ORDER_REJECTED"
	}
	message := result.Message
	if message == "" {
		message = "order rejected"
	}

	h.enqueue(client, NewEnvelope(MessageOrderRejected, OrderRejectedPayload{
		Code:    code,
		Message: message,
	}))
}

func (h *Hub) removePendingOrderRequests(client *Client) {
	for clientRequestID, pendingClient := range h.pendingOrderClients {
		if pendingClient == client {
			delete(h.pendingOrderClients, clientRequestID)
		}
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
