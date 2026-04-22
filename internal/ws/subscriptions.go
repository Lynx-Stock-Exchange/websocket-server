package ws

// TODO: Define subscription channel names from the spec:
// - PRICE_FEED with tickers: ["ARKA", "MNVS"]
// - ORDER_UPDATES for platform/order status changes.
// - MARKET_EVENTS for event pushes.
// - ORDER_BOOK with ticker: "ARKA"
//
// TODO: Parse SUBSCRIBE payloads, normalize tickers, and ask the hub to update in-memory and persisted subscription state.
// Multiple subscriptions can be active simultaneously on the same websocket connection.
