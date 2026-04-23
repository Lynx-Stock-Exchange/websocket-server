package ws

import "testing"

func TestNormalizeTickers(t *testing.T) {
	got := normalizeTickers([]string{" arka ", "ARKA", "mnvs", "", "   ", "MnVs"})
	want := []string{"ARKA", "MNVS"}

	if len(got) != len(want) {
		t.Fatalf("normalizeTickers length = %d, want %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalizeTickers[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeTickersEmptyOnlyReturnsNil(t *testing.T) {
	if got := normalizeTickers([]string{"", "   "}); got != nil {
		t.Fatalf("normalizeTickers returned %v, want nil", got)
	}
}

func TestHubApplySubscriptionStoresPriceFeedTicker(t *testing.T) {
	hub := NewHub()
	client := newClient(hub, nil, "platform-1", 1)

	hub.applySubscription(SubscriptionRequest{
		Client:  client,
		Channel: ChannelPriceFeed,
		Tickers: []string{"ARKA"},
	})

	if !hub.priceFeedByTicker["ARKA"][client] {
		t.Fatalf("client was not stored under ARKA")
	}
	if !client.subscriptions.PriceFeedTickers["ARKA"] {
		t.Fatalf("client local subscription state missing ARKA")
	}
}

func TestRemoveClientSubscriptionsCleansIndexes(t *testing.T) {
	hub := NewHub()
	client := newClient(hub, nil, "platform-1", 1)

	hub.clients[client] = true
	hub.applySubscription(SubscriptionRequest{
		Client:  client,
		Channel: ChannelPriceFeed,
		Tickers: []string{"ARKA"},
	})
	hub.applySubscription(SubscriptionRequest{
		Client:  client,
		Channel: ChannelOrderBook,
		Ticker:  "ARKA",
	})
	hub.applySubscription(SubscriptionRequest{
		Client:  client,
		Channel: ChannelOrderUpdates,
	})
	hub.applySubscription(SubscriptionRequest{
		Client:  client,
		Channel: ChannelMarketEvents,
	})

	hub.removeClientSubscriptions(client)

	if _, ok := hub.priceFeedByTicker["ARKA"]; ok {
		t.Fatalf("priceFeedByTicker still contains ARKA")
	}
	if _, ok := hub.orderBookByTicker["ARKA"]; ok {
		t.Fatalf("orderBookByTicker still contains ARKA")
	}
	if _, ok := hub.orderUpdatesByPlatform["platform-1"]; ok {
		t.Fatalf("orderUpdatesByPlatform still contains platform-1")
	}
	if hub.marketEventsSubscribers[client] {
		t.Fatalf("marketEventsSubscribers still contains client")
	}
}

func TestPublishPriceUpdateFanout(t *testing.T) {
	hub := NewHub()
	clientARKA1 := newClient(hub, nil, "platform-1", 1)
	clientARKA2 := newClient(hub, nil, "platform-2", 1)
	clientMNVS := newClient(hub, nil, "platform-3", 1)

	hub.applySubscription(SubscriptionRequest{
		Client:  clientARKA1,
		Channel: ChannelPriceFeed,
		Tickers: []string{"ARKA"},
	})
	hub.applySubscription(SubscriptionRequest{
		Client:  clientARKA2,
		Channel: ChannelPriceFeed,
		Tickers: []string{"ARKA"},
	})
	hub.applySubscription(SubscriptionRequest{
		Client:  clientMNVS,
		Channel: ChannelPriceFeed,
		Tickers: []string{"MNVS"},
	})

	payload := PriceUpdatePayload{Ticker: "ARKA"}
	for client := range hub.priceFeedByTicker[payload.Ticker] {
		client.send <- NewEnvelope(MessagePriceUpdate, payload)
	}

	assertPriceUpdateQueued(t, clientARKA1.send, "clientARKA1")
	assertPriceUpdateQueued(t, clientARKA2.send, "clientARKA2")

	select {
	case msg := <-clientMNVS.send:
		t.Fatalf("clientMNVS unexpectedly received message %#v", msg)
	default:
	}
}

func assertPriceUpdateQueued(t *testing.T, send <-chan Envelope, name string) {
	t.Helper()

	select {
	case msg := <-send:
		if msg.Type != MessagePriceUpdate {
			t.Fatalf("%s received message type %q, want %q", name, msg.Type, MessagePriceUpdate)
		}
	default:
		t.Fatalf("%s did not receive a price update", name)
	}
}
