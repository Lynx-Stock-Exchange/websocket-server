package ws

import (
	"testing"
	"time"
)

func TestPublishOrderUpdateRoutesByPlatform(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	target := newClient(hub, nil, "platform-a", nil, 1)
	other := newClient(hub, nil, "platform-b", nil, 1)

	hub.Subscribe(SubscriptionRequest{Client: target, Channel: ChannelOrderUpdates})
	hub.Subscribe(SubscriptionRequest{Client: other, Channel: ChannelOrderUpdates})
	hub.PublishOrderUpdate("platform-a", OrderUpdatePayload{
		OrderID: "ord-1",
		Status:  "FILLED",
	})

	msg := readEnvelope(t, target.send)
	if msg.Type != MessageOrderUpdate {
		t.Fatalf("message type = %q, want %q", msg.Type, MessageOrderUpdate)
	}

	assertNoEnvelope(t, other.send)
}

func TestPublishOrderBookUpdateRoutesByTicker(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	target := newClient(hub, nil, "platform-a", nil, 1)
	other := newClient(hub, nil, "platform-b", nil, 1)

	hub.Subscribe(SubscriptionRequest{Client: target, Channel: ChannelOrderBook, Ticker: "ARKA"})
	hub.Subscribe(SubscriptionRequest{Client: other, Channel: ChannelOrderBook, Ticker: "MNVS"})
	hub.PublishOrderBookUpdate(OrderBookUpdatePayload{
		Ticker: "ARKA",
		Bids:   []BookLevel{{Price: 129.5, Quantity: 300}},
		Asks:   []BookLevel{{Price: 130.1, Quantity: 150}},
	})

	msg := readEnvelope(t, target.send)
	if msg.Type != MessageOrderBookUpdate {
		t.Fatalf("message type = %q, want %q", msg.Type, MessageOrderBookUpdate)
	}

	assertNoEnvelope(t, other.send)
}

func TestPublishMarketEventRoutesToAllSubscribers(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	first := newClient(hub, nil, "platform-a", nil, 1)
	second := newClient(hub, nil, "platform-b", nil, 1)

	hub.Subscribe(SubscriptionRequest{Client: first, Channel: ChannelMarketEvents})
	hub.Subscribe(SubscriptionRequest{Client: second, Channel: ChannelMarketEvents})
	hub.PublishMarketEvent(MarketEventPayload{
		EventID:   "event-1",
		EventType: "NEWS",
		Headline:  "ARKA announces earnings",
	})

	for _, ch := range []chan Envelope{first.send, second.send} {
		msg := readEnvelope(t, ch)
		if msg.Type != MessageMarketEvent {
			t.Fatalf("message type = %q, want %q", msg.Type, MessageMarketEvent)
		}
	}
}

func TestHubSendDeliversToRegisteredClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := newClient(hub, nil, "platform-a", nil, 1)
	hub.Register(client)
	hub.Send(client, NewEnvelope(MessageConnected, ConnectedPayload{
		PlatformID: "platform-a",
	}))

	msg := readEnvelope(t, client.send)
	if msg.Type != MessageConnected {
		t.Fatalf("message type = %q, want %q", msg.Type, MessageConnected)
	}
}

func TestHubSendIgnoresUnregisteredClient(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := newClient(hub, nil, "platform-a", nil, 1)
	hub.Send(client, NewEnvelope(MessageConnected, ConnectedPayload{
		PlatformID: "platform-a",
	}))

	assertNoEnvelope(t, client.send)
}

func TestEnqueueRemovesClientWhenSendBufferIsFull(t *testing.T) {
	hub := NewHub()
	client := newClient(hub, nil, "platform-a", nil, 1)
	hub.clients[client] = true
	hub.priceFeedByTicker["ARKA"] = map[*Client]bool{client: true}
	client.subscriptions.PriceFeedTickers["ARKA"] = true

	client.send <- NewEnvelope(MessageConnected, ConnectedPayload{})
	hub.enqueue(client, NewEnvelope(MessagePriceUpdate, PriceUpdatePayload{Ticker: "ARKA"}))

	if hub.clients[client] {
		t.Fatalf("client still registered after full send buffer")
	}
	if _, exists := hub.priceFeedByTicker["ARKA"]; exists {
		t.Fatalf("client subscription was not cleaned up")
	}

	<-client.send
	if _, ok := <-client.send; ok {
		t.Fatalf("client send channel is still open")
	}
}

func readEnvelope(t *testing.T, ch <-chan Envelope) Envelope {
	t.Helper()

	select {
	case msg := <-ch:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for websocket envelope")
	}

	return Envelope{}
}

func assertNoEnvelope(t *testing.T, ch <-chan Envelope) {
	t.Helper()

	select {
	case msg := <-ch:
		t.Fatalf("unexpected websocket envelope: %#v", msg)
	case <-time.After(50 * time.Millisecond):
	}
}
