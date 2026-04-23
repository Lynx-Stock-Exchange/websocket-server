# Stock Exchange WebSocket Scaffold

This is a Go websocket server scaffold for the exchange WebSocket service described in `stock_exchange_spec.docx` and the WebSocket flow diagram.

The project now includes the core websocket flow:
- gorilla websocket upgrade and connection handling
- client and hub registration
- read pump and write pump goroutines
- in-memory subscription routing
- `PLACE_ORDER` decoding, validation, and service boundary wiring
- tests for subscriptions and order handling

## Spec anchors

- Connection: `wss://<exchange-host>/ws?api_key=<key>&api_secret=<secret>`
- First server message: `CONNECTED(platform_id, server_market_time)`
- Envelope: `{ "type": "<MESSAGE_TYPE>", "payload": { ... } }`
- Client messages: `SUBSCRIBE`, `PLACE_ORDER`
- Server pushes: `PRICE_UPDATE`, `ORDER_UPDATE`, `ORDER_BOOK_UPDATE`, `MARKET_EVENT`
- Async order responses: `ORDER_ACK`, `ORDER_REJECTED`

## Layout

- `cmd/exchange`: executable entry point.
- `internal/httpserver`: HTTP route setup, including `/ws`.
- `internal/ws`: gorilla websocket handler, hub, clients, read pump, write pump, subscriptions, message contracts.
- `internal/services`: interfaces to admin, price simulation, order book, market events, and order placement.
- `internal/store`: persistence boundaries for session/channel state.
- `pkg/contracts`: exported DTO location if broker-facing contracts need to be shared later.

## Current status

Implemented:
1. Websocket message contracts.
2. Hub and client skeleton.
3. Gorilla websocket handler for `/ws`.
4. `readPump` and `writePump`.
5. In-memory subscription routing.
6. `PLACE_ORDER` payload validation and order-service boundary wiring.

Still to do:
1. Real auth service implementation.
2. Real order service implementation behind the `OrderService` interface.
3. Internal HTTP/gRPC ingress for price, order, order book, and market-event pushes.
4. Full HTTP server wiring and runtime configuration.
