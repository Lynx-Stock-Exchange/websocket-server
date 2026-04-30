# WebSocket Server

Real-time WebSocket gateway for the Lynx Stock Exchange prototype.

Broker platforms connect over WebSocket. Internal exchange services communicate with this server through Kafka topics. The websocket hub fans Kafka updates out to subscribed broker clients.

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go |
| WebSocket | `github.com/gorilla/websocket` |
| Kafka client | `github.com/segmentio/kafka-go` |
| Local broker | Redpanda |

## Architecture

```text
microservice -> Kafka update topic -> websocket hub -> broker client
broker client -> websocket PLACE_ORDER -> Kafka order command topic -> order service
order service -> Kafka result topic -> websocket hub -> original broker client
```

## WebSocket Connection

Connect with fake prototype credentials:

```text
ws://localhost:8080/ws?api_key=test-api-key&api_secret=test-api-secret
```

On success, the server sends:

```json
{
  "type": "CONNECTED",
  "payload": {
    "platform_id": "platform-xyz",
    "server_market_time": "2026-04-30T10:00:00Z"
  }
}
```

## WebSocket Messages

Every message uses:

```json
{
  "type": "MESSAGE_TYPE",
  "payload": {}
}
```

Subscribe to price feed:

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "PRICE_FEED",
    "tickers": ["AAPL"]
  }
}
```

Subscribe to order book:

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "ORDER_BOOK",
    "ticker": "AAPL"
  }
}
```

Subscribe to order updates:

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "ORDER_UPDATES"
  }
}
```

Subscribe to market events:

```json
{
  "type": "SUBSCRIBE",
  "payload": {
    "channel": "MARKET_EVENTS"
  }
}
```

Place order:

```json
{
  "type": "PLACE_ORDER",
  "payload": {
    "platform_user_id": "user-abc-123",
    "instrument_type": "STOCK",
    "instrument_id": "AAPL",
    "order_type": "LIMIT",
    "side": "BUY",
    "quantity": 50,
    "limit_price": 128.00,
    "expires_at": "2026-04-30T14:00:00Z"
  }
}
```

## Kafka Topics

Default local broker:

```text
localhost:19092
```

Default topics:

```text
lynx.price-updates
lynx.order-updates
lynx.order-book-updates
lynx.market-events
lynx.order-commands
lynx.order-command-results
```

Environment variables:

```text
KAFKA_BROKERS=localhost:19092
KAFKA_GROUP_ID=websocket-server
```

Order updates include `platform_id` for routing:

```json
{
  "platform_id": "platform-xyz",
  "payload": {
    "order_id": "ord-123",
    "status": "FILLED",
    "filled_quantity": 10,
    "average_fill_price": 150.75,
    "exchange_fee": 0.15,
    "market_time": "2026-04-30T10:00:00Z"
  }
}
```

Order commands include `client_request_id` so the websocket server can route the later result back to the original connection.

## Demo Runbook

Start Kafka locally:

```bash
cd ~/stock-exchange-ws
docker compose up -d redpanda
```

Start the websocket server:

```bash
cd ~/stock-exchange-ws
go run cmd/exchange/main.go
```

Run one broker client subscribed to every channel:

```bash
cd ~/stock-exchange-ws
go run cmd/client/all_channels/all_channels_client.go
```

Run fake internal services. This publishes fake market data and acts as a fake order service over Kafka:

```bash
cd ~/stock-exchange-ws
go run cmd/tests/test.go
```

Demo order placement:

```bash
cd ~/stock-exchange-ws
go run cmd/client/place_order/place_order_client.go
```

Run tests:

```bash
cd ~/stock-exchange-ws
go test ./...
```
