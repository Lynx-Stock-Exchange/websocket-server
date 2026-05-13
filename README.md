# WebSocket Server

A real-time WebSocket server for the Lynx Stock Exchange platform. It streams live price updates, order status changes, order book depth, and market events to connected trading clients. Internal services publish data to Kafka topics, and the server's consumers broadcast it instantly to all subscribed WebSocket connections.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go |
| WebSocket | `github.com/gorilla/websocket` |
| HTTP Router | Go standard library (`net/http`) |
| Message Broker | Kafka (Redpanda in Docker) |
| Container | Docker |
| Orchestration | Docker Compose |

---

## Running with Docker Compose

```bash
docker compose up -d
```

The server starts on **port 8080**.

To stop:

```bash
docker compose down
```


---

## Integration Guide for other Teams

### WebSocket Connection

Connect to the WebSocket endpoint with your platform credentials as query parameters:

```
ws://localhost:8080/ws?api_key=<YOUR_API_KEY>&api_secret=<YOUR_API_SECRET>
```

On successful connection the server sends a `CONNECTED` message:


```json
{
  "type": "CONNECTED",
  "payload": {
    "platform_id": "platform-xyz",
    "server_market_time": "local_time"
  }
}
```

---

### Message Envelope

Every message in both directions uses this envelope:

```json
{
  "type": "<MESSAGE_TYPE>",
  "payload": { }
}
```

---
### Client → Server Message Types ( what you send to the server )

### Subscribing to Channels

Send a `SUBSCRIBE` message after connecting.

This is the `type` and `payload `examples each of your services should send to the websoket

**Price feed (one or more tickers):**

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "PRICE_FEED",
    "tickers": ["AAPL", "MSFT", "BTC", etc..]
  }
}
```

**Order book (single ticker):**

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "ORDER_BOOK",
    "ticker": "AAPL"
  }
}
```

**Order updates (your platform's orders):**

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "ORDER_UPDATES"
  }
}
```

**Market-wide events:**

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "MARKET_EVENTS"
  }
}
```

---

### Server → Client Message Types ( what you receive from the server )

**`PRICE_UPDATE`** — emitted on every price change for a subscribed ticker:

```json
{
  "type": "PRICE_UPDATE",
  "payload": {
    "ticker": "AAPL",
    "price": 150.75,
    "change": 0.50,
    "change_pct": 0.33,
    "volume": 1000000,
    "market_time": "2026-04-27T15:30:00Z"
  }
}
```

**`ORDER_UPDATE`** — emitted when an order's status changes:

```json
{
  "type": "ORDER_UPDATE",
  "payload": {
    "order_id": "ord-123",
    "status": "FILLED",
    "filled_quantity": 10,
    "average_fill_price": 150.75,
    "exchange_fee": 0.25,
    "market_time": "2026-04-27T15:30:00Z"
  }
}
```

**`ORDER_BOOK_UPDATE`** — emitted when bids/asks change for a subscribed ticker:

```json
{
  "type": "ORDER_BOOK_UPDATE",
  "payload": {
    "ticker": "AAPL",
    "bids": [
      { "price": 150.50, "quantity": 100 },
      { "price": 150.25, "quantity": 250 }
    ],
    "asks": [
      { "price": 150.75, "quantity": 80 },
      { "price": 151.00, "quantity": 150 }
    ]
  }
}
```

**`MARKET_EVENT`** — market-wide event (halt, volatility spike, earnings, etc.):

```json
{
  "type": "MARKET_EVENT",
  "payload": {
    "event_id": "evt-456",
    "event_type": "HALT",
    "headline": "Trading halted for AAPL",
    "scope": "TICKER",
    "target": "AAPL",
    "magnitude": 0.0,
    "duration_ticks": 5,
    "market_time": "2026-04-27T15:30:00Z"
  }
}
```
 ## Placing order via Websocket
**`ORDER_ACK`** — your order was accepted:

```json
{
  "type": "ORDER_ACK",
  "payload": {
    "order_id": "ord-123",
    "status": "PENDING"
  }
}
```

**`ORDER_REJECTED`** — your order was rejected:

```json
{
  "type": "ORDER_REJECTED",
  "payload": {
    "code": "INVALID_ORDER",
    "message": "limit_price is required for LIMIT orders"
  }
}
```

Rejection codes: `INVALID_ORDER`, `ORDER_SERVICE_UNAVAILABLE`, `MARKET_CLOSED`, `ORDER_REJECTED`.

---

### Placing an Order

Send a `PLACE_ORDER` message over the WebSocket:

```json
{
  "type": "PLACE_ORDER",
  "payload": {
    "platform_user_id": "user-789",
    "instrument_type": "STOCK",
    "instrument_id": "AAPL",
    "order_type": "LIMIT",
    "side": "BUY",
    "quantity": 10,
    "limit_price": 150.00,
    "expires_at": "2026-04-27T16:00:00Z"
  }
}
```



---

## Integration Guide for Internal Services (Kafka)

Internal services — price feed, matching engine, risk engine — publish data directly to Kafka topics. The WebSocket server's consumers pick up each message and broadcast it to the relevant subscribed clients. Your service never calls the WebSocket server directly.

### Kafka broker address

Set the `KAFKA_BROKERS` environment variable in your service to point at the shared broker:

| Environment | Address |
|---|---|
| Local (no Docker) | `localhost:9092` |
| Docker Compose (internal) | `redpanda:9092` |

---

### Topic: `stock.prices`

Published by: price feed engine

```json
{
  "ticker": "AAPL",
  "price": 150.75,
  "change": 0.50,
  "change_pct": 0.33,
  "volume": 1000000,
  "market_time": "2026-04-27T15:30:00Z"
}
```

Delivered to: all clients subscribed to `PRICE_FEED` for that ticker.

---

### Topic: `orders.updates`

Published by: matching engine

```json
{
  "platform_id": "platform-xyz",
  "order_id": "ord-123",
  "status": "FILLED",
  "filled_quantity": 10,
  "average_fill_price": 150.75,
  "exchange_fee": 0.15,
  "market_time": "2026-04-27T15:30:00Z"
}
```

`platform_id` must be present — the server uses it to route the update only to clients of that platform subscribed to `ORDER_UPDATES`.

---

### Topic: `orders.volumes`

Published by: matching engine / book aggregator

```json
{
  "ticker": "AAPL",
  "bids": [
    { "price": 150.50, "quantity": 300 },
    { "price": 150.25, "quantity": 450 }
  ],
  "asks": [
    { "price": 150.75, "quantity": 250 },
    { "price": 151.00, "quantity": 500 }
  ]
}
```

Delivered to: all clients subscribed to `ORDER_BOOK` for that ticker.

---

### Topic: `market.events`

Published by: news feed / risk engine

```json
{
  "event_id": "evt-456",
  "event_type": "HALT",
  "headline": "Trading halted for AAPL",
  "scope": "TICKER",
  "target": "AAPL",
  "magnitude": 0.0,
  "duration_ticks": 5,
  "market_time": "2026-04-27T15:30:00Z"
}
```

Delivered to: all clients subscribed to `MARKET_EVENTS` (global broadcast).

---


**For a complete working example of all four topics, see [`cmd/tests/test.go`](cmd/tests/test.go).**

---

## Run and debug existent tests to help you understand how to connect your service to the Websocket

Make sure the docker container is running and you are in project directory, then start the logs

```bash
docker compose logs -f exchange-server
```

To watch the messages arrive, run one of the test clients in another terminal:

```bash
go run cmd/client/your_service
```

then in a separate terminal start the test (this test broadcast test data to all internal services but you will only receave the message for only the service that your client is `SUBSCRIBED` to):


```bash
go run cmd/tests/test.go
```

