package ws

// TODO: Decode and validate PLACE_ORDER payloads from readPump.
// Required fields from the spec include platform_user_id, instrument_type, instrument_id, order_type, side, and quantity.
// TODO: Call the order placement service and enqueue ORDER_ACK or ORDER_REJECTED on the same websocket connection.
// This path must not block readPump for slow downstream order processing; use service timeouts or async handoff as needed.
