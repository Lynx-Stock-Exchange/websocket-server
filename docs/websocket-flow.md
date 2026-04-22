# WebSocket Flow Notes

This file is a working checklist for implementing the websocket exactly from the spec and diagram.

## Handshake

1. Broker connects to `GET /ws?api_key=<key>&api_secret=<secret>`.
2. Exchange API service validates credentials through the admin service.
3. Admin service loads platform registration from the Admin DB.
4. Exchange API service receives the authorized platform.
5. Exchange API service opens websocket session and registers the connection with the websocket/push service.
6. Websocket/push service persists session and channel state.
7. Server sends `CONNECTED` with `platform_id` and `server_market_time`.

## Subscriptions

8. Client sends `SUBSCRIBE` for `PRICE_FEED`, `ORDER_UPDATES`, `MARKET_EVENTS`, or `ORDER_BOOK`.
9. Exchange registers subscriptions with the websocket/push service.
10. Websocket/push service stores channel subscriptions.
11. Server confirms subscription is active.

## Pushes

12-13. Price simulation service emits `PRICE_UPDATE`; hub fans out to matching `PRICE_FEED` subscribers.
14-15. Order book service emits `ORDER_UPDATE` or `ORDER_BOOK_UPDATE`; hub fans out to matching subscribers.
16-17. Market events service emits `MARKET_EVENT`; hub fans out to `MARKET_EVENTS` subscribers.

## Orders Over WebSocket

18. Client sends `PLACE_ORDER` over the same websocket.
19. Exchange validates and creates the order through the order book/order service.
20. Service returns `ORDER_ACK` or `ORDER_REJECTED`.
21. Server sends the async acknowledgement on the same websocket connection.

## Goroutine Rule

- One hub goroutine owns global client/subscription maps.
- One read pump goroutine per client reads from gorilla websocket.
- One write pump goroutine per client writes to gorilla websocket.
- Other services publish events into the hub, never directly into websocket connections.
