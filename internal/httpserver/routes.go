package httpserver

// TODO: Build the HTTP router and mount GET /ws.
// The /ws route should call the websocket handler that performs gorilla upgrade, auth, session creation, and pump startup.
// Keep REST endpoints outside this package unless they are required to bootstrap the websocket service.
