# Stock Exchange WebSocket Scaffold

This is an empty Go scaffold for the exchange WebSocket service described in `stock_exchange_spec.docx` and the WebSocket flow diagram.

For now the files contain comments only, plus package declarations, so the next step is implementation.

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

## Next implementation pass

1. Wire the HTTP server and `/ws` route.
2. Implement gorilla upgrade and credential validation.
3. Implement `Hub.Run`, client registration, unregistration, and fanout.
4. Implement `readPump` for `SUBSCRIBE` and `PLACE_ORDER`.
5. Implement `writePump` as the only goroutine allowed to write to the socket.
6. Add service adapters for Admin DB, Push DB, price simulation, order book, and market events.
