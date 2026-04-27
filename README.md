# WebSocket Server

A real-time WebSocket server for the Lynx Stock Exchange platform. It streams live price updates, order status changes, order book depth, and market events to connected trading clients. Internal services push data into the server via a lightweight HTTP endpoint, and the hub broadcasts it instantly to all subscribed WebSocket connections.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go |
| WebSocket | `github.com/gorilla/websocket` |
| HTTP Router | Go standard library (`net/http`) |
| Container | Docker  |
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

## Pushing Price Updates (Internal Services)

Internal services — such as a price feed engine — push price updates to the server via HTTP. The server then broadcasts them to all WebSocket clients subscribed to that ticker.

```
POST http://localhost:8080/internal/push/price-update
Content-Type: application/json
```

Request body:

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

Responses:

| Status | Meaning |
|---|---|
| `202 Accepted` | Update received and queued for broadcast |
| `400 Bad Request` | Missing ticker or malformed JSON |
| `500 Internal Server Error` | Hub not initialised |

**For a working implementation of this, see [`cmd/tests/test.go`](cmd/tests/..).** [`cmd/client/..`](cmd/tests/test.go) to see a working implementation 
of the services

---

## Run and debug existent tests to help you understand how to connect your service to the Websocket

Start the server first and start the logs

```bash
docker compose logs -f 
```

To watch the messages arrive, run one of the test clients in another terminal:

```bash
go run cmd/client/your_service
```

then in a separate terminal start a test:


```bash
go run cmd/tests/your_service
```


